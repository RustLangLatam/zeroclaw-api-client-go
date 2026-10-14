---
id: ADR-003
title: Go 1.23 floor
date: 2026-10-13
status: accepted
relates-to:
  - go.mod
  - .github/workflows/ci.yml
---

# ADR-003: Go 1.23 Floor

## Context

The client is meant to be embedded in other applications, and those often
pin older toolchains than the latest Go release. Every dependency added to
the client becomes one of theirs too.

## Decision

`go.mod` declares `go 1.23`, and CI tests on Go 1.23 and on the latest
stable release. The client has one dependency, `github.com/coder/websocket`,
for the WebSocket chat and feeds; everything else is the standard library
(`net/http`, `crypto/tls`, `encoding/json`).

## Consequences

- New dependencies, or newer versions of the current one, must build on
  1.23, or the floor moves in a recorded change.
- Standard-library APIs and language features newer than 1.23 are not used.
- A new dependency needs a reason a few lines of code cannot meet.
