package zeroclaw

// Pairing (how a client gets its bearer token) and the paired devices.
//
// Gateway handlers: crates/zeroclaw-gateway/src/lib.rs (/pair, /pair/code,
// /admin/paircode*) and api_pairing.rs. A pairing code is six digits,
// single use, valid ten minutes; the token it buys is zc_ + 64 hex
// characters. Exchange a code for a token with [PairingAPI.Pair], then get
// a client that uses it with [Client.WithToken].

import (
	"context"
	"fmt"
	"net/http"
)

// PairingAPI holds the pairing endpoints, from [Client.Pairing].
type PairingAPI struct{ c *Client }

// Pairing returns the pairing and paired devices endpoints.
func (c *Client) Pairing() PairingAPI { return PairingAPI{c} }

// Pair exchanges a pairing code for a bearer token, registering this client
// as a device (POST /api/pair). Empty deviceName and deviceType are sent as
// null. Public.
func (a PairingAPI) Pair(ctx context.Context, code, deviceName, deviceType string) (*Paired, error) {
	body := map[string]any{"code": code, "device_name": nil, "device_type": nil}
	if deviceName != "" {
		body["device_name"] = deviceName
	}
	if deviceType != "" {
		body["device_type"] = deviceType
	}
	var out Paired
	return &out, a.c.call(ctx, http.MethodPost, []string{"api", "pair"}, nil, body, &out)
}

// PairLegacy is the original pairing call, with the code in the
// X-Pairing-Code header (POST /pair). Public.
func (a PairingAPI) PairLegacy(ctx context.Context, code string) (*Paired, error) {
	req, err := a.c.request(ctx, http.MethodPost, []string{"pair"}, nil, nil, "")
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Pairing-Code", code)
	var out Paired
	return &out, a.c.do(req, &out)
}

// InitialCode returns the first-run pairing code, shown only while no
// client is paired yet (GET /pair/code). Public.
func (a PairingAPI) InitialCode(ctx context.Context) (*PairingCode, error) {
	var out PairingCode
	return &out, a.c.get(ctx, []string{"pair", "code"}, nil, &out)
}

// NewCode mints a new pairing code (replacing a pending one) so another
// device can pair (POST /api/pairing/initiate).
func (a PairingAPI) NewCode(ctx context.Context) (string, error) {
	return callField[string](ctx, a.c, http.MethodPost, []string{"api", "pairing", "initiate"}, nil, nil, "pairing_code")
}

// AdminCode returns the pending pairing code (GET /admin/paircode).
// Loopback only.
func (a PairingAPI) AdminCode(ctx context.Context) (*PairingCode, error) {
	var out PairingCode
	return &out, a.c.get(ctx, []string{"admin", "paircode"}, nil, &out)
}

// AdminNewCode mints a pairing code (POST /admin/paircode/new). A non-empty
// rotate revokes first: "all" every token and device, otherwise the device
// with that id. Loopback only.
func (a PairingAPI) AdminNewCode(ctx context.Context, rotate string) (*PairingCode, error) {
	var out PairingCode
	return &out, a.c.call(ctx, http.MethodPost, []string{"admin", "paircode", "new"}, params("rotate", rotate), nil, &out)
}

// Devices returns the paired devices (GET /api/devices).
func (a PairingAPI) Devices(ctx context.Context) ([]Device, error) {
	return callField[[]Device](ctx, a.c, http.MethodGet, []string{"api", "devices"}, nil, nil, "devices")
}

// RevokeDevice revokes a device and its token; it may be this client's own
// (DELETE /api/devices/{id}).
func (a PairingAPI) RevokeDevice(ctx context.Context, id string) error {
	return a.c.call(ctx, http.MethodDelete, []string{"api", "devices", id}, nil, nil, nil)
}

// RotateDeviceToken revokes a device's token and mints a pairing code for
// it to pair again; the code is empty when one was already pending or
// pairing is off (POST /api/devices/{id}/token/rotate).
func (a PairingAPI) RotateDeviceToken(ctx context.Context, id string) (string, error) {
	var out struct {
		PairingCode *string `json:"pairing_code"`
	}
	if err := a.c.call(ctx, http.MethodPost, []string{"api", "devices", id, "token", "rotate"}, nil, nil, &out); err != nil || out.PairingCode == nil {
		return "", err
	}
	return *out.PairingCode, nil
}

// SetCapabilities replaces the capabilities this client's device advertises
// and returns them (POST /api/devices/me/capabilities).
func (a PairingAPI) SetCapabilities(ctx context.Context, capabilities []string) ([]string, error) {
	if capabilities == nil {
		capabilities = []string{}
	}
	return callField[[]string](ctx, a.c, http.MethodPost, []string{"api", "devices", "me", "capabilities"},
		nil, map[string][]string{"capabilities": capabilities}, "capabilities")
}

// Paired is a successful pairing. Its String and GoString hide the token.
type Paired struct {
	// Paired is always true.
	Paired bool `json:"paired"`
	// Persisted says whether the token was saved in the gateway's
	// configuration.
	Persisted bool `json:"persisted,omitempty"`
	// Token is the bearer token. Keep it secret.
	Token string `json:"token"`
	// Message is the gateway's note.
	Message *string `json:"message,omitempty"`
}

// String describes the pairing without its token, so it never reaches logs.
func (p Paired) String() string {
	return fmt.Sprintf("zeroclaw.Paired{paired: %t, persisted: %t, token: <redacted>}", p.Paired, p.Persisted)
}

// GoString hides the token from %#v too.
func (p Paired) GoString() string { return p.String() }

// PairingCode is a pairing code and whether pairing is required.
type PairingCode struct {
	// Success says whether the request succeeded.
	Success bool `json:"success,omitempty"`
	// PairingRequired says whether the gateway requires pairing.
	PairingRequired bool `json:"pairing_required"`
	// PairingCode is the six-digit code, when one is pending and may be
	// shown.
	PairingCode *string `json:"pairing_code,omitempty"`
	// Message is the gateway's note.
	Message *string `json:"message,omitempty"`
}

// Device is a paired device.
type Device struct {
	// ID is the device id.
	ID string `json:"id"`
	// Name is the name given at pairing.
	Name *string `json:"name,omitempty"`
	// DeviceType is the type given at pairing (legacy for tokens older than
	// the registry).
	DeviceType *string `json:"device_type,omitempty"`
	// PairedAt is when it paired (RFC 3339).
	PairedAt string `json:"paired_at"`
	// LastSeen is when it was last seen (RFC 3339).
	LastSeen string `json:"last_seen"`
	// IPAddress is the address it paired from.
	IPAddress *string `json:"ip_address,omitempty"`
	// Capabilities are the advertised capabilities.
	Capabilities []string `json:"capabilities,omitempty"`
}
