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
  go test -tags live -run TestLiveEveryReadDecodes -v .
```

The write test (`live_write_test.go`) exercises every other route against a
real gateway. Each subtest creates what it needs under the `sdk-go-test`
prefix and removes it afterwards; it only runs with `ZEROCLAW_WRITE=1`, and
starts agent turns (chat, webhook, A2A: they spend model tokens) only with
`ZEROCLAW_TURNS=1`:

```sh
ZEROCLAW_URL=… ZEROCLAW_TOKEN=… ZEROCLAW_AGENT=<alias> \
  ZEROCLAW_WRITE=1 ZEROCLAW_TURNS=1 go test -tags live -run TestLiveWrites -v .
```

Subtests skip what the gateway does not offer:

| Subtest | Needs |
|---|---|
| `sops` | `[sop] sops_dir` set, and the admin routes (the engine loads SOPs on a daemon reload) |
| `admin` | the test running on the gateway host: admin routes answer loopback only. For a gateway in Docker, build the binary (`CGO_ENABLED=0 go test -c -tags live`) and run it with `docker run --network container:<gateway>` |
| `a2a` | `a2a.server.enabled = true` and the agent's `a2a.published = true` |
| `webauthn` | a ZeroClaw built with `--features webauthn` and `security.webauthn.enabled = true`. The test signs with a software ES256 authenticator: it registers, authenticates twice, checks that a replayed counter and a forged signature are refused, and deletes the key |

Never run it against a gateway you cannot afford to have changed: a run cut
short may leave `sdk-go-test` leftovers. Channel writes (sending, group
chats, relinking), `Shutdown`, `Upgrade`, `Quickstart().Apply`,
`Config().Init` and `SelectSectionItem` are not exercised live: they reach
real people or reconfigure the instance. The mock gateway covers them.
