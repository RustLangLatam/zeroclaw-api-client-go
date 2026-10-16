# zeroclaw-api-client-go — next release

Changes since the last tag. They become the release notes of the next version.

## What's New

- First release: the whole client surface of the ZeroClaw 0.8.5 gateway
  (HTTP API, Server-Sent Events and WebSocket chat), ported from
  [zeroclaw-api-client-rust](https://github.com/RustLangLatam/zeroclaw-api-client-rust).
- Beyond the Rust client: opt-in circuit breaker (`WithCircuitBreaker`,
  sony/gobreaker) and client-side rate limit (`WithRateLimit`,
  x/time/rate); retries with jitter that honor `Retry-After`.

## Fixes

- v0.2.0: `zeroclaw.Version` is `0.2.0`.

- v0.1.2: the README shows the official Go gopher (designed by Renée French,
  CC BY 4.0) instead of an emoji. No code change.

- v0.1.1 retracts v0.1.0, which shipped a file that does not belong to the
  module. The code is the same.

## Breaking Changes

None.
