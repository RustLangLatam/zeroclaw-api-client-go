# Errors

Every call returns an `error` as its last result. Match it with
`errors.Is` and `errors.As`:

| Error | When |
|---|---|
| `*zeroclaw.APIError{Status, Message, Body}` | The gateway answered with a non-success status. `Message` is its explanation (the `error` or `message` field, or the body text); `Body` keeps the JSON error body as `json.RawMessage`. |
| any other error, wrapping the `net/http` one | No HTTP answer: connection refused, TLS, a broken connection. Its text starts with `request to the gateway failed`. |
| `zeroclaw.ErrTimeout` | The gateway did not answer in time, after any retries. |
| `ctx.Err()` | Your context was cancelled or reached its deadline first. |
| your middleware's error, wrapped | A [middleware](./resilience.md#your-own-middleware) you added refused the request. |
| `zeroclaw.ErrDecode` (wrapped) | A successful answer did not have the expected shape. |
| `zeroclaw.ErrWebSocket` (wrapped) | A WebSocket failed after its handshake. |
| `*zeroclaw.TurnError{Code, Message}` | A chat turn ended in an `error` frame, or was aborted (`ABORTED`). |
| `zeroclaw.ErrInvalidURL` (wrapped) | The base URL is not an absolute `http`/`https` URL. |

`zeroclaw.StatusCode(err)`, `zeroclaw.IsNotFound(err)`,
`zeroclaw.IsUnauthorized(err)` (401 and 403), `zeroclaw.IsRateLimited(err)`
(429) and `zeroclaw.IsTransient(err)` (no connection, a timeout, or 408,
429, 502, 503, 504: worth trying again later) answer the usual questions on
any error. Idempotent calls were already retried before a transient error
reaches you; see [Resilience](./resilience.md).

## The gateway's error bodies

The gateway does not use one error shape, and the client reads all of them
into `Message`:

- `{"error": "…"}`, most routes, sometimes with `retry_after` (seconds) on
  429;
- `{"code": "…", "message": "…", "path": "…", "op_index": n}` on
  configuration routes (`path_not_found`, `value_type_mismatch`,
  `config_changed_externally`, …) and uploads;
- `{"channel": "…", "error": "…"}` on channel actions;
- plain text when the request is refused before its handler runs (a
  malformed body, a body over 64 KiB, the device endpoints' 401), and an
  empty body on the gateway's request timeout (408).

Read further fields from `Body`:

```go
_, err := client.Config().Set(ctx, "gateway.port", "not a number", "")
var apiErr *zeroclaw.APIError
if errors.As(err, &apiErr) && apiErr.Status == 400 {
	var body struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(apiErr.Body, &body) == nil {
		fmt.Println(body.Code) // value_type_mismatch
	}
}
```
