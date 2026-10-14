# Chatting with an agent

## Over the WebSocket

`client.Chat(ctx, alias, opts)` opens `/ws/chat` for agent `alias`. The
connection is a session (`gw_<session_id>`); pass
`zeroclaw.ChatOptions{SessionID: …}` to resume one. `ctx` bounds the
opening only; each later call takes its own.

`Ask` sends a message and waits for the reply:

```go
chat, err := client.Chat(ctx, "helper", zeroclaw.ChatOptions{})
if err != nil {
	return err
}
defer chat.Close()
reply, err := chat.Ask(ctx, "How many orders are pending?")
if err != nil {
	return err
}
fmt.Println(reply.FullResponse)
if reply.TokensUsed != nil {
	fmt.Println(*reply.TokensUsed, "tokens")
}
```

To show progress, or to answer tool approvals, read frames one by one.
`frame.Type` is one of the `Frame*` constants and says which fields are set:

```go
chat, err := client.Chat(ctx, "helper", zeroclaw.ChatOptions{})
if err != nil {
	return err
}
defer chat.Close()
if err := chat.Send(ctx, "Archive last month's invoices"); err != nil {
	return err
}
for {
	frame, err := chat.NextFrame(ctx)
	if errors.Is(err, io.EOF) {
		return nil // the gateway closed the chat
	}
	if err != nil {
		return err
	}
	switch frame.Type {
	case zeroclaw.FrameChunk:
		fmt.Print(frame.Content) // append to the screen
	case zeroclaw.FrameToolCall:
		fmt.Println("running", frame.Name)
	case zeroclaw.FrameApprovalRequest:
		if err := chat.AnswerApproval(ctx, frame.RequestID, "approve"); err != nil {
			return err
		}
	case zeroclaw.FrameDone:
		return nil // frame.Done holds the tokens and cost
	case zeroclaw.FrameError:
		return &zeroclaw.TurnError{Code: frame.Code, Message: frame.Message}
	}
}
```

Frames are read by one goroutine at a time; sending is safe from any. A
message sent while a turn runs steers it. Closing the chat, or
`Sessions().Abort(ctx, id)`, cancels the running turn. `Ask` returns a failed
turn as a `*zeroclaw.TurnError` with its `Code` (`PROVIDER_ERROR`,
`SESSION_QUEUE_FULL`, `ABORTED`, …).

## One-shot: webhook and A2A

`client.Webhook().Send(ctx, message, opts)` posts to `/webhook` and returns
the agent's reply in one HTTP call. `zeroclaw.WebhookOptions` carries the
agent, the `X-Webhook-Secret` when the gateway sets one, an idempotency key
(a repeat answers a `WebhookReply` with `Duplicate` set, without running
again) and a session id to keep calls in one conversation.

`client.Webhook().A2ASend(ctx, alias, text)` does the same over the A2A
protocol (`message/send`) for agents published with `a2a.server.enabled`.
