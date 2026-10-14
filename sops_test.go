package zeroclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestDecisionsAreExternallyTagged(t *testing.T) {
	vectors := []struct {
		decision Decision
		want     string
	}{
		{ApproveDecision(), `"approve"`},
		{DenyDecision("no"), `{"deny":{"reason":"no"}}`},
		{DenyDecision(""), `{"deny":{}}`},
		{AmendDecision("t"), `{"amend":{"text":"t"}}`},
		{ReviseDecision("shorter"), `{"revise":{"guidance":"shorter"}}`},
	}
	for _, v := range vectors {
		got, err := json.Marshal(v.decision)
		if err != nil {
			t.Fatal(err)
		}
		if !sameJSON(t, got, []byte(v.want)) {
			t.Errorf("%+v encodes as %s, want %s", v.decision, got, v.want)
		}
	}
}

func TestSopsAuthorsAndRuns(t *testing.T) {
	sop := `{"name":"restock","description":"d","version":"1","priority":"normal",
		"execution_mode":"auto","triggers":[{"type":"manual"}],"steps":[]}`
	g, client := gateway(t)
	g.on("GET", "/api/sops", reply(`{"sops":[`+sop+`]}`))
	g.on("GET", "/api/sops/restock/full", reply(sop))
	g.on("POST", "/api/sops", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, sop)
		reply(`{"created":"restock"}`)(w, r)
	})
	g.on("PUT", "/api/sops/restock", reply(`{"saved":"restock"}`))
	g.on("DELETE", "/api/sops/restock", reply(`{"deleted":"restock"}`))
	for _, route := range []string{"/api/sops/restock/graph", "/api/sops/graph-legend", "/api/sops/trigger-sources"} {
		g.on("GET", route, reply(`{"ok":"`+route+`"}`))
	}
	g.on("POST", "/api/sops/graph-draft", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"sop":`+sop+`}`)
		reply(`{"nodes":[]}`)(w, r)
	})
	g.on("POST", "/api/sops/wire-draft", reply(`{"sop":`+sop+`,"graph":{}}`))

	ctx, sops := context.Background(), client.Sops()
	raw := json.RawMessage(sop)
	if got := must[[]json.RawMessage](t)(sops.List(ctx)); len(got) != 1 {
		t.Errorf("list: %s", got)
	}
	var def struct{ Name string }
	json.Unmarshal(must[json.RawMessage](t)(sops.Get(ctx, "restock")), &def)
	if def.Name != "restock" {
		t.Errorf("get: %+v", def)
	}
	for _, err := range []error{sops.Create(ctx, raw), sops.Save(ctx, "restock", raw), sops.Delete(ctx, "restock")} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := must[json.RawMessage](t)(sops.Graph(ctx, "restock")); !sameJSON(t, got, []byte(`{"ok":"/api/sops/restock/graph"}`)) {
		t.Errorf("graph: %s", got)
	}
	must[json.RawMessage](t)(sops.GraphLegend(ctx))
	must[json.RawMessage](t)(sops.TriggerSources(ctx))
	must[json.RawMessage](t)(sops.GraphDraft(ctx, raw))
	edit := map[string]any{"op": "connect", "from": 1, "to": 2, "role": "sequence"}
	var wired struct{ Graph map[string]any }
	json.Unmarshal(must[json.RawMessage](t)(sops.WireDraft(ctx, raw, edit)), &wired)
	if wired.Graph == nil {
		t.Errorf("wire draft: %+v", wired)
	}
}

func TestSopsFollowsAndDecidesRuns(t *testing.T) {
	g, client := gateway(t)
	g.on("POST", "/api/sops/restock/run", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"payload":"{\"sku\":1}"}`)
		reply(`{"run_id":"r1"}`)(w, r)
	})
	g.on("GET", "/api/sops/runs", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "sop", "restock")
		reply(`{"runs":[{"run_id":"r1","sop_name":"restock","status":"waiting_approval","current_step":1,
			"total_steps":2,"started_at":"t","completed_at":null,"trigger_source":"manual","active":true}]}`)(w, r)
	})
	g.on("GET", "/api/sops/restock/runs/r1/overlay", reply(`{"run_id":"r1","waiting":true}`))
	g.on("POST", "/api/sops/restock/runs/r1/decide", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"deny":{"reason":"not now"}}`)
		reply(`{"run_id":"r1","status":"failed"}`)(w, r)
	})
	g.on("POST", "/api/sops/restock/runs/r1/cancel", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"reason":null}`)
		replyStatus(202, `{"run_id":"r1","status":"cancel_requested"}`)(w, r)
	})
	g.on("GET", "/admin/sop/pending", reply(`{"pending":[{"run_id":"r1","sop_name":"restock","step":1,
		"total_steps":2,"waiting_since":"t","kind":"approval"}]}`))
	g.on("POST", "/admin/sop/approve", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"run_id":"r1","reason":null}`)
		reply(`{"outcome":"resumed","run_id":"r1"}`)(w, r)
	})
	g.on("POST", "/admin/sop/deny", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"run_id":"r1","reason":"no"}`)
		reply(`{"outcome":"denied","run_id":"r1"}`)(w, r)
	})

	ctx, sops := context.Background(), client.Sops()
	if got := must[string](t)(sops.Run(ctx, "restock", `{"sku":1}`)); got != "r1" {
		t.Errorf("run: %q", got)
	}
	if got := must[[]SopRun](t)(sops.Runs(ctx, "restock")); !got[0].Active || got[0].CompletedAt != nil {
		t.Errorf("runs: %+v", got)
	}
	if got := must[json.RawMessage](t)(sops.RunOverlay(ctx, "restock", "r1")); !sameJSON(t, got, []byte(`{"run_id":"r1","waiting":true}`)) {
		t.Errorf("overlay: %s", got)
	}
	var status struct{ Status string }
	json.Unmarshal(must[json.RawMessage](t)(sops.Decide(ctx, "restock", "r1", DenyDecision("not now"))), &status)
	if status.Status != "failed" {
		t.Errorf("decide: %+v", status)
	}
	json.Unmarshal(must[json.RawMessage](t)(sops.Cancel(ctx, "restock", "r1", "")), &status)
	if status.Status != "cancel_requested" {
		t.Errorf("cancel: %+v", status)
	}
	if got := must[[]PendingApproval](t)(sops.Pending(ctx)); got[0].Kind != "approval" {
		t.Errorf("pending: %+v", got)
	}
	if got := must[string](t)(sops.Approve(ctx, "r1")); got != "resumed" {
		t.Errorf("approve: %q", got)
	}
	if got := must[string](t)(sops.Deny(ctx, "r1", "no")); got != "denied" {
		t.Errorf("deny: %q", got)
	}
}
