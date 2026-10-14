# First calls

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	zeroclaw "github.com/RustLangLatam/zeroclaw-api-client-go"
)

func main() {
	ctx := context.Background()
	client, err := zeroclaw.New("http://127.0.0.1:42617",
		zeroclaw.WithToken(os.Getenv("ZEROCLAW_TOKEN")))
	if err != nil {
		log.Fatal(err)
	}

	// Public: no token needed.
	health, err := client.System().Health(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(health.Status)

	// Version, model, channels and process figures.
	status, err := client.System().Status(ctx, "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(status.Version, status.Model)

	// Every configured channel and its state.
	channels, err := client.Channels().List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, channel := range channels {
		fmt.Println(channel.Name, channel.Status, channel.Health)
	}

	// One configuration field.
	port, err := client.Config().Get(ctx, "gateway.port")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(port.Value))
}
```

## Timeouts and retries

A request waits 60 seconds for the gateway's answer unless an option says
otherwise (`zeroclaw.WithTimeout`); a manual cron run waits up to 660, as
the gateway allows it 600. The timeout bounds the answer's headers only, so
the live event stream is never cut, and the `ctx` you pass bounds the whole
call. Failed idempotent requests are retried three times with backoff. See
[Resilience](../guides/resilience.md).

## Examples

The module ships runnable examples under `examples/`:

| Example | Shows |
|---|---|
| `status` | version, model and channels |
| `pairing` | exchanging a pairing code for a token |
| `config` | reading fields, listing agents, an atomic patch |
| `group_chats` | channels, their group chats and members |
| `events` | following the live event stream, reconnecting |
| `chat` | streaming an agent's reply over the WebSocket |
| `announce` | posting on a channel through a one-off cron job |
| `resilient` | timeouts, retries, rate limit, circuit breaker, concurrency limit and a custom middleware |

```sh
ZEROCLAW_URL=http://127.0.0.1:42617 ZEROCLAW_TOKEN=zc_… \
  go run ./examples/status
ZEROCLAW_URL=… ZEROCLAW_TOKEN=… go run ./examples/chat helper "Hello"
```
