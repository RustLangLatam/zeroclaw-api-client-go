package zeroclaw

// The gateway's live event stream (Server-Sent Events) and its recent
// history.
//
// Gateway handler: crates/zeroclaw-gateway/src/sse.rs. Every frame is
// "data: <one JSON object>" with no event name, so frames are told apart by
// their content: observer events and cron results carry a "type"
// (llm_request, tool_call, tool_call_start, agent_start, agent_end, error,
// history_trimmed, cron_result, …); structured log events carry
// "@timestamp" instead. Frames missed while lagging are not replayed.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// EventsAPI holds the event endpoints, from [Client.Events].
type EventsAPI struct{ c *Client }

// Events returns the live event stream endpoints (/api/events).
func (c *Client) Events() EventsAPI { return EventsAPI{c} }

// Subscribe opens the live stream (GET /api/events). The stream ends when
// the gateway closes the connection (e.g. on reload); subscribe again to
// resume. The client's timeout bounds only the wait for the stream to open,
// never the stream itself; ctx bounds the whole stream.
func (a EventsAPI) Subscribe(ctx context.Context) (*EventStream, error) {
	req, err := a.c.request(ctx, http.MethodGet, []string{"api", "events"}, nil, nil, "")
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := a.c.send(req)
	if err != nil {
		return nil, err
	}
	return newEventStream(resp.Body), nil
}

// History returns the latest observer events, oldest first (up to 500, kept
// in memory): GET /api/events/history.
func (a EventsAPI) History(ctx context.Context) ([]Event, error) {
	return callField[[]Event](ctx, a.c, http.MethodGet, []string{"api", "events", "history"}, nil, nil, "events")
}

// Event is one event, as the gateway sent it: its top-level fields, each
// kept as raw JSON.
type Event map[string]json.RawMessage

// Kind is the event's "type" (agent_end, cron_result, …); empty for log
// events.
func (e Event) Kind() string {
	var s string
	json.Unmarshal(e["type"], &s)
	return s
}

// IsLog reports whether it is a structured log event (@timestamp, event,
// service, zeroclaw attribution).
func (e Event) IsLog() bool {
	_, ok := e["@timestamp"]
	return ok
}

// Decode decodes field into out; it fails when the field is absent.
func (e Event) Decode(field string, out any) error {
	raw, ok := e[field]
	if !ok {
		return fmt.Errorf("%w: event has no field %q", ErrDecode, field)
	}
	return decode(raw, out)
}

// EventStream is the live stream of [EventsAPI.Subscribe]. Read it with
// [EventStream.Next] and close it when done.
type EventStream struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
	data    bytes.Buffer
	done    bool
}

func newEventStream(body io.ReadCloser) *EventStream {
	s := bufio.NewScanner(body)
	s.Buffer(make([]byte, 0, 64<<10), 16<<20) // a frame may be one long line
	return &EventStream{body: body, scanner: s}
}

// Next returns the next event. It returns [io.EOF] once the gateway closed
// the stream. A frame that is not JSON is an error wrapping [ErrDecode];
// the stream goes on after it. Any other error ends the stream.
func (s *EventStream) Next() (Event, error) {
	for !s.done {
		// "data:" lines are joined until a blank line ends the frame;
		// comments (keep-alives) and other fields are skipped. The scanner
		// accepts CRLF line endings.
		more := s.scanner.Scan()
		line := s.scanner.Bytes()
		if !more {
			if err := s.scanner.Err(); err != nil {
				s.done = true
				return nil, fmt.Errorf("request to the gateway failed: reading the event stream: %w", err)
			}
			// A last frame without its blank line still counts.
			s.done, line = true, nil
		}
		if data, ok := bytes.CutPrefix(line, []byte("data:")); ok {
			if s.data.Len() > 0 {
				s.data.WriteByte('\n')
			}
			s.data.Write(bytes.TrimPrefix(data, []byte(" ")))
			continue
		}
		if len(line) > 0 || s.data.Len() == 0 {
			continue
		}
		var e Event
		err := json.Unmarshal(s.data.Bytes(), &e)
		s.data.Reset()
		if err != nil {
			return nil, fmt.Errorf("%w: event is not JSON: %w", ErrDecode, err)
		}
		return e, nil
	}
	return nil, io.EOF
}

// Close closes the stream.
func (s *EventStream) Close() error { return s.body.Close() }
