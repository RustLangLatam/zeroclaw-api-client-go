// Command config reads configuration fields, lists the agents, and, with
// -set, changes an agent's model atomically: the patch only applies if the
// model is still the one read.
//
//	ZEROCLAW_URL=… ZEROCLAW_TOKEN=… go run ./examples/config
//	ZEROCLAW_URL=… ZEROCLAW_TOKEN=… go run ./examples/config -set helper gpt-5.5
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
)

func main() {
	set := flag.Bool("set", false, "change <agent>'s model to <model>")
	flag.Parse()
	client, err := zeroclaw.New(gatewayURL(), zeroclaw.WithToken(os.Getenv("ZEROCLAW_TOKEN")))
	if err != nil {
		log.Fatal(err)
	}
	ctx, config := context.Background(), client.Config()

	list, err := config.List(ctx, "gateway")
	if err != nil {
		log.Fatal(err)
	}
	for _, entry := range list.Entries[:min(8, len(list.Entries))] {
		value := "<secret>"
		if entry.Value != nil {
			value = string(entry.Value)
		}
		fmt.Printf("%-40s %-10s %s\n", entry.Path, entry.Kind, value)
	}
	agents, err := config.MapKeys(ctx, "agents")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("agents:", agents)

	if !*set {
		return
	}
	if flag.NArg() != 2 {
		log.Fatal("usage: config -set <agent> <model>")
	}
	agent, model := flag.Arg(0), flag.Arg(1)
	path := "agents." + agent + ".model"
	current, err := config.Get(ctx, path)
	if err != nil {
		log.Fatal(err)
	}
	if current.IsSecret() {
		log.Fatal("the model is a secret?")
	}
	patched, err := config.Patch(ctx, []zeroclaw.PatchOp{
		zeroclaw.TestOp(path, current.Value),
		zeroclaw.ReplaceOp(path, model),
		zeroclaw.CommentOp(path, "changed with the config example"),
	}, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %s -> %s (saved: %t)\n", path, current.Value, model, patched.Saved)
	if pending, err := config.PendingReload(ctx); err == nil && pending {
		fmt.Println("a daemon reload is due for every subsystem to see it")
	}
}

func gatewayURL() string {
	if url := os.Getenv("ZEROCLAW_URL"); url != "" {
		return url
	}
	return "http://127.0.0.1:42617"
}
