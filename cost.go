package zeroclaw

// Model usage and its cost.
//
// Gateway handler: crates/zeroclaw-gateway/src/api.rs (/api/cost); types in
// crates/zeroclaw-config/src/cost/types.rs.

import (
	"context"
	"net/http"
	"net/url"
)

// CostAPI holds the cost endpoint, from [Client.Cost].
type CostAPI struct{ c *Client }

// Cost returns the usage and cost endpoint (/api/cost).
func (c *Client) Cost() CostAPI { return CostAPI{c} }

// Summary returns the usage in [from, to) (RFC 3339; unbounded when
// empty), broken down by model and by agent (GET /api/cost).
func (a CostAPI) Summary(ctx context.Context, from, to string) (*CostSummary, error) {
	return a.fetch(ctx, params("from", from, "to", to))
}

// Agent returns one agent's session and daily figures (GET
// /api/cost?agent=); the gateway ignores a time window here.
func (a CostAPI) Agent(ctx context.Context, agent string) (*CostSummary, error) {
	return a.fetch(ctx, url.Values{"agent": {agent}})
}

func (a CostAPI) fetch(ctx context.Context, q url.Values) (*CostSummary, error) {
	return callField[*CostSummary](ctx, a.c, http.MethodGet, []string{"api", "cost"}, q, nil, "cost")
}

// CostSummary holds usage totals.
type CostSummary struct {
	// SessionCostUSD is the cost of the current session, USD.
	SessionCostUSD float64 `json:"session_cost_usd"`
	// DailyCostUSD is the cost today, USD.
	DailyCostUSD float64 `json:"daily_cost_usd"`
	// MonthlyCostUSD is the cost this month, USD.
	MonthlyCostUSD float64 `json:"monthly_cost_usd"`
	// TotalTokens is the tokens in the window.
	TotalTokens uint64 `json:"total_tokens"`
	// RequestCount is the model calls in the window.
	RequestCount int `json:"request_count"`
	// ByModel is the usage by model.
	ByModel map[string]ModelStats `json:"by_model,omitempty"`
	// ByAgent is the usage by agent.
	ByAgent map[string]AgentCostStats `json:"by_agent,omitempty"`
}

// ModelStats is one model's usage.
type ModelStats struct {
	// Model is the model.
	Model string `json:"model"`
	// CostUSD is the cost, USD.
	CostUSD float64 `json:"cost_usd"`
	// InputTokens is the input tokens.
	InputTokens uint64 `json:"input_tokens"`
	// CachedInputTokens is the cached input tokens.
	CachedInputTokens uint64 `json:"cached_input_tokens"`
	// OutputTokens is the output tokens.
	OutputTokens uint64 `json:"output_tokens"`
	// TotalTokens is all tokens.
	TotalTokens uint64 `json:"total_tokens"`
	// RequestCount is the calls.
	RequestCount int `json:"request_count"`
}

// AgentCostStats is one agent's usage.
type AgentCostStats struct {
	// AgentAlias is the agent.
	AgentAlias string `json:"agent_alias"`
	// CostUSD is the cost, USD.
	CostUSD float64 `json:"cost_usd"`
	// TotalTokens is all tokens.
	TotalTokens uint64 `json:"total_tokens"`
	// InputTokens is the input tokens.
	InputTokens uint64 `json:"input_tokens"`
	// OutputTokens is the output tokens.
	OutputTokens uint64 `json:"output_tokens"`
	// CachedInputTokens is the cached input tokens.
	CachedInputTokens uint64 `json:"cached_input_tokens"`
	// RequestCount is the calls.
	RequestCount int `json:"request_count"`
}
