package zeroclaw

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// eventsFrom feeds chunks to the SSE parser one read at a time.
func eventsFrom(chunks ...string) *EventStream {
	readers := make([]io.Reader, len(chunks))
	for i, c := range chunks {
		readers[i] = strings.NewReader(c)
	}
	return newEventStream(io.NopCloser(io.MultiReader(readers...)))
}

// eventKinds returns the event types of chunks: "log" for log events,
// "error" for a frame that is not JSON.
func eventKinds(t *testing.T, chunks ...string) []string {
	t.Helper()
	stream := eventsFrom(chunks...)
	kinds := []string{}
	for {
		e, err := stream.Next()
		switch {
		case errors.Is(err, io.EOF):
			return kinds
		case errors.Is(err, ErrDecode):
			kinds = append(kinds, "error")
		case err != nil:
			t.Fatal(err)
		case e.IsLog():
			kinds = append(kinds, "log")
		case e.Kind() == "":
			kinds = append(kinds, "none")
		default:
			kinds = append(kinds, e.Kind())
		}
	}
}

func TestSSEVectors(t *testing.T) {
	vectors := []struct {
		chunks []string
		want   []string
	}{
		{[]string{"data: {\"type\":\"a\"}\n\n"}, []string{"a"}},
		{[]string{"data: {\"type\":\"a\"}\n\ndata: {\"type\":\"b\"}\n\n"}, []string{"a", "b"}},
		{[]string{"data: {\"ty", "pe\":\"a\"}\n", "\n"}, []string{"a"}},
		{[]string{"data: {\"type\":\"a\"}\r\n\r\n"}, []string{"a"}},
		{[]string{": keep-alive\n\n", ": keep-alive\n\n"}, []string{}},
		{[]string{"event: x\nid: 3\ndata: {\"type\":\"a\"}\n\n"}, []string{"a"}},
		{[]string{"data: {\"@timestamp\":\"t\"}\n\n"}, []string{"log"}},
		{[]string{"data: not json\n\ndata: {\"type\":\"a\"}\n\n"}, []string{"error", "a"}},
		{[]string{"data: {\"type\":\"a\"}"}, []string{"a"}},
	}
	for _, v := range vectors {
		if got := eventKinds(t, v.chunks...); !slices.Equal(got, v.want) {
			t.Errorf("%q: got %v, want %v", v.chunks, got, v.want)
		}
	}
}

func TestSSEMultiLineDataIsJoined(t *testing.T) {
	if got := eventKinds(t, "data: {\"type\":\ndata: \"a\"}\n\n"); !slices.Equal(got, []string{"a"}) {
		t.Errorf("got %v", got)
	}
}

func TestSSEFramesSplitAcrossChunksBecomeEvents(t *testing.T) {
	stream := eventsFrom(
		": keep-alive\n\ndata: {\"type\":\"agent_start\",\"mo",
		"del\":\"m\"}\r\n\r\ndata: {\"@timestamp\":\"t\"}\n",
		"\n",
	)
	first := must[Event](t)(stream.Next())
	var model string
	if err := first.Decode("model", &model); err != nil || first.Kind() != "agent_start" || model != "m" {
		t.Errorf("first: %v (%v)", first, err)
	}
	if second := must[Event](t)(stream.Next()); !second.IsLog() || second.Kind() != "" {
		t.Errorf("second: %v", second)
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("end: %v", err)
	}
}

func TestSSEALastFrameWithoutItsBlankLineStillCounts(t *testing.T) {
	if got := eventKinds(t, "data: {\"type\":\"cron_result\"}"); len(got) != 1 {
		t.Errorf("got %v", got)
	}
}

func TestEventsStreamAndReplay(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, ": keep-alive\n\n"+
			"data: {\"type\":\"agent_end\",\"source\":\"observability\",\"cost_usd\":0.01}\n\n"+
			"data: {\"@timestamp\":\"t\",\"event\":{\"category\":\"channel\"}}\n\n")
	})
	g.on("GET", "/api/events/history", reply(`{"events":[{"type":"tool_call","tool":"shell"}]}`))

	ctx := context.Background()
	stream := must[*EventStream](t)(client.Events().Subscribe(ctx))
	defer stream.Close()
	var events []Event
	for {
		e, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	if len(events) != 2 || events[0].Kind() != "agent_end" || !events[1].IsLog() {
		t.Errorf("events: %v", events)
	}
	if got := must[[]Event](t)(client.Events().History(ctx)); got[0].Kind() != "tool_call" {
		t.Errorf("history: %v", got)
	}
}
