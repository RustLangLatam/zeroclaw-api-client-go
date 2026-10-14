package zeroclaw

import (
	"context"
	"net/http"
	"strings"
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

func TestChannelsSendAndManageGroupChats(t *testing.T) {
	g, client := gateway(t)
	room := "120363000000000000@g.us"
	g.on("POST", "/api/channels/whatsapp.main/send", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"to":"+34600000000","content":"Hola","thread_id":null}`)
		reply(`{"channel":"whatsapp.main","to":"+34600000000","outcome":"sent"}`)(w, r)
	})
	g.on("GET", "/api/channels/whatsapp.main/rooms", reply(`{"channel":"whatsapp.main","rooms":[
		{"id":"`+room+`","name":"Team","participants":3}]}`))
	g.on("POST", "/api/channels/whatsapp.main/rooms", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"name":"Team","topic":null,"invites":["+34600000000"]}`)
		reply(`{"channel":"whatsapp.main","room":"`+room+`","outcome":"created",
			"participants":[{"user":"+34600000000","outcome":"added","detail":null}]}`)(w, r)
	})
	g.on("GET", "/api/channels/whatsapp.main/rooms/"+room+"/members", reply(`{"channel":"whatsapp.main","room":"`+room+`",
		"members":[{"id":"+34600000000","admin":true}]}`))
	g.on("POST", "/api/channels/whatsapp.main/rooms/"+room+"/invites", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"user":"+34611111111"}`)
		reply(`{"channel":"whatsapp.main","room":"`+room+`","user":"+34611111111","outcome":"invited","detail":null}`)(w, r)
	})
	g.on("DELETE", "/api/channels/whatsapp.main/rooms/"+room+"/members/+34611111111",
		reply(`{"channel":"whatsapp.main","room":"`+room+`","user":"+34611111111","outcome":"removed"}`))

	ctx, channels := context.Background(), client.Channels()
	if err := channels.Send(ctx, testChannel, "+34600000000", "Hola", ""); err != nil {
		t.Fatal(err)
	}
	if got := must[[]RoomSummary](t)(channels.Rooms(ctx, testChannel)); got[0].Participants != 3 {
		t.Errorf("rooms: %+v", got)
	}
	if got := must[*CreatedRoom](t)(channels.CreateRoom(ctx, testChannel, "Team", "", []string{"+34600000000"})); got.Participants[0].Outcome != "added" {
		t.Errorf("create room: %+v", got)
	}
	if !must[[]RoomMember](t)(channels.RoomMembers(ctx, testChannel, room))[0].Admin {
		t.Error("room members")
	}
	if got := must[*Participant](t)(channels.Invite(ctx, testChannel, room, "+34611111111")); got.Outcome != "invited" {
		t.Errorf("invite: %+v", got)
	}
	if err := channels.RemoveMember(ctx, testChannel, room, "+34611111111"); err != nil {
		t.Fatal(err)
	}
}

func TestChannelsRefusalsKeepTheirReason(t *testing.T) {
	g, client := gateway(t)
	g.on("POST", "/api/channels/whatsapp.main/send", replyStatus(403,
		`{"channel":"whatsapp.main","error":"channel whatsapp.main is not in [gateway] send_channels"}`))
	err := client.Channels().Send(context.Background(), testChannel, "x", "y", "")
	if !IsUnauthorized(err) || !strings.Contains(err.Error(), "send_channels") {
		t.Errorf("%v", err)
	}
}
