package zeroclaw

// The chat WebSocket against a minimal gateway that speaks its frames.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

const chatToken = "zc_chat"

// chatGateway checks the handshake, opens session s1 and answers each
// message with the frames reply gives for its content.
func chatGateway(t *testing.T, reply func(content string) []string) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+chatToken || r.URL.Query().Get("agent") != "helper" {
			http.Error(w, "Unauthorized: provide a token", http.StatusUnauthorized)
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"zeroclaw.v1"}})
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		if conn.Subprotocol() != "zeroclaw.v1" {
			t.Errorf("subprotocol %q", conn.Subprotocol())
		}
		ctx := r.Context()
		conn.Write(ctx, websocket.MessageText, []byte(`{"type":"session_start","session_id":"s1","resumed":false,"message_count":0}`))
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var frame struct{ Content string }
			json.Unmarshal(data, &frame)
			for _, out := range reply(frame.Content) {
				conn.Write(ctx, websocket.MessageText, []byte(out))
			}
		}
	}))
	t.Cleanup(srv.Close)
	client, err := New(srv.URL, WithToken(chatToken))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func answer(content string) []string {
	return []string{
		`{"type":"chunk","content":"Hel"}`,
		`{"type":"tool_call","id":"c1","name":"search","args":{"q":"` + content + `"}}`,
		`{"type":"plan","entries":[]}`,
		`{"type":"done","full_response":"Hello, ` + content + `","model":"m","provider":"p","tokens_used":5,
		  "input_tokens":3,"output_tokens":2,"cost_usd":null,"max_context_tokens":8192,"last_input_tokens":3}`,
	}
}

func TestAQuestionGetsTheTurnsReply(t *testing.T) {
	ctx := context.Background()
	chat, err := chatGateway(t, answer).Chat(ctx, "helper", ChatOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer chat.Close()
	if chat.SessionID() != "s1" || chat.Resumed() || chat.MessageCount() != 0 {
		t.Errorf("%+v", chat)
	}
	reply, err := chat.Ask(ctx, "Ana")
	if err != nil {
		t.Fatal(err)
	}
	if reply.FullResponse != "Hello, Ana" || *reply.TokensUsed != 5 || reply.CostUSD != nil {
		t.Errorf("%+v", reply)
	}
}

func TestFramesCanBeReadOneByOne(t *testing.T) {
	ctx := context.Background()
	chat, err := chatGateway(t, answer).Chat(ctx, "helper", ChatOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := chat.Send(ctx, "Ana"); err != nil {
		t.Fatal(err)
	}
	next := func() ChatFrame {
		t.Helper()
		f, err := chat.NextFrame(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	if f := next(); f.Type != FrameChunk || f.Content != "Hel" {
		t.Errorf("%+v", f)
	}
	if f := next(); f.Type != FrameToolCall || f.Name != "search" || string(f.Args) != `{"q":"Ana"}` {
		t.Errorf("%+v", f)
	}
	if f := next(); f.Type != "plan" || !strings.Contains(string(f.Raw), "entries") {
		t.Errorf("%+v", f)
	}
	if f := next(); f.Type != FrameDone || f.Done.Model != "m" {
		t.Errorf("%+v", f)
	}
	if err := chat.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAFailedTurnIsAnErrorWithItsCode(t *testing.T) {
	ctx := context.Background()
	client := chatGateway(t, func(string) []string {
		return []string{`{"type":"error","message":"provider down","code":"PROVIDER_ERROR"}`}
	})
	chat, err := client.Chat(ctx, "helper", ChatOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer chat.Close()
	_, err = chat.Ask(ctx, "Hi")
	var turn *TurnError
	if !errors.As(err, &turn) || turn.Code != "PROVIDER_ERROR" || turn.Message != "provider down" {
		t.Fatalf("%v", err)
	}
}

func TestAnAbortedTurnIsAnError(t *testing.T) {
	ctx := context.Background()
	chat, err := chatGateway(t, func(string) []string { return []string{`{"type":"aborted"}`} }).Chat(ctx, "helper", ChatOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer chat.Close()
	var turn *TurnError
	if _, err := chat.Ask(ctx, "Hi"); !errors.As(err, &turn) || turn.Code != "ABORTED" {
		t.Fatalf("%v", err)
	}
}

func TestARefusedHandshakeKeepsItsStatus(t *testing.T) {
	client := chatGateway(t, answer).WithToken("zc_wrong")
	_, err := client.Chat(context.Background(), "helper", ChatOptions{})
	if StatusCode(err) != 401 || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("%v", err)
	}
}

// pushingGateway pushes frames on route, then closes.
func pushingGateway(t *testing.T, route string, frames ...string) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != route {
			t.Errorf("path %s, want %s", r.URL.Path, route)
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"zeroclaw.v1"}})
		if err != nil {
			t.Error(err)
			return
		}
		for _, f := range frames {
			conn.Write(r.Context(), websocket.MessageText, []byte(f))
		}
		conn.Close(websocket.StatusNormalClosure, "")
	}))
	t.Cleanup(srv.Close)
	client, _ := New(srv.URL, WithToken(chatToken))
	return client
}

// drain reads every frame of feed until it closes.
func drain(t *testing.T, feed *Feed) []map[string]json.RawMessage {
	t.Helper()
	var frames []map[string]json.RawMessage
	for {
		raw, err := feed.Next(context.Background())
		if errors.Is(err, io.EOF) {
			return frames
		}
		if err != nil {
			t.Fatal(err)
		}
		var f map[string]json.RawMessage
		json.Unmarshal(raw, &f)
		frames = append(frames, f)
	}
}

func TestSopRunsAndCanvasesStreamTheirFrames(t *testing.T) {
	ctx := context.Background()
	client := pushingGateway(t, "/ws/sops/runs", `{"type":"snapshot","runs":[]}`, `{"type":"run","run":{"run_id":"r1"}}`)
	feed, err := client.Sops().WatchRuns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	frames := drain(t, feed)
	if len(frames) != 2 || !sameJSON(t, frames[1]["run"], []byte(`{"run_id":"r1"}`)) {
		t.Errorf("%s", frames)
	}

	client = pushingGateway(t, "/ws/canvas/board", `{"type":"frame","canvas_id":"board","frame":{"content":"# Hi"}}`)
	feed, err = client.Canvas().Watch(ctx, "board")
	if err != nil {
		t.Fatal(err)
	}
	frames = drain(t, feed)
	if len(frames) != 1 || !sameJSON(t, frames[0]["frame"], []byte(`{"content":"# Hi"}`)) {
		t.Errorf("%s", frames)
	}
}

func TestChatFrameVectors(t *testing.T) {
	vectors := []struct {
		text string
		want ChatFrame
	}{
		{`{"type":"connected","message":"Connection established"}`, ChatFrame{Type: FrameConnected}},
		{`{"type":"chunk","content":"Ho"}`, ChatFrame{Type: FrameChunk, Content: "Ho"}},
		{`{"type":"thinking","content":"…"}`, ChatFrame{Type: FrameThinking, Content: "…"}},
		{`{"type":"aborted"}`, ChatFrame{Type: FrameAborted}},
		{`{"type":"tool_call","id":"c1","name":"shell","args":{"cmd":"ls"}}`,
			ChatFrame{Type: FrameToolCall, ID: "c1", Name: "shell", Args: json.RawMessage(`{"cmd":"ls"}`)}},
		{`{"type":"tool_result","id":"c1","name":"shell","output":"ok"}`,
			ChatFrame{Type: FrameToolResult, ID: "c1", Name: "shell", Output: json.RawMessage(`"ok"`)}},
		{`{"type":"approval_request","request_id":"r1","tool":"shell","arguments_summary":"rm x","timeout_secs":120}`,
			ChatFrame{Type: FrameApprovalRequest, RequestID: "r1", Tool: "shell", ArgumentsSummary: "rm x", TimeoutSecs: 120}},
		{`{"type":"error","error":"needs_onboarding","code":"NEEDS_ONBOARDING","message":"Set up first"}`,
			ChatFrame{Type: FrameError, Message: "Set up first", Code: "NEEDS_ONBOARDING"}},
		{`{"type":"error","error":"boom"}`, ChatFrame{Type: FrameError, Message: "boom"}},
		{`{"type":"error"}`, ChatFrame{Type: FrameError, Message: "unknown error"}},
		{`{"type":"session_start","session_id":"s","resumed":true,"message_count":4,"name":"Support"}`,
			ChatFrame{Type: FrameSessionStart, SessionID: "s", Resumed: true, MessageCount: 4, Name: "Support"}},
		{`{"type":"plan","entries":[]}`, ChatFrame{Type: "plan"}},
		{`{"type":"cron_result"}`, ChatFrame{Type: "cron_result"}},
		{`{"no_type":1}`, ChatFrame{}},
		{`{"type":"plan","content":5}`, ChatFrame{Type: "plan"}},
	}
	for _, v := range vectors {
		got, err := parseChatFrame([]byte(v.text))
		if err != nil {
			t.Errorf("%s: %v", v.text, err)
			continue
		}
		if string(got.Raw) != v.text {
			t.Errorf("%s: raw %s", v.text, got.Raw)
		}
		got.Raw = nil
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(v.want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s:\n got %s\nwant %s", v.text, gotJSON, wantJSON)
		}
	}
	if _, err := parseChatFrame([]byte("not json")); !errors.Is(err, ErrDecode) {
		t.Errorf("not json: %v", err)
	}
}

func TestDoneFramesCarryTheTurn(t *testing.T) {
	f, err := parseChatFrame([]byte(`{"type":"done","full_response":"Hi there","model":"m","provider":"p","input_tokens":3,
		"output_tokens":2,"tokens_used":5,"cost_usd":null,"max_context_tokens":8192,"last_input_tokens":3}`))
	if err != nil || f.Done == nil || f.Done.FullResponse != "Hi there" || *f.Done.TokensUsed != 5 || *f.Done.MaxContextTokens != 8192 {
		t.Fatalf("%+v %v", f.Done, err)
	}
}
