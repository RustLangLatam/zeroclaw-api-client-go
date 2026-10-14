# Contributing

Read [`AGENTS.md`](https://github.com/RustLangLatam/zeroclaw-api-client-go/blob/main/AGENTS.md)
first: it holds the rules every change follows. This page explains the most
common change, covering a gateway route.

## Adding or changing an endpoint

1. **Find the handler.** In the ZeroClaw build listed in
   [Compatibility](../reference/compatibility.md), find the route in
   `crates/zeroclaw-gateway/src/lib.rs` and read its handler: auth, query,
   body, every status it answers, and the struct or `json!` it returns.
2. **Pick the file.** Routes belong to the file of their area (`config.go`,
   `channels.go`, …). A new area gets a new file with an accessor on
   `Client`.
3. **Write the method.** It takes `ctx context.Context` first, and its doc
   comment names the route in parentheses
   (`// Run runs a job now and waits for it (POST /api/cron/{id}/run).`).
   Build the path with `Client.url` segments, never string formatting.
   Unwrap single-field envelopes (`{"jobs": […]}` returns `[]CronJob`).
4. **Write the types.** Mirror the gateway's fields; pointers or `omitempty`
   for anything optional or newer; `json.RawMessage` for dynamic shapes
   ([ADR-002](../architecture/decisions/ADR-002-dynamic-shapes-stay-json.md)).
5. **Test it.** Add the call to the area's `<area>_test.go`, against the
   mock gateway of `gateway_test.go`: assert the method, path, query and
   body the gateway expects, and decode a body copied from the handler.
   Capture a response vector for new types (see [Testing](./testing.md)).
6. **Update the docs.** Regenerate the [endpoint map](../reference/endpoints.md)
   with `python3 scripts/endpoint-map.py --write` (CI fails while it is
   stale), and update the guide of the area.

## Checks

```sh
test -z "$(gofmt -l .)"
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
python3 scripts/endpoint-map.py --check
mdbook build docs/book
```

CI runs the tests on Go 1.23 and on the latest stable release.

## Branches and pull requests

Branch from `main` (`feat/*`, `fix/*`, `docs/*`) and open a PR with the
template filled in; never push to `main` directly. Use conventional
commits. Releases are tagged `vX.Y.Z` from `main`; the Go module proxy and
pkg.go.dev pick the tag up, and `CHANGELOG-next.md` becomes the release
notes.
