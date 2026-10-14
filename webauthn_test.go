package zeroclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestWebAuthnCarriesCeremonies(t *testing.T) {
	g, client := gateway(t)
	g.on("POST", "/api/webauthn/register/start", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"user_id":"u","user_name":"Ana"}`)
		reply(`{"challenge":"c"}`)(w, r)
	})
	for _, route := range []string{"/api/webauthn/register/finish", "/api/webauthn/auth/start", "/api/webauthn/auth/finish"} {
		g.on("POST", route, reply(`{"status":"ok"}`))
	}
	g.on("GET", "/api/webauthn/credentials", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "user_id", "u")
		reply(`{"credentials":[]}`)(w, r)
	})
	g.on("DELETE", "/api/webauthn/credentials/k1", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "user_id", "u")
		reply(`{"status":"deleted"}`)(w, r)
	})

	ctx, webauthn := context.Background(), client.WebAuthn()
	var start struct{ Challenge string }
	if json.Unmarshal(must[json.RawMessage](t)(webauthn.RegisterStart(ctx, "u", "Ana")), &start); start.Challenge != "c" {
		t.Errorf("register start: %+v", start)
	}
	must[json.RawMessage](t)(webauthn.RegisterFinish(ctx, map[string]any{}))
	must[json.RawMessage](t)(webauthn.AuthStart(ctx, "u"))
	must[json.RawMessage](t)(webauthn.AuthFinish(ctx, map[string]any{}))
	var creds struct{ Credentials []json.RawMessage }
	if json.Unmarshal(must[json.RawMessage](t)(webauthn.Credentials(ctx, "u")), &creds); creds.Credentials == nil {
		t.Error("credentials is not an array")
	}
	if err := webauthn.DeleteCredential(ctx, "u", "k1"); err != nil {
		t.Fatal(err)
	}
}
