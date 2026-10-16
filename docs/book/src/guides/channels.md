# Channels and group chats

A channel is named `type.alias` (`whatsapp.main`, `telegram.bot`), as
`Channels().List(ctx)` returns it with its status, health and readiness.

## What the gateway requires

| Action | Needs |
|---|---|
| list channels, bind an identity, relink | a token |
| list rooms and members | a token, pairing on, the channel configured and running |
| send, create rooms, invite, remove members | the above, plus the channel in `[gateway] send_channels` |
| create rooms and manage members on WhatsApp Web | also `room_management = true` on the channel |

Refusals carry the channel and the reason
(`{"channel": "whatsapp.main", "error": "…"}`), which the
[error](./errors.md) keeps as its message.

## Sending

`Channels().Send(ctx, channel, to, content, threadID)` delivers plain text
with no agent turn: for notifications a person, not the agent, is sending.
An empty `threadID` sends outside a thread. With
`[gateway] send_groups_only`, only group chats can be addressed.

## Group chats

```go
channels := client.Channels()
created, err := channels.CreateRoom(ctx, "whatsapp.main", "Orders team", "", []string{"+34600000000"})
if err != nil {
	return err
}
rooms, err := channels.Rooms(ctx, "whatsapp.main")
if err != nil {
	return err
}
for _, room := range rooms {
	members, err := channels.RoomMembers(ctx, "whatsapp.main", room.ID)
	if err != nil {
		return err
	}
	fmt.Println(room.Name, len(members))
}
if _, err := channels.Invite(ctx, "whatsapp.main", created.Room, "+34611111111"); err != nil {
	return err
}
if err := channels.RemoveMember(ctx, "whatsapp.main", created.Room, "+34611111111"); err != nil {
	return err
}
```

On WhatsApp, invitees are also added to the channel's allowlist, and the new
group to its allowed groups, so the agent answers in it. Creating rooms and
invitations are rate-limited per channel (`[gateway] rooms_per_hour`,
`invites_per_hour`, `invites_per_day`): over the limit the call fails with
429.

## Identities and logins

- `Bind(ctx, channelType, alias, identity)` allows a Telegram, WeChat or
  LINE user to talk to the channel.
- `Relink(ctx, channel)` clears a QR-paired channel's saved login (WhatsApp
  Web); after `System().Reload(ctx)` the channel asks to be paired again.
