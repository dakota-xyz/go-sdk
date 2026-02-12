# Dakota Go SDK

Go SDK for Dakota Platform integrations, focused on secure webhook handling.

## Packages

- `github.com/dakota-xyz/go-sdk/errors`
- `github.com/dakota-xyz/go-sdk/log`
- `github.com/dakota-xyz/go-sdk/webhook`
- `github.com/dakota-xyz/go-sdk/webhook/idempotency`
- `github.com/dakota-xyz/go-sdk/webhook/types`

## Import aliases

Because this SDK has `errors` and `log` packages, aliasing avoids collisions
with standard library imports:

```go
import (
    sdkerrors "github.com/dakota-xyz/go-sdk/errors"
    sdklog "github.com/dakota-xyz/go-sdk/log"
)
```

## Install

```bash
go get github.com/dakota-xyz/go-sdk
```

## Webhook quick start

```go
event, err := webhook.ConstructEvent(
    payload,
    r.Header.Get(webhook.SignatureHeader),
    r.Header.Get(webhook.TimestampHeader),
    publicKeyHex,
)
if err != nil {
    // signature, timestamp, or payload validation failed
    return
}
_ = event
```

## HTTP handler

```go
handler, err := webhook.NewHandler(
    webhook.WithPublicKey(publicKeyHex),
    webhook.WithIdempotencyStore(idempotency.NewMemoryStore()),
    webhook.WithAckPolicy(webhook.AckOnSuccess),
    webhook.OnDefault(func(ctx context.Context, event webhook.Event) error {
        // process event
        return nil
    }),
)
if err != nil {
    panic(err)
}

http.Handle("/webhook", handler)
```

## Delivery acknowledgement policy

- `webhook.AckOnSuccess` (default): returns `2xx` only when delivery succeeds.
- `webhook.AckAlways`: always returns `2xx`, even when delivery fails.

`AckOnSuccess` is safer for at-least-once delivery because upstream retries on failures.

## Idempotency behavior

When an idempotency store is configured:

1. The handler atomically acquires a reservation for an event ID.
2. The event is delivered.
3. On successful delivery, the event ID is committed as processed.
4. On failed delivery, the reservation is released for retry.

`webhook/idempotency.NewMemoryStore()` implements this atomic flow.

## Listener

```go
listener, err := webhook.NewListener(
    webhook.WithAddr("127.0.0.1:0"),
    webhook.WithHandlerOptions(
        webhook.WithPublicKey(publicKeyHex),
        webhook.WithChannel(100),
    ),
)
if err != nil {
    panic(err)
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
    _ = listener.Start(ctx)
}()

addr := listener.Addr() // concrete bound address after start
_ = addr
```

Listener also exposes `GET /healthz` for liveness checks.

## Typed event payloads

```go
customer, err := webhook.EventDataAs[types.CustomerData](event)
if err != nil {
    // malformed payload for this type
    return
}
_ = customer
```

## Errors

Errors are structured and support `errors.Is` / `errors.As`:

```go
if sdkerrors.Is(err, sdkerrors.ErrInvalidSignature) {
    // handle invalid signature
}
```

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
```
