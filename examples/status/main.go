// Command status prints the gateway's version, model and channels.
//
//	ZEROCLAW_URL=http://127.0.0.1:42617 ZEROCLAW_TOKEN=zc_… go run ./examples/status
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
)

func main() {
	client, err := zeroclaw.New(gatewayURL(), zeroclaw.WithToken(os.Getenv("ZEROCLAW_TOKEN")))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	status, err := client.System().Status(ctx, "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ZeroClaw %s — model %s\n", status.Version, status.Model)
	channels, err := client.Channels().List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, ch := range channels {
		fmt.Printf("  %-40s %-10s %s\n", ch.Name, ch.Status, ch.Health)
	}
}

func gatewayURL() string {
	if url := os.Getenv("ZEROCLAW_URL"); url != "" {
		return url
	}
	return "http://127.0.0.1:42617"
}
