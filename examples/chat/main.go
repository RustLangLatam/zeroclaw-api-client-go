// Command chat asks an agent one question and streams its reply.
//
//	ZEROCLAW_URL=… ZEROCLAW_TOKEN=… go run ./examples/chat helper "Hello"
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: chat <agent> <question>")
	}
	agent, question := os.Args[1], os.Args[2]
	client, err := zeroclaw.New(gatewayURL(), zeroclaw.WithToken(os.Getenv("ZEROCLAW_TOKEN")))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	chat, err := client.Chat(ctx, agent, zeroclaw.ChatOptions{})
	if err != nil {
		log.Fatal(err)
	}
	defer chat.Close()
	if err := chat.Send(ctx, question); err != nil {
		log.Fatal(err)
	}
	for {
		frame, err := chat.NextFrame(ctx)
		if err != nil {
			log.Fatal(err)
		}
		switch frame.Type {
		case zeroclaw.FrameChunk:
			fmt.Print(frame.Content)
		case zeroclaw.FrameToolCall:
			fmt.Printf("\n[tool: %s]\n", frame.Name)
		case zeroclaw.FrameDone:
			var tokens uint64
			if frame.Done.TokensUsed != nil {
				tokens = *frame.Done.TokensUsed
			}
			fmt.Printf("\n— %d tokens\n", tokens)
			return
		case zeroclaw.FrameError:
			log.Fatal(frame.Message)
		}
	}
}

func gatewayURL() string {
	if url := os.Getenv("ZEROCLAW_URL"); url != "" {
		return url
	}
	return "http://127.0.0.1:42617"
}
