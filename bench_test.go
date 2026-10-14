package zeroclaw

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func BenchmarkEventStream(b *testing.B) {
	frame := ": keep-alive\n\ndata: {\"type\":\"agent_end\",\"agent_alias\":\"helper\",\"tokens_used\":1234,\"cost_usd\":0.01}\n\n"
	body := strings.Repeat(frame, 1000)
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for range b.N {
		s := newEventStream(io.NopCloser(strings.NewReader(body)))
		for {
			if _, err := s.Next(); err != nil {
				break
			}
		}
	}
}

func BenchmarkParseChatFrame(b *testing.B) {
	frame := []byte(`{"type":"tool_call","id":"c1","name":"search","args":{"q":"orders pending","limit":20}}`)
	b.ReportAllocs()
	for range b.N {
		if _, err := parseChatFrame(frame); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCall(b *testing.B) {
	g, client := gateway(b)
	g.on(http.MethodGet, "/api/config/map-keys", reply(`{"path":"agents","keys":["a","b","c"]}`))
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := client.Config().MapKeys(ctx, "agents"); err != nil {
			b.Fatal(err)
		}
	}
}
