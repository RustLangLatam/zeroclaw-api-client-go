package zeroclaw

// Conversation sessions: listing, transcripts, state, renaming, aborting and
// adding messages.
//
// Gateway handlers: crates/zeroclaw-gateway/src/api.rs (/api/sessions*). A
// session id is either the display id or the full session key (gw_…,
// whatsapp_…); the gateway resolves both.

import (
	"context"
	"net/http"
)

// SessionsAPI holds the session endpoints, from [Client.Sessions].
type SessionsAPI struct{ c *Client }

// Sessions returns the conversation sessions endpoints (/api/sessions).
func (c *Client) Sessions() SessionsAPI { return SessionsAPI{c} }

// List returns every stored session, empty when the gateway keeps none
// (GET /api/sessions).
func (a SessionsAPI) List(ctx context.Context) ([]Session, error) {
	return callField[[]Session](ctx, a.c, http.MethodGet, []string{"api", "sessions"}, nil, nil, "sessions")
}

// Running returns the gateway (web chat) sessions (GET
// /api/sessions/running).
func (a SessionsAPI) Running(ctx context.Context) ([]Session, error) {
	return callField[[]Session](ctx, a.c, http.MethodGet, []string{"api", "sessions", "running"}, nil, nil, "sessions")
}

// Messages returns the transcript, empty for an unknown session (GET
// /api/sessions/{id}/messages).
func (a SessionsAPI) Messages(ctx context.Context, id string) ([]SessionMessage, error) {
	return callField[[]SessionMessage](ctx, a.c, http.MethodGet, []string{"api", "sessions", id, "messages"}, nil, nil, "messages")
}

// AddMessage appends an assistant message to the session, shown to chat
// clients; no agent turn runs (POST /api/sessions/{id}/messages).
func (a SessionsAPI) AddMessage(ctx context.Context, id, content string) error {
	return a.c.call(ctx, http.MethodPost, []string{"api", "sessions", id, "messages"}, nil, map[string]string{"content": content}, nil)
}

// State reports whether a turn is running (GET /api/sessions/{id}/state).
func (a SessionsAPI) State(ctx context.Context, id string) (*SessionState, error) {
	var out SessionState
	return &out, a.c.get(ctx, []string{"api", "sessions", id, "state"}, nil, &out)
}

// Rename renames a session (PUT /api/sessions/{id}).
func (a SessionsAPI) Rename(ctx context.Context, id, name string) error {
	return a.c.call(ctx, http.MethodPut, []string{"api", "sessions", id}, nil, map[string]string{"name": name}, nil)
}

// Delete cancels its running turn and deletes it (DELETE
// /api/sessions/{id}).
func (a SessionsAPI) Delete(ctx context.Context, id string) error {
	return a.c.call(ctx, http.MethodDelete, []string{"api", "sessions", id}, nil, nil, nil)
}

// Abort cancels the running turn and reports whether there was one (POST
// /api/sessions/{id}/abort).
func (a SessionsAPI) Abort(ctx context.Context, id string) (bool, error) {
	status, err := callField[string](ctx, a.c, http.MethodPost, []string{"api", "sessions", id, "abort"}, nil, nil, "status")
	return status == "aborted", err
}

// Session is a stored session.
type Session struct {
	// SessionID is the display id (gw_ removed).
	SessionID string `json:"session_id"`
	// SessionKey is the full key (gw_…, whatsapp_<channel>_<peer>…).
	SessionKey *string `json:"session_key,omitempty"`
	// CreatedAt is the creation time (RFC 3339).
	CreatedAt string `json:"created_at"`
	// LastActivity is the last activity (RFC 3339).
	LastActivity string `json:"last_activity"`
	// MessageCount is the number of messages in it.
	MessageCount int `json:"message_count"`
	// AgentAlias is the agent that answers it.
	AgentAlias *string `json:"agent_alias,omitempty"`
	// ChannelID is the channel it came through (whatsapp.alias).
	ChannelID *string `json:"channel_id,omitempty"`
	// Name is the name given to it.
	Name *string `json:"name,omitempty"`
}

// SessionMessage is a message of a transcript.
type SessionMessage struct {
	// Role is user, assistant, tool, …
	Role string `json:"role"`
	// Content is the content (tool calls and results are JSON text).
	Content string `json:"content"`
	// CreatedAt is when it was stored.
	CreatedAt *string `json:"created_at,omitempty"`
}

// SessionState says whether a session is busy.
type SessionState struct {
	// SessionID is the session id as asked.
	SessionID string `json:"session_id"`
	// State is idle, running or error.
	State string `json:"state"`
	// TurnID is the running turn.
	TurnID *string `json:"turn_id,omitempty"`
	// TurnStartedAt is when it started.
	TurnStartedAt *string `json:"turn_started_at,omitempty"`
}
