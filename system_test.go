package zeroclaw

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

const systemSnapshot = `{"pid":7,"updated_at":"t","uptime_seconds":60,"components":{
	"gateway":{"status":"ok","updated_at":"t","last_ok":"t","last_error":null,"restart_count":0}}}`

func TestSystemHealthAndMetricsArePublic(t *testing.T) {
	g, client := gateway(t)
	g.public("GET", "/health", reply(`{"status":"ok","paired":true,"require_pairing":true,"runtime":`+systemSnapshot+`}`))
	g.public("GET", "/metrics", replyStatus(200, "# HELP x\nx 1\n"))
	g.on("GET", "/api/health", reply(`{"health":`+systemSnapshot+`}`))

	ctx, system := context.Background(), client.System()
	if !must[*Health](t)(system.Health(ctx)).RequirePairing {
		t.Error("health: require_pairing")
	}
	if got := must[string](t)(system.Metrics(ctx)); !strings.Contains(got, "x 1") {
		t.Errorf("metrics: %q", got)
	}
	if got := must[*HealthSnapshot](t)(system.RuntimeHealth(ctx)); got.Components["gateway"].Status != "ok" {
		t.Errorf("runtime health: %+v", got)
	}
}

func TestSystemStatusAsksAboutOneAgent(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/status", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "agent", "helper")
		reply(`{"version":"0.8.5","model_provider":"openai.default","model":"m",
			"temperature":0.7,"uptime_seconds":1,"daemon_started_at":"t",
			"gateway_port":42617,"locale":"en","memory_backend":"sqlite","paired":true,
			"channels":{"whatsapp.main":true},"nodes":{"connected":[],"mdns_peers":[]},
			"health":`+systemSnapshot+`,"agent_alias":"helper",
			"process":{"rss_bytes":1,"system_ram_total_bytes":2,"cpu_percent":null,"num_cpus":4},
			"check_updates":true,"allow_self_upgrade":false,"restart_mode":"manual",
			"restart_hint":"","live_channel_config":["sender_routes"]}`)(w, r)
	})

	status := must[*Status](t)(client.System().Status(context.Background(), "helper"))
	if status.AgentAlias == nil || *status.AgentAlias != "helper" {
		t.Errorf("agent alias: %v", status.AgentAlias)
	}
	if len(status.LiveChannelConfig) != 1 || status.LiveChannelConfig[0] != "sender_routes" {
		t.Errorf("live channel config: %v", status.LiveChannelConfig)
	}
}

func TestSystemDiagnosesAndListsToolsAndIntegrations(t *testing.T) {
	g, client := gateway(t)
	g.on("POST", "/api/doctor", reply(`{"results":[{"severity":"warn","category":"memory","message":"m"}],
		"summary":{"ok":0,"warnings":1,"errors":0}}`))
	g.on("GET", "/api/tools", reply(`{"tools":[{"name":"shell","description":"d","parameters":{"type":"object"}}]}`))
	g.on("POST", "/api/tools/param-options", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"domain":"channel_refs","agent":null,"args":null}`)
		reply(`{"options":[{"value":"whatsapp.main"}]}`)(w, r)
	})
	g.on("GET", "/api/integrations", reply(`{"integrations":[{"name":"WhatsApp","description":"d","category":"Chat",
		"category_label":"Chat Providers","status":"Active"}]}`))
	g.on("GET", "/api/integrations/settings", reply(`{"settings":{"WhatsApp":{"enabled":true,"category":"Chat","status":"Active"}}}`))
	g.on("GET", "/api/cli-tools", reply(`{"cli_tools":[{"name":"git","path":"/usr/bin/git","version":"2","category":"VersionControl"}]}`))
	g.on("GET", "/api/tuis", reply(`{"tuis":[]}`))

	ctx, system := context.Background(), client.System()
	if got := must[*DoctorReport](t)(system.Doctor(ctx)); got.Summary.Warnings != 1 {
		t.Errorf("doctor: %+v", got)
	}
	if got := must[[]ToolSpec](t)(system.Tools(ctx, "")); got[0].Name != "shell" {
		t.Errorf("tools: %+v", got)
	}
	if got := must[[]ToolOption](t)(system.ToolParamOptions(ctx, "channel_refs", "", nil)); got[0].Value != "whatsapp.main" {
		t.Errorf("param options: %+v", got)
	}
	if got := must[[]Integration](t)(system.Integrations(ctx)); got[0].Status != "Active" {
		t.Errorf("integrations: %+v", got)
	}
	if got := must[map[string]IntegrationSetting](t)(system.IntegrationSettings(ctx)); !got["WhatsApp"].Enabled {
		t.Errorf("integration settings: %+v", got)
	}
	if got := must[[]CLITool](t)(system.CLITools(ctx)); got[0].Name != "git" {
		t.Errorf("cli tools: %+v", got)
	}
	if got := must[[]Tui](t)(system.Tuis(ctx)); len(got) != 0 {
		t.Errorf("tuis: %+v", got)
	}
}

func TestSystemLogFiltersBecomeQueryParameters(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/logs", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "until_line_offset", "40")
		wantQuery(t, r, "limit", "10")
		wantQuery(t, r, "hide_internal", "true")
		wantQuery(t, r, "agent_alias", "helper")
		reply(`{"events":[{"@timestamp":"t"}],"next_cursor":null,
			"next_cursor_line_offset":30,"at_end":false,
			"daemon_started_at":"t","attribution_keys":["agent_alias"]}`)(w, r)
	})

	offset := uint64(40)
	page := must[*LogPage](t)(client.System().Logs(context.Background(), LogQuery{
		UntilLineOffset: &offset,
		Limit:           10,
		HideInternal:    true,
		Attributes:      map[string]string{"agent_alias": "helper"},
	}))
	if page.NextCursorLineOffset == nil || *page.NextCursorLineOffset != 30 {
		t.Errorf("next cursor: %v", page.NextCursorLineOffset)
	}
}

func TestSystemChecksAndRunsUpgrades(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/version/check", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "force", "true")
		reply(`{"current_version":"0.8.5","latest_version":"0.8.6","is_newer":true}`)(w, r)
	})
	g.on("POST", "/api/version/upgrade", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"version":null,"auto_restart":true}`)
		replyStatus(http.StatusAccepted, `{"handoff_id":"h1"}`)(w, r)
	})
	g.on("GET", "/api/version/upgrade/status", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "handoff_id", "h1")
		reply(`{"state":"running","phase":2}`)(w, r)
	})

	ctx, system := context.Background(), client.System()
	if !must[*VersionCheck](t)(system.VersionCheck(ctx, true, "")).IsNewer {
		t.Error("version check: is_newer")
	}
	if got := must[string](t)(system.Upgrade(ctx, "", true)); got != "h1" {
		t.Errorf("upgrade: %q", got)
	}
	if got := must[*UpgradeStatus](t)(system.UpgradeStatus(ctx, "h1")); got.Phase == nil || *got.Phase != 2 {
		t.Errorf("upgrade status: %+v", got)
	}
}

func TestSystemAdministersTheDaemon(t *testing.T) {
	g, client := gateway(t)
	g.on("POST", "/admin/reload", reply(`{"success":true,"message":"Daemon reload initiated"}`))
	g.on("POST", "/admin/shutdown", reply(`{"success":true,"message":"Gateway shutdown initiated"}`))
	g.public("GET", "/api/openapi.json", reply(`{"openapi":"3.1.0"}`))

	ctx, system := context.Background(), client.System()
	if !must[*AdminResponse](t)(system.Reload(ctx)).Success {
		t.Error("reload")
	}
	if !must[*AdminResponse](t)(system.Shutdown(ctx)).Success {
		t.Error("shutdown")
	}
	doc := must[[]byte](t)(system.OpenAPI(ctx))
	if !sameJSON(t, doc, []byte(`{"openapi":"3.1.0"}`)) {
		t.Errorf("openapi: %s", doc)
	}
}
