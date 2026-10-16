package zeroclaw

import (
	"context"
	"net/http"
	"testing"
)

const testChannel = "whatsapp.main"

func TestChannelsListBindAndRelink(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/channels", reply(`{"channels":[{
		"name":"whatsapp.main","type":"whatsapp","alias":"main","owning_agent":"helper",
		"enabled":true,"compiled":true,"status":"active","message_count":0,
		"last_message_at":null,"health":"healthy",
		"readiness":{"enabled":"ready","bound_to_agent":"ready","authenticated":"ready",
			"listening":"ready","requirements":[],"notes":[]}}]}`))
	g.on("POST", "/api/channels/bind", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"channel_type":"telegram","alias":"bot","identity":"42"}`)
		reply(`{"saved":true,"already_bound":false,"group":"telegram_bot","channel":"telegram.bot"}`)(w, r)
	})
	g.on("POST", "/api/channels/whatsapp.main/relink", reply(`{"channel":"whatsapp.main","outcome":"cleared",
		"removed":["/wa/session.db"],"restart_required":true,"note":"reload"}`))

	ctx, channels := context.Background(), client.Channels()
	if got := must[[]Channel](t)(channels.List(ctx)); got[0].Type != "whatsapp" || got[0].Health != "healthy" {
		t.Errorf("list: %+v", got)
	}
	if got := must[*Bound](t)(channels.Bind(ctx, "telegram", "bot", "42")); got.Group != "telegram_bot" {
		t.Errorf("bind: %+v", got)
	}
	if !must[*Relinked](t)(channels.Relink(ctx, testChannel)).RestartRequired {
		t.Error("relink")
	}
}
