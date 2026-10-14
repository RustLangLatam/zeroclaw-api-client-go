package zeroclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Version is the SDK version, sent in the User-Agent header.
const Version = "0.1.0"

// DefaultTimeout is how long a request waits for the gateway's answer (its
// headers: a streamed body is not bounded) unless [WithTimeout] says
// otherwise. The gateway's own limit is 30 s; calls that do longer work set
// their own.
const DefaultTimeout = 60 * time.Second

// connectTimeout is how long opening a connection may take.
const connectTimeout = 10 * time.Second

// Client is a connection to one ZeroClaw gateway.
//
// It is safe for concurrent use; copies made with [Client.WithToken] share
// the connection pool and the concurrency limit. Resource groups are reached
// through accessors such as [Client.Cron] or [Client.Config].
type Client struct {
	http    *http.Client // sends through the stack
	ws      *http.Client // WebSocket handshakes, outside the stack
	base    *url.URL
	token   string
	timeout time.Duration
}

// String describes the client without its token, so it never reaches logs.
func (c *Client) String() string {
	return fmt.Sprintf("zeroclaw.Client{base: %s, authenticated: %t}", c.base, c.token != "")
}

// GoString hides the token from %#v too.
func (c *Client) GoString() string { return c.String() }

// Option configures a [Client].
type Option func(*settings)

type settings struct {
	token       string
	timeout     time.Duration
	retry       RetryPolicy
	concurrency int
	middleware  []Middleware
	transport   http.RoundTripper
}

// WithToken sets the bearer token sent with every request (obtained by
// pairing, see [PairingAPI]). Without one, only public endpoints answer.
func WithToken(token string) Option { return func(s *settings) { s.token = token } }

// WithTimeout sets how long a request waits for the gateway's answer
// (default [DefaultTimeout]). It bounds the headers only, so the event
// stream is not cut; long gateway calls (a manual cron run) use their own,
// longer limit. Zero or less waits without limit.
func WithTimeout(d time.Duration) Option { return func(s *settings) { s.timeout = d } }

// WithRetry sets how failed idempotent requests are retried (default
// [DefaultRetryPolicy]). [NoRetries] turns retries off.
func WithRetry(p RetryPolicy) Option { return func(s *settings) { s.retry = p } }

// WithConcurrencyLimit lets at most max requests wait for the gateway's
// answer at once, shared by every copy of the client; the rest wait their
// turn. A request holds its slot until the answer's headers arrive, retries
// and their backoff included; reading a body (an open event stream) holds
// none.
func WithConcurrencyLimit(max int) Option { return func(s *settings) { s.concurrency = max } }

// WithMiddleware wraps every HTTP request in middleware of your own
// (metrics, tracing, request signing, a circuit breaker, …). Middleware
// added first is outermost: it sees the request first and the response
// last, around the client's own retry and timeout. An error it returns is
// returned wrapped; [errors.Is] and [errors.As] reach it.
func WithMiddleware(m ...Middleware) Option {
	return func(s *settings) { s.middleware = append(s.middleware, m...) }
}

// WithTransport uses an existing transport (its proxy, TLS and connection
// settings apply); the client's timeout and retry still wrap it. It also
// carries the WebSocket handshakes, so it must support HTTP/1.1 upgrades
// ([*http.Transport] does).
func WithTransport(rt http.RoundTripper) Option { return func(s *settings) { s.transport = rt } }

// New returns a client for the gateway at baseURL (e.g.
// "http://127.0.0.1:42617"). It fails with [ErrInvalidURL] when baseURL is
// not an absolute http or https URL.
func New(baseURL string, opts ...Option) (*Client, error) {
	s := settings{timeout: DefaultTimeout, retry: DefaultRetryPolicy()}
	for _, o := range opts {
		o(&s)
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidURL, err)
	}
	if (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.Opaque != "" {
		return nil, fmt.Errorf("%w: %q is not an absolute http or https URL", ErrInvalidURL, baseURL)
	}
	// Segments are appended to the base path, so it must not end in "/".
	base.Path = strings.TrimRight(base.Path, "/")
	base.RawPath = ""
	base.RawQuery, base.Fragment = "", ""
	if s.transport == nil {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.DialContext = (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext
		s.transport = t
	}
	return &Client{
		http:    &http.Client{Transport: stack(s.transport, s.timeout, s.retry, s.concurrency, s.middleware)},
		ws:      &http.Client{Transport: s.transport},
		base:    base,
		token:   s.token,
		timeout: s.timeout,
	}, nil
}

// WithToken returns a client for the same gateway and connection pool with
// another bearer token, e.g. the one a pairing just returned.
func (c *Client) WithToken(token string) *Client {
	other := *c
	other.token = token
	return &other
}

// BaseURL is the gateway's base URL.
func (c *Client) BaseURL() *url.URL {
	u := *c.base
	return &u
}

// url is the URL of segments under the base, each segment percent-encoded
// (an id holding "/" or "?" stays one segment), with query appended.
func (c *Client) url(segments []string, query url.Values) *url.URL {
	u := *c.base
	var path, raw strings.Builder
	path.WriteString(c.base.Path)
	raw.WriteString(c.base.EscapedPath())
	for _, s := range segments {
		path.WriteByte('/')
		path.WriteString(s)
		raw.WriteByte('/')
		raw.WriteString(url.PathEscape(s))
	}
	u.Path, u.RawPath = path.String(), raw.String()
	u.RawQuery = query.Encode()
	return &u
}

// request builds a request to segments with the bearer token attached. A
// non-nil body is sent with contentType.
func (c *Client) request(ctx context.Context, method string, segments []string, query url.Values, body io.Reader, contentType string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.url(segments, query).String(), body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidURL, err)
	}
	if body != nil && contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", "zeroclaw-api-client-go/"+Version)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

// jsonRequest builds a request whose body is in encoded as JSON (no body
// when in is nil).
func (c *Client) jsonRequest(ctx context.Context, method string, segments []string, query url.Values, in any) (*http.Request, error) {
	if in == nil {
		return c.request(ctx, method, segments, query, nil, "")
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("encoding the request: %w", err)
	}
	return c.request(ctx, method, segments, query, bytes.NewReader(b), "application/json")
}

// send sends req, turning a non-success status into an [*APIError]. The
// caller closes the body of the response it gets.
func (c *Client) send(req *http.Request) (*http.Response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, ErrTimeout) {
			return nil, ErrTimeout
		}
		if ctxErr := req.Context().Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("request to the gateway failed: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return nil, newAPIError(resp.StatusCode, text)
}

// readBody reads and closes the body of a successful response.
func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("request to the gateway failed: reading the answer: %w", err)
	}
	return b, nil
}

// decode decodes a JSON body into out.
func decode(b []byte, out any) error {
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%w: %w", ErrDecode, err)
	}
	return nil
}

// do sends req and decodes its JSON body into out (the body is discarded
// when out is nil).
func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.send(req)
	if err != nil {
		return err
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return nil
	}
	b, err := readBody(resp)
	if err != nil {
		return err
	}
	return decode(b, out)
}

// call makes a JSON call: method on segments with in as the body (none when
// nil), decoding the answer into out (discarded when nil).
func (c *Client) call(ctx context.Context, method string, segments []string, query url.Values, in, out any) error {
	req, err := c.jsonRequest(ctx, method, segments, query, in)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

// get is a GET decoded into out.
func (c *Client) get(ctx context.Context, segments []string, query url.Values, out any) error {
	return c.call(ctx, http.MethodGet, segments, query, nil, out)
}

// text sends req and returns its body as text.
func (c *Client) text(req *http.Request) (string, error) {
	resp, err := c.send(req)
	if err != nil {
		return "", err
	}
	b, err := readBody(resp)
	return string(b), err
}

// callField makes a JSON call and decodes only one field of the answer's
// envelope (e.g. "jobs" of {"jobs": [...]}).
func callField[T any](ctx context.Context, c *Client, method string, segments []string, query url.Values, in any, name string) (T, error) {
	var envelope map[string]json.RawMessage
	var out T
	if err := c.call(ctx, method, segments, query, in, &envelope); err != nil {
		return out, err
	}
	raw, ok := envelope[name]
	if !ok {
		return out, fmt.Errorf("%w: missing field %q", ErrDecode, name)
	}
	return out, decode(raw, &out)
}

// params builds query parameters from key, value pairs, leaving out the
// pairs whose value is empty (the optional parameters that are not set).
func params(kv ...string) url.Values {
	q := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			q.Set(kv[i], kv[i+1])
		}
	}
	return q
}
