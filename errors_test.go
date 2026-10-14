package zeroclaw

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestGatewayErrorBodyVectors(t *testing.T) {
	vectors := []struct {
		status                                     int
		body, message                              string
		notFound, unauthorized, limited, transient bool
	}{
		{404, `{"error":"Cron job not found: x"}`, "Cron job not found: x", true, false, false, false},
		{400, `{"code":"value_type_mismatch","message":"expected integer","path":"gateway.port"}`, "expected integer", false, false, false, false},
		{403, `{"channel":"whatsapp.main","error":"not in send_channels"}`, "not in send_channels", false, true, false, false},
		{401, "Unauthorized — provide a token\n", "Unauthorized — provide a token", false, true, false, false},
		{429, `{"error":"Too many pairing requests.","retry_after":60}`, "Too many pairing requests.", false, false, true, true},
		{502, `{"channel":"whatsapp.main","error":"channel did not create the room"}`, "channel did not create the room", false, false, false, true},
		{503, `{"error":"SOP subsystem not enabled"}`, "SOP subsystem not enabled", false, false, false, true},
		{408, "", "", false, false, false, true},
		{413, "payload too large", "payload too large", false, false, false, false},
		{422, "Failed to deserialize the JSON body", "Failed to deserialize the JSON body", false, false, false, false},
	}
	for _, v := range vectors {
		var err error = newAPIError(v.status, []byte(v.body))
		if StatusCode(err) != v.status {
			t.Errorf("%s: status %d", v.body, StatusCode(err))
		}
		if want := fmt.Sprintf("gateway answered %d: %s", v.status, v.message); err.Error() != want {
			t.Errorf("%q: %q, want %q", v.body, err.Error(), want)
		}
		got := [4]bool{IsNotFound(err), IsUnauthorized(err), IsRateLimited(err), IsTransient(err)}
		if want := [4]bool{v.notFound, v.unauthorized, v.limited, v.transient}; got != want {
			t.Errorf("%d %s: %v, want %v", v.status, v.body, got, want)
		}
		if body := err.(*APIError).Body; body != nil && body[0] != '{' {
			t.Errorf("%s: body %s", v.body, body)
		}
		// Classification sees through wrapping.
		if StatusCode(fmt.Errorf("listing: %w", err)) != v.status {
			t.Errorf("%d: wrapped status lost", v.status)
		}
	}
}

func TestNonAPIErrorsHaveNoStatus(t *testing.T) {
	for _, err := range []error{ErrTimeout, ErrDecode, ErrWebSocket, &TurnError{Message: "x"}} {
		if StatusCode(err) != 0 {
			t.Errorf("%v has a status", err)
		}
	}
	if !IsTransient(ErrTimeout) || IsTransient(ErrDecode) || IsTransient(nil) {
		t.Error("transient classification")
	}
}

func TestAPIErrorsTakeTheGatewaysMessage(t *testing.T) {
	err := newAPIError(404, []byte(`{"error":"Cron job not found: x"}`))
	if err.Error() != "gateway answered 404: Cron job not found: x" || !IsNotFound(err) || IsUnauthorized(err) {
		t.Error(err)
	}
	err = newAPIError(401, []byte("Unauthorized\n"))
	if err.Error() != "gateway answered 401: Unauthorized" || !IsUnauthorized(err) {
		t.Error(err)
	}
	err = newAPIError(400, []byte(`{"error":{"code":"bad_path"}}`))
	if err.Status != 400 || !strings.Contains(err.Error(), "bad_path") {
		t.Error(err)
	}
}

func TestConnectionLostVectors(t *testing.T) {
	op := func(errno error) error { return &net.OpError{Op: "read", Err: os.NewSyscallError("read", errno)} }
	vectors := []struct {
		err  error
		want bool
	}{
		{op(syscall.ECONNRESET), true},
		{op(syscall.ECONNABORTED), true},
		{op(syscall.EPIPE), true},
		{fmt.Errorf("request failed: %w", io.ErrUnexpectedEOF), true},
		{fmt.Errorf("request failed: %w", io.EOF), true},
		{op(syscall.EACCES), false},
		{errors.New("tls handshake failed"), false},
	}
	for _, v := range vectors {
		if got := isConnectionLost(v.err); got != v.want {
			t.Errorf("%v: %t, want %t", v.err, got, v.want)
		}
	}
}

func TestRetryableErrorVectors(t *testing.T) {
	vectors := []struct {
		err  error
		want bool
	}{
		{ErrTimeout, true},
		{fmt.Errorf("x: %w", ErrTimeout), true},
		{&net.OpError{Op: "dial", Err: errors.New("no route")}, true},
		{fmt.Errorf("x: %w", syscall.ECONNREFUSED), true},
		{errors.New("a middleware refused"), false},
		{nil, false},
	}
	for _, v := range vectors {
		if got := isRetryableError(v.err); got != v.want {
			t.Errorf("%v: %t, want %t", v.err, got, v.want)
		}
	}
}
