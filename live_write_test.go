//go:build live

package zeroclaw

// Writes against a real gateway: every subtest creates what it needs under
// the sdk-go-test prefix and removes it in t.Cleanup. Built only with the
// live tag, and only run with ZEROCLAW_WRITE=1:
//
//	ZEROCLAW_URL=… ZEROCLAW_TOKEN=… ZEROCLAW_AGENT=<alias> ZEROCLAW_WRITE=1 \
//	  go test -tags live -run TestLiveWrites -v .
//
// Some subtests need more of the gateway and skip without it:
//   - ZEROCLAW_TURNS=1 lets them start agent turns (chat, webhook, A2A),
//     which spend model tokens;
//   - admin routes answer only on the gateway host (run the test binary in
//     the gateway's network namespace);
//   - SOPs need [sop] sops_dir, A2A needs a2a.server.enabled and the agent
//     published, WebAuthn a build with the webauthn feature.
//
// Never run against a gateway you cannot afford to have changed: a test cut
// short may leave sdk-go-test leftovers.

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

const liveName = "sdk-go-test"

// liveSkip skips when the gateway does not offer what err says it lacks: a
// disabled subsystem (503), a missing route or feature (404), or an admin
// route called from off the host (403).
func liveSkip(t *testing.T, what string, err error) {
	t.Helper()
	switch StatusCode(err) {
	case 403, 404, 503:
		t.Skipf("%s not offered by this gateway: %v", what, err)
	}
}

func liveOK[T any](t *testing.T, what string) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		return v
	}
}

func TestLiveWrites(t *testing.T) {
	url, token, agent := os.Getenv("ZEROCLAW_URL"), os.Getenv("ZEROCLAW_TOKEN"), os.Getenv("ZEROCLAW_AGENT")
	if url == "" || token == "" || agent == "" || os.Getenv("ZEROCLAW_WRITE") != "1" {
		t.Skip("set ZEROCLAW_URL, ZEROCLAW_TOKEN, ZEROCLAW_AGENT and ZEROCLAW_WRITE=1")
	}
	turns := os.Getenv("ZEROCLAW_TURNS") == "1"
	client, err := New(url, WithToken(token))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	t.Run("config", func(t *testing.T) {
		config := client.Config()
		const path = "gateway.webhook_rate_limit_per_minute"
		before := liveOK[*PropValue](t, "get")(config.Get(ctx, path))
		t.Cleanup(func() { config.Patch(ctx, []PatchOp{ReplaceOp(path, before.Value)}, true) })
		liveOK[*PropValue](t, "set")(config.Set(ctx, path, 61, "set by "+liveName))
		patched := liveOK[*PatchResponse](t, "patch")(config.Patch(ctx, []PatchOp{
			TestOp(path, "61"), ReplaceOp(path, 62), CommentOp(path, liveName),
		}, false))
		if !patched.Saved || len(patched.Results) != 3 {
			t.Errorf("patch: %+v", patched)
		}
		if _, err := config.Patch(ctx, []PatchOp{TestOp(path, "1"), ReplaceOp(path, 1)}, false); err == nil {
			t.Error("a failing test op did not refuse the patch")
		}
		// Reset on a field already at its default, so nothing changes.
		liveOK[*PropValue](t, "reset")(config.Reset(ctx, "security.webauthn.rp_name"))

		const profiles = "risk_profiles"
		key, renamed := strings.ReplaceAll(liveName, "-", "_"), strings.ReplaceAll(liveName, "-", "_")+"_2"
		t.Cleanup(func() {
			config.DeleteMapKey(ctx, profiles, key)
			config.DeleteMapKey(ctx, profiles, renamed)
		})
		if !liveOK[*MapKeyResponse](t, "create map key")(config.CreateMapKey(ctx, profiles, key)).Created {
			t.Error("not created")
		}
		liveOK[*DeletePlan](t, "delete plan")(config.DeletePlan(ctx, profiles, key))
		if !liveOK[*RenameResponse](t, "rename")(config.RenameMapKey(ctx, profiles, key, renamed)).Renamed {
			t.Error("not renamed")
		}
		liveOK[*MapKeyResponse](t, "delete map key")(config.DeleteMapKey(ctx, profiles, renamed))
		liveOK[[]string](t, "alias values")(config.AliasValues(ctx, AliasRiskProfiles))
		liveOK[*CatalogModels](t, "catalog models")(config.CatalogModels(ctx, "openai"))
		sections := liveOK[[]ConfigSection](t, "sections")(config.Sections(ctx))
		for _, s := range sections {
			if s.HasPicker {
				liveOK[*SectionItems](t, "section items")(config.SectionItems(ctx, s.Key))
				break
			}
		}
		liveOK[json.RawMessage](t, "schema")(config.Schema(ctx))
		liveOK[json.RawMessage](t, "prop schema")(config.PropSchema(ctx, path))
	})

	t.Run("cron", func(t *testing.T) {
		cron := client.Cron()
		job := liveOK[*CronJob](t, "create")(cron.Create(ctx, &NewCronJob{
			Agent: agent, Name: liveName, Schedule: "0 0 1 1 *",
			JobType: "shell", Command: "echo " + liveName, ShellOutputFormat: "raw",
		}))
		t.Cleanup(func() { cron.Delete(ctx, job.ID) })
		disabled := false
		if liveOK[*CronJob](t, "update")(cron.Update(ctx, job.ID, &CronJobPatch{Enabled: &disabled})).Enabled {
			t.Error("still enabled")
		}
		run := liveOK[*CronRunResult](t, "run")(cron.Run(ctx, job.ID))
		t.Logf("run: success=%t output=%q", run.Success, run.Output)
		liveOK[[]CronRun](t, "runs")(cron.Runs(ctx, job.ID, 5))
		settings := liveOK[*CronSettings](t, "settings")(cron.Settings(ctx))
		liveOK[*CronSettings](t, "update settings")(cron.UpdateSettings(ctx, &CronSettingsPatch{Enabled: &settings.Enabled}))
		if err := cron.Delete(ctx, job.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("memory", func(t *testing.T) {
		memory := client.Memory()
		t.Cleanup(func() { memory.Forget(ctx, liveName, agent) })
		if err := memory.Store(ctx, liveName, "written by the Go SDK live test", "", agent); err != nil {
			t.Fatal(err)
		}
		liveOK[[]MemoryEntry](t, "search")(memory.Search(ctx, MemoryQuery{Query: "Go SDK live test", Agent: agent}))
		if !liveOK[bool](t, "forget")(memory.Forget(ctx, liveName, agent)) {
			t.Error("nothing forgotten")
		}
	})

	t.Run("files", func(t *testing.T) {
		files := client.Files()
		t.Cleanup(func() { files.RemoveDir(ctx, liveName) })
		if err := files.MakeDir(ctx, liveName); err != nil {
			t.Fatal(err)
		}
		liveOK[*FileListing](t, "list")(files.List(ctx, liveName))
		if err := files.RemoveDir(ctx, liveName); err != nil {
			t.Fatal(err)
		}
		// The smallest valid PNG: one transparent pixel.
		png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89" +
			"\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82")
		up, err := files.UploadImage(ctx, agent, png)
		liveSkip(t, "image upload", err)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("uploaded to %s", up.Path)
	})

	t.Run("agent workspace", func(t *testing.T) {
		a := client.Agent(agent)
		dir, moved := liveName, liveName+"-moved"
		t.Cleanup(func() { a.RemovePath(ctx, dir); a.RemovePath(ctx, moved) })
		if err := a.MakeDir(ctx, dir); err != nil {
			t.Fatal(err)
		}
		if err := a.MovePath(ctx, dir, moved); err != nil {
			t.Fatal(err)
		}
		if err := a.RemovePath(ctx, moved); err != nil {
			t.Fatal(err)
		}
		soul := liveOK[*PersonalityFile](t, "personality")(a.Personality(ctx, "SOUL.md"))
		if soul.Exists && soul.Content != nil && !soul.Truncated {
			// Writes the same content back: exercises the drift guard without
			// changing the agent.
			liveOK[*PersonalityWritten](t, "write personality")(a.WritePersonality(ctx, "SOUL.md", *soul.Content, soul.MtimeMs))
		}
		liveOK[*PersonalityTemplates](t, "templates")(a.PersonalityTemplates(ctx, PersonalityContext{AgentName: liveName}))
	})

	t.Run("skills", func(t *testing.T) {
		config, skills := client.Config(), client.Skills()
		bundle := strings.ReplaceAll(liveName, "-", "_")
		t.Cleanup(func() { config.DeleteMapKey(ctx, "skill_bundles", bundle) })
		liveOK[*MapKeyResponse](t, "create bundle")(config.CreateMapKey(ctx, "skill_bundles", bundle))
		t.Cleanup(func() { skills.Delete(ctx, bundle, liveName, true) })
		err := skills.Create(ctx, bundle, liveName, map[string]any{"name": liveName, "description": "Go SDK live test"}, "Say hi.", false)
		liveSkip(t, "skill authoring", err)
		if err != nil {
			t.Fatal(err)
		}
		liveOK[*Skill](t, "read")(skills.Read(ctx, bundle, liveName))
		if err := skills.Write(ctx, bundle, liveName, map[string]any{"name": liveName, "description": "Go SDK live test, edited"}, "Say hello."); err != nil {
			t.Fatal(err)
		}
		liveOK[[]SkillSummary](t, "list")(skills.List(ctx, bundle))
		if err := skills.Delete(ctx, bundle, liveName, true); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("canvas", func(t *testing.T) {
		canvas := client.Canvas()
		t.Cleanup(func() { canvas.Clear(ctx, liveName) })
		feed, err := canvas.Watch(ctx, liveName)
		if err != nil {
			t.Fatal(err)
		}
		defer feed.Close()
		liveOK[*CanvasFrame](t, "publish")(canvas.Publish(ctx, liveName, "markdown", "# "+liveName))
		frameCtx, stop := context.WithTimeout(ctx, 10*time.Second)
		defer stop()
		for {
			raw, err := feed.Next(frameCtx)
			if err != nil {
				t.Fatalf("watch: %v", err)
			}
			if strings.Contains(string(raw), "# "+liveName) {
				break
			}
		}
		liveOK[*CanvasFrame](t, "current")(canvas.Current(ctx, liveName))
		if len(liveOK[[]CanvasFrame](t, "history")(canvas.History(ctx, liveName))) == 0 {
			t.Error("empty history")
		}
		if err := canvas.Clear(ctx, liveName); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("pairing", func(t *testing.T) {
		pairing := client.Pairing()
		code, err := pairing.NewCode(ctx)
		liveSkip(t, "minting pairing codes", err)
		if err != nil {
			t.Fatal(err)
		}
		paired := liveOK[*Paired](t, "pair")(pairing.Pair(ctx, code, liveName, "service"))
		device := client.WithToken(paired.Token)
		devices := liveOK[[]Device](t, "devices")(device.Pairing().Devices(ctx))
		i := slices.IndexFunc(devices, func(d Device) bool { return d.Name != nil && *d.Name == liveName })
		if i < 0 {
			t.Fatalf("the new device is not listed: %+v", devices)
		}
		id := devices[i].ID
		t.Cleanup(func() { pairing.RevokeDevice(ctx, id) })
		liveOK[[]string](t, "capabilities")(device.Pairing().SetCapabilities(ctx, []string{"chat"}))

		// Rotating revokes the device and mints a code to pair it again.
		again := liveOK[string](t, "rotate")(pairing.RotateDeviceToken(ctx, id))
		if _, err := device.Pairing().Devices(ctx); !IsUnauthorized(err) {
			t.Errorf("a rotated token still works: %v", err)
		}
		if again == "" {
			t.Fatal("rotate minted no code")
		}
		repaired := liveOK[*Paired](t, "pair again")(pairing.Pair(ctx, again, liveName, "service"))
		devices = liveOK[[]Device](t, "devices")(pairing.Devices(ctx))
		i = slices.IndexFunc(devices, func(d Device) bool { return d.Name != nil && *d.Name == liveName })
		if i < 0 {
			t.Fatalf("the re-paired device is not listed: %+v", devices)
		}
		id = devices[i].ID
		if err := pairing.RevokeDevice(ctx, id); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WithToken(repaired.Token).Pairing().Devices(ctx); !IsUnauthorized(err) {
			t.Errorf("a revoked token still works: %v", err)
		}
	})

	t.Run("system", func(t *testing.T) {
		system := client.System()
		liveOK[*DoctorReport](t, "doctor")(system.Doctor(ctx))
		liveOK[[]CLITool](t, "cli tools")(system.CLITools(ctx))
		liveOK[*LogPage](t, "logs")(system.Logs(ctx, LogQuery{Limit: 5}))
		liveOK[json.RawMessage](t, "openapi")(system.OpenAPI(ctx))
		if v, err := system.VersionCheck(ctx, false, ""); err != nil {
			t.Logf("version check (needs internet): %v", err)
		} else {
			t.Logf("version check: %+v", v)
		}
		liveOK[*UpgradeStatus](t, "upgrade status")(system.UpgradeStatus(ctx, ""))
		if opts, err := system.ToolParamOptions(ctx, "channel_refs", agent, nil); err != nil {
			liveSkip(t, "tool parameter options", err)
			t.Fatal(err)
		} else {
			t.Logf("tool parameter options: %d", len(opts))
		}
	})

	t.Run("quickstart", func(t *testing.T) {
		q := client.Quickstart()
		liveOK[json.RawMessage](t, "state")(q.State(ctx))
		liveOK[json.RawMessage](t, "fields")(q.Fields(ctx, "model_provider", "openai"))
		if _, err := q.Validate(ctx, map[string]any{}); err != nil && StatusCode(err) == 0 {
			t.Fatal(err) // a refusal of an empty submission is the expected answer
		}
	})

	t.Run("sops", func(t *testing.T) {
		sops := client.Sops()
		sop := map[string]any{
			"name": liveName, "description": "Go SDK live test", "version": "1.0.0",
			"priority": "low", "execution_mode": "supervised",
			"triggers": []map[string]any{{"type": "manual"}},
			"steps": []map[string]any{{
				"number": 1, "title": "Greet", "body": "Reply with hi.", "requires_confirmation": true,
			}},
		}
		err := sops.Create(ctx, sop)
		liveSkip(t, "SOPs", err)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { sops.Delete(ctx, liveName) })
		liveOK[json.RawMessage](t, "get")(sops.Get(ctx, liveName))
		sop["description"] = "Go SDK live test, saved"
		if err := sops.Save(ctx, liveName, sop); err != nil {
			t.Fatal(err)
		}
		liveOK[json.RawMessage](t, "graph")(sops.Graph(ctx, liveName))
		liveOK[json.RawMessage](t, "graph draft")(sops.GraphDraft(ctx, sop))
		liveOK[json.RawMessage](t, "legend")(sops.GraphLegend(ctx))
		liveOK[json.RawMessage](t, "trigger sources")(sops.TriggerSources(ctx))
		if _, err := sops.WireDraft(ctx, sop, map[string]any{"op": "noop"}); err != nil && StatusCode(err) == 0 {
			t.Fatal(err) // the gateway may refuse an edit it does not know
		}

		// The engine loads SOPs when the daemon starts: reload it (an admin
		// route, gateway host only) and wait for the gateway to come back.
		if _, err := client.System().Reload(ctx); err != nil {
			liveSkip(t, "running SOPs (needs a daemon reload)", err)
			t.Fatal(err)
		}
		liveWaitHealthy(t, ctx, client)

		feed, err := sops.WatchRuns(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer feed.Close()
		runID := liveOK[string](t, "run")(sops.Run(ctx, liveName, `{"from":"`+liveName+`"}`))
		// The engine audits every run into memory as sop_run_<id>.
		t.Cleanup(func() { client.Memory().Forget(ctx, "sop_run_"+runID, "") })
		frameCtx, stop := context.WithTimeout(ctx, 10*time.Second)
		defer stop()
		for {
			raw, err := feed.Next(frameCtx)
			if err != nil {
				t.Fatalf("watch runs: %v", err)
			}
			if strings.Contains(string(raw), runID) {
				break
			}
		}
		runs := liveOK[[]SopRun](t, "runs")(sops.Runs(ctx, liveName))
		if !slices.ContainsFunc(runs, func(r SopRun) bool { return r.RunID == runID }) {
			t.Errorf("run %s not listed", runID)
		}
		liveOK[json.RawMessage](t, "overlay")(sops.RunOverlay(ctx, liveName, runID))

		// The supervised run waits for an operator before its first step.
		if pending, err := sops.Pending(ctx); err != nil {
			t.Logf("pending (admin, gateway host only): %v", err)
		} else {
			t.Logf("pending approvals: %d", len(pending))
		}
		if _, err := sops.Decide(ctx, liveName, runID, DenyDecision(liveName)); err != nil {
			t.Logf("decide: %v", err)
		}
		if _, err := sops.Cancel(ctx, liveName, runID, liveName); err != nil {
			t.Logf("cancel (the run may have ended): %v", err)
		}
		// A second run, for the host-only approve and deny routes.
		second := liveOK[string](t, "second run")(sops.Run(ctx, liveName, ""))
		t.Cleanup(func() { client.Memory().Forget(ctx, "sop_run_"+second, "") })
		if _, err := sops.Deny(ctx, second, liveName); err != nil {
			t.Logf("deny (admin, gateway host only): %v", err)
			sops.Cancel(ctx, liveName, second, liveName)
		}
		if err := sops.Delete(ctx, liveName); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("admin", func(t *testing.T) {
		pairing := client.Pairing()
		_, err := pairing.AdminCode(ctx)
		liveSkip(t, "admin routes", err)
		if err != nil {
			t.Fatal(err)
		}
		liveOK[*PairingCode](t, "initial code")(pairing.InitialCode(ctx))
		liveOK[*PairingCode](t, "admin new code")(pairing.AdminNewCode(ctx, ""))

		// The legacy pairing route, with a code minted for it. The new
		// device is the one the list did not have before.
		before := liveOK[[]Device](t, "devices")(pairing.Devices(ctx))
		code := liveOK[string](t, "new code")(pairing.NewCode(ctx))
		legacy := liveOK[*Paired](t, "pair legacy")(pairing.PairLegacy(ctx, code))
		after := liveOK[[]Device](t, "devices")(client.WithToken(legacy.Token).Pairing().Devices(ctx))
		for _, d := range after {
			if !slices.ContainsFunc(before, func(b Device) bool { return b.ID == d.ID }) {
				if err := pairing.RevokeDevice(ctx, d.ID); err != nil {
					t.Errorf("revoking the legacy device: %v", err)
				}
			}
		}

		// Already on the current schema: nothing to migrate.
		m := liveOK[*MigrateResponse](t, "migrate")(client.Config().Migrate(ctx))
		t.Logf("migrate: migrated=%t schema=%d", m.Migrated, m.SchemaVersion)
	})

	t.Run("webauthn", func(t *testing.T) {
		w := client.WebAuthn()
		_, err := w.Credentials(ctx, liveName)
		liveSkip(t, "WebAuthn", err)
		if err != nil {
			t.Fatal(err)
		}
		origin := liveOK[*PropValue](t, "rp origin")(client.Config().Get(ctx, "security.webauthn.rp_origin"))
		rpID := liveOK[*PropValue](t, "rp id")(client.Config().Get(ctx, "security.webauthn.rp_id"))
		key := newSoftKey(t, liveString(origin.Value), liveString(rpID.Value))
		t.Cleanup(func() { w.DeleteCredential(ctx, liveName, key.id) })

		// Registration: the gateway's challenge, signed into a credential.
		creation := liveOK[json.RawMessage](t, "register start")(w.RegisterStart(ctx, liveName, liveName))
		liveOK[json.RawMessage](t, "register finish")(w.RegisterFinish(ctx, key.register(t, liveChallenge(t, creation))))
		creds := liveOK[json.RawMessage](t, "credentials")(w.Credentials(ctx, liveName))
		if !strings.Contains(string(creds), key.id) {
			t.Fatalf("the new key is not listed: %s", creds)
		}

		// Authentication, twice: the sign counter must rise each time.
		for range 2 {
			request := liveOK[json.RawMessage](t, "auth start")(w.AuthStart(ctx, liveName))
			liveOK[json.RawMessage](t, "auth finish")(w.AuthFinish(ctx, key.assert(t, liveChallenge(t, request), true)))
		}
		// A replayed counter (a cloned key) and a forged signature are refused.
		request := liveOK[json.RawMessage](t, "auth start")(w.AuthStart(ctx, liveName))
		key.count--
		if _, err := w.AuthFinish(ctx, key.assert(t, liveChallenge(t, request), true)); !IsUnauthorized(err) {
			t.Errorf("a counter that did not rise was accepted: %v", err)
		}
		request = liveOK[json.RawMessage](t, "auth start")(w.AuthStart(ctx, liveName))
		if _, err := w.AuthFinish(ctx, key.assert(t, liveChallenge(t, request), false)); !IsUnauthorized(err) {
			t.Errorf("a forged signature was accepted: %v", err)
		}

		if err := w.DeleteCredential(ctx, liveName, key.id); err != nil {
			t.Fatal(err)
		}
		if creds := liveOK[json.RawMessage](t, "credentials")(w.Credentials(ctx, liveName)); strings.Contains(string(creds), key.id) {
			t.Errorf("the key is still listed: %s", creds)
		}
	})

	t.Run("a2a", func(t *testing.T) {
		webhook := client.Webhook()
		_, err := webhook.AgentCard(ctx, agent)
		liveSkip(t, "A2A", err)
		if err != nil {
			t.Fatal(err)
		}
		if !turns {
			t.Skip("ZEROCLAW_TURNS=1 to send an A2A message")
		}
		reply := liveOK[string](t, "a2a send")(webhook.A2ASend(ctx, agent, "Reply with the single word: pong"))
		t.Logf("a2a reply: %q", reply)
	})

	t.Run("turns", func(t *testing.T) {
		if !turns {
			t.Skip("ZEROCLAW_TURNS=1 to start agent turns (they spend tokens)")
		}
		chat := liveOK[*ChatSession](t, "chat")(client.Chat(ctx, agent, ChatOptions{Name: liveName}))
		reply, err := chat.Ask(ctx, "Reply with the single word: pong")
		if err != nil {
			t.Fatalf("ask: %v", err)
		}
		t.Logf("chat reply: %q (%s)", reply.FullResponse, reply.Model)
		if err := chat.Send(ctx, "Reply with the single word: ping"); err != nil {
			t.Fatal(err)
		}
		for {
			frame, err := chat.NextFrame(ctx)
			if err != nil {
				t.Fatalf("next frame: %v", err)
			}
			if frame.Type == FrameDone || frame.Type == FrameError {
				break
			}
		}
		id := chat.SessionID()
		chat.Close()

		sessions := client.Sessions()
		key := "gw_" + id
		t.Cleanup(func() { sessions.Delete(ctx, key) })
		liveOK[[]SessionMessage](t, "messages")(sessions.Messages(ctx, key))
		liveOK[*SessionState](t, "state")(sessions.State(ctx, key))
		if err := sessions.Rename(ctx, key, liveName+"-renamed"); err != nil {
			t.Fatal(err)
		}
		if err := sessions.AddMessage(ctx, key, "note from "+liveName); err != nil {
			liveSkip(t, "adding messages", err)
			t.Fatal(err)
		}
		liveOK[bool](t, "abort")(sessions.Abort(ctx, key))
		if err := sessions.Delete(ctx, key); err != nil {
			t.Fatal(err)
		}

		// A webhook turn, then the same idempotency key again: the gateway
		// must not run it twice.
		webhook := client.Webhook()
		opts := WebhookOptions{Agent: agent, IdempotencyKey: liveName + "-" + time.Now().Format("150405.000")}
		first, err := webhook.Send(ctx, "Reply with the single word: pong", opts)
		liveSkip(t, "webhook", err)
		if err != nil {
			t.Fatal(err)
		}
		if first.Duplicate || first.Reply == nil {
			t.Fatalf("first webhook: %+v", first)
		}
		if again := liveOK[*WebhookReply](t, "duplicate")(webhook.Send(ctx, "Reply with the single word: pong", opts)); !again.Duplicate {
			t.Errorf("the duplicate ran again: %+v", again)
		}
	})

	t.Run("events", func(t *testing.T) {
		// A manual cron run publishes a cron_result: subscribe, run, wait.
		cron := client.Cron()
		job := liveOK[*CronJob](t, "create")(cron.Create(ctx, &NewCronJob{
			Agent: agent, Name: liveName + "-events", Schedule: "0 0 1 1 *",
			JobType: "shell", Command: "echo " + liveName,
		}))
		t.Cleanup(func() { cron.Delete(ctx, job.ID) })
		streamCtx, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		events, err := client.Events().Subscribe(streamCtx)
		if err != nil {
			t.Fatal(err)
		}
		defer events.Close()
		go cron.Run(ctx, job.ID)
		for {
			e, err := events.Next()
			if errors.Is(err, ErrDecode) {
				continue
			}
			if err != nil {
				t.Fatalf("no cron_result for %s: %v", job.ID, err)
			}
			var id string
			if e.Kind() == "cron_result" && e.Decode("job_id", &id) == nil && id == job.ID {
				t.Logf("cron_result for %s", id)
				return
			}
		}
	})
}

// liveWaitHealthy waits up to a minute for the gateway to answer again.
func liveWaitHealthy(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	time.Sleep(2 * time.Second)
	for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(time.Second) {
		if _, err := client.System().Health(ctx); err == nil {
			return
		}
	}
	t.Fatal("the gateway did not come back after the reload")
}

// softKey is a software WebAuthn authenticator: an ES256 key that signs the
// gateway's challenges as a hardware key would.
type softKey struct {
	key          *ecdsa.PrivateKey
	id           string
	origin, rpID string
	count        uint32
}

func newSoftKey(t *testing.T, origin, rpID string) *softKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &softKey{key: key, id: b64(append([]byte(liveName+"-"), key.X.Bytes()[:8]...)), origin: origin, rpID: rpID}
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (k *softKey) clientData(kind, challenge string) []byte {
	b, _ := json.Marshal(map[string]any{"type": kind, "challenge": challenge, "origin": k.origin, "crossOrigin": false})
	return b
}

// register answers a creation challenge with the key's public point, in the
// attestation form the gateway's enrollment accepts.
func (k *softKey) register(t *testing.T, challenge string) map[string]any {
	point, err := k.key.PublicKey.ECDH()
	if err != nil {
		t.Fatal(err)
	}
	attestation, _ := json.Marshal(map[string]any{"public_key": b64(point.Bytes()), "sign_count": 0})
	return map[string]any{
		"challenge":          challenge,
		"id":                 k.id,
		"attestation_object": b64(attestation),
		"client_data_json":   b64(k.clientData("webauthn.create", challenge)),
		"label":              liveName,
	}
}

// assert answers a request challenge: authenticator data (rpIdHash, user
// present, counter) signed with clientData's hash. Without valid, the
// signature is over other data.
func (k *softKey) assert(t *testing.T, challenge string, valid bool) map[string]any {
	k.count++
	rpHash := sha256.Sum256([]byte(k.rpID))
	authData := append(rpHash[:], 0x05, 0, 0, 0, 0) // user present + verified
	binary.BigEndian.PutUint32(authData[33:], k.count)
	clientData := k.clientData("webauthn.get", challenge)
	cdHash := sha256.Sum256(clientData)
	digest := sha256.Sum256(append(slices.Clone(authData), cdHash[:]...))
	if !valid {
		digest = sha256.Sum256([]byte("forged"))
	}
	sig, err := ecdsa.SignASN1(rand.Reader, k.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"challenge":          challenge,
		"id":                 k.id,
		"authenticator_data": b64(authData),
		"client_data_json":   b64(clientData),
		"signature":          b64(sig),
	}
}

// liveChallenge is the challenge of a ceremony's options.
func liveChallenge(t *testing.T, options json.RawMessage) string {
	var o struct {
		Challenge string `json:"challenge"`
	}
	if json.Unmarshal(options, &o) != nil || o.Challenge == "" {
		t.Fatalf("no challenge in %s", options)
	}
	return o.Challenge
}

// liveString is a config display value as a plain string.
func liveString(raw json.RawMessage) string {
	var s string
	json.Unmarshal(raw, &s)
	return s
}
