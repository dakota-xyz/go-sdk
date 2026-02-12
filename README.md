# Dakota Go SDK

Go SDK for Dakota Platform integrations.

## Packages

- `github.com/dakota-xyz/go-sdk/client`
- `github.com/dakota-xyz/go-sdk/client/gen`
- `github.com/dakota-xyz/go-sdk/errors`
- `github.com/dakota-xyz/go-sdk/log`
- `github.com/dakota-xyz/go-sdk/webhook`
- `github.com/dakota-xyz/go-sdk/webhook/idempotency`
- `github.com/dakota-xyz/go-sdk/webhook/types`

## Import aliases

Because this SDK has `errors` and `log` packages, aliasing avoids collisions with standard library imports:

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

## Platform API client

`client.New()` is safe-by-default:

- sandbox environment by default
- API key header injection
- automatic idempotency key generation for `POST`
- bounded retries with exponential backoff + `Retry-After`
- typed error mapping
- structured logs with secret redaction

```go
package main

import (
    "context"
    "fmt"

    "github.com/dakota-xyz/go-sdk/client"
)

func main() {
    c, err := client.New(
        client.WithAPIKey("dakota_api_key"),
        // Optional: client.WithEnvironment(client.EnvironmentProduction),
    )
    if err != nil {
        panic(err)
    }

    resp, err := client.CheckResponse(
        c.Raw().ListCustomersWithResponse(context.Background(), nil),
    )
    if err != nil {
        panic(err)
    }

    fmt.Println("customers", len(resp.JSON200.Data))
}
```

### Typed error handling

```go
resp, err := client.CheckResponse(
    c.Raw().ListApplicationsWithResponse(ctx, nil),
)
if err != nil {
    var apiErr *client.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("code=%s status=%d request_id=%s\n", apiErr.Code, apiErr.StatusCode, apiErr.RequestID)
    }
    return
}
_ = resp
```

### Pagination helpers

```go
it := c.ApplicationsIterator(nil)
for {
    app, ok, err := it.Next(ctx)
    if err != nil {
        panic(err)
    }
    if !ok {
        break
    }
    fmt.Println(app.ApplicationId)
}
```

For customer-scoped lists, pass `customerID`:

```go
txIt := c.TransactionsIterator(customerID, nil)
recipientsIt := c.RecipientsIterator(customerID, nil)
_, _ = txIt, recipientsIt
```

### Parser helpers

The client package includes parsers from generated API models into SDK-facing models:

```go
parsed := client.ParseCustomers(resp.JSON200.Data)
fmt.Println(parsed[0].ID, parsed[0].Name)
```

## Regenerating the API client

The OpenAPI source of truth lives at `client/gen/openapi.yaml`.

Regenerate generated client/types:

```bash
make generate-client
```

or:

```bash
./scripts/generate-client.sh
```

Generation is pinned to `oapi-codegen v2.5.1` and configured through `client/gen/oapi-codegen.yaml`.

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

## HTTP webhook handler

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

## Typed webhook event payloads

```go
customer, err := webhook.EventDataAs[types.CustomerData](event)
if err != nil {
    // malformed payload for this type
    return
}
_ = customer
```

## Development

```bash
make generate-client
make test
make test-race
make vet
```
