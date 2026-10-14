# zeroclaw-api-client-go — next release

Changes since the last tag. They become the release notes of the next version.

## What's New

- First release: the whole client surface of the ZeroClaw 0.8.5 gateway
  (HTTP API, Server-Sent Events and WebSocket chat), ported from
  [zeroclaw-api-client-rust](https://github.com/RustLangLatam/zeroclaw-api-client-rust).
- Beyond the Rust client: opt-in circuit breaker (`WithCircuitBreaker`,
  sony/gobreaker) and client-side rate limit (`WithRateLimit`,
  x/time/rate); retries with jitter that honor `Retry-After`.

## Breaking Changes

None.
