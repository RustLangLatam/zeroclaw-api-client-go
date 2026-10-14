# Resilience

Every HTTP request goes through a stack of `http.RoundTripper`s
(`transport.go`). From your code to the network:

```text
your middleware → circuit breaker → concurrency limit → retry → rate limit → deadline → http.Transport → gateway
```

The circuit breaker and the rate limit are off unless you turn them on.

```text
with neither: your middleware → concurrency limit → retry → deadline → http.Transport
```

## Timeouts

`zeroclaw.WithTimeout` (default 60 s, `zeroclaw.DefaultTimeout`) bounds how
long a request waits for the gateway's **answer headers**. A streamed body
is not bounded, so `Events().Subscribe(ctx)` stays open for as long as the
gateway keeps it. Calls that do long work on the gateway carry their own
limit: `Cron().Run` waits up to 660 s, as the gateway allows a manual run
600 s. A zero or negative timeout waits forever. If you pass your own
transport (`zeroclaw.WithTransport`), the client's timeout and retry still
wrap it; do not wrap it in an `http.Client` with a total `Timeout` of your
own: that one applies to whole bodies and would cut the stream. Opening a
WebSocket (chat, canvas and SOP-run feeds) uses the same timeout for its
handshake.

A timeout is `zeroclaw.ErrTimeout` (`errors.Is(err, zeroclaw.ErrTimeout)`).
The `ctx` you pass bounds the whole call, retries included; when it ends
first, the call returns `ctx.Err()` (`context.Canceled`,
`context.DeadlineExceeded`), not `ErrTimeout`.

## Retries

A failed request is tried again when **both** hold:

- its method is idempotent: `GET`, `HEAD`, `OPTIONS`, `PUT`, `DELETE`. A
  `POST` (creating a cron job, sending a message, starting an SOP) or a
  `PATCH` (a config patch) is never repeated: it could apply twice;
- the failure is one a retry can fix: no connection, a connection dropped
  before the answer (the gateway does it while it restarts), a timeout, or a
  408, 429, 502, 503 or 504 answer. A 4xx that describes the request (400, 404,
  409, …) and a 500 are returned at once.

`RetryPolicy` sets how many retries and how far apart: retry `n` waits
`BaseDelay × 2ⁿ`, capped at `MaxDelay`. With `Jitter`, each wait is a random
point between half and all of that delay, so clients that failed together
(the gateway restarted under all of them) do not come back together. The
default (`zeroclaw.DefaultRetryPolicy()`) is three retries, 200 ms doubling
to 5 s (about 200, 400, 800 ms), with jitter. `zeroclaw.NoRetries()` turns
them off.

When a 429 or 503 carries a `Retry-After` header (seconds or an HTTP date),
the client waits that long instead, when it is longer than the backoff, but
never beyond `MaxDelay`.

```go
client, err := zeroclaw.New("http://127.0.0.1:42617",
	zeroclaw.WithToken("zc_…"),
	zeroclaw.WithRetry(zeroclaw.RetryPolicy{
		MaxRetries: 5,
		BaseDelay:  250 * time.Millisecond,
		MaxDelay:   8 * time.Second,
	}),
)
if err != nil {
	return err
}
```

The gateway restarts itself on `System().Reload(ctx)` and some
configuration changes; for a few seconds it refuses connections. Retries
carry reads through that window.

## Concurrency limit

`zeroclaw.WithConcurrencyLimit(n)` lets at most `n` requests wait for the
gateway's answer at once; the rest wait their turn (or until their `ctx`
ends). A request holds its slot until the answer's headers arrive, its
retries and their backoff included; reading a body afterwards (an open event
stream) holds none. Every copy of the client (`WithToken`) shares the limit,
so one setting protects the gateway from a whole application.

## Rate limit

`zeroclaw.WithRateLimit(r, burst)` keeps the client under `r` requests per
second, with bursts of up to `burst`, using a token bucket from
[`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate).
Every attempt waits for a token, retries included, so the client stays
under the gateway's own limits instead of collecting 429s. The limit is
shared by every copy of the client. A wait that would outlast the `ctx`
deadline fails at once, without sending anything, with
`zeroclaw.ErrRateLimited` (`IsRateLimited` and `IsTransient` say yes).

```go
client, err := zeroclaw.New("http://127.0.0.1:42617",
	zeroclaw.WithToken("zc_…"),
	zeroclaw.WithRateLimit(rate.Limit(10), 20), // 10 per second, bursts of 20
)
if err != nil {
	return err
}
```

## Circuit breaker

`zeroclaw.WithCircuitBreaker(settings)` stops calling a gateway that keeps
failing, with [`github.com/sony/gobreaker/v2`](https://pkg.go.dev/github.com/sony/gobreaker/v2).
A transport error, a timeout, a 5xx, a 408 or a 429 count as failures; a
404 or another answer about the request does not, and neither does the
caller cancelling its own `ctx`. After `ReadyToTrip` says so (by default 5
failures in a row) the breaker opens: calls fail at once with
`zeroclaw.ErrCircuitOpen`, without reaching the gateway, for `Timeout`
(default 30 s). Then it lets `MaxRequests` trial calls through (default 1);
if they work, it closes again. The breaker sits outside the retries, so it
counts one failure per call, not per attempt, and every copy of the client
shares it.

```go
client, err := zeroclaw.New("http://127.0.0.1:42617",
	zeroclaw.WithToken("zc_…"),
	zeroclaw.WithCircuitBreaker(gobreaker.Settings{
		Timeout: 10 * time.Second,
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("gateway circuit %s → %s", from, to)
		},
	}),
)
if err != nil {
	return err
}
```

`errors.Is(err, zeroclaw.ErrCircuitOpen)` tells a refusal of the breaker
apart from a gateway answer; `IsTransient` says yes to both.

## Your own middleware

`zeroclaw.WithMiddleware` wraps every request in any
`zeroclaw.Middleware`, a `func(next http.RoundTripper) http.RoundTripper`:
metrics, tracing (for example `otelhttp.NewTransport` from OpenTelemetry),
request signing. `zeroclaw.RoundTripperFunc` adapts a function to `http.RoundTripper`.
Middleware added first is outermost: it sees each call once, with the
client's retries inside it.

```go
client, err := zeroclaw.New("http://127.0.0.1:42617",
	zeroclaw.WithToken("zc_…"),
	zeroclaw.WithMiddleware(
		// Adds a header to every request.
		func(next http.RoundTripper) http.RoundTripper {
			return zeroclaw.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
				r = r.Clone(r.Context()) // a RoundTripper must not change its request
				r.Header.Set("X-Caller", "billing")
				return next.RoundTrip(r)
			})
		},
		// Times every call.
		func(next http.RoundTripper) http.RoundTripper {
			return zeroclaw.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
				started := time.Now()
				resp, err := next.RoundTrip(r)
				log.Printf("%s %s took %v", r.Method, r.URL.Path, time.Since(started))
				return resp, err
			})
		},
	),
)
if err != nil {
	return err
}
```

An error a middleware returns makes the call fail with it, wrapped:
`errors.Is` and `errors.As` reach it.

## What the client does not do

It starts no goroutines of its own that outlive a call: the stack runs on
the caller's goroutine (`net/http` keeps its usual connection pool), and streams are driven by whoever reads them. It does not
reconnect streams by itself either: when `Events().Subscribe` ends with
`io.EOF` (the gateway reloaded), subscribe again; the `events` example shows
the loop. The WebSocket chat is not an HTTP request and does not go through
the stack; a failed turn is reported, not retried.
