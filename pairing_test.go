package zeroclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestPairingNeverShowsItsToken(t *testing.T) {
	var paired Paired
	if err := json.Unmarshal([]byte(`{"paired":true,"persisted":true,"token":"zc_secret","message":"Pairing successful"}`), &paired); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
		if s := fmt.Sprintf(format, paired); strings.Contains(s, "zc_secret") {
			t.Errorf("%s shows the token: %s", format, s)
		}
		if s := fmt.Sprintf(format, &paired); strings.Contains(s, "zc_secret") {
			t.Errorf("%s of a pointer shows the token: %s", format, s)
		}
	}
}

func TestPairingACodeBuysATokenThatLaterCallsUse(t *testing.T) {
	g, _ := gateway(t)
	g.public("POST", "/api/pair", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"code":"123456","device_name":"sdk","device_type":null}`)
		reply(`{"paired":true,"persisted":true,"token":"zc_new","message":"Pairing successful"}`)(w, r)
	})
	g.public("GET", "/api/devices", func(w http.ResponseWriter, r *http.Request) {
		wantHeader(t, r, "Authorization", "Bearer zc_new")
		reply(`{"devices":[{"id":"d1","name":"sdk","device_type":null,"paired_at":"t",
			"last_seen":"t","ip_address":"127.0.0.1"}],"count":1}`)(w, r)
	})

	ctx := context.Background()
	anonymous := must[*Client](t)(New(g.URL))
	paired := must[*Paired](t)(anonymous.Pairing().Pair(ctx, "123456", "sdk", ""))
	client := anonymous.WithToken(paired.Token)
	if got := must[[]Device](t)(client.Pairing().Devices(ctx)); got[0].ID != "d1" {
		t.Errorf("devices: %+v", got)
	}
}

func TestPairingWrongCodesAreRefusedWithTheGatewaysReason(t *testing.T) {
	g, client := gateway(t)
	g.public("POST", "/api/pair", replyStatus(400, `{"paired":false,"error":"Invalid or expired pairing code"}`))
	g.public("POST", "/pair", func(w http.ResponseWriter, r *http.Request) {
		wantHeader(t, r, "X-Pairing-Code", "000000")
		replyStatus(429, `{"error":"Too many pairing requests. Please retry later.","retry_after":60}`)(w, r)
	})

	ctx := context.Background()
	_, err := client.Pairing().Pair(ctx, "1", "", "")
	if StatusCode(err) != 400 || !strings.Contains(err.Error(), "Invalid or expired") {
		t.Errorf("pair: %v", err)
	}
	if _, err := client.Pairing().PairLegacy(ctx, "000000"); !IsRateLimited(err) {
		t.Errorf("pair legacy: %v", err)
	}
}

func TestPairingCodesAreMintedAndDevicesManaged(t *testing.T) {
	g, client := gateway(t)
	g.public("GET", "/pair/code", reply(`{"success":true,"pairing_required":true,"pairing_code":null}`))
	g.on("POST", "/api/pairing/initiate", reply(`{"pairing_code":"654321","message":"New pairing code generated"}`))
	g.on("GET", "/admin/paircode", reply(`{"success":true,"pairing_required":true,"pairing_code":"111111"}`))
	g.on("POST", "/admin/paircode/new", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "rotate", "all")
		reply(`{"success":true,"pairing_required":true,"pairing_code":"222222"}`)(w, r)
	})
	g.on("DELETE", "/api/devices/d1", reply(`{"message":"Device revoked","device_id":"d1"}`))
	g.on("POST", "/api/devices/d2/token/rotate", reply(`{"device_id":"d2","pairing_code":"333333","message":""}`))
	g.on("POST", "/api/devices/me/capabilities", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"capabilities":["chat"]}`)
		reply(`{"message":"Capabilities updated","capabilities":["chat"]}`)(w, r)
	})

	ctx, pairing := context.Background(), client.Pairing()
	if got := must[*PairingCode](t)(pairing.InitialCode(ctx)); got.PairingCode != nil {
		t.Errorf("initial code: %+v", got)
	}
	if got := must[string](t)(pairing.NewCode(ctx)); got != "654321" {
		t.Errorf("new code: %s", got)
	}
	if got := must[*PairingCode](t)(pairing.AdminCode(ctx)); got.PairingCode == nil || *got.PairingCode != "111111" {
		t.Errorf("admin code: %+v", got)
	}
	if got := must[*PairingCode](t)(pairing.AdminNewCode(ctx, "all")); got.PairingCode == nil || *got.PairingCode != "222222" {
		t.Errorf("admin new code: %+v", got)
	}
	if err := pairing.RevokeDevice(ctx, "d1"); err != nil {
		t.Fatal(err)
	}
	if got := must[string](t)(pairing.RotateDeviceToken(ctx, "d2")); got != "333333" {
		t.Errorf("rotate: %s", got)
	}
	if got := must[[]string](t)(pairing.SetCapabilities(ctx, []string{"chat"})); !slices.Equal(got, []string{"chat"}) {
		t.Errorf("capabilities: %v", got)
	}
}
