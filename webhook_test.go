package zeroclaw

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestWebhookSendsHeadersAndDetectsDuplicates(t *testing.T) {
	g, client := gateway(t)
	g.on("POST", "/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Idempotency-Key") == "k2" {
			reply(`{"status":"duplicate","idempotent":true,"message":""}`)(w, r)
			return
		}
		wantQuery(t, r, "agent", "helper")
		wantHeader(t, r, "X-Webhook-Secret", "s3")
		wantHeader(t, r, "X-Idempotency-Key", "k1")
		wantBody(t, r, `{"message":"Ping"}`)
		reply(`{"response":"Pong","model":"m"}`)(w, r)
	})

	ctx, webhook := context.Background(), client.Webhook()
	got := must[*WebhookReply](t)(webhook.Send(ctx, "Ping", WebhookOptions{Agent: "helper", Secret: "s3", IdempotencyKey: "k1"}))
	if got.Duplicate || got.Reply == nil || got.Reply.Response != "Pong" {
		t.Errorf("reply: %+v", got)
	}
	if got := must[*WebhookReply](t)(webhook.Send(ctx, "Ping", WebhookOptions{IdempotencyKey: "k2"})); !got.Duplicate || got.Reply != nil {
		t.Errorf("duplicate: %+v", got)
	}
}

func TestWebhookA2AReturnsTheAnswerTextOrTheRPCError(t *testing.T) {
	g, client := gateway(t)
	g.on("POST", "/a2a/helper", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"jsonrpc":"2.0","id":1,"method":"message/send",
			"params":{"message":{"parts":[{"kind":"text","text":"Hi"}]}}}`)
		reply(`{"jsonrpc":"2.0","id":1,"result":{
			"id":"t","kind":"task","status":{"state":"completed"},
			"artifacts":[{"artifactId":"a","parts":[{"kind":"text","text":"Hello"}]}]}}`)(w, r)
	})
	g.on("POST", "/a2a/other", reply(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"agent failed"}}`))
	g.on("GET", "/a2a/helper/.well-known/agent-card.json", reply(`{"name":"helper"}`))

	ctx, webhook := context.Background(), client.Webhook()
	if got := must[string](t)(webhook.A2ASend(ctx, "helper", "Hi")); got != "Hello" {
		t.Errorf("a2a: %q", got)
	}
	if _, err := webhook.A2ASend(ctx, "other", "Hi"); err == nil || !strings.Contains(err.Error(), "agent failed") || StatusCode(err) != 200 {
		t.Errorf("a2a error: %v", err)
	}
	if card := must[[]byte](t)(webhook.AgentCard(ctx, "helper")); !sameJSON(t, card, []byte(`{"name":"helper"}`)) {
		t.Errorf("card: %s", card)
	}
}
