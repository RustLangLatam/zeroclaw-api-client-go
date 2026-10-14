// Command pairing exchanges a pairing code for a bearer token, then lists
// the paired devices with it.
//
//	ZEROCLAW_URL=http://127.0.0.1:42617 go run ./examples/pairing 123456 my-service
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: pairing <code> [device name]")
	}
	code, name := os.Args[1], ""
	if len(os.Args) > 2 {
		name = os.Args[2]
	}
	anonymous, err := zeroclaw.New(gatewayURL())
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	paired, err := anonymous.Pairing().Pair(ctx, code, name, "service")
	if err != nil {
		log.Fatal(err)
	}
	client := anonymous.WithToken(paired.Token)
	fmt.Printf("Paired. Keep this token secret:\n%s\n\n", paired.Token)

	devices, err := client.Pairing().Devices(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range devices {
		name := ""
		if d.Name != nil {
			name = *d.Name
		}
		fmt.Printf("%s  %-20s paired %s\n", d.ID, name, d.PairedAt)
	}
}

func gatewayURL() string {
	if url := os.Getenv("ZEROCLAW_URL"); url != "" {
		return url
	}
	return "http://127.0.0.1:42617"
}
