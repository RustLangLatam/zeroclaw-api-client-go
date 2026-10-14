---
id: ADR-002
title: Dynamic shapes stay JSON
date: 2026-10-13
status: accepted
relates-to:
  - sops.go
  - config.go
---

# ADR-002: Dynamic Shapes Stay JSON

## Context

Some gateway payloads are large, change with every ZeroClaw release, or
depend on the configuration: the whole-config snapshot and its JSON Schema,
SOP definitions, graphs and run overlays, quickstart submissions, skill front
matter, log and event frames, WebAuthn ceremonies.

## Decision

Those stay `json.RawMessage` in the client (an `Event` is a
`map[string]json.RawMessage`), and requests that send them take `any`.
Stable, small shapes (channels, sessions, cron jobs, memory entries, cost,
devices, …) are typed structs. Typed structs decode leniently: unknown
fields are ignored and optional or newer ones are pointers or `omitempty`.

## Consequences

- A ZeroClaw upgrade that reshapes an SOP or a config section does not break
  the client.
- Callers of the JSON-shaped calls decode the value themselves
  (`json.Unmarshal` into a struct or a map of their own); the docs of each
  call list its top-level fields.
- A shape can move from `json.RawMessage` to a struct later, as a breaking
  change, when it has stabilized upstream.
