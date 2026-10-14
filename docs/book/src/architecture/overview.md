# Architecture

The module is one package, `zeroclaw`, with one file per area of the
gateway.

## The client

`Client` holds the base URL, the bearer token and two `*http.Client`s over
one transport (its connection pool): one sends through the resilience stack,
the other carries WebSocket handshakes. `zeroclaw.New` takes options
(`WithToken`, `WithTimeout`, `WithRetry`, `WithConcurrencyLimit`,
`WithRateLimit`, `WithCircuitBreaker`, `WithMiddleware`, `WithTransport`). A `*Client` is safe for concurrent use;
`WithToken` returns a copy that shares the pool and the concurrency limit.

Requests are sent through a stack of `http.RoundTripper`s (`transport.go`,
[ADR-004](./decisions/ADR-004-roundtripper-stack.md)): the caller's
middleware, an optional circuit breaker, an optional concurrency limit,
retry, an optional rate limit and a deadline, around
`http.Transport`. All HTTP goes through a few unexported helpers in
`client.go`:

- `url(segments, query)` appends **path segments**, each percent-encoded, to
  the base URL. Ids are never formatted into a path string, so an id holding
  `/` or `?` stays one segment;
- `request` attaches the bearer token and the `User-Agent`;
- `send` passes the request through the stack and turns a non-success status
  into an `*APIError`, keeping the gateway's message and JSON body;
- `call`, `get`, `text` and `callField` (which unwraps single-field
  envelopes) decode.

## Resource accessors

Each file adds an accessor to `Client` that returns a small value type
holding the client (`ConfigAPI`, `ChannelsAPI`, …) whose methods are the
endpoints. `client.Agent(alias)` also carries the agent alias. A method's
doc comment names the route it calls, which the
[endpoint map](../reference/endpoints.md) is generated from.

## Streams

Two kinds of live data are read by the caller, one value at a time (the
client starts no goroutines for them):

- **Server-Sent Events** (`Events().Subscribe(ctx)`): `events.go` splits the
  body into frames and decodes their JSON; `EventStream.Next()` returns
  `io.EOF` at the end;
- **WebSockets** (`chat.go`, on `github.com/coder/websocket`): `connect`
  opens a socket with ZeroClaw's `zeroclaw.v1` subprotocol and the bearer
  token, turning a refused handshake into an `*APIError`. The chat reads
  typed `ChatFrame`s (`ChatSession.NextFrame(ctx)`); the canvas and SOP-run
  feeds are `*Feed`s of JSON frames (`Feed.Next(ctx)`).

## Types

Response types mirror the gateway's serialized structs and `json!` literals,
field for field, and name them in their docs. Shapes that are dynamic or
versioned with ZeroClaw stay `json.RawMessage`
([ADR-002](./decisions/ADR-002-dynamic-shapes-stay-json.md)).
