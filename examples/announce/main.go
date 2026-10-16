// Command announce posts a message on a channel through an agent's channel
// binding: a one-off shell cron job whose raw output is announced, run at
// once and deleted.
//
//	ZEROCLAW_URL=… ZEROCLAW_TOKEN=… go run ./examples/announce helper whatsapp.main +34600000000 "Hello"
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
)

func main() {
	if len(os.Args) != 5 {
		log.Fatal("usage: announce <agent> <channel> <to> <text>")
	}
	agent, channel, to, text := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	client, err := zeroclaw.New(gatewayURL(), zeroclaw.WithToken(os.Getenv("ZEROCLAW_TOKEN")))
	if err != nil {
		log.Fatal(err)
	}
	ctx, cron := context.Background(), client.Cron()

	// echo is what an agent's shell policy allows; quote for the shell.
	delivery := zeroclaw.AnnounceDelivery(channel, to)
	job, err := cron.Create(ctx, &zeroclaw.NewCronJob{
		Agent: agent,
		Name:  "announce-example",
		// Never due on its own: it is run by hand right below.
		Schedule:          "0 0 1 1 *",
		JobType:           "shell",
		Command:           "echo '" + strings.ReplaceAll(text, "'", `'\''`) + "'",
		ShellOutputFormat: "raw",
		Delivery:          &delivery,
	})
	if err != nil {
		log.Fatal(err)
	}
	run, runErr := cron.Run(ctx, job.ID)
	if err := cron.Delete(ctx, job.ID); err != nil {
		log.Fatal(err)
	}
	if runErr != nil {
		log.Fatal(runErr)
	}
	result := "failed"
	if run.Success {
		result = "sent"
	}
	fmt.Printf("%s in %d ms\n", result, run.DurationMs)
}

func gatewayURL() string {
	if url := os.Getenv("ZEROCLAW_URL"); url != "" {
		return url
	}
	return "http://127.0.0.1:42617"
}
