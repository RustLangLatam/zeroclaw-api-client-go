# Testing

Four layers of tests, each catching what the others cannot:

| Kind | Where | Catches |
|---|---|---|
| Table tests | `*_test.go`, next to the logic | logic mistakes: parsing, mapping, decisions |
| HTTP boundary | `<area>_test.go`, against the mock gateway in `gateway_test.go` | wrong method, path, query or body; decoding of the documented shape |
| Response vectors | `testdata/vectors/*.json` + `vectors_test.go` | types that do not match what a real gateway sends |
| Live | `TestLive`, behind the `live` build tag | anything a real gateway does differently today |

Run them all with the race detector:

```sh
go test -race ./...
```

## Table tests

Logic is tested as a table of cases and one loop, so a new case is one line
and a failure names the case:

```go,ignore
vectors := map[int]bool{
	200: false, 400: false, 408: true, 429: true, 500: false, 503: true,
}
for status, want := range vectors {
	if isRetryableStatus(status) != want {
		t.Errorf("%d: %t", status, !want)
	}
}
```

## Response vectors

`scripts/capture-vectors.py` calls the gateway's read-only routes and saves
each answer, anonymized, as `{"route", "build", "response"}`. Phone numbers,
ids, UUIDs, tokens, IPs and e-mails are replaced; `ZEROCLAW_REDACT` replaces
the words of your own deployment (aliases, names); MCP tools and routes that
hold people's words (logs, memory, transcripts) are left out. Review the
files before committing them.

```sh
ZEROCLAW_URL=http://127.0.0.1:42617 ZEROCLAW_TOKEN=zc_… \
  ZEROCLAW_AGENT=<alias> ZEROCLAW_REDACT=<your,own,names> \
  python3 scripts/capture-vectors.py
go test -run TestEveryCapturedAnswerDecodes -v .
```

`vectors_test.go` decodes every file into the type its table names, and
re-encodes it to check nothing is lost. A vector without a type fails the
test, so a new capture is never silently untested.

## Live

Against a running gateway, the read-only live test checks that every type
decodes real answers:

```sh
ZEROCLAW_URL=http://127.0.0.1:42617 ZEROCLAW_TOKEN=zc_… \
  go test -tags live -run TestLive -v .
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
