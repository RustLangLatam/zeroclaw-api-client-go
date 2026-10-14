package zeroclaw

// Standard operating procedures (SOPs): authoring, graphs, runs, approvals
// and cancellation.
//
// Gateway handlers: crates/zeroclaw-gateway/src/api_sop_author.rs and
// api_sop.rs. An SOP definition (name, description, version, priority,
// execution_mode, triggers, steps, …) is large and versioned with ZeroClaw,
// so it travels as json.RawMessage; so do graphs and run overlays. Every
// route answers 503 while the SOP subsystem is disabled.

import (
	"context"
	"encoding/json"
	"net/http"
)

// SopsAPI holds the SOP endpoints, from [Client.Sops].
type SopsAPI struct{ c *Client }

// Sops returns the standard operating procedure endpoints (/api/sops).
func (c *Client) Sops() SopsAPI { return SopsAPI{c} }

// List returns every SOP definition that loads (GET /api/sops).
func (a SopsAPI) List(ctx context.Context) ([]json.RawMessage, error) {
	return callField[[]json.RawMessage](ctx, a.c, http.MethodGet, []string{"api", "sops"}, nil, nil, "sops")
}

// Get returns one SOP definition (GET /api/sops/{name}/full).
func (a SopsAPI) Get(ctx context.Context, name string) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, []string{"api", "sops", name, "full"}, nil, &out)
}

// Create creates an SOP, 409 when the name exists (POST /api/sops). sop is
// encoded as JSON.
func (a SopsAPI) Create(ctx context.Context, sop any) error {
	return a.c.call(ctx, http.MethodPost, []string{"api", "sops"}, nil, sop, nil)
}

// Save saves an SOP; steps are renumbered 1..N (PUT /api/sops/{name}).
func (a SopsAPI) Save(ctx context.Context, name string, sop any) error {
	return a.c.call(ctx, http.MethodPut, []string{"api", "sops", name}, nil, sop, nil)
}

// Delete deletes an SOP (DELETE /api/sops/{name}).
func (a SopsAPI) Delete(ctx context.Context, name string) error {
	return a.c.call(ctx, http.MethodDelete, []string{"api", "sops", name}, nil, nil, nil)
}

// Graph returns the SOP as a node graph with layout and diagnostics
// (GET /api/sops/{name}/graph).
func (a SopsAPI) Graph(ctx context.Context, name string) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, []string{"api", "sops", name, "graph"}, nil, &out)
}

// GraphDraft returns the graph of an unsaved definition
// (POST /api/sops/graph-draft).
func (a SopsAPI) GraphDraft(ctx context.Context, sop any) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.call(ctx, http.MethodPost, []string{"api", "sops", "graph-draft"}, nil, map[string]any{"sop": sop}, &out)
}

// WireDraft applies a wiring edit to an unsaved definition and returns
// {sop, graph} (POST /api/sops/wire-draft). edit is {op: "connect" |
// "disconnect", from, to, role, port?}.
func (a SopsAPI) WireDraft(ctx context.Context, sop, edit any) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.call(ctx, http.MethodPost, []string{"api", "sops", "wire-draft"}, nil, map[string]any{"sop": sop, "edit": edit}, &out)
}

// GraphLegend says what the graph's roles, pins and run states mean
// (GET /api/sops/graph-legend).
func (a SopsAPI) GraphLegend(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, []string{"api", "sops", "graph-legend"}, nil, &out)
}

// TriggerSources returns the trigger sources, their fields and the channels
// that can start an SOP (GET /api/sops/trigger-sources).
func (a SopsAPI) TriggerSources(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, []string{"api", "sops", "trigger-sources"}, nil, &out)
}

// Run starts the SOP through its manual trigger (POST /api/sops/{name}/run);
// payload is JSON text handed to the run ("" for none). It returns the run
// id (an existing run's when the SOP coalesces).
func (a SopsAPI) Run(ctx context.Context, name, payload string) (string, error) {
	return callField[string](ctx, a.c, http.MethodPost, []string{"api", "sops", name, "run"}, nil,
		map[string]any{"payload": sopsNullable(payload)}, "run_id")
}

// Runs returns active and recent runs, newest first, of one SOP when sop is
// not "" (GET /api/sops/runs).
func (a SopsAPI) Runs(ctx context.Context, sop string) ([]SopRun, error) {
	return callField[[]SopRun](ctx, a.c, http.MethodGet, []string{"api", "sops", "runs"}, params("sop", sop), nil, "runs")
}

// WatchRuns follows SOP runs live (GET /ws/sops/runs): a
// {"type": "snapshot", "runs": […]} first, then {"type": "run", "run": …}
// on every change (lagged, disabled and error frames too).
func (a SopsAPI) WatchRuns(ctx context.Context) (*Feed, error) {
	return a.c.feed(ctx, "ws", "sops", "runs")
}

// RunOverlay returns a run's state per step, with its tool calls
// (GET /api/sops/{name}/runs/{run_id}/overlay).
func (a SopsAPI) RunOverlay(ctx context.Context, name, runID string) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, []string{"api", "sops", name, "runs", runID, "overlay"}, nil, &out)
}

// Decide answers a run waiting for approval or at a checkpoint and returns
// the run overlay (POST /api/sops/{name}/runs/{run_id}/decide).
func (a SopsAPI) Decide(ctx context.Context, name, runID string, decision Decision) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.call(ctx, http.MethodPost, []string{"api", "sops", name, "runs", runID, "decide"}, nil, decision, &out)
}

// Cancel cancels a run at its next step boundary; reason may be ""
// (POST /api/sops/{name}/runs/{run_id}/cancel).
func (a SopsAPI) Cancel(ctx context.Context, name, runID, reason string) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.call(ctx, http.MethodPost, []string{"api", "sops", name, "runs", runID, "cancel"}, nil,
		map[string]any{"reason": sopsNullable(reason)}, &out)
}

// Pending returns the runs waiting for an approval or a checkpoint
// (GET /admin/sop/pending). Loopback, or remote admin with pairing.
func (a SopsAPI) Pending(ctx context.Context) ([]PendingApproval, error) {
	return callField[[]PendingApproval](ctx, a.c, http.MethodGet, []string{"admin", "sop", "pending"}, nil, nil, "pending")
}

// Approve approves a waiting run (POST /admin/sop/approve). It returns the
// outcome (resumed, already_resolved, pending_quorum (…), …).
func (a SopsAPI) Approve(ctx context.Context, runID string) (string, error) {
	return a.resolve(ctx, "approve", runID, "")
}

// Deny denies a waiting run; reason may be "" (POST /admin/sop/deny).
func (a SopsAPI) Deny(ctx context.Context, runID, reason string) (string, error) {
	return a.resolve(ctx, "deny", runID, reason)
}

func (a SopsAPI) resolve(ctx context.Context, action, runID, reason string) (string, error) {
	return callField[string](ctx, a.c, http.MethodPost, []string{"admin", "sop", action}, nil,
		map[string]any{"run_id": runID, "reason": sopsNullable(reason)}, "outcome")
}

// sopsNullable encodes "" as JSON null, as the gateway expects for an
// absent optional string.
func sopsNullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Decision is an answer to a waiting SOP run, built with [ApproveDecision],
// [DenyDecision], [AmendDecision] or [ReviseDecision]. It encodes externally
// tagged: "approve", {"deny": {"reason": …}}, {"amend": {"text": …}} or
// {"revise": {"guidance": …}}.
type Decision struct {
	// Kind is approve, deny, amend or revise.
	Kind string
	// Reason says why, for deny ("" for none).
	Reason string
	// Text is the checkpoint's new text, for amend.
	Text string
	// Guidance is how to redo the step, for revise.
	Guidance string
}

// ApproveDecision goes on.
func ApproveDecision() Decision { return Decision{Kind: "approve"} }

// DenyDecision stops the run; reason may be "".
func DenyDecision(reason string) Decision { return Decision{Kind: "deny", Reason: reason} }

// AmendDecision changes the checkpoint's text and goes on.
func AmendDecision(text string) Decision { return Decision{Kind: "amend", Text: text} }

// ReviseDecision redoes the step with guidance.
func ReviseDecision(guidance string) Decision { return Decision{Kind: "revise", Guidance: guidance} }

// MarshalJSON encodes d externally tagged, as the gateway expects.
func (d Decision) MarshalJSON() ([]byte, error) {
	var body any
	switch d.Kind {
	case "deny":
		body = map[string]any{}
		if d.Reason != "" {
			body = map[string]string{"reason": d.Reason}
		}
	case "amend":
		body = map[string]string{"text": d.Text}
	case "revise":
		body = map[string]string{"guidance": d.Guidance}
	default:
		return json.Marshal(d.Kind)
	}
	return json.Marshal(map[string]any{d.Kind: body})
}

// SopRun is a run in a listing.
type SopRun struct {
	// RunID is the run id.
	RunID string `json:"run_id"`
	// SopName is the SOP.
	SopName string `json:"sop_name"`
	// Status is pending, running, cancel_requested, waiting_approval,
	// paused_checkpoint, completed, failed or cancelled.
	Status string `json:"status"`
	// CurrentStep is the current step.
	CurrentStep uint32 `json:"current_step"`
	// TotalSteps is the number of steps in all.
	TotalSteps uint32 `json:"total_steps"`
	// StartedAt is the start (RFC 3339).
	StartedAt string `json:"started_at"`
	// CompletedAt is the end.
	CompletedAt *string `json:"completed_at,omitempty"`
	// TriggerSource is what started it.
	TriggerSource string `json:"trigger_source"`
	// Active is whether it is still running.
	Active bool `json:"active"`
}

// PendingApproval is a run waiting for an operator.
type PendingApproval struct {
	// RunID is the run id.
	RunID string `json:"run_id"`
	// SopName is the SOP.
	SopName string `json:"sop_name"`
	// Step is the step waiting.
	Step uint32 `json:"step"`
	// TotalSteps is the number of steps in all.
	TotalSteps uint32 `json:"total_steps"`
	// WaitingSince is since when.
	WaitingSince *string `json:"waiting_since,omitempty"`
	// Kind is approval or checkpoint.
	Kind string `json:"kind"`
}
