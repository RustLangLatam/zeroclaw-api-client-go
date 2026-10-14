# Getting Started

## Install

Add the module from [pkg.go.dev](https://pkg.go.dev/github.com/RustLangLatam/zeroclaw-api-client-go):

```sh
go get github.com/RustLangLatam/zeroclaw-api-client-go
```

and import it as one package:

```go
import zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
```

It needs Go 1.23 or newer
([why](../architecture/decisions/ADR-003-go-floor.md)) and brings one
dependency, `github.com/coder/websocket`.

## What you need from the gateway

- **Its URL.** The gateway listens on `127.0.0.1:42617` by default. Behind a
  reverse proxy, or with `[gateway] path_prefix`, use the full base URL
  (`https://host/zeroclaw`): the client appends routes to it.
- **A bearer token**, unless the gateway runs with pairing off. Tokens come
  from pairing: see [Connecting and pairing](./pairing.md).
- **Remote admin rights**, only for `/admin/*` calls from another host:
  those need `[gateway] allow_remote_admin = true` and pairing on.

Then make your [first calls](./first-calls.md).

The snippets in this book, past the first calls, run inside a function that
returns an `error`, with a `ctx context.Context` and a
`client *zeroclaw.Client` at hand.
