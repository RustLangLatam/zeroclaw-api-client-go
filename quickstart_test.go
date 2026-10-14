package zeroclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestQuickstartWalksThrough(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/quickstart/state", reply(`{"quickstart_completed":false}`))
	g.on("POST", "/api/quickstart/fields", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"section":"channel","type_key":"whatsapp"}`)
		reply(`{"fields":[]}`)(w, r)
	})
	g.on("POST", "/api/quickstart/validate", reply(`{"kind":"ok"}`))
	g.on("POST", "/api/quickstart/apply", reply(`{"kind":"applied","daemon_restarted":true}`))
	g.on("POST", "/api/quickstart/dismiss", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"run_id":"q1","surface":"test","last_step":null}`)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx, quickstart := context.Background(), client.Quickstart()
	submission := map[string]any{"agent": map[string]any{"name": "helper"}}
	if got := must[json.RawMessage](t)(quickstart.State(ctx)); !sameJSON(t, got, []byte(`{"quickstart_completed":false}`)) {
		t.Errorf("state: %s", got)
	}
	var fields struct{ Fields []any }
	json.Unmarshal(must[json.RawMessage](t)(quickstart.Fields(ctx, "channel", "whatsapp")), &fields)
	if fields.Fields == nil {
		t.Errorf("fields: %+v", fields)
	}
	var answer struct{ Kind string }
	json.Unmarshal(must[json.RawMessage](t)(quickstart.Validate(ctx, submission)), &answer)
	if answer.Kind != "ok" {
		t.Errorf("validate: %+v", answer)
	}
	json.Unmarshal(must[json.RawMessage](t)(quickstart.Apply(ctx, submission)), &answer)
	if answer.Kind != "applied" {
		t.Errorf("apply: %+v", answer)
	}
	if err := quickstart.Dismiss(ctx, "q1", "test", ""); err != nil {
		t.Fatal(err)
	}
}
