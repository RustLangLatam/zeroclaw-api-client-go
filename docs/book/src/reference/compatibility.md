# Compatibility

## ZeroClaw versions

The client targets the ZeroClaw **0.8.5** gateway. It is verified against the
RustLangLatam fork's production build, `0.8.5+unified.13` (branch
`fix/whatsapp-silence-stopped-channel`), whose router is the
[source of truth](../architecture/decisions/ADR-001-gateway-is-the-source-of-truth.md)
for every request and response shape here.

Against an upstream ZeroClaw 0.8.5 build everything works except these
routes, which the fork adds; upstream answers them with its web UI fallback
or 404:

| Route | Function |
|---|---|
| `POST /api/channels/{channel}/send` | `Channels().Send` |
| `GET /api/channels/{channel}/rooms` | `Channels().Rooms` |
| `POST /api/channels/{channel}/rooms` | `Channels().CreateRoom` |
| `GET /api/channels/{channel}/rooms/{room}/members` | `Channels().RoomMembers` |
| `POST /api/channels/{channel}/rooms/{room}/invites` | `Channels().Invite` |
| `DELETE /api/channels/{channel}/rooms/{room}/members/{user}` | `Channels().RemoveMember` |

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
[`github.com/coder/websocket`](https://github.com/coder/websocket), its one
dependency.
