# Channels

A channel is named `type.alias` (`whatsapp.main`, `telegram.bot`), as
`Channels().List(ctx)` returns it with its status, health and readiness.

## Identities and logins

- `Bind(ctx, channelType, alias, identity)` allows a Telegram, WeChat or
  LINE user to talk to the channel.
- `Relink(ctx, channel)` clears a QR-paired channel's saved login (WhatsApp
  Web); after `System().Reload(ctx)` the channel asks to be paired again.

## Sending

The official gateway has no route that sends through a channel without an
agent turn. To post a message, let an agent do it: a one-off cron job with a
`Delivery` announces its output on a channel (see `examples/announce`).
