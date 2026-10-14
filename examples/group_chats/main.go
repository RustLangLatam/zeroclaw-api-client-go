// Command group_chats lists every channel, the group chats it takes part in
// and their members. Read-only.
//
//	ZEROCLAW_URL=… ZEROCLAW_TOKEN=… go run ./examples/group_chats
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
	ctx, channels := context.Background(), client.Channels()

	list, err := channels.List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, ch := range list {
		fmt.Printf("%s (%s, %s)\n", ch.Name, ch.Status, ch.Health)
		rooms, err := channels.Rooms(ctx, ch.Name)
		if err != nil {
			// Not running, or a build without the route: say why and go on.
			fmt.Printf("  rooms unavailable: %v\n", err)
			continue
		}
		for _, room := range rooms {
			fmt.Printf("  %s — %s (%d members)\n", room.Name, room.ID, room.Participants)
			members, err := channels.RoomMembers(ctx, ch.Name, room.ID)
			if err != nil {
				log.Fatal(err)
			}
			for _, m := range members {
				role := "member"
				if m.Admin {
					role = "admin"
				}
				fmt.Printf("    %-20s %s\n", m.ID, role)
			}
		}
	}
}

func gatewayURL() string {
	if url := os.Getenv("ZEROCLAW_URL"); url != "" {
		return url
	}
	return "http://127.0.0.1:42617"
}
