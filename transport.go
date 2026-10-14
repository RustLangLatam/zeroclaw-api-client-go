package zeroclaw

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/sony/gobreaker/v2"
	"golang.org/x/time/rate"
)

// The stack every HTTP request goes through, from the caller to the network:
//
//	your middleware → circuit breaker → concurrency limit → retry → rate limit → deadline → http.Transport
//
//   - Circuit breaker: optional ([WithCircuitBreaker]); after repeated
//     failures it refuses calls at once with [ErrCircuitOpen] for a while,
//     instead of piling them on a gateway that is down.
//   - Rate limit: optional ([WithRateLimit]); every attempt, retries
//     included, waits for a token, so the client stays under the gateway's
//     limits instead of collecting 429s.
//   - Deadline: bounds the wait for the response headers, so a long
//     Server-Sent Events body is never cut. A request carrying its own
//     timeout (the client sets one on long gateway calls such as a manual
//     cron run) uses it instead of the client's.
//   - Retry: idempotent requests (GET, HEAD, OPTIONS, PUT, DELETE) are
//     retried on connection failures, timeouts and the statuses that mean
//     "try again" (408, 429, 502, 503, 504), with exponential backoff. A POST
//     or PATCH is never retried: it could apply twice. A Retry-After header
//     on a 429 or 503 is honored, up to the policy's MaxDelay.
//   - Concurrency limit: optional, shared by every copy of the client.
//
// The WebSocket chat and feeds are not HTTP requests and do not go through
// this stack.

// Middleware wraps the client's HTTP transport (metrics, tracing, request
// signing, a circuit breaker, …). See [WithMiddleware].
type Middleware func(next http.RoundTripper) http.RoundTripper

// RoundTripperFunc adapts a function to [http.RoundTripper], for writing a
// [Middleware] inline.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip calls f(r).
func (f RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// RetryPolicy says when and how often a failed idempotent request is tried
// again. The delay before retry n (0-based) is BaseDelay × 2ⁿ, capped at
// MaxDelay; with Jitter, the wait is a random point in its upper half, so
// clients that failed together do not retry together. The zero value makes
// no retries.
type RetryPolicy struct {
	// MaxRetries is how many times a request is tried again at most.
	MaxRetries int
	// BaseDelay is the wait before the first retry.
	BaseDelay time.Duration
	// MaxDelay caps the wait between retries.
	MaxDelay time.Duration
	// Jitter randomizes each wait between half and all of its delay.
	Jitter bool
}

// DefaultRetryPolicy is three retries, 200 ms then doubling, at most 5 s
// apart, with jitter.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxRetries: 3, BaseDelay: 200 * time.Millisecond, MaxDelay: 5 * time.Second, Jitter: true}
}

// NoRetries is a policy that never retries.
func NoRetries() RetryPolicy { return RetryPolicy{} }

// Delay is how long to wait before retry attempt (0-based).
func (p RetryPolicy) Delay(attempt int) time.Duration {
	d := p.BaseDelay
	for i := 0; i < attempt && d > 0 && d < p.MaxDelay; i++ {
		d *= 2
	}
	if d < 0 { // overflowed
		return p.MaxDelay
	}
	return min(d, p.MaxDelay)
}

// wait is how long to sleep before retry attempt after resp (nil after a
// transport error): the policy's delay, jittered, or the gateway's
// Retry-After when longer, never beyond MaxDelay.
func (p RetryPolicy) wait(attempt int, resp *http.Response) time.Duration {
	d := p.Delay(attempt)
	if p.Jitter && d > 1 {
		d = d/2 + rand.N(d/2+1)
	}
	if after := retryAfter(resp, time.Now()); after > d {
		d = min(after, p.MaxDelay)
	}
	return d
}

// retryAfter is the wait a 429 or 503 asks for in its Retry-After header
// (seconds or an HTTP date), or 0.
func retryAfter(resp *http.Response, now time.Time) time.Duration {
	if resp == nil || (resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable) {
		return 0
	}
	v := resp.Header.Get("Retry-After")
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if at, err := http.ParseTime(v); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

// isIdempotent reports whether repeating a request with this method is safe.
func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

// timeoutKey carries a per-request answer timeout in the request context.
type timeoutKey struct{}

// withRequestTimeout makes the deadline layer wait up to d for the answer to
// requests made with the returned context, instead of the client's timeout.
func withRequestTimeout(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, timeoutKey{}, d)
}

// deadline bounds the wait for a response's headers: the request's own
// timeout, or the client's. Elapsing is [ErrTimeout].
type deadline struct {
	next    http.RoundTripper
	timeout time.Duration
}

func (d deadline) RoundTrip(req *http.Request) (*http.Response, error) {
	limit := d.timeout
	if own, ok := req.Context().Value(timeoutKey{}).(time.Duration); ok {
		limit = own
	}
	if limit <= 0 {
		return d.next.RoundTrip(req)
	}
	ctx, cancel := context.WithCancelCause(req.Context())
	timer := time.AfterFunc(limit, func() { cancel(ErrTimeout) })
	resp, err := d.next.RoundTrip(req.WithContext(ctx))
	if !timer.Stop() && context.Cause(ctx) == ErrTimeout {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, ErrTimeout
	}
	if err != nil {
		cancel(nil)
		return nil, err
	}
	// The body outlives the deadline; its context ends when it is closed.
	resp.Body = cancelOnClose{resp.Body, cancel}
	return resp, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelCauseFunc
}

func (c cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel(nil)
	return err
}

// retry tries failed idempotent requests again, following policy.
type retry struct {
	next   http.RoundTripper
	policy RetryPolicy
}

func (r retry) RoundTrip(req *http.Request) (*http.Response, error) {
	if r.policy.MaxRetries <= 0 || !isIdempotent(req.Method) || (req.Body != nil && req.GetBody == nil) {
		return r.next.RoundTrip(req)
	}
	ctx := req.Context()
	for attempt := 0; ; attempt++ {
		try := req
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			try = req.Clone(ctx)
			try.Body = body
		}
		resp, err := r.next.RoundTrip(try)
		retryable := err == nil && isRetryableStatus(resp.StatusCode) || err != nil && isRetryableError(err)
		if !retryable || attempt >= r.policy.MaxRetries || ctx.Err() != nil {
			return resp, err
		}
		if resp != nil {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10)) // keeps the connection reusable
			resp.Body.Close()
		}
		wait := time.NewTimer(r.policy.wait(attempt, resp))
		select {
		case <-ctx.Done():
			wait.Stop()
			return nil, ctx.Err()
		case <-wait.C:
		}
	}
}

// limit lets at most cap(slots) requests wait for their answer at once.
type limit struct {
	next  http.RoundTripper
	slots chan struct{}
}

func (l limit) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case l.slots <- struct{}{}:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
	defer func() { <-l.slots }()
	return l.next.RoundTrip(req)
}

// throttle makes every attempt wait for a token of limiter.
type throttle struct {
	next    http.RoundTripper
	limiter *rate.Limiter
}

func (t throttle) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.limiter.Wait(req.Context()); err != nil {
		if ctxErr := req.Context().Err(); ctxErr != nil {
			return nil, ctxErr
		}
		// The wait would outlast the caller's deadline.
		return nil, fmt.Errorf("%w: %w", ErrRateLimited, err)
	}
	return t.next.RoundTrip(req)
}

// errFailedAnswer marks, inside the breaker, an answer that counts as a
// failure (a 5xx, 408 or 429) although the transport worked.
var errFailedAnswer = errors.New("failed answer")

// breaker refuses calls while the gateway keeps failing.
type breaker struct {
	next http.RoundTripper
	cb   *gobreaker.CircuitBreaker[*http.Response]
}

func (b breaker) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := b.cb.Execute(func() (*http.Response, error) {
		resp, err := b.next.RoundTrip(req)
		if err == nil && (resp.StatusCode >= 500 || isRetryableStatus(resp.StatusCode)) {
			return resp, errFailedAnswer
		}
		return resp, err
	})
	switch {
	case errors.Is(err, errFailedAnswer):
		return resp, nil
	case errors.Is(err, gobreaker.ErrOpenState), errors.Is(err, gobreaker.ErrTooManyRequests):
		return nil, fmt.Errorf("%w: %w", ErrCircuitOpen, err)
	}
	return resp, err
}

// breakerSettings fills what the caller left unset: the breaker trips
// after 5 failures in a row, stays open 30 s, and the caller's own
// cancellations do not count as failures.
func breakerSettings(st gobreaker.Settings) gobreaker.Settings {
	if st.Name == "" {
		st.Name = "zeroclaw"
	}
	if st.Timeout == 0 {
		st.Timeout = 30 * time.Second
	}
	if st.ReadyToTrip == nil {
		st.ReadyToTrip = func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 5 }
	}
	if st.IsSuccessful == nil {
		st.IsSuccessful = func(err error) bool {
			return err == nil || errors.Is(err, context.Canceled)
		}
	}
	return st
}

// stack builds the transport over base, outermost first: middleware in the
// order it was added, the circuit breaker, the concurrency limit, retry,
// the rate limit, deadline.
func stack(base http.RoundTripper, s *settings) http.RoundTripper {
	var rt http.RoundTripper = deadline{next: base, timeout: s.timeout}
	if s.limiter != nil {
		rt = throttle{next: rt, limiter: s.limiter}
	}
	rt = retry{next: rt, policy: s.retry}
	if s.concurrency > 0 {
		rt = limit{next: rt, slots: make(chan struct{}, s.concurrency)}
	}
	if s.breaker != nil {
		rt = breaker{next: rt, cb: gobreaker.NewCircuitBreaker[*http.Response](breakerSettings(*s.breaker))}
	}
	for i := len(s.middleware) - 1; i >= 0; i-- {
		rt = s.middleware[i](rt)
	}
	return rt
}
