package zeroclaw

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
)

// Sentinel errors. Match them with [errors.Is].
var (
	// ErrInvalidURL means the gateway URL, or a URL built from it, is not
	// valid.
	ErrInvalidURL = errors.New("invalid gateway URL")

	// ErrTimeout means the gateway did not answer within the client's
	// timeout (after any retries). A deadline or cancellation of the
	// caller's own context is returned as the context's error instead.
	ErrTimeout = errors.New("the gateway did not answer in time")

	// ErrDecode means the gateway answered successfully but the body did
	// not have the expected shape.
	ErrDecode = errors.New("unexpected gateway response")

	// ErrWebSocket means a WebSocket connection to the gateway failed.
	ErrWebSocket = errors.New("websocket error")

	// ErrCircuitOpen means the circuit breaker ([WithCircuitBreaker])
	// refused the call because the gateway kept failing; it was not sent.
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrRateLimited means the client's own rate limit ([WithRateLimit])
	// could not let the call through before the caller's deadline; it was
	// not sent.
	ErrRateLimited = errors.New("client rate limit exceeded")
)

// APIError is a gateway answer with a non-success status.
//
// Message is the gateway's own explanation (its "error" or "message" field)
// when the body carries one, otherwise the body text; Body keeps the JSON
// body when there was one, for fields beyond the message.
type APIError struct {
	// Status is the HTTP status code.
	Status int
	// Message is the human-readable reason given by the gateway.
	Message string
	// Body is the JSON error body, when the gateway sent one.
	Body json.RawMessage
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gateway answered %d: %s", e.Status, e.Message)
}

// TurnError is an agent turn on the chat that ended in an "error" frame, or
// was aborted (Code "ABORTED").
type TurnError struct {
	// Code is the stable code, e.g. "PROVIDER_ERROR" or "ABORTED"; empty
	// when the gateway sent none.
	Code string
	// Message says what went wrong.
	Message string
}

func (e *TurnError) Error() string { return "agent turn failed: " + e.Message }

// StatusCode is the HTTP status of an [*APIError] in err's chain, or 0.
func StatusCode(err error) int {
	var api *APIError
	if errors.As(err, &api) {
		return api.Status
	}
	return 0
}

// IsNotFound reports whether the gateway answered 404 (the resource does
// not exist).
func IsNotFound(err error) bool { return StatusCode(err) == 404 }

// IsUnauthorized reports whether the gateway refused the credentials (401)
// or the action (403).
func IsUnauthorized(err error) bool {
	s := StatusCode(err)
	return s == 401 || s == 403
}

// IsRateLimited reports whether the gateway rate-limited the call (429), or
// the client's own rate limit held it back ([ErrRateLimited]).
func IsRateLimited(err error) bool { return StatusCode(err) == 429 || errors.Is(err, ErrRateLimited) }

// IsTransient reports whether the call may work if made again later: no
// connection, a connection dropped before the answer, a timeout, or a "try
// again" status (408, 429, 502, 503, 504). The client already retried
// idempotent calls before returning it.
func IsTransient(err error) bool {
	if s := StatusCode(err); s != 0 {
		return isRetryableStatus(s)
	}
	// The client's own refusals pass with time, but retrying them inside
	// the call would not help.
	return errors.Is(err, ErrCircuitOpen) || errors.Is(err, ErrRateLimited) || isRetryableError(err)
}

// newAPIError builds an [*APIError] from a failed response's status and body.
func newAPIError(status int, text []byte) *APIError {
	e := &APIError{Status: status}
	if json.Valid(text) {
		e.Body = json.RawMessage(bytes.Clone(text))
		var fields map[string]json.RawMessage
		if json.Unmarshal(text, &fields) == nil {
			m, ok := fields["error"]
			if !ok {
				m, ok = fields["message"]
			}
			if ok {
				var s string
				if json.Unmarshal(m, &s) == nil {
					e.Message = s
				} else {
					e.Message = string(m)
				}
				return e
			}
		}
	}
	e.Message = strings.TrimSpace(string(text))
	return e
}

// isRetryableStatus reports whether a status asks the client to try again
// later.
func isRetryableStatus(status int) bool {
	switch status {
	case 408, 429, 502, 503, 504:
		return true
	}
	return false
}

// isRetryableError reports whether a failure is one a retry may fix: no
// connection, no answer in time, or a connection the gateway dropped before
// answering (it does while it restarts).
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrTimeout) {
		return true
	}
	if errors.Is(err, syscall.ECONNREFUSED) || isConnectionLost(err) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// isConnectionLost reports whether err comes from a connection that closed
// or reset before the answer arrived.
func isConnectionLost(err error) bool {
	return errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}
