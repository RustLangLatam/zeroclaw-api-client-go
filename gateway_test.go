package zeroclaw

// A mock gateway for the HTTP-boundary tests.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

// testToken is the token every test client sends.
const testToken = "zc_0000000000000000000000000000000000000000000000000000000000000000"

// mockGateway answers the routes a test registers and fails the test on
// any other request.
type mockGateway struct {
	*httptest.Server
	t      testing.TB
	mu     sync.Mutex
	routes map[string]route
	hits   map[string]int
}

type route struct {
	handler http.HandlerFunc
	public  bool
}

// gateway starts a mock gateway and a client authenticated against it.
func gateway(t testing.TB, opts ...Option) (*mockGateway, *Client) {
	t.Helper()
	g := &mockGateway{t: t, routes: map[string]route{}, hits: map[string]int{}}
	g.Server = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.Close)
	client, err := New(g.URL, append([]Option{WithToken(testToken)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return g, client
}

func (g *mockGateway) serve(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.EscapedPath()
	g.mu.Lock()
	rt, ok := g.routes[key]
	g.hits[key]++
	g.mu.Unlock()
	if !ok {
		g.t.Errorf("unexpected request %s", key)
		http.Error(w, "no such route in the mock", http.StatusNotFound)
		return
	}
	if !rt.public && r.Header.Get("Authorization") != "Bearer "+testToken {
		g.t.Errorf("%s: missing bearer token", key)
	}
	rt.handler(w, r)
}

// on answers method on path (escaped, as sent) with h, expecting the token.
func (g *mockGateway) on(method, path string, h http.HandlerFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.routes[method+" "+path] = route{handler: h}
}

// public answers method on path with h, without expecting a token.
func (g *mockGateway) public(method, path string, h http.HandlerFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.routes[method+" "+path] = route{handler: h, public: true}
}

// calls is how many times method on path was requested.
func (g *mockGateway) calls(method, path string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.hits[method+" "+path]
}

// reply answers 200 with body, a JSON document.
func reply(body string) http.HandlerFunc {
	return replyStatus(http.StatusOK, body)
}

// replyStatus answers status with body (JSON when it parses as JSON).
func replyStatus(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if json.Valid([]byte(body)) {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		io.WriteString(w, body)
	}
}

// sameJSON reports whether a and b are the same JSON value.
func sameJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Errorf("not JSON: %s", a)
		return false
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Errorf("not JSON: %s", b)
		return false
	}
	return reflect.DeepEqual(x, y)
}

// wantBody fails the test unless r's body is the JSON document want.
func wantBody(t *testing.T, r *http.Request, want string) {
	t.Helper()
	got, _ := io.ReadAll(r.Body)
	if !sameJSON(t, got, []byte(want)) {
		t.Errorf("%s %s body = %s, want %s", r.Method, r.URL.Path, got, want)
	}
}

// wantQuery fails the test unless r's query parameter key is value.
func wantQuery(t *testing.T, r *http.Request, key, value string) {
	t.Helper()
	if got := r.URL.Query().Get(key); got != value {
		t.Errorf("%s %s ?%s = %q, want %q", r.Method, r.URL.Path, key, got, value)
	}
}

// wantHeader fails the test unless r's header key is value.
func wantHeader(t *testing.T, r *http.Request, key, value string) {
	t.Helper()
	if got := r.Header.Get(key); got != value {
		t.Errorf("%s %s header %s = %q, want %q", r.Method, r.URL.Path, key, got, value)
	}
}

// must fails the test on err and returns v.
func must[T any](t *testing.T) func(v T, err error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}
