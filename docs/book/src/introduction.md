# ZeroClaw API Client

`zeroclaw-api-client-go` is a typed Go client for the
[ZeroClaw](https://github.com/zeroclaw-labs/zeroclaw) gateway: the HTTP and
WebSocket surface a ZeroClaw daemon exposes for its dashboard, its CLI and
third-party tooling. It is the Go port of
[zeroclaw-api-client-rust](https://github.com/RustLangLatam/zeroclaw-api-client-rust),
and covers the same routes.

With it an application can do everything the gateway allows a client to do:

- read and change the configuration, one field at a time or in atomic
  batches, including agents, providers and channels;
- pair new clients and manage the paired devices;
- see channels, send through them, and manage WhatsApp group chats and their
  members;
- chat with an agent, streaming its reply, tool calls and approval requests;
- follow the live event stream and read session transcripts;
- schedule cron jobs, store and recall memory, and read usage and cost;
- edit agent workspaces, personality files, skills and SOPs, and run SOPs;
- check health, run diagnostics, read logs and upgrade the daemon.

It is a library: it never prints, logs or starts goroutines that outlive a
call. Every call returns a typed result or a typed [error](./guides/errors.md)
carrying the gateway's own explanation.

This is an independent, community-maintained client. It is not affiliated
with or endorsed by ZeroClaw Labs.

## How it is organized

A [`Client`](./architecture/overview.md) holds the gateway URL and the bearer
token. Each group of endpoints is reached through an accessor, so call sites
read like the gateway's routes:

```go
client, err := zeroclaw.New("http://127.0.0.1:42617", zeroclaw.WithToken("zc_…"))
if err != nil {
	return err
}

channels, err := client.Channels().List(ctx)
_, err = client.Config().Set(ctx, "agents.helper.model", "gpt-5.4-mini", "")
chat, err := client.Chat(ctx, "helper", zeroclaw.ChatOptions{})
reply, err := chat.Ask(ctx, "Hi")
fmt.Println(len(channels), reply.FullResponse)
```

Every call takes a `context.Context` first and returns an error to check
(the checks are left out above). Start with [Getting Started](./getting-started/index.md).
The [endpoint map](./reference/endpoints.md) lists every route and the
function that calls it.
