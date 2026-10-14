package zeroclaw

import (
	"context"
	"net/http"
	"testing"
)

func TestSessionsReadAndManage(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/sessions", reply(`{"sessions":[{"session_id":"abc","session_key":"gw_abc","created_at":"t",
		"last_activity":"t","message_count":2,"agent_alias":"helper","channel_id":null}]}`))
	g.on("GET", "/api/sessions/running", reply(`{"sessions":[
		{"session_id":"abc","created_at":"t","last_activity":"t","message_count":2}]}`))
	g.on("GET", "/api/sessions/gw_abc/messages", reply(`{"session_id":"gw_abc","session_persistence":true,"messages":[
		{"role":"user","content":"hi","created_at":"t"}]}`))
	g.on("POST", "/api/sessions/gw_abc/messages", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"content":"Reminder"}`)
		reply(`{"status":"ok"}`)(w, r)
	})
	g.on("GET", "/api/sessions/gw_abc/state", reply(`{"session_id":"gw_abc","state":"running","turn_id":"t1"}`))
	g.on("PUT", "/api/sessions/gw_abc", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"name":"Support"}`)
		reply(`{"session_id":"gw_abc","name":"Support"}`)(w, r)
	})
	g.on("POST", "/api/sessions/gw_abc/abort", reply(`{"status":"no_active_response"}`))
	g.on("DELETE", "/api/sessions/gw_abc", reply(`{"deleted":true,"session_id":"gw_abc"}`))

	ctx, sessions := context.Background(), client.Sessions()
	if got := must[[]Session](t)(sessions.List(ctx)); got[0].SessionKey == nil || *got[0].SessionKey != "gw_abc" {
		t.Errorf("list: %+v", got)
	}
	if got := must[[]Session](t)(sessions.Running(ctx)); len(got) != 1 {
		t.Errorf("running: %+v", got)
	}
	if got := must[[]SessionMessage](t)(sessions.Messages(ctx, "gw_abc")); got[0].Role != "user" {
		t.Errorf("messages: %+v", got)
	}
	if err := sessions.AddMessage(ctx, "gw_abc", "Reminder"); err != nil {
		t.Fatal(err)
	}
	if got := must[*SessionState](t)(sessions.State(ctx, "gw_abc")); got.State != "running" {
		t.Errorf("state: %+v", got)
	}
	if err := sessions.Rename(ctx, "gw_abc", "Support"); err != nil {
		t.Fatal(err)
	}
	if must[bool](t)(sessions.Abort(ctx, "gw_abc")) {
		t.Error("abort reported a running turn")
	}
	if err := sessions.Delete(ctx, "gw_abc"); err != nil {
		t.Fatal(err)
	}
}
