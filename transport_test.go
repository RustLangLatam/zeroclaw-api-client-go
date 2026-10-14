package zeroclaw

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestIdempotencyVectors(t *testing.T) {
	vectors := map[string]bool{
		"GET": true, "HEAD": true, "OPTIONS": true, "PUT": true, "DELETE": true,
		"POST": false, "PATCH": false,
	}
	for method, want := range vectors {
		if isIdempotent(method) != want {
			t.Errorf("%s: %t", method, !want)
		}
	}
}

func TestRetryableStatusVectors(t *testing.T) {
	vectors := map[int]bool{
		200: false, 204: false, 400: false, 401: false, 404: false, 408: true,
		409: false, 429: true, 500: false, 502: true, 503: true, 504: true,
	}
	for status, want := range vectors {
		if isRetryableStatus(status) != want {
			t.Errorf("%d: %t", status, !want)
		}
	}
}

func TestBackoffVectors(t *testing.T) {
	policy := RetryPolicy{MaxRetries: 10, BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}
	vectors := []struct {
		attempt int
		millis  int
	}{{0, 100}, {1, 200}, {2, 400}, {3, 800}, {4, 1000}, {9, 1000}, {40, 1000}}
	for _, v := range vectors {
		if got := policy.Delay(v.attempt); got != time.Duration(v.millis)*time.Millisecond {
			t.Errorf("attempt %d: %v", v.attempt, got)
		}
	}
	if NoRetries().Delay(3) != 0 {
		t.Error("no retries waits")
	}
	huge := RetryPolicy{BaseDelay: time.Hour, MaxDelay: 1<<63 - 1}
	if huge.Delay(100) <= 0 {
		t.Error("backoff overflowed")
	}
}

// statuses answers each status in turn and counts the calls.
type statuses struct {
	codes []int
	calls int
}

func (s *statuses) RoundTrip(r *http.Request) (*http.Response, error) {
	code := s.codes[min(s.calls, len(s.codes)-1)]
	s.calls++
	if r.Body != nil {
		io.Copy(io.Discard, r.Body)
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
}

func TestRetryDecisionVectors(t *testing.T) {
	// (method, statuses answered, retries allowed, calls made)
	vectors := []struct {
		method string
		codes  []int
		max    int
		calls  int
	}{
		{"GET", []int{503, 200}, 3, 2},
		{"GET", []int{503, 503, 503, 200}, 3, 4},
		{"GET", []int{503}, 3, 4},
		{"GET", []int{200}, 3, 1},
		{"GET", []int{500}, 3, 1},
		{"PUT", []int{429, 200}, 3, 2},
		{"DELETE", []int{502, 200}, 3, 2},
		{"POST", []int{503}, 3, 1},
		{"PATCH", []int{503}, 3, 1},
		{"GET", []int{503}, 0, 1},
	}
	for _, v := range vectors {
		inner := &statuses{codes: v.codes}
		rt := retry{next: inner, policy: RetryPolicy{MaxRetries: v.max, BaseDelay: time.Microsecond, MaxDelay: time.Microsecond}}
		req, _ := http.NewRequest(v.method, "http://gateway.test/api/x", strings.NewReader("{}"))
		resp, err := rt.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if inner.calls != v.calls {
			t.Errorf("%s %v max %d: %d calls, want %d", v.method, v.codes, v.max, inner.calls, v.calls)
		}
	}
}

func TestRetriedRequestsResendTheirBody(t *testing.T) {
	var bodies []string
	inner := RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		code := 503
		if len(bodies) == 3 {
			code = 200
		}
		return &http.Response{StatusCode: code, Body: http.NoBody, Request: r}, nil
	})
	rt := retry{next: inner, policy: RetryPolicy{MaxRetries: 3, BaseDelay: time.Microsecond, MaxDelay: time.Microsecond}}
	req, _ := http.NewRequest("PUT", "http://gateway.test/api/x", strings.NewReader(`{"a":1}`))
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	for _, b := range bodies {
		if b != `{"a":1}` {
			t.Errorf("bodies: %q", bodies)
		}
	}
}

func TestJitteredWaitsStayInTheUpperHalf(t *testing.T) {
	p := RetryPolicy{MaxRetries: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second, Jitter: true}
	for attempt := range 5 {
		full := p.Delay(attempt)
		for range 100 {
			if w := p.wait(attempt, nil); w < full/2 || w > full {
				t.Fatalf("attempt %d: %v outside [%v, %v]", attempt, w, full/2, full)
			}
		}
	}
	if !DefaultRetryPolicy().Jitter {
		t.Error("the default policy has no jitter")
	}
}

func TestRetryAfterVectors(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	answer := func(status int, header string) *http.Response {
		h := http.Header{}
		if header != "" {
			h.Set("Retry-After", header)
		}
		return &http.Response{StatusCode: status, Header: h}
	}
	vectors := []struct {
		resp *http.Response
		want time.Duration
	}{
		{answer(429, "3"), 3 * time.Second},
		{answer(503, "1"), time.Second},
		{answer(503, now.Add(2*time.Second).Format(http.TimeFormat)), 2 * time.Second},
		{answer(503, now.Add(-time.Second).Format(http.TimeFormat)), 0},
		{answer(502, "3"), 0},
		{answer(429, ""), 0},
		{answer(429, "soon"), 0},
		{answer(429, "-1"), 0},
		{nil, 0},
	}
	for _, v := range vectors {
		if got := retryAfter(v.resp, now); got != v.want {
			t.Errorf("%v: %v, want %v", v.resp, got, v.want)
		}
	}
	// Honored, but never beyond MaxDelay.
	p := RetryPolicy{BaseDelay: time.Millisecond, MaxDelay: 2 * time.Second}
	if got := p.wait(0, answer(429, "60")); got != 2*time.Second {
		t.Errorf("capped wait %v", got)
	}
}
