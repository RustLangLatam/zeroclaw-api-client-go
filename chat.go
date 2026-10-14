package zeroclaw

// Talking to an agent over the gateway's chat WebSocket (/ws/chat).
//
// Gateway handler: crates/zeroclaw-gateway/src/ws.rs. A connection is one
// session (gw_<session_id> in [SessionsAPI]); reconnecting with the same
// session id resumes it. Each message starts a turn that streams frames
// (text chunks, tool calls, approval requests, …) and ends with done,
// aborted or error. A message sent during a turn steers it.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/coder/websocket"
)

// subprotocol is the WebSocket subprotocol ZeroClaw's chat speaks.
const subprotocol = "zeroclaw.v1"

// maxFrame bounds one WebSocket message (a long reply arrives in one done
// frame).
const maxFrame = 64 << 20

// ChatOptions says where a chat connects. Empty fields are left out.
type ChatOptions struct {
	// SessionID is the session to resume or create (a new one without it).
	SessionID string
	// Name names the session.
	Name string
	// Cwd is the working directory for the session (must exist on the host).
	Cwd string
}

// Chat opens a chat with agent alias (GET /ws/chat, upgraded to a
// WebSocket) and waits for the gateway to confirm the session. ctx bounds
// the opening only; each later call takes its own.
func (c *Client) Chat(ctx context.Context, alias string, opts ChatOptions) (*ChatSession, error) {
	conn, err := c.connect(ctx, []string{"ws", "chat"},
		params("agent", alias, "session_id", opts.SessionID, "name", opts.Name, "cwd", opts.Cwd))
	if err != nil {
		return nil, err
	}
	chat := &ChatSession{conn: conn}
	frame, err := chat.NextFrame(ctx)
	switch {
	case err != nil:
	case frame.Type == FrameSessionStart:
		chat.sessionID, chat.resumed, chat.messageCount = frame.SessionID, frame.Resumed, frame.MessageCount
		return chat, nil
	case frame.Type == FrameError:
		err = &TurnError{Code: frame.Code, Message: frame.Message}
	default:
		err = fmt.Errorf("%w: expected session_start, got %s", ErrWebSocket, frame.Raw)
	}
	conn.CloseNow()
	if errors.Is(err, io.EOF) {
		err = fmt.Errorf("%w: the gateway closed the chat before the session started", ErrWebSocket)
	}
	return nil, err
}

// ChatSession is an open chat with an agent. Frames are read by one
// goroutine at a time; sending is safe from any.
type ChatSession struct {
	conn         *websocket.Conn
	sessionID    string
	resumed      bool
	messageCount int
}

// SessionID is the session id (its key is gw_<id>).
func (s *ChatSession) SessionID() string { return s.sessionID }

// Resumed reports whether an existing session was resumed.
func (s *ChatSession) Resumed() bool { return s.resumed }

// MessageCount is the number of messages the session held when it was
// opened.
func (s *ChatSession) MessageCount() int { return s.messageCount }

// Send sends a message: it starts a turn, or steers the running one.
func (s *ChatSession) Send(ctx context.Context, content string) error {
	return s.sendFrame(ctx, map[string]string{"type": "message", "content": content})
}

// AnswerApproval answers an approval_request with decision "approve",
// "deny" or "always".
func (s *ChatSession) AnswerApproval(ctx context.Context, requestID, decision string) error {
	return s.sendFrame(ctx, map[string]string{
		"type": "approval_response", "request_id": requestID, "decision": decision,
	})
}

// NextFrame returns the next frame from the gateway, or [io.EOF] once the
// socket closed. Cancelling ctx while it waits closes the chat (the
// WebSocket cannot resume a half-read message).
func (s *ChatSession) NextFrame(ctx context.Context) (ChatFrame, error) {
	data, err := readText(ctx, s.conn)
	if err != nil {
		return ChatFrame{}, err
	}
	return parseChatFrame(data)
}

// Ask sends content and waits for the turn to end, ignoring the frames in
// between. An error frame becomes a [*TurnError]; an aborted turn too, with
// code "ABORTED". Tool approvals are not answered here: use
// [ChatSession.NextFrame] for agents that ask for them.
func (s *ChatSession) Ask(ctx context.Context, content string) (*TurnResult, error) {
	if err := s.Send(ctx, content); err != nil {
		return nil, err
	}
	for {
		frame, err := s.NextFrame(ctx)
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: the gateway closed the chat before the turn ended", ErrWebSocket)
		}
		if err != nil {
			return nil, err
		}
		switch frame.Type {
		case FrameDone:
			return frame.Done, nil
		case FrameError:
			return nil, &TurnError{Code: frame.Code, Message: frame.Message}
		case FrameAborted:
			return nil, &TurnError{Code: "ABORTED", Message: "the turn was aborted"}
		}
	}
}

// Close closes the chat. Closing during a turn cancels it.
func (s *ChatSession) Close() error {
	if err := s.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		return fmt.Errorf("%w: %w", ErrWebSocket, err)
	}
	return nil
}

func (s *ChatSession) sendFrame(ctx context.Context, frame any) error {
	b, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("encoding the frame: %w", err)
	}
	if err := s.conn.Write(ctx, websocket.MessageText, b); err != nil {
		return fmt.Errorf("%w: %w", ErrWebSocket, err)
	}
	return nil
}

// Chat frame types. Frames of any other type ("plan", "history_trimmed",
// "cron_result", session events, …) keep their type and only Raw is set.
const (
	FrameSessionStart    = "session_start"    // the session is open
	FrameConnected       = "connected"        // answer to a "connect" frame
	FrameChunk           = "chunk"            // a piece of the reply, in Content
	FrameThinking        = "thinking"         // a piece of the model's reasoning, in Content
	FrameToolCall        = "tool_call"        // the agent calls a tool
	FrameToolResult      = "tool_result"      // a tool's result
	FrameApprovalRequest = "approval_request" // the agent needs approval to run a tool
	FrameDone            = "done"             // the turn ended; see Done
	FrameAborted         = "aborted"          // the turn was cancelled
	FrameError           = "error"            // the turn (or the session) failed
)

// ChatFrame is a frame the gateway sends on the chat. Type says which
// fields are set.
type ChatFrame struct {
	// Type is the frame type, one of the Frame* constants or another.
	Type string

	// Content of a chunk or thinking frame.
	Content string

	// SessionID, Resumed (whether it existed), MessageCount (messages it
	// holds) and Name of a session_start frame.
	SessionID    string
	Resumed      bool
	MessageCount int
	// Name of the session (session_start) or of the tool (tool_call,
	// tool_result).
	Name string

	// ID of a tool_call or tool_result; Args of the call; Output of the
	// result.
	ID     string
	Args   json.RawMessage
	Output json.RawMessage

	// RequestID, Tool, ArgumentsSummary (what it would do) and TimeoutSecs
	// (seconds before it is denied) of an approval_request; answer it with
	// [ChatSession.AnswerApproval].
	RequestID        string
	Tool             string
	ArgumentsSummary string
	TimeoutSecs      uint64

	// Done is the end of the turn, for a done frame.
	Done *TurnResult

	// Message and Code (PROVIDER_ERROR, SESSION_QUEUE_FULL, …) of an error
	// frame.
	Message string
	Code    string

	// Raw is the frame as sent.
	Raw json.RawMessage
}

// TurnResult is the end of a turn.
type TurnResult struct {
	// FullResponse is the whole reply.
	FullResponse string `json:"full_response"`
	// InputTokens used.
	InputTokens *uint64 `json:"input_tokens,omitempty"`
	// OutputTokens used.
	OutputTokens *uint64 `json:"output_tokens,omitempty"`
	// TokensUsed in all.
	TokensUsed *uint64 `json:"tokens_used,omitempty"`
	// CostUSD of the turn.
	CostUSD *float64 `json:"cost_usd,omitempty"`
	// Model used.
	Model string `json:"model"`
	// Provider used.
	Provider string `json:"provider"`
	// MaxContextTokens is the model's context size.
	MaxContextTokens *uint64 `json:"max_context_tokens,omitempty"`
	// LastInputTokens is the input tokens of the last model call.
	LastInputTokens *uint64 `json:"last_input_tokens,omitempty"`
}

func parseChatFrame(data []byte) (ChatFrame, error) {
	var wire struct {
		Type             string          `json:"type"`
		Content          string          `json:"content"`
		SessionID        string          `json:"session_id"`
		Resumed          bool            `json:"resumed"`
		MessageCount     int             `json:"message_count"`
		Name             string          `json:"name"`
		ID               string          `json:"id"`
		Args             json.RawMessage `json:"args"`
		Output           json.RawMessage `json:"output"`
		RequestID        string          `json:"request_id"`
		Tool             string          `json:"tool"`
		ArgumentsSummary string          `json:"arguments_summary"`
		TimeoutSecs      uint64          `json:"timeout_secs"`
		Message          string          `json:"message"`
		Error            string          `json:"error"`
		Code             string          `json:"code"`
	}
	raw := json.RawMessage(data)
	if err := json.Unmarshal(data, &wire); err != nil {
		// A field of an unexpected type in a frame we do not model still
		// reaches the caller as Raw, as long as the frame is JSON.
		var probe struct {
			Type string `json:"type"`
		}
		if !json.Valid(data) {
			return ChatFrame{}, fmt.Errorf("%w: chat frame is not JSON: %w", ErrDecode, err)
		}
		json.Unmarshal(data, &probe)
		return ChatFrame{Type: probe.Type, Raw: raw}, nil
	}
	f := ChatFrame{Type: wire.Type, Raw: raw}
	switch wire.Type {
	case FrameSessionStart:
		f.SessionID, f.Resumed, f.MessageCount, f.Name = wire.SessionID, wire.Resumed, wire.MessageCount, wire.Name
	case FrameChunk, FrameThinking:
		f.Content = wire.Content
	case FrameToolCall:
		f.ID, f.Name, f.Args = wire.ID, wire.Name, wire.Args
	case FrameToolResult:
		f.ID, f.Name, f.Output = wire.ID, wire.Name, wire.Output
	case FrameApprovalRequest:
		f.RequestID, f.Tool, f.ArgumentsSummary, f.TimeoutSecs = wire.RequestID, wire.Tool, wire.ArgumentsSummary, wire.TimeoutSecs
	case FrameDone:
		f.Done = new(TurnResult)
		if err := json.Unmarshal(data, f.Done); err != nil {
			return ChatFrame{}, fmt.Errorf("%w: done frame: %w", ErrDecode, err)
		}
	case FrameError:
		f.Message, f.Code = wire.Message, wire.Code
		if f.Message == "" {
			f.Message = wire.Error
		}
		if f.Message == "" {
			f.Message = "unknown error"
		}
	}
	return f, nil
}

// connect opens a WebSocket to segments on the gateway (ws for http, wss
// for https), offering ZeroClaw's subprotocol and the bearer token. The
// client's timeout bounds the handshake.
func (c *Client) connect(ctx context.Context, segments []string, query url.Values) (*websocket.Conn, error) {
	u := c.url(segments, query)
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	header := http.Header{"User-Agent": {"zeroclaw-api-client-go/" + Version}}
	if c.token != "" {
		header.Set("Authorization", "Bearer "+c.token)
	}
	dialCtx := ctx
	if c.timeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeoutCause(ctx, c.timeout, ErrTimeout)
		defer cancel()
	}
	conn, resp, err := websocket.Dial(dialCtx, u.String(), &websocket.DialOptions{
		HTTPClient:      c.ws,
		HTTPHeader:      header,
		Subprotocols:    []string{subprotocol},
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		// Keep the gateway's HTTP refusal (401 without a valid token, 400
		// for an unknown agent) as an APIError.
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols && resp.StatusCode != 0 {
			var body []byte
			if resp.Body != nil {
				body, _ = io.ReadAll(resp.Body)
			}
			return nil, newAPIError(resp.StatusCode, body)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if context.Cause(dialCtx) == ErrTimeout {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("%w: %w", ErrWebSocket, err)
	}
	conn.SetReadLimit(maxFrame)
	return conn, nil
}

// readText returns the next text message of conn, skipping others, or
// [io.EOF] once the socket closed.
func readText(ctx context.Context, conn *websocket.Conn) ([]byte, error) {
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != -1 || errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("%w: %w", ErrWebSocket, err)
		}
		if typ == websocket.MessageText {
			return data, nil
		}
	}
}

// Feed is a live WebSocket feed of JSON frames the gateway pushes (canvas,
// SOP runs). Read it with [Feed.Next] and close it when done.
type Feed struct{ conn *websocket.Conn }

// Next returns the next frame, or [io.EOF] once the gateway closed the
// feed. A frame that is not JSON is an error wrapping [ErrDecode]; the feed
// goes on after it. Cancelling ctx while it waits closes the feed.
func (f *Feed) Next(ctx context.Context) (json.RawMessage, error) {
	data, err := readText(ctx, f.conn)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("%w: frame is not JSON", ErrDecode)
	}
	return data, nil
}

// Close closes the feed.
func (f *Feed) Close() error {
	if err := f.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		return fmt.Errorf("%w: %w", ErrWebSocket, err)
	}
	return nil
}

// feed opens a server-push WebSocket feed on segments.
func (c *Client) feed(ctx context.Context, segments ...string) (*Feed, error) {
	conn, err := c.connect(ctx, segments, nil)
	if err != nil {
		return nil, err
	}
	return &Feed{conn}, nil
}
