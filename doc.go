// Package zeroclaw is a typed, resilient client for the ZeroClaw gateway
// (https://github.com/zeroclaw-labs/zeroclaw): its HTTP API, its
// Server-Sent Events stream and its WebSocket chat.
//
// A [Client] holds the gateway URL and the bearer token; each resource
// group is reached through an accessor:
//
//	client, err := zeroclaw.New("http://127.0.0.1:42617", zeroclaw.WithToken("zc_…"))
//	if err != nil { … }
//	channels, err := client.Channels().List(ctx)
//	port, err := client.Config().Get(ctx, "gateway.port")
//
// Every HTTP request goes through a transport stack: an answer timeout that
// never cuts a stream, retries with jittered backoff for idempotent calls
// that honor Retry-After, and, opt-in, a circuit breaker
// ([WithCircuitBreaker]), a client-side rate limit ([WithRateLimit]), a
// concurrency limit ([WithConcurrencyLimit]) and any [Middleware] of your
// own ([WithMiddleware]).
//
// Errors are typed: a gateway refusal is an [*APIError] that keeps its
// status and the gateway's own message; [IsNotFound], [IsUnauthorized],
// [IsRateLimited] and [IsTransient] classify any error. Shapes the gateway
// builds dynamically (config sections, SOP definitions, log events) are
// [encoding/json.RawMessage].
//
// Each method's documentation starts with the route it calls. The SDK
// targets the ZeroClaw 0.8.5 gateway; see docs/ for the endpoint map and
// the builds it is verified against.
//
// This is an independent, community-maintained client. It is not
// affiliated with or endorsed by ZeroClaw Labs.
package zeroclaw
