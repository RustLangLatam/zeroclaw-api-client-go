---
id: ADR-004
title: Requests go through a RoundTripper stack
date: 2026-10-13
status: accepted
relates-to:
  - transport.go
  - docs/book/src/guides/resilience.md
---

# ADR-004: Requests Go Through A RoundTripper Stack

## Context

A ZeroClaw gateway restarts on reloads and some configuration changes,
refusing connections for seconds; it answers 502 and 503 while a channel
reconnects and 429 when rate-limited; some calls take minutes. Applications
need timeouts, retries and back-pressure, and often their own cross-cutting
concerns (metrics, tracing). Hand-written loops in each client method would
repeat that logic and could not be extended by callers.

## Decision

Every HTTP request is sent through a stack of `http.RoundTripper`s built
once per client: the caller's `Middleware`, an optional global concurrency
limit, a retry policy and a deadline, around `http.Transport` (or the
caller's transport, `WithTransport`). `http.RoundTripper` is the standard
library's own extension point, so the stack needs no framework, and a
`Middleware` is just `func(http.RoundTripper) http.RoundTripper`: any
existing RoundTripper wrapper (tracing, metrics) plugs in as is.

- **Retry** only for idempotent methods and failures a retry can fix, with
  exponential backoff; never a `POST` or `PATCH`. A request body is replayed
  through `GetBody`.
- **Deadline** is a small RoundTripper of the client's own rather than
  `http.Client.Timeout`: it bounds the response headers only, so streams are
  not cut, and honors a per-request timeout for long gateway calls. The
  caller's `context.Context` still bounds the whole call.
- **Concurrency limit** is a buffered channel used as a semaphore, shared by
  every copy of the client; waiting for a slot honors the request's context.
- **No background goroutines:** the stack runs on the caller's goroutine.
  Callers who want a queue or a rate limiter add it as middleware.

## Consequences

- Resilience is configured, not coded per call, and covered by table tests
  of each decision (method, status, attempt) and by tests against the mock
  gateway (`resilience_test.go`).
- Callers reuse any `http.RoundTripper` middleware through `WithMiddleware`.
- WebSockets are outside the stack: a dropped chat or feed is the caller's to
  reopen.
