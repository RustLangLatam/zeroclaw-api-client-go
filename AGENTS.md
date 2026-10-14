# AGENTS.md - ZeroClaw API client for Go

Core instructions for AI coding assistants working in this repository. Read `docs/book/src/contributing/architecture.md` before a non-trivial change.

## Single Source Of Truth

The gateway is the source of truth for every request and response shape. This SDK mirrors the router of the ZeroClaw build it targets (`crates/zeroclaw-gateway/src/lib.rs` in ZeroClaw) and never invents fields:

1. Before adding or changing a type, find the gateway handler and the struct (or `json!` literal) it serializes, and name it in the type's doc comment.
2. Shapes the gateway builds ad hoc, or that change with configuration (config sections, SOP definitions, catalogs), stay `json.RawMessage` instead of guessed structs.
3. Optional and newer fields are pointers or `omitempty` fields, so an older or newer gateway still decodes.

## Safety And Privacy

- Never commit secrets, tokens, credentials, personal data, or real identities. Tests use invented tokens and hosts.
- The bearer token never appears in formatted output (`%v`, `%+v`, `%#v`), errors, or URLs.
- Production paths return errors. No `panic` outside tests.
- Do not keep unused production code; remove it or use it.
- A library never prints, logs, or starts goroutines that outlive a call: it returns typed errors and leaves streams to the caller.

## Working Rules

1. Read the owning file, its tests, and the matching gateway handler before editing.
2. One resource group per file (`cron.go`, `config.go`, …), reached from a `Client` accessor; path segments go through `Client.url`, never string formatting, so ids are always encoded.
3. Keep one concern per PR. Avoid unrelated cleanup and do not mix broad formatting changes with functional changes.
4. Do not add dependencies for minor convenience, speculative abstractions, or options without a concrete use. The only dependency is `github.com/coder/websocket`.
5. Every endpoint gets a test at the HTTP boundary (`<area>_test.go`, against the mock gateway of `gateway_test.go`) that checks method, path, query, body, and decoding. Logic (parsing, mapping, decisions) gets table-driven tests: a list of inputs and expected outputs, one assertion loop. New response types get a captured vector (`scripts/capture-vectors.py`).
6. Work on a non-`main` branch (`feat/*`, `fix/*`, `docs/*`), open a PR to `main`, and never push directly to `main`. Releases are tagged `vX.Y.Z` from `main`; the Go module proxy and pkg.go.dev pick the tag up.
7. Use conventional commits.
8. Name no application, company or deployment that uses the client: not in code, docs, test vectors, commits or PRs. Say "an application". Capture vectors with `ZEROCLAW_REDACT` set to your own names.

## User-Facing Text

- Code, comments, errors, and documentation are in English.
- English Markdown under `docs/book/` is the documentation source of truth; Go doc comments cover each exported identifier.

## Validation

```bash
gofmt -l .            # prints nothing
go vet ./...
staticcheck ./...
go test -race ./...
govulncheck ./...
python3 scripts/endpoint-map.py --check
```

Docs changes also build the book: `mdbook build docs/book`.
