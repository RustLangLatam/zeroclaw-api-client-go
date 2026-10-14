package zeroclaw

import (
	"context"
	"net/http"
	"testing"
)

func TestCostReportsByWindowAndAgent(t *testing.T) {
	g, client := gateway(t)
	cost := `{"cost":{
		"session_cost_usd": 0.0, "daily_cost_usd": 0.1, "monthly_cost_usd": 1.0,
		"total_tokens": 10, "request_count": 2,
		"by_model": { "m": { "model": "m", "cost_usd": 1.0, "input_tokens": 6,
		                     "cached_input_tokens": 0, "output_tokens": 4, "total_tokens": 10, "request_count": 2 } },
		"by_agent": {}
	}}`
	// One route serves both calls (the mock keys on method and path); the
	// query tells them apart.
	g.on("GET", "/api/cost", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("agent") {
			wantQuery(t, r, "agent", "helper")
		} else {
			wantQuery(t, r, "from", "2026-10-01T00:00:00Z")
			if r.URL.Query().Has("to") {
				t.Error("unset to is sent")
			}
		}
		reply(cost)(w, r)
	})

	ctx := context.Background()
	window := must[*CostSummary](t)(client.Cost().Summary(ctx, "2026-10-01T00:00:00Z", ""))
	if window.ByModel["m"].TotalTokens != 10 {
		t.Errorf("window %+v", window)
	}
	if agent := must[*CostSummary](t)(client.Cost().Agent(ctx, "helper")); agent.RequestCount != 2 {
		t.Errorf("agent %+v", agent)
	}
	if n := g.calls("GET", "/api/cost"); n != 2 {
		t.Errorf("%d calls", n)
	}
}
