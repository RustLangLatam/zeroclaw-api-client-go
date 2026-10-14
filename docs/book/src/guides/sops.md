# SOPs

Standard operating procedures are step-by-step workflows the agents run,
started by triggers (manual, cron, webhook, channel messages, …). Their
definitions are large and evolve with ZeroClaw, so `client.Sops()` handles
them as `json.RawMessage`
([why](../architecture/decisions/ADR-002-dynamic-shapes-stay-json.md)).

- **Authoring:** `List`, `Get`, `Create`, `Save`, `Delete`; `Graph` returns
  an SOP as a node graph with layout and diagnostics; `GraphDraft` and
  `WireDraft` do the same for unsaved edits; `GraphLegend` and
  `TriggerSources` describe what can be used.
- **Running:** `Run(ctx, name, payload)` starts one through its manual
  trigger and returns the run id; `Runs` lists active and recent runs;
  `RunOverlay` shows a run's state per step with its tool calls; `WatchRuns`
  streams run changes live (a `*Feed`, read with `Next(ctx)` until
  `io.EOF`); `Cancel` stops a run at its next step boundary.
- **Approvals:** a run can wait for an operator. Answer it with
  `Decide(ctx, name, runID, zeroclaw.ApproveDecision())` (or
  `DenyDecision`, `AmendDecision`, `ReviseDecision`), or, on the gateway
  host, with `Pending`, `Approve` and `Deny`.

Every SOP route answers 503 while the SOP subsystem is disabled.

## Canvases and the quickstart

`client.Canvas()` publishes HTML, SVG, Markdown or text frames to a named
canvas, reads them back and streams them live (`Watch`, a `*Feed`).
`client.Quickstart()` drives the first-agent setup the dashboard offers:
`State`, `Fields`, `Validate`, `Apply`.
