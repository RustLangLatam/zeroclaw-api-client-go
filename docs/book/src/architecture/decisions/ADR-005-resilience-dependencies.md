# ADR-005: Proven packages for rate limiting and circuit breaking

## Status

Accepted.

## Context

Applications embed the client in services that call the gateway all day.
Two failure modes are not covered by timeouts and retries alone:

- a gateway that is down, or restarting in a loop, keeps receiving every
  call (each with its retries) and every caller waits for its timeout;
- a busy application sends faster than the gateway's limits and turns
  into a stream of 429s.

Both have well-known answers (a circuit breaker, a token bucket) whose
details are easy to get subtly wrong: half-open trial calls, generation
counting, bursts, reservations cancelled when a context ends. Writing them
again inside the client would be one more thing to maintain and test.

## Decision

Use two small, widely used, pure-Go packages, each behind an opt-in option:

- [`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate),
  maintained by the Go team, for `WithRateLimit`: a token bucket every
  attempt waits on, inside the retry loop;
- [`github.com/sony/gobreaker/v2`](https://github.com/sony/gobreaker) for
  `WithCircuitBreaker`: a breaker outside the retry loop, so one call counts
  once.

Retry jitter and `Retry-After` stay in the client: they are a few lines of
the standard library (`math/rand/v2`, `http.ParseTime`).

`hashicorp/go-retryablehttp` was considered and not taken: it wraps
`http.Client` with its own request type and retry rules, while the client
needs retries that never repeat a `POST`, never cut a stream, and compose
with the rest of the `RoundTripper` stack ([ADR-004](./ADR-004-roundtripper-stack.md)).

## Consequences

- Three dependencies instead of one; all build on the Go 1.23 floor
  ([ADR-003](./ADR-003-go-floor.md)) and are checked by `govulncheck` in CI.
- The breaker's settings are gobreaker's own `Settings` type, so its
  documentation applies as is.
- Neither is active unless the application turns it on: the default
  behavior is the Rust client's.
