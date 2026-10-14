package zeroclaw

// The quickstart: setting up a first agent (model provider, risk and runtime
// profiles, memory, channels and personality) in one submission.
//
// Gateway handler: crates/zeroclaw-gateway/src/api_quickstart.rs; submission
// type BuilderSubmission in crates/zeroclaw-config/src/presets.rs.
// Validation problems come back in a successful answer
// ({"kind": "errors", …}), not as HTTP errors.

import (
	"context"
	"encoding/json"
	"net/http"
)

// QuickstartAPI holds the quickstart endpoints, from [Client.Quickstart].
type QuickstartAPI struct{ c *Client }

// Quickstart returns the first-agent quickstart endpoints (/api/quickstart).
func (c *Client) Quickstart() QuickstartAPI { return QuickstartAPI{c} }

// State returns what exists and what can be chosen: provider and channel
// types, presets, memory kinds, … (GET /api/quickstart/state).
func (a QuickstartAPI) State(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, []string{"api", "quickstart", "state"}, nil, &out)
}

// Fields returns the fields a section (model_provider, channel or
// peer_group) of type typeKey needs (POST /api/quickstart/fields).
func (a QuickstartAPI) Fields(ctx context.Context, section, typeKey string) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.call(ctx, http.MethodPost, []string{"api", "quickstart", "fields"}, nil,
		map[string]string{"section": section, "type_key": typeKey}, &out)
}

// Validate checks a submission, encoded as JSON (POST
// /api/quickstart/validate): {"kind": "ok"} or
// {"kind": "errors", "errors": […]}.
func (a QuickstartAPI) Validate(ctx context.Context, submission any) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.call(ctx, http.MethodPost, []string{"api", "quickstart", "validate"}, nil, submission, &out)
}

// Apply saves a submission (POST /api/quickstart/apply). On
// {"kind": "applied", …, "daemon_restarted": true} the gateway drops
// connections briefly while it reloads.
func (a QuickstartAPI) Apply(ctx context.Context, submission any) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.call(ctx, http.MethodPost, []string{"api", "quickstart", "apply"}, nil, submission, &out)
}

// Dismiss records that the quickstart was closed (POST
// /api/quickstart/dismiss). surface is web, tui, cli or test; lastStep may
// be "".
func (a QuickstartAPI) Dismiss(ctx context.Context, runID, surface, lastStep string) error {
	return a.c.call(ctx, http.MethodPost, []string{"api", "quickstart", "dismiss"}, nil,
		map[string]any{"run_id": runID, "surface": surface, "last_step": sopsNullable(lastStep)}, nil)
}
