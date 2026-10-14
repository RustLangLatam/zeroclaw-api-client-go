# Guides

Each guide covers one area of the gateway and the file that calls it:

| Guide | File | Accessor |
|---|---|---|
| [Configuration](./configuration.md) | `config.go` | `client.Config()` |
| [Agents, personality and skills](./agents.md) | `agents.go`, `skills.go`, `files.go` | `client.Agent(alias)`, `client.Skills()`, `client.Files()` |
| [Channels and group chats](./channels.md) | `channels.go` | `client.Channels()` |
| [Chatting with an agent](./chat.md) | `chat.go`, `webhook.go` | `client.Chat(ctx, alias, …)`, `client.Webhook()` |
| [Sessions and live events](./events.md) | `sessions.go`, `events.go` | `client.Sessions()`, `client.Events()` |
| [Cron, memory and cost](./operations.md) | `cron.go`, `memory.go`, `cost.go`, `system.go` | `client.Cron()`, `client.Memory()`, `client.Cost()`, `client.System()` |
| [SOPs](./sops.md) | `sops.go`, `canvas.go`, `quickstart.go` | `client.Sops()`, `client.Canvas()`, `client.Quickstart()` |
| [Resilience](./resilience.md) | `transport.go` | `zeroclaw.New` options |
| [Errors](./errors.md) | `errors.go` | — |

Pairing is covered in [Getting Started](../getting-started/pairing.md);
WebAuthn (`client.WebAuthn()`) only carries the JSON of a hardware-key
ceremony run elsewhere.

The snippets run inside a function that returns an `error`, with a
`ctx context.Context` and a `client *zeroclaw.Client` at hand.
