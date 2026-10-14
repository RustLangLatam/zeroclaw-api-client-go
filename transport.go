package zeroclaw

import (
	"context"
	"io"
	"net/http"
	"time"
)

// The stack every HTTP request goes through, from the caller to the network:
//
//	your middleware (WithMiddleware) → concurrency limit → retry → deadline → http.Transport
//
//   - Deadline: bounds the wait for the response headers, so a long
//     Server-Sent Events body is never cut. A request carrying its own
//     timeout (the client sets one on long gateway calls such as a manual
//     cron run) uses it instead of the client's.
//   - Retry: idempotent requests (GET, HEAD, OPTIONS, PUT, DELETE) are
//     retried on connection failures, timeouts and the statuses that mean
//     "try again" (408, 429, 502, 503, 504), with exponential backoff. A POST
//     or PATCH is never retried: it could apply twice.
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
// MaxDelay. The zero value makes no retries.
type RetryPolicy struct {
	// MaxRetries is how many times a request is tried again at most.
	MaxRetries int
	// BaseDelay is the wait before the first retry.
	BaseDelay time.Duration
	// MaxDelay caps the wait between retries.
	MaxDelay time.Duration
}

// DefaultRetryPolicy is three retries, 200 ms then doubling, at most 5 s
// apart.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxRetries: 3, BaseDelay: 200 * time.Millisecond, MaxDelay: 5 * time.Second}
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
		wait := time.NewTimer(r.policy.Delay(attempt))
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

// stack builds the transport over base, outermost first: middleware in the
// order it was added, the concurrency limit, retry, deadline.
func stack(base http.RoundTripper, timeout time.Duration, policy RetryPolicy, concurrency int, middleware []Middleware) http.RoundTripper {
	var rt http.RoundTripper = deadline{next: base, timeout: timeout}
	rt = retry{next: rt, policy: policy}
	if concurrency > 0 {
		rt = limit{next: rt, slots: make(chan struct{}, concurrency)}
	}
	for i := len(middleware) - 1; i >= 0; i-- {
		rt = middleware[i](rt)
	}
	return rt
}
