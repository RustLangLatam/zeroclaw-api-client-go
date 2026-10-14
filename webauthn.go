package zeroclaw

// Hardware-key (WebAuthn) registration and assertion, an extra check on top
// of the bearer token.
//
// Gateway handler: crates/zeroclaw-gateway/src/api_webauthn.rs (built with
// the webauthn feature; 404 when it is off). The ceremony itself runs in the
// authenticator (a browser or a security-key library): this API only
// carries its JSON, with ZeroClaw's snake_case field names and base64url
// binary fields.

import (
	"context"
	"encoding/json"
	"net/http"
)

// WebAuthnAPI holds the WebAuthn endpoints, from [Client.WebAuthn].
type WebAuthnAPI struct{ c *Client }

// WebAuthn returns the hardware-key registration and assertion endpoints
// (/api/webauthn).
func (c *Client) WebAuthn() WebAuthnAPI { return WebAuthnAPI{c} }

// RegisterStart returns the creation challenge for a new key of userID
// (POST /api/webauthn/register/start).
func (a WebAuthnAPI) RegisterStart(ctx context.Context, userID, userName string) (json.RawMessage, error) {
	return a.post(ctx, map[string]string{"user_id": userID, "user_name": userName}, "register", "start")
}

// RegisterFinish sends the authenticator's answer (challenge, id,
// attestation_object, client_data_json, label?), encoded as JSON, and
// returns {credential_id, label, registered_at} (POST
// /api/webauthn/register/finish).
func (a WebAuthnAPI) RegisterFinish(ctx context.Context, credential any) (json.RawMessage, error) {
	return a.post(ctx, credential, "register", "finish")
}

// AuthStart returns the assertion challenge for userID (POST
// /api/webauthn/auth/start).
func (a WebAuthnAPI) AuthStart(ctx context.Context, userID string) (json.RawMessage, error) {
	return a.post(ctx, map[string]string{"user_id": userID}, "auth", "start")
}

// AuthFinish sends the authenticator's assertion (challenge, id,
// authenticator_data, client_data_json, signature), encoded as JSON (POST
// /api/webauthn/auth/finish). Refused with 401 when it does not verify.
func (a WebAuthnAPI) AuthFinish(ctx context.Context, assertion any) (json.RawMessage, error) {
	return a.post(ctx, assertion, "auth", "finish")
}

// Credentials returns the keys of userID (GET /api/webauthn/credentials).
func (a WebAuthnAPI) Credentials(ctx context.Context, userID string) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, []string{"api", "webauthn", "credentials"}, params("user_id", userID), &out)
}

// DeleteCredential removes key credentialID of userID (DELETE
// /api/webauthn/credentials/{id}).
func (a WebAuthnAPI) DeleteCredential(ctx context.Context, userID, credentialID string) error {
	return a.c.call(ctx, http.MethodDelete, []string{"api", "webauthn", "credentials", credentialID}, params("user_id", userID), nil, nil)
}

// post posts body to /api/webauthn/{action...}.
func (a WebAuthnAPI) post(ctx context.Context, body any, action ...string) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.call(ctx, http.MethodPost, append([]string{"api", "webauthn"}, action...), nil, body, &out)
}
