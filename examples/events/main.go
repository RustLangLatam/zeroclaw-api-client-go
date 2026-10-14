// Command events follows the gateway's live events and prints one line per
// event: agent turns with their tokens and cost, tool calls, cron results
// and log events. Reconnects when the gateway restarts.
//
//	ZEROCLAW_URL=… ZEROCLAW_TOKEN=… go run ./examples/events
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
)

func main() {
	client, err := zeroclaw.New(gatewayURL(), zeroclaw.WithToken(os.Getenv("ZEROCLAW_TOKEN")))
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	for ctx.Err() == nil {
		events, err := client.Events().Subscribe(ctx)
		if zeroclaw.IsTransient(err) {
			fmt.Printf("gateway unavailable (%v); retrying\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if err != nil {
			log.Fatal(err)
		}
		follow(events)
		events.Close()
		fmt.Println("stream closed; reconnecting")
	}
}

func follow(events *zeroclaw.EventStream) {
	for {
		event, err := events.Next()
		if errors.Is(err, zeroclaw.ErrDecode) {
			continue // one bad frame; the stream goes on
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Println("stream failed:", err)
			}
			return
		}
		field := func(name string) string { return string(event[name]) }
		switch kind := event.Kind(); {
		case kind == "agent_end":
			fmt.Printf("turn    %s %s tokens $%s\n", field("agent_alias"), field("tokens_used"), field("cost_usd"))
		case kind == "tool_call":
			fmt.Printf("tool    %s %s ok=%s\n", field("agent_alias"), field("tool"), field("success"))
		case kind == "cron_result":
			fmt.Printf("cron    %s ok=%s\n", field("job_id"), field("success"))
		case kind != "":
			raw, _ := json.Marshal(event)
			fmt.Printf("%-7s %s\n", kind, raw)
		case event.IsLog():
			fmt.Printf("log     %s\n", field("message"))
		}
	}
}

func gatewayURL() string {
	if url := os.Getenv("ZEROCLAW_URL"); url != "" {
		return url
	}
	return "http://127.0.0.1:42617"
}
