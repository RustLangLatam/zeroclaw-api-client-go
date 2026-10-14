package zeroclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

func TestPatchOpVectors(t *testing.T) {
	vectors := []struct {
		op   PatchOp
		want string
	}{
		{ReplaceOp("a.b", 1), `{"op":"replace","path":"a.b","value":1}`},
		{AddOp("agents.x.model", "m"), `{"op":"add","path":"agents.x.model","value":"m"}`},
		{RemoveOp("/a/b"), `{"op":"remove","path":"/a/b"}`},
		{TestOp("a", true), `{"op":"test","path":"a","value":true}`},
		{CommentOp("a", "why"), `{"op":"comment","path":"a","comment":"why"}`},
		{ReplaceOp("a", []string{"x", "y"}), `{"op":"replace","path":"a","value":["x","y"]}`},
	}
	for _, v := range vectors {
		got, err := json.Marshal(v.op)
		if err != nil {
			t.Fatal(err)
		}
		if !sameJSON(t, got, []byte(v.want)) {
			t.Errorf("%+v encodes as %s, want %s", v.op, got, v.want)
		}
	}
}

func TestAliasSourceVectors(t *testing.T) {
	vectors := map[AliasSource]string{
		AliasModelProviders: "model_providers", AliasTTSProviders: "tts_providers",
		AliasTranscriptionProviders: "transcription_providers", AliasChannels: "channels",
		AliasRiskProfiles: "risk_profiles", AliasRuntimeProfiles: "runtime_profiles",
		AliasAgents: "agents", AliasSkillBundles: "skill_bundles",
		AliasKnowledgeBundles: "knowledge_bundles", AliasMCPBundles: "mcp_bundles",
	}
	for source, name := range vectors {
		if string(source) != name {
			t.Errorf("%s != %s", source, name)
		}
	}
}

func TestPropsDecodeAsValuesOrSecrets(t *testing.T) {
	var value PropValue
	if err := json.Unmarshal([]byte(`{"path":"gateway.port","value":"42617"}`), &value); err != nil {
		t.Fatal(err)
	}
	if value.IsSecret() || string(value.Value) != `"42617"` {
		t.Errorf("value: %+v", value)
	}
	var secret PropValue
	if err := json.Unmarshal([]byte(`{"path":"providers.models.x.default.api_key","populated":true}`), &secret); err != nil {
		t.Fatal(err)
	}
	if !secret.IsSecret() || !*secret.Populated {
		t.Errorf("secret: %+v", secret)
	}
}

func TestConfigReadsWritesAndResetsOneField(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/config/prop", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "path", "gateway.port")
		reply(`{"path":"gateway.port","value":"42617"}`)(w, r)
	})
	g.on("PUT", "/api/config/prop", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"path":"agents.x.model","value":"m","comment":"set"}`)
		reply(`{"path":"agents.x.model","value":"m"}`)(w, r)
	})
	g.on("DELETE", "/api/config/prop", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "path", "providers.models.x.default.api_key")
		reply(`{"path":"providers.models.x.default.api_key","populated":false}`)(w, r)
	})

	ctx, config := context.Background(), client.Config()
	if got := must[*PropValue](t)(config.Get(ctx, "gateway.port")); string(got.Value) != `"42617"` {
		t.Errorf("get: %s", got.Value)
	}
	must[*PropValue](t)(config.Set(ctx, "agents.x.model", "m", "set"))
	if got := must[*PropValue](t)(config.Reset(ctx, "providers.models.x.default.api_key")); !got.IsSecret() || *got.Populated {
		t.Errorf("reset: %+v", got)
	}
}

func TestConfigPatchesAtomicallyAndCanOverrideDrift(t *testing.T) {
	g, client := gateway(t)
	g.on("PATCH", "/api/config", func(w http.ResponseWriter, r *http.Request) {
		wantHeader(t, r, "X-ZeroClaw-Override-Drift", "true")
		wantBody(t, r, `[{"op":"replace","path":"gateway.port","value":1},{"op":"remove","path":"agents.x.model"}]`)
		reply(`{"saved":true,"results":[
			{"op":"replace","path":"gateway.port","value":"1"},
			{"op":"remove","path":"agents.x.model","value":null}]}`)(w, r)
	})
	patched := must[*PatchResponse](t)(client.Config().Patch(context.Background(),
		[]PatchOp{ReplaceOp("gateway.port", 1), RemoveOp("agents.x.model")}, true))
	if !patched.Saved || len(patched.Results) != 2 {
		t.Errorf("%+v", patched)
	}
}

func TestConfigErrorsKeepTheGatewaysMessage(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/config/prop", replyStatus(404, `{"code":"path_not_found","message":"no field `+"`x`"+`","path":"x"}`))
	_, err := client.Config().Get(context.Background(), "x")
	if !IsNotFound(err) || err.Error() != "gateway answered 404: no field `x`" {
		t.Errorf("%v", err)
	}
}

func TestConfigListsDriftAndStatus(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/config/list", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "prefix", "gateway")
		reply(`{"entries":[{"path":"gateway.port","category":"gateway","kind":"integer",
			"type_hint":"u16","value":"42617","populated":true,"is_secret":false}]}`)(w, r)
	})
	g.on("GET", "/api/config/drift", reply(`{"drifted":[{"path":"a","drifted":true}]}`))
	g.on("GET", "/api/config/reload-status", reply(`{"pending_reload":true}`))
	g.on("GET", "/api/config/status", reply(`{"needs_quickstart":false,"reason":"has_dispatchable_agent",
		"has_partial_state":false,"missing":[]}`))
	g.on("GET", "/api/config", reply(`{"gateway":{"port":42617}}`))

	ctx, config := context.Background(), client.Config()
	if got := must[*ConfigList](t)(config.List(ctx, "gateway")); got.Entries[0].Kind != "integer" {
		t.Errorf("list: %+v", got)
	}
	if got := must[[]DriftEntry](t)(config.Drift(ctx)); got[0].Path != "a" {
		t.Errorf("drift: %+v", got)
	}
	if !must[bool](t)(config.PendingReload(ctx)) {
		t.Error("pending reload")
	}
	if must[*ConfigStatus](t)(config.Status(ctx)).NeedsQuickstart {
		t.Error("status")
	}
	var snapshot struct {
		Gateway struct{ Port int }
	}
	json.Unmarshal(must[json.RawMessage](t)(config.Snapshot(ctx)), &snapshot)
	if snapshot.Gateway.Port != 42617 {
		t.Errorf("snapshot: %+v", snapshot)
	}
}

func TestConfigManagesMapAliases(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/config/map-keys", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "path", "agents")
		reply(`{"path":"agents","keys":["a","b"]}`)(w, r)
	})
	for _, verb := range []string{"POST", "DELETE"} {
		g.on(verb, "/api/config/map-key", func(w http.ResponseWriter, r *http.Request) {
			wantQuery(t, r, "path", "agents")
			wantQuery(t, r, "key", "a")
			created := "false"
			if r.Method == "POST" {
				created = "true"
			}
			reply(`{"path":"agents","key":"a","created":`+created+`}`)(w, r)
		})
	}
	g.on("GET", "/api/config/delete-plan", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "key", "a")
		reply(`{"path":"agents","key":"a","allowed":true,"blockers":[],
			"scrubs":[{"path":"heartbeat.agent","raw_value":"a"}],
			"live_acp_sessions":0,"cascades_owned_state":true}`)(w, r)
	})
	g.on("POST", "/api/config/rename-map-key", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"path":"agents","from":"a","to":"c"}`)
		reply(`{"path":"agents","from":"a","to":"c","renamed":true}`)(w, r)
	})
	g.on("GET", "/api/config/resolve-alias-source", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "source", "model_providers")
		reply(`{"source":"model_providers","values":["openai.default"]}`)(w, r)
	})

	ctx, config := context.Background(), client.Config()
	if got := must[[]string](t)(config.MapKeys(ctx, "agents")); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("map keys: %v", got)
	}
	if !must[*MapKeyResponse](t)(config.CreateMapKey(ctx, "agents", "a")).Created {
		t.Error("create")
	}
	if must[*MapKeyResponse](t)(config.DeleteMapKey(ctx, "agents", "a")).Created {
		t.Error("delete")
	}
	if got := must[*DeletePlan](t)(config.DeletePlan(ctx, "agents", "a")); len(got.Scrubs) != 1 {
		t.Errorf("delete plan: %+v", got)
	}
	if !must[*RenameResponse](t)(config.RenameMapKey(ctx, "agents", "a", "c")).Renamed {
		t.Error("rename")
	}
	if got := must[[]string](t)(config.AliasValues(ctx, AliasModelProviders)); !slices.Equal(got, []string{"openai.default"}) {
		t.Errorf("alias values: %v", got)
	}
}

func TestConfigMaintainsTheConfigFile(t *testing.T) {
	g, client := gateway(t)
	g.on("POST", "/api/config/init", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "section", "memory")
		reply(`{"initialized":["memory.sqlite"]}`)(w, r)
	})
	g.on("POST", "/api/config/migrate", reply(`{"migrated":true,"backup_path":"/c.bak","schema_version":3}`))
	g.on("GET", "/api/config/templates", reply(`{"templates":[{"path":"agents","kind":"map","value_type":"AgentConfig","description":"Agents"}]}`))
	g.on("POST", "/api/config/model-providers/anthropic/default/refresh-context-window",
		reply(`{"path":"providers.models.anthropic.default","context_window":200000}`))

	ctx, config := context.Background(), client.Config()
	if got := must[[]string](t)(config.Init(ctx, "memory")); !slices.Equal(got, []string{"memory.sqlite"}) {
		t.Errorf("init: %v", got)
	}
	if got := must[*MigrateResponse](t)(config.Migrate(ctx)); got.SchemaVersion != 3 {
		t.Errorf("migrate: %+v", got)
	}
	if got := must[[]TemplateEntry](t)(config.Templates(ctx)); got[0].Kind != "map" {
		t.Errorf("templates: %+v", got)
	}
	if got := must[uint64](t)(config.RefreshContextWindow(ctx, "anthropic", "default")); got != 200_000 {
		t.Errorf("context window: %d", got)
	}
}

func TestConfigReadsCatalogsAndSections(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/config/catalog", reply(`{"model_providers":[{"name":"openai","display_name":"OpenAI","local":false}]}`))
	g.on("GET", "/api/config/catalog/models", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "model_provider", "openai")
		reply(`{"model_provider":"openai","models":["gpt-x"],"local":false,"live":true}`)(w, r)
	})
	g.on("GET", "/api/config/agent-options", reply(`{"channels":[],"channel_types":[],"model_providers":["openai.default"],
		"risk_profiles":[],"runtime_profiles":[],"skill_bundles":[],"knowledge_bundles":[],"mcp_bundles":[],"agents":["helper"]}`))
	g.on("GET", "/api/config/sections", reply(`{"sections":[{"key":"channels","label":"Channels","help":"","has_picker":true,"completed":false}]}`))
	g.on("GET", "/api/config/sections/channels", reply(`{"section":"channels","help":"","items":[
		{"key":"whatsapp","label":"WhatsApp","badge":"configured"}]}`))
	g.on("POST", "/api/config/sections/channels/items/whatsapp", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"alias":"main"}`)
		reply(`{"fields_prefix":"channels.whatsapp.main","created":true}`)(w, r)
	})

	ctx, config := context.Background(), client.Config()
	if got := must[[]CatalogModelProvider](t)(config.Catalog(ctx)); got[0].Name != "openai" {
		t.Errorf("catalog: %+v", got)
	}
	if !must[*CatalogModels](t)(config.CatalogModels(ctx, "openai")).Live {
		t.Error("catalog models")
	}
	if got := must[*AgentOptions](t)(config.AgentOptions(ctx)); !slices.Equal(got.Agents, []string{"helper"}) {
		t.Errorf("agent options: %+v", got)
	}
	if !must[[]ConfigSection](t)(config.Sections(ctx))[0].HasPicker {
		t.Error("sections")
	}
	if got := must[*SectionItems](t)(config.SectionItems(ctx, "channels")); *got.Items[0].Badge != "configured" {
		t.Errorf("section items: %+v", got)
	}
	if got := must[*SelectedItem](t)(config.SelectSectionItem(ctx, "channels", "whatsapp", "main")); got.FieldsPrefix != "channels.whatsapp.main" {
		t.Errorf("select: %+v", got)
	}
}

func TestConfigSchemasUseOptions(t *testing.T) {
	g, client := gateway(t)
	g.public("OPTIONS", "/api/config", reply(`{"title":"Config"}`))
	g.public("OPTIONS", "/api/config/prop", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "path", "gateway.port")
		reply(`{"x-zeroclaw-prop":{"kind":"integer"}}`)(w, r)
	})
	ctx := context.Background()
	if got := must[json.RawMessage](t)(client.Config().Schema(ctx)); !sameJSON(t, got, []byte(`{"title":"Config"}`)) {
		t.Errorf("schema: %s", got)
	}
	if got := must[json.RawMessage](t)(client.Config().PropSchema(ctx, "gateway.port")); !sameJSON(t, got, []byte(`{"x-zeroclaw-prop":{"kind":"integer"}}`)) {
		t.Errorf("prop schema: %s", got)
	}
}
