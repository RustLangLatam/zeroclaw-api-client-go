# Sessions and live events

## Sessions

Every conversation is a session: web chats are `gw_<id>`, channel
conversations `whatsapp_<channel>_<peer>…`. `Sessions().List(ctx)` returns
them with their agent and channel; `Messages(ctx, id)` the transcript (tool
calls and results included, as JSON text); `State(ctx, id)` whether a turn
is running.

`AddMessage(ctx, id, content)` appends an assistant message (shown to chat
clients, with no agent turn); `Rename`, `Abort` and `Delete` do what they
say.

## The live event stream

`Events().Subscribe(ctx)` follows `/api/events`, the gateway's Server-Sent
Events stream. Read it with `Next` until `io.EOF`:

```go
events, err := client.Events().Subscribe(ctx)
if err != nil {
	return err
}
defer events.Close()
for {
	event, err := events.Next()
	if errors.Is(err, io.EOF) {
		return nil // the gateway closed it: subscribe again
	}
	if errors.Is(err, zeroclaw.ErrDecode) {
		continue // a frame that is not JSON; the stream goes on
	}
	if err != nil {
		return err
	}
	switch {
	case event.Kind() == "agent_end":
		var cost float64 // tokens and cost of a turn
		if err := event.Decode("cost_usd", &cost); err == nil {
			fmt.Println("turn cost", cost)
		}
	case event.Kind() == "cron_result":
		// a job ran
	case event.IsLog():
		// a structured log event
	}
}
```

An `Event` is a `map[string]json.RawMessage` of the frame's top-level
fields. Frames have no SSE event name; `Event.Kind` reads their `type`.
Observer events (`llm_request`, `tool_call`, `tool_call_start`,
`agent_start`, `agent_end`, `error`, `history_trimmed`) carry
`source: "observability"`; log events carry `@timestamp` instead of a
`type`. Frames of a single session are not published here; they reach that
session's chat. The stream ends when the gateway closes it (on reload, for
instance): subscribe again. `Events().History(ctx)` returns the latest
observer events, kept in memory.

The client's timeout bounds only the wait for the stream to open; once
events flow it stays open as long as the gateway keeps it, or until `ctx`
ends or you call `Close`.
