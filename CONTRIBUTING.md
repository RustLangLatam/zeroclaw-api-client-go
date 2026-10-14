# Contributing to zeroclaw-api-client-go

Thanks for your interest. Every kind of contribution helps: code, docs, bug
reports and reports of gateway changes the client does not follow yet.

All PRs target **`main`**; never push to it directly.

1. **Find the gateway side.** Every change starts from the ZeroClaw build the
   client targets ([Compatibility](docs/book/src/reference/compatibility.md)):
   the route in `crates/zeroclaw-gateway/src/lib.rs` and its handler.
2. **Branch.** `feat/*`, `fix/*` or `docs/*` from `main`.
3. **Change, test, document.** Follow [AGENTS.md](AGENTS.md) and the
   step-by-step guide in
   [docs/book/src/contributing/architecture.md](docs/book/src/contributing/architecture.md).
4. **Open the PR** with the template filled in and conventional commit titles
   (`feat: …`, `fix: …`, `docs: …`). Name no application or company that
   uses the client. Add what changed for users to `CHANGELOG-next.md`.

## Checks

```sh
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
python3 scripts/endpoint-map.py --check
mdbook build docs/book
```

Against a running gateway, the read-only live test checks that every type
decodes real answers:

```sh
ZEROCLAW_URL=http://127.0.0.1:42617 ZEROCLAW_TOKEN=zc_… \
  go test -run TestLive -v -tags live ./...
```
