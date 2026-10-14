# Connecting and pairing

A ZeroClaw gateway with pairing on (the default) answers only clients that
send a paired bearer token: `Authorization: Bearer zc_<64 hex>`. A client
gets one by exchanging a six-digit pairing code, which is single use and
valid for ten minutes.

## Getting a code

- **First run:** the daemon prints a code when it starts and no client is
  paired yet; [`Pairing().InitialCode(ctx)`](../reference/endpoints.md) also
  returns it until the first pairing.
- **From a paired client:** `Pairing().NewCode(ctx)` mints one for another
  device.
- **On the gateway host:** `Pairing().AdminCode(ctx)` and
  `Pairing().AdminNewCode(ctx, rotate)` (loopback only); `rotate` can also
  revoke every token (`"all"`) or one device's.

## Exchanging it

```go
anonymous, err := zeroclaw.New("http://127.0.0.1:42617")
if err != nil {
	return err
}
paired, err := anonymous.Pairing().Pair(ctx, "123456", "billing-service", "server")
if err != nil {
	return err
}
// Store paired.Token as a secret; it is shown only once.
client := anonymous.WithToken(paired.Token)
```

`WithToken` returns a client for the same gateway that shares the connection
pool. A wrong or expired code is refused with 400; too many attempts are
rate-limited with 429 (`zeroclaw.IsRateLimited(err)`), with a `retry_after`
field in the error body (`APIError.Body`).

## Managing devices

`Pairing().Devices(ctx)` lists the paired devices, `RevokeDevice(ctx, id)`
revokes one and its token, and `RotateDeviceToken(ctx, id)` revokes it and
mints a code for it to pair again.

## Keeping the token safe

The client never prints its token: `Client`'s `String` and `GoString` (what
`%v` and `%#v` print) show only the base URL and whether it is
authenticated, and `Paired`'s hide the token too. The token never appears in
an error or a URL.
