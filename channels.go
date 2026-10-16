package zeroclaw

// Channels (WhatsApp, Telegram, …): their state, sending through them,
// group chats ("rooms") and their members.
//
// Gateway handlers: crates/zeroclaw-gateway/src/api.rs and api_config.rs
// (bind). A channel is named type.alias, as [ChannelsAPI.List] returns it.
// Sending and managing rooms need pairing on and the channel listed in
// [gateway] send_channels; reading rooms needs pairing only. Errors on these
// routes carry the channel: {"channel": …, "error": …}.

import (
	"context"
	"encoding/json"
	"net/http"
)

// ChannelsAPI holds the channel endpoints, from [Client.Channels].
type ChannelsAPI struct{ c *Client }

// Channels returns the channels and their group chats (/api/channels).
func (c *Client) Channels() ChannelsAPI { return ChannelsAPI{c} }

// channelsOptional is s as JSON, null when empty.
func channelsOptional(s string) any {
	if s == "" {
		return nil
	}
	return s
}

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

// Send sends plain text to to, with no agent turn (POST
// /api/channels/{channel}/send). An empty threadID sends outside a thread.
func (a ChannelsAPI) Send(ctx context.Context, channel, to, content, threadID string) error {
	body := map[string]any{"to": to, "content": content, "thread_id": channelsOptional(threadID)}
	return a.c.call(ctx, http.MethodPost, []string{"api", "channels", channel, "send"}, nil, body, nil)
}

// Rooms returns the group chats the running channel takes part in (GET
// /api/channels/{channel}/rooms).
func (a ChannelsAPI) Rooms(ctx context.Context, channel string) ([]RoomSummary, error) {
	return callField[[]RoomSummary](ctx, a.c, http.MethodGet, []string{"api", "channels", channel, "rooms"}, nil, nil, "rooms")
}

// CreateRoom creates a group chat and invites invites (phone numbers on
// WhatsApp, which are also allowed to talk to the channel): POST
// /api/channels/{channel}/rooms. An empty topic sets none.
func (a ChannelsAPI) CreateRoom(ctx context.Context, channel, name, topic string, invites []string) (*CreatedRoom, error) {
	if invites == nil {
		invites = []string{}
	}
	body := map[string]any{"name": name, "topic": channelsOptional(topic), "invites": invites}
	var out CreatedRoom
	return &out, a.c.call(ctx, http.MethodPost, []string{"api", "channels", channel, "rooms"}, nil, body, &out)
}

// RoomMembers returns a group chat's members (GET
// /api/channels/{channel}/rooms/{room}/members).
func (a ChannelsAPI) RoomMembers(ctx context.Context, channel, room string) ([]RoomMember, error) {
	return callField[[]RoomMember](ctx, a.c, http.MethodGet,
		[]string{"api", "channels", channel, "rooms", room, "members"}, nil, nil, "members")
}

// Invite adds or invites user to a group chat (POST
// /api/channels/{channel}/rooms/{room}/invites).
func (a ChannelsAPI) Invite(ctx context.Context, channel, room, user string) (*Participant, error) {
	var out Participant
	return &out, a.c.call(ctx, http.MethodPost, []string{"api", "channels", channel, "rooms", room, "invites"},
		nil, map[string]string{"user": user}, &out)
}

// RemoveMember removes a member from a group chat (DELETE
// /api/channels/{channel}/rooms/{room}/members/{user}).
func (a ChannelsAPI) RemoveMember(ctx context.Context, channel, room, user string) error {
	return a.c.call(ctx, http.MethodDelete, []string{"api", "channels", channel, "rooms", room, "members", user}, nil, nil, nil)
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

// RoomSummary is a group chat.
type RoomSummary struct {
	// ID is the id in the channel's addressing (…@g.us on WhatsApp).
	ID string `json:"id"`
	// Name is the name (the WhatsApp group subject).
	Name string `json:"name"`
	// Participants is the member count.
	Participants int `json:"participants"`
}

// RoomMember is a member of a group chat.
type RoomMember struct {
	// ID is the member: +<digits> on WhatsApp when the number is known.
	ID string `json:"id"`
	// Admin says whether they administer the group.
	Admin bool `json:"admin"`
}

// CreatedRoom is the result of [ChannelsAPI.CreateRoom].
type CreatedRoom struct {
	// Channel is the channel.
	Channel string `json:"channel"`
	// Room is the new room's id.
	Room string `json:"room"`
	// Outcome is always created.
	Outcome string `json:"outcome"`
	// Participants says what happened to each invitee.
	Participants []Participant `json:"participants,omitempty"`
}

// Participant is what happened to someone added to a group chat.
type Participant struct {
	// User is who.
	User string `json:"user"`
	// Outcome is added, invited, refused or failed.
	Outcome string `json:"outcome"`
	// Detail says why, when it did not work.
	Detail *string `json:"detail,omitempty"`
}
