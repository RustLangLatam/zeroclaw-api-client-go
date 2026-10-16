package zeroclaw

// Channels (WhatsApp, Telegram, …): their state, identities and logins.
//
// Gateway handlers: crates/zeroclaw-gateway/src/api.rs and api_config.rs
// (bind). A channel is named type.alias, as [ChannelsAPI.List] returns it.

import (
	"context"
	"encoding/json"
	"net/http"
)

// ChannelsAPI holds the channel endpoints, from [Client.Channels].
type ChannelsAPI struct{ c *Client }

// Channels returns the channels (/api/channels).
func (c *Client) Channels() ChannelsAPI { return ChannelsAPI{c} }

// List returns every configured channel and its state (GET /api/channels).
func (a ChannelsAPI) List(ctx context.Context) ([]Channel, error) {
	return callField[[]Channel](ctx, a.c, http.MethodGet, []string{"api", "channels"}, nil, nil, "channels")
}

// Bind lets identity talk to the channel (Telegram, WeChat and LINE),
// adding it to the channel's peer group (POST /api/channels/bind).
func (a ChannelsAPI) Bind(ctx context.Context, channelType, alias, identity string) (*Bound, error) {
	body := map[string]string{"channel_type": channelType, "alias": alias, "identity": identity}
	var out Bound
	return &out, a.c.call(ctx, http.MethodPost, []string{"api", "channels", "bind"}, nil, body, &out)
}

// Relink clears a QR-paired channel's saved login so it pairs again after a
// reload ([SystemAPI.Reload]): POST /api/channels/{channel}/relink.
func (a ChannelsAPI) Relink(ctx context.Context, channel string) (*Relinked, error) {
	var out Relinked
	return &out, a.c.call(ctx, http.MethodPost, []string{"api", "channels", channel, "relink"}, nil, nil, &out)
}

// Channel is a configured channel.
type Channel struct {
	// Name is type.alias, the name the other calls take.
	Name string `json:"name"`
	// Type is the channel type (whatsapp, telegram, …).
	Type string `json:"type"`
	// Alias is the alias.
	Alias string `json:"alias"`
	// OwningAgent is the agent it belongs to.
	OwningAgent *string `json:"owning_agent,omitempty"`
	// Enabled says whether it is enabled.
	Enabled bool `json:"enabled"`
	// Compiled says whether this build includes its type.
	Compiled bool `json:"compiled"`
	// Status is active, inactive, error, unknown or not_compiled.
	Status string `json:"status"`
	// Health is healthy, degraded, down or unavailable.
	Health string `json:"health"`
	// Readiness says what it still needs (enabled, bound_to_agent,
	// authenticated, listening, each ready/missing/unknown, plus notes).
	Readiness json.RawMessage `json:"readiness,omitempty"`
	// MessageCount is the number of messages the channel has carried.
	MessageCount int `json:"message_count"`
	// LastMessageAt is when the last one went through (RFC 3339).
	LastMessageAt *string `json:"last_message_at,omitempty"`
}

// Bound is the result of [ChannelsAPI.Bind].
type Bound struct {
	// Saved says whether the configuration was saved.
	Saved bool `json:"saved"`
	// AlreadyBound says whether the identity was already allowed.
	AlreadyBound bool `json:"already_bound"`
	// Group is the peer group it was added to.
	Group string `json:"group"`
	// Channel is the channel.
	Channel string `json:"channel"`
}

// Relinked is the result of [ChannelsAPI.Relink].
type Relinked struct {
	// Channel is the channel.
	Channel string `json:"channel"`
	// Outcome is cleared or nothing_to_clear.
	Outcome string `json:"outcome"`
	// Removed lists the files removed.
	Removed []string `json:"removed,omitempty"`
	// RestartRequired says whether a reload is needed for the channel to
	// pair again.
	RestartRequired bool `json:"restart_required,omitempty"`
	// Note is the gateway's note.
	Note *string `json:"note,omitempty"`
}
