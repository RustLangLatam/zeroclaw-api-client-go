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
