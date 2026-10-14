package zeroclaw

// The transport stack at the HTTP boundary: retries, timeouts, the
// concurrency limit and custom middleware.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fastRetries(max int) Option {
	return WithRetry(RetryPolicy{MaxRetries: max, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond})
}

// sequence answers each status in turn with {"error":"busy"}, then 200.
func sequence(codes ...int) http.HandlerFunc {
	var calls atomic.Int32
	return func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1)) - 1
		if n < len(codes) {
			replyStatus(codes[n], `{"error":"busy"}`)(w, r)
			return
		}
		reply(`{"path":"agents","keys":["a"]}`)(w, r)
	}
}

func TestIdempotentCallsAreRetriedUntilTheyWork(t *testing.T) {
	g, client := gateway(t, fastRetries(3))
	g.on("GET", "/api/config/map-keys", sequence(503, 502, 429))
	keys, err := client.Config().MapKeys(context.Background(), "agents")
	if err != nil || len(keys) != 1 {
		t.Fatal(keys, err)
	}
	if n := g.calls("GET", "/api/config/map-keys"); n != 4 {
		t.Errorf("%d calls", n)
	}
}

func TestRetriesStopAtThePolicysLimit(t *testing.T) {
	g, client := gateway(t, fastRetries(2))
	g.on("GET", "/api/config/map-keys", sequence(503, 503, 503, 503, 503))
	_, err := client.Config().MapKeys(context.Background(), "agents")
	if StatusCode(err) != 503 || !IsTransient(err) {
		t.Fatal(err)
	}
	if n := g.calls("GET", "/api/config/map-keys"); n != 3 {
		t.Errorf("%d calls", n)
	}
}

func TestNonIdempotentCallsAreNeverRetried(t *testing.T) {
	g, client := gateway(t, fastRetries(3))
	g.on("POST", "/api/config/map-key", sequence(503, 503))
	_, err := client.Config().CreateMapKey(context.Background(), "agents", "a")
	if StatusCode(err) != 503 {
		t.Fatal(err)
	}
	if n := g.calls("POST", "/api/config/map-key"); n != 1 {
		t.Errorf("%d calls", n)
	}
}

func TestErrorsThatARetryCannotFixAreReturnedAtOnce(t *testing.T) {
	g, client := gateway(t, fastRetries(3))
	g.on("GET", "/api/config/map-keys", sequence(404, 404))
	if _, err := client.Config().MapKeys(context.Background(), "agents"); !IsNotFound(err) {
		t.Fatal(err)
	}
	if n := g.calls("GET", "/api/config/map-keys"); n != 1 {
		t.Errorf("%d calls", n)
	}
}

func TestASlowGatewayTimesOut(t *testing.T) {
	g, client := gateway(t, WithTimeout(100*time.Millisecond), WithRetry(NoRetries()))
	g.on("GET", "/api/config/map-keys", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
	})
	_, err := client.Config().MapKeys(context.Background(), "agents")
	if !errors.Is(err, ErrTimeout) || !IsTransient(err) {
		t.Fatalf("%v", err)
	}
}

func TestTheCallersContextIsNotATimeout(t *testing.T) {
	g, client := gateway(t, WithRetry(NoRetries()))
	g.on("GET", "/api/config/map-keys", func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := client.Config().MapKeys(ctx, "agents")
	if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrTimeout) {
		t.Fatalf("%v", err)
	}
}

func TestAnUnreachableGatewayIsATransientTransportError(t *testing.T) {
	// Nothing listens on port 9 (discard) on the loopback.
	client, _ := New("http://127.0.0.1:9", WithRetry(NoRetries()))
	_, err := client.Config().MapKeys(context.Background(), "agents")
	if err == nil || StatusCode(err) != 0 || !IsTransient(err) {
		t.Fatalf("%v", err)
	}
}

func TestTheTimeoutDoesNotCutTheEventStream(t *testing.T) {
	g, client := gateway(t, WithTimeout(200*time.Millisecond))
	g.on("GET", "/api/events", func(w http.ResponseWriter, r *http.Request) {
		wantHeader(t, r, "Accept", "text/event-stream")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		// The body arrives after the client's timeout: only the headers count.
		time.Sleep(400 * time.Millisecond)
		fmt.Fprint(w, "data: {\"type\":\"agent_end\"}\n\n")
	})
	events, err := client.Events().Subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer events.Close()
	first, err := events.Next()
	if err != nil || first.Kind() != "agent_end" {
		t.Fatal(first, err)
	}
}

func TestMiddlewareSeesEveryRequest(t *testing.T) {
	var seen atomic.Int32
	caller := func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			seen.Add(1)
			r = r.Clone(r.Context())
			r.Header.Set("X-Caller", "billing")
			return next.RoundTrip(r)
		})
	}
	g, client := gateway(t, WithMiddleware(caller))
	g.on("GET", "/api/config/map-keys", func(w http.ResponseWriter, r *http.Request) {
		wantHeader(t, r, "X-Caller", "billing")
		reply(`{"keys":["a"]}`)(w, r)
	})
	for range 2 {
		if _, err := client.Config().MapKeys(context.Background(), "agents"); err != nil {
			t.Fatal(err)
		}
	}
	if seen.Load() != 2 {
		t.Errorf("%d", seen.Load())
	}
}

func TestMiddlewareRunsOutermostFirst(t *testing.T) {
	var order []string
	named := func(name string) Middleware {
		return func(next http.RoundTripper) http.RoundTripper {
			return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
				order = append(order, name)
				return next.RoundTrip(r)
			})
		}
	}
	g, client := gateway(t, WithMiddleware(named("first"), named("second")))
	g.on("GET", "/api/config/map-keys", reply(`{"keys":[]}`))
	client.Config().MapKeys(context.Background(), "agents")
	if fmt.Sprint(order) != "[first second]" {
		t.Errorf("%v", order)
	}
}

var errCircuitOpen = errors.New("circuit open")

func TestAMiddlewareThatRefusesIsReturnedWrapped(t *testing.T) {
	open := func(http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, errCircuitOpen })
	}
	_, client := gateway(t, WithMiddleware(open))
	_, err := client.Config().MapKeys(context.Background(), "agents")
	if !errors.Is(err, errCircuitOpen) {
		t.Fatalf("%v", err)
	}
}

func TestTheConcurrencyLimitIsSharedByCopies(t *testing.T) {
	g, unlimited := gateway(t)
	g.on("GET", "/api/config/map-keys", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		reply(`{"keys":[]}`)(w, r)
	})
	limited, _ := New(g.URL, WithToken(testToken), WithConcurrencyLimit(2))
	sixCalls := func(client *Client) time.Duration {
		start := time.Now()
		var wg sync.WaitGroup
		for range 6 {
			wg.Add(1)
			go func(c *Client) {
				defer wg.Done()
				if _, err := c.Config().MapKeys(context.Background(), "agents"); err != nil {
					t.Error(err)
				}
			}(client.WithToken(testToken)) // a copy shares the limit
		}
		wg.Wait()
		return time.Since(start)
	}
	// Six calls of 100 ms: all at once without a limit, three rounds of two
	// with it.
	if d := sixCalls(unlimited); d >= 250*time.Millisecond {
		t.Errorf("unlimited: %v", d)
	}
	if d := sixCalls(limited); d < 300*time.Millisecond {
		t.Errorf("limited: %v", d)
	}
}

// restartingGateway drops the first drops connections without answering (as
// the gateway does while it restarts), then answers {"keys":[]}.
func restartingGateway(t *testing.T, drops int) (string, *atomic.Int32) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	var connections atomic.Int32
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 4096)
			conn.Read(buf)
			if int(connections.Add(1)) <= drops {
				conn.Close()
				continue
			}
			body := `{"keys":[]}`
			fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\ncontent-type: application/json\r\ncontent-length: %d\r\nconnection: close\r\n\r\n%s", len(body), body)
			conn.Close()
		}
	}()
	return "http://" + listener.Addr().String(), &connections
}

func TestAConnectionDroppedBeforeTheAnswerIsRetried(t *testing.T) {
	base, connections := restartingGateway(t, 2)
	client, _ := New(base, fastRetries(3))
	if _, err := client.Config().MapKeys(context.Background(), "agents"); err != nil {
		t.Fatal(err)
	}
	if n := connections.Load(); n != 3 {
		t.Errorf("%d connections", n)
	}
}

func TestADroppedConnectionWithoutRetriesIsTransient(t *testing.T) {
	base, _ := restartingGateway(t, 1)
	client, _ := New(base, WithRetry(NoRetries()))
	if _, err := client.Config().MapKeys(context.Background(), "agents"); err == nil || !IsTransient(err) {
		t.Fatalf("%v", err)
	}
}
