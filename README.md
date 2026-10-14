<h1 align="center"><img src="docs/assets/gopher.svg" alt="Go gopher" height="40" align="absmiddle" /> zeroclaw-api-client-go</h1>

<p align="center">
  <strong>Your ZeroClaw, from Go. Typed calls, live streams, resilient by default.</strong>
</p>

<p align="center">
  <a href="https://github.com/RustLangLatam/zeroclaw-api-client-go/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/RustLangLatam/zeroclaw-api-client-go/ci.yml?branch=main&label=build" alt="Build Status" /></a>
  <a href="https://pkg.go.dev/github.com/RustLangLatam/zeroclaw-api-client-go"><img src="https://pkg.go.dev/badge/github.com/RustLangLatam/zeroclaw-api-client-go.svg" alt="Go Reference" /></a>
  <a href="https://goreportcard.com/report/github.com/RustLangLatam/zeroclaw-api-client-go"><img src="https://goreportcard.com/badge/github.com/RustLangLatam/zeroclaw-api-client-go" alt="Go Report Card" /></a>
  <a href="https://github.com/RustLangLatam/zeroclaw-api-client-go/releases/latest"><img src="https://img.shields.io/github/v/release/RustLangLatam/zeroclaw-api-client-go?label=release" alt="Latest release" /></a>
  <a href="https://github.com/RustLangLatam/zeroclaw-api-client-go/blob/main/LICENSE-APACHE"><img src="https://img.shields.io/badge/license-MIT%20OR%20Apache%202.0-blue.svg" alt="License" /></a>
  <a href="https://github.com/RustLangLatam/zeroclaw-api-client-go/blob/main/docs/book/src/architecture/decisions/ADR-003-go-floor.md"><img src="https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go" alt="Go 1.23+" /></a>
  <a href="https://github.com/zeroclaw-labs/zeroclaw"><img src="https://img.shields.io/badge/ZeroClaw-0.8.5-black" alt="ZeroClaw 0.8.5" /></a>
</p>

<p align="center">
  <a href="https://pkg.go.dev/net/http"><img src="https://img.shields.io/badge/HTTP-net%2Fhttp-00ADD8" alt="net/http" /></a>
  <a href="https://github.com/coder/websocket"><img src="https://img.shields.io/badge/WebSocket-coder%2Fwebsocket-475569" alt="coder/websocket" /></a>
  <a href="https://pkg.go.dev/encoding/json"><img src="https://img.shields.io/badge/serialization-encoding%2Fjson-c2410c" alt="encoding/json" /></a>
  <a href="https://go.dev/blog/vuln"><img src="https://img.shields.io/badge/supply%20chain-govulncheck-15803d" alt="govulncheck" /></a>
</p>

<p align="center">
  <a href="docs/book/src/introduction.md">Docs</a> ·
  <a href="docs/book/src/getting-started/index.md">Quick start</a> ·
  <a href="docs/book/src/guides/index.md">Guides</a> ·
  <a href="docs/book/src/guides/resilience.md">Resilience</a> ·
  <a href="docs/book/src/reference/endpoints.md">Endpoint map</a> ·
  <a href="docs/book/src/architecture/overview.md">Architecture</a> ·
  <a href="https://github.com/zeroclaw-labs/zeroclaw">ZeroClaw</a>
</p>

---

`zeroclaw-api-client-go` is a typed, resilient Go client for the
[ZeroClaw](https://github.com/zeroclaw-labs/zeroclaw) gateway: its HTTP API,
its Server-Sent Events stream and its WebSocket chat.

> An independent, community-maintained client. It is not affiliated with or
> endorsed by ZeroClaw Labs; ZeroClaw is a trademark of ZeroClaw Labs.

```sh
go get github.com/RustLangLatam/zeroclaw-api-client-go
```

```go
package main

import (
	"context"
	"fmt"
	"log"

	zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
)

func main() {
	client, err := zeroclaw.New("http://127.0.0.1:42617", zeroclaw.WithToken("zc_…"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	status, _ := client.System().Status(ctx, "")
	channels, _ := client.Channels().List(ctx)
	client.Config().Set(ctx, "agents.helper.model", "gpt-5.4-mini", "")

	chat, err := client.Chat(ctx, "helper", zeroclaw.ChatOptions{})
	if err != nil {
		log.Fatal(err)
	}
	defer chat.Close()
	reply, err := chat.Ask(ctx, "How many orders are pending?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(status.Version, len(channels), reply.FullResponse)
}
```

## Why an SDK

ZeroClaw is a self-hosted AI agent runtime. Everything an operator can do
with it (configure agents, models and channels, pair devices, run SOPs,
schedule jobs, read sessions and costs, talk to an agent) is exposed by its
gateway over HTTP and WebSockets. Calling that surface by hand from every
application that embeds ZeroClaw means repeating, in each of them:

- **the contract**: about 150 routes, a dozen response shapes for errors, a
  JSON Patch dialect for the configuration, an event stream whose frames have
  no names, a chat protocol with typed frames;
- **the edge cases**: secrets that are never returned, paths that must be
  encoded segment by segment, a gateway that drops connections while it
  reloads, routes that only exist with some build features;
- **the resilience**: timeouts that must not cut a stream, retries that must
  never repeat a write, back-pressure.

This module does it once, typed and tested, so applications talk to ZeroClaw
in terms of channels, agents and sessions instead of URLs and JSON. When the
gateway changes, one place changes.

It complements what ZeroClaw ships: the gateway's own OpenAPI document
(`/api/openapi.json`, browsable at `/api/docs`) describes part of the router,
and `zerocode` speaks ACP over `/acp`; this client covers the whole REST,
event and chat surface from Go.

## Features

- **The whole client surface** of the gateway, one accessor per area:

  | Accessor | Covers |
  |---|---|
  | `Config()` | per-field reads and writes, atomic JSON Patch, map aliases (agents, providers, channels), drift, catalogs, setup sections |
  | `Pairing()` | pairing codes, bearer tokens, paired devices |
  | `Channels()` | channel state, sending, group chats and their members, identity binding, relinking |
  | `Chat()` | agent chat over WebSocket: streamed replies, tool calls, tool approvals |
  | `Sessions()`, `Events()` | transcripts, session state, the live Server-Sent Events stream |
  | `Agent()`, `Files()`, `Skills()` | agent workspaces, personality files, shared files, image uploads, skill bundles |
  | `Cron()`, `Memory()`, `Cost()` | scheduled jobs, long-term memory, usage and cost |
  | `Sops()`, `Canvas()`, `Quickstart()` | SOP authoring, runs and approvals; canvases; the first-agent setup |
  | `System()` | health, status, metrics, diagnostics, tools, logs, upgrades, reload |
  | `Webhook()`, `WebAuthn()` | one-shot webhook and A2A messages; hardware-key ceremonies |

- **Resilient by default**, built as a stack of `http.RoundTripper`s:
  answer timeouts that never cut a stream, retries with jittered exponential
  backoff for idempotent calls only that honor `Retry-After`, and, opt-in, a
  circuit breaker ([sony/gobreaker](https://github.com/sony/gobreaker)), a
  client-side rate limit ([x/time/rate](https://pkg.go.dev/golang.org/x/time/rate))
  and a shared concurrency limit; plus your own middleware (metrics,
  tracing, signing) around every request.
- **Typed errors**: a gateway refusal is an `*APIError` that keeps its
  status, its explanation and its JSON body; `IsNotFound`, `IsUnauthorized`,
  `IsRateLimited` and `IsTransient` classify any error, and sentinels
  (`ErrTimeout`, `ErrDecode`, …) work with `errors.Is`.
- **Pull, not callbacks**: the event feed is an `EventStream` and the canvas
  and SOP-run feeds a `Feed` you read with `Next`; a chat is a session you
  pull frames from (`NextFrame`). The client starts no background work of
  its own.
- **Context everywhere**: every call takes a `context.Context` for
  cancellation and deadlines.
- **Safe by construction**: every path segment is percent-encoded, the token
  never appears in formatted output (`%v`, `%+v`, `%#v`), nothing is printed
  or logged.
- **Few, proven dependencies**: [coder/websocket](https://github.com/coder/websocket)
  for the chat, [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate)
  and [sony/gobreaker](https://github.com/sony/gobreaker) for resilience;
  everything else is the standard library.

## Technologies

| Concern | Choice |
|---|---|
| Language | Go 1.23+ |
| HTTP | `net/http` (TLS from `crypto/tls`, no cgo) |
| Middleware | `http.RoundTripper` stack: deadline, rate limit, retry (jitter, `Retry-After`), concurrency limit, circuit breaker, your own `Middleware` |
| Rate limiting | [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) token bucket |
| Circuit breaking | [sony/gobreaker/v2](https://github.com/sony/gobreaker) |
| WebSockets | [coder/websocket](https://github.com/coder/websocket) (`zeroclaw.v1` subprotocol) |
| Serialization | `encoding/json` |
| Tests | `net/http/httptest` mock gateway, an in-test WebSocket gateway, captured response vectors, table-driven tests, benchmarks |
| Docs | Go doc comments on [pkg.go.dev](https://pkg.go.dev/github.com/RustLangLatam/zeroclaw-api-client-go) and an [mdBook](https://rust-lang.github.io/mdBook/) book |
| Supply chain | [govulncheck](https://go.dev/blog/vuln), `go mod tidy -diff` |

## Architecture

```text
 your code
    │  client.Channels().Rooms(ctx, "whatsapp.main")
    ▼
 Client ── resource accessors (Config, Channels, Chat, …) ── typed requests/responses
    │
    ├─ HTTP ──► your middleware ─► circuit breaker ─► concurrency limit ─► retry ─► rate limit ─► deadline ─► http.Transport ─► gateway
    ├─ SSE ───► the same stack; the body is read as a stream of events
    └─ WebSocket ─► coder/websocket (chat, canvas and SOP-run feeds)
```

Each method's documentation names the route it calls; the
[endpoint map](docs/book/src/reference/endpoints.md) is generated from them.
See [Architecture](docs/book/src/architecture/overview.md) and the
[decision records](docs/book/src/architecture/decisions/index.md).

## Resilience

```go
client, err := zeroclaw.New("http://127.0.0.1:42617",
	zeroclaw.WithToken("zc_…"),
	zeroclaw.WithTimeout(10*time.Second), // wait for an answer at most 10 s
	zeroclaw.WithRetry(zeroclaw.RetryPolicy{MaxRetries: 5, BaseDelay: 250 * time.Millisecond, MaxDelay: 8 * time.Second}),
	zeroclaw.WithConcurrencyLimit(4),                // shared by every copy
	zeroclaw.WithRateLimit(rate.Limit(10), 20),      // 10 requests/s, bursts of 20
	zeroclaw.WithCircuitBreaker(gobreaker.Settings{}), // open after 5 failures in a row, for 30 s
	zeroclaw.WithMiddleware(func(next http.RoundTripper) http.RoundTripper { return next }),
)
```

Retries apply to `GET`, `HEAD`, `OPTIONS`, `PUT` and `DELETE` after a
connection failure, a connection dropped before the answer, a timeout, or a
408, 429, 502, 503 or 504, with jittered backoff and the gateway's
`Retry-After` honored. A `POST` or `PATCH` is never repeated. An open
breaker fails calls at once with `ErrCircuitOpen`; the rate limit fails a
wait past the deadline with `ErrRateLimited`. The
timeout bounds the wait for the answer's headers, so the event stream stays
open. See the [resilience guide](docs/book/src/guides/resilience.md).

## Examples

Runnable programs under `examples/`:

| Example | Shows |
|---|---|
| `status` | version, model and channels |
| `pairing` | exchanging a pairing code for a token |
| `config` | reading fields, listing agents, an atomic patch |
| `group_chats` | channels, their group chats and members |
| `events` | following the live event stream, reconnecting |
| `chat` | streaming an agent's reply over the WebSocket |
| `announce` | posting on a channel through a one-off cron job |
| `resilient` | timeouts, retries, rate limit, circuit breaker, concurrency limit and a custom middleware |

```sh
ZEROCLAW_URL=http://127.0.0.1:42617 ZEROCLAW_TOKEN=zc_… go run ./examples/status
```

## Testing

- **Table-driven unit tests** for every piece of logic: error mapping, URL
  encoding, SSE parsing, chat frames, retry decisions, backoff, patch
  operations, schedules.
- **HTTP-boundary tests** for every endpoint against a mock gateway: method,
  path, query, body and decoding; plus retry, timeout, concurrency and
  middleware behavior, all under the race detector.
- **Response vectors**: answers captured from a real gateway
  (`scripts/capture-vectors.py`, anonymized) must decode into the SDK's
  types and re-encode without losing a field.
- **Benchmarks** for the hot paths: `go test -run x -bench .`
- **Live tests** against a running gateway: every read decodes real
  answers, and a write suite covers the rest of the surface (config, cron,
  memory, files, skills, canvas, pairing, SOPs, admin, A2A, chat, webhooks,
  events and WebAuthn with a software authenticator), cleaning up after
  itself. See [Testing](docs/book/src/contributing/testing.md).

```sh
go test -race ./...
```

## Compatibility

Targets the ZeroClaw **0.8.5** gateway; see
[Compatibility](docs/book/src/reference/compatibility.md) for the builds it is
verified against and the routes that need a newer one.

## Documentation

- The API: [pkg.go.dev/github.com/RustLangLatam/zeroclaw-api-client-go](https://pkg.go.dev/github.com/RustLangLatam/zeroclaw-api-client-go).
- The book: `mdbook serve docs/book` — getting started, guides, reference,
  architecture and decisions.

## Other languages

The same client, with the same surface, behavior and tests:

| Language | Repository | Install |
|---|---|---|
| Rust | [zeroclaw-api-client-rust](https://github.com/RustLangLatam/zeroclaw-api-client-rust) | `cargo add zeroclaw-api-client` |
| Python | [zeroclaw-api-client-python](https://github.com/RustLangLatam/zeroclaw-api-client-python) | `pip install zeroclaw-api-client` |

## Contributing

Work happens on branches with pull requests to `main`; see
[CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md).

## License

Licensed under either of [Apache License, Version 2.0](LICENSE-APACHE) or
[MIT license](LICENSE-MIT), at your option.

The Go gopher (`docs/assets/gopher.svg`) was designed by
[Renée French](https://reneefrench.blogspot.com/) and is licensed under
[Creative Commons Attribution 4.0](https://creativecommons.org/licenses/by/4.0/);
the artwork comes from [golang/website](https://github.com/golang/website).
