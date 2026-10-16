# Compatibility

## ZeroClaw versions

The client targets the official ZeroClaw **v0.8.5** gateway
([zeroclaw-labs/zeroclaw](https://github.com/zeroclaw-labs/zeroclaw)), whose
router is the
[source of truth](../architecture/decisions/ADR-001-gateway-is-the-source-of-truth.md)
for every request and response shape here. It covers only routes that exist
in official ZeroClaw or are proposed there in a pull request.

Some routes exist only when the gateway is built with a feature: WebAuthn
(`webauthn`) and A2A (`a2a`). Without it they answer 404.

Response types decode leniently: fields a gateway adds are ignored, and
optional or newer fields are pointers or `omitempty`, so a nearby build still
decodes.

## Go

The minimum supported Go version is **1.23**, the `go` line of `go.mod`
([ADR-003](../architecture/decisions/ADR-003-go-floor.md)). CI tests on it
and on the latest stable release.

## Runtime

The client uses the standard library's `net/http` and `crypto/tls`, so it
needs no cgo and no system OpenSSL. WebSockets use
[`github.com/coder/websocket`](https://github.com/coder/websocket); the
optional rate limit and circuit breaker use
[`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate) and
[`github.com/sony/gobreaker/v2`](https://github.com/sony/gobreaker).
