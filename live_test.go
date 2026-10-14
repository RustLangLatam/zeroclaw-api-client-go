//go:build live

package zeroclaw

// Read-only calls against a real gateway, to check that every type decodes
// what the gateway actually sends. Built only with the live tag:
//
//	ZEROCLAW_URL=http://127.0.0.1:42617 ZEROCLAW_TOKEN=zc_… \
//	  go test -tags live -run TestLive -v .
//
// Optional: ZEROCLAW_AGENT (an agent alias) adds the per-agent reads.
// Nothing is written. Calls a build may not offer (features off, SOPs
// disabled, a channel not running) are reported, not failed.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// liveCheck fails on a decode or transport error; a refusal by the gateway
// (a route or feature it does not offer) is only reported.
func liveCheck[T any](t *testing.T, what string) func(T, error) {
	return func(_ T, err error) {
		t.Helper()
		var api *APIError
		switch {
		case err == nil:
			t.Logf("ok      %s", what)
		case errors.As(err, &api):
			t.Logf("refused %s: %d %s", what, api.Status, api.Message)
		default:
			t.Errorf("%s: %v", what, err)
		}
	}
}

func TestLiveEveryReadDecodes(t *testing.T) {
	url, token := os.Getenv("ZEROCLAW_URL"), os.Getenv("ZEROCLAW_TOKEN")
	if url == "" || token == "" {
		t.Skip("set ZEROCLAW_URL and ZEROCLAW_TOKEN")
	}
	client, err := New(url, WithToken(token))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	system := client.System()
	liveCheck[*Health](t, "health")(system.Health(ctx))
	liveCheck[*HealthSnapshot](t, "runtime_health")(system.RuntimeHealth(ctx))
	liveCheck[string](t, "metrics")(system.Metrics(ctx))
	liveCheck[*Status](t, "status")(system.Status(ctx, ""))
	liveCheck[[]ToolSpec](t, "tools")(system.Tools(ctx, ""))
	liveCheck[[]Integration](t, "integrations")(system.Integrations(ctx))
	liveCheck[map[string]IntegrationSetting](t, "integration_settings")(system.IntegrationSettings(ctx))
	liveCheck[[]Tui](t, "tuis")(system.Tuis(ctx))
	liveCheck[*LogPage](t, "logs")(system.Logs(ctx, LogQuery{}))
	liveCheck[*UpgradeStatus](t, "upgrade_status")(system.UpgradeStatus(ctx, ""))

	config := client.Config()
	liveCheck[json.RawMessage](t, "config.snapshot")(config.Snapshot(ctx))
	liveCheck[*PropValue](t, "config.get")(config.Get(ctx, "gateway.port"))
	liveCheck[*ConfigList](t, "config.list")(config.List(ctx, "gateway"))
	liveCheck[[]DriftEntry](t, "config.drift")(config.Drift(ctx))
	liveCheck[bool](t, "config.pending_reload")(config.PendingReload(ctx))
	liveCheck[[]TemplateEntry](t, "config.templates")(config.Templates(ctx))
	liveCheck[[]string](t, "config.map_keys")(config.MapKeys(ctx, "agents"))
	liveCheck[[]CatalogModelProvider](t, "config.catalog")(config.Catalog(ctx))
	liveCheck[*ConfigStatus](t, "config.status")(config.Status(ctx))
	liveCheck[*AgentOptions](t, "config.agent_options")(config.AgentOptions(ctx))
	liveCheck[[]ConfigSection](t, "config.sections")(config.Sections(ctx))

	liveCheck[[]Device](t, "pairing.devices")(client.Pairing().Devices(ctx))
	liveCheck[[]CronJob](t, "cron.list")(client.Cron().List(ctx))
	liveCheck[*CronSettings](t, "cron.settings")(client.Cron().Settings(ctx))
	liveCheck[[]MemoryEntry](t, "memory.search")(client.Memory().Search(ctx, MemoryQuery{}))
	liveCheck[*CostSummary](t, "cost.summary")(client.Cost().Summary(ctx, "", ""))
	liveCheck[[]Session](t, "sessions.running")(client.Sessions().Running(ctx))
	liveCheck[[]Event](t, "events.history")(client.Events().History(ctx))
	liveCheck[*FileListing](t, "files.list")(client.Files().List(ctx, ""))
	liveCheck[[]SkillBundle](t, "skills.bundles")(client.Skills().Bundles(ctx))
	liveCheck[[]json.RawMessage](t, "skills.slash_option_kinds")(client.Skills().SlashOptionKinds(ctx))
	liveCheck[[]json.RawMessage](t, "sops.list")(client.Sops().List(ctx))
	liveCheck[[]SopRun](t, "sops.runs")(client.Sops().Runs(ctx, ""))
	liveCheck[[]string](t, "canvas.list")(client.Canvas().List(ctx))
	liveCheck[json.RawMessage](t, "quickstart.state")(client.Quickstart().State(ctx))

	channels, err := client.Channels().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range channels {
		rooms, err := client.Channels().Rooms(ctx, ch.Name)
		if err != nil {
			continue
		}
		t.Logf("ok      rooms of %s: %d", ch.Name, len(rooms))
		if len(rooms) > 0 {
			liveCheck[[]RoomMember](t, "room_members")(client.Channels().RoomMembers(ctx, ch.Name, rooms[0].ID))
		}
	}
	sessions, err := client.Sessions().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) > 0 {
		id := sessions[0].SessionID
		if sessions[0].SessionKey != nil {
			id = *sessions[0].SessionKey
		}
		liveCheck[[]SessionMessage](t, "sessions.messages")(client.Sessions().Messages(ctx, id))
		liveCheck[*SessionState](t, "sessions.state")(client.Sessions().State(ctx, id))
	}

	if alias := os.Getenv("ZEROCLAW_AGENT"); alias != "" {
		agent := client.Agent(alias)
		liveCheck[*FileListing](t, "agent.list_files")(agent.ListFiles(ctx, ""))
		liveCheck[*PersonalityFiles](t, "agent.personality_files")(agent.PersonalityFiles(ctx))
		liveCheck[*PersonalityFile](t, "agent.personality")(agent.Personality(ctx, "SOUL.md"))
		liveCheck[*AgentSkills](t, "agent.skills")(agent.Skills(ctx))
		liveCheck[[]ToolSpec](t, "system.tools(agent)")(system.Tools(ctx, alias))
		liveCheck[*CostSummary](t, "cost.agent")(client.Cost().Agent(ctx, alias))
	}
}
