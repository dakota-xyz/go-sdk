# Dakota Go SDK

Official Go SDK for the [Dakota Platform](https://dakota.xyz) — infrastructure for stablecoin payments, on/off-ramps, and non-custodial wallets.

## What is Dakota?

Dakota provides APIs to:

- **On-ramp**: Accept USD bank transfers and deliver stablecoins (USDC/USDT) to blockchain wallets
- **Off-ramp**: Convert stablecoins to USD and deposit to bank accounts via ACH/Wire
- **Swap**: Exchange stablecoins across networks (e.g., USDC on Ethereum → USDT on Polygon)
- **Wallets**: Create non-custodial multi-sig wallets with policy controls

## Install

```bash
go get github.com/dakota-xyz/go-sdk
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/dakota-xyz/go-sdk/client"
    "github.com/dakota-xyz/go-sdk/client/gen"
)

func main() {
    c, err := client.New(
        client.WithAPIKey("your_api_key"),
        // Sandbox by default. For production:
        // client.WithEnvironment(client.EnvironmentProduction),
    )
    if err != nil {
        log.Fatal(err)
    }

    // List your customers
    resp, err := client.CheckResponse(
        c.Raw().ListCustomersWithResponse(context.Background(), nil),
    )
    if err != nil {
        log.Fatal(err)
    }

    for _, customer := range resp.JSON200.Data {
        fmt.Printf("Customer: %s (KYB: %s)\n", customer.Name, customer.KybStatus)
    }
}
```

## Complete Flow: Off-Ramp (Crypto → USD)

This example shows a complete off-ramp flow where a customer sends USDC and receives USD in their bank account.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/dakota-xyz/go-sdk/client"
    "github.com/dakota-xyz/go-sdk/client/gen"
)

func main() {
    ctx := context.Background()

    c, err := client.New(client.WithAPIKey("your_api_key"))
    if err != nil {
        log.Fatal(err)
    }

    // Step 1: Create a customer (triggers KYB onboarding)
    customerResp, err := client.CheckResponse(
        c.Raw().CreateCustomerWithResponse(ctx, gen.CustomerCreateRequest{
            Name:         "Acme Corporation",
            CustomerType: gen.CustomerCreateRequestCustomerTypeBusiness,
            ExternalId:   ptr("acme-123"), // Your internal ID
        }),
    )
    if err != nil {
        log.Fatal(err)
    }

    customerID := customerResp.JSON201.Id
    fmt.Printf("Created customer: %s\n", customerID)
    fmt.Printf("KYB onboarding URL: %s\n", *customerResp.JSON201.ApplicationUrl)

    // Customer completes KYB at the onboarding URL...
    // You'll receive webhooks as status changes.
    // Wait until kyb_status becomes "active" before proceeding.

    // Step 2: Create a recipient (the entity receiving USD)
    recipientResp, err := client.CheckResponse(
        c.Raw().CreateRecipientWithResponse(ctx, customerID, gen.RecipientRequest{
            Name: "Acme Treasury",
        }),
    )
    if err != nil {
        log.Fatal(err)
    }

    recipientID := recipientResp.JSON201.Id
    fmt.Printf("Created recipient: %s\n", recipientID)

    // Step 3: Create a bank destination (where USD will be sent)
    accountType := gen.FiatUSDestinationRequestAccountTypeChecking
    destResp, err := client.CheckResponse(
        c.Raw().CreateDestinationWithResponse(ctx, recipientID, gen.DestinationRequest{
            FromFiatUSDestinationRequest: &gen.FiatUSDestinationRequest{
                DestinationType:    gen.FiatUSDestinationRequestDestinationTypeFiatUs,
                BankName:           "Chase Bank",
                AccountHolderName:  "Acme Corporation",
                AccountNumber:      "123456789",
                RoutingNumber:      "021000021",
                AccountType:        &accountType,
            },
        }),
    )
    if err != nil {
        log.Fatal(err)
    }

    destinationID := getDestinationID(destResp.JSON201)
    fmt.Printf("Created bank destination: %s\n", destinationID)

    // Step 4: Create an off-ramp account
    // Dakota returns a crypto address where customer sends USDC
    accountResp, err := client.CheckResponse(
        c.Raw().CreateAccountWithResponse(ctx, gen.AccountCreateRequest{
            FromOfframpAccountCreateRequest: &gen.OfframpAccountCreateRequest{
                AccountType:      gen.OfframpAccountCreateRequestAccountTypeOfframp,
                CustomerId:       customerID,
                DestinationId:    destinationID,
                SourceAsset:      "USDC",
                SourceNetworkId:  gen.NetworkId("ethereum-mainnet"),
                DestinationAsset: "USD",
                DestinationRail:  gen.PaymentCapability("ach"),
            },
        }),
    )
    if err != nil {
        log.Fatal(err)
    }

    // This is the address where the customer sends USDC
    account, _ := accountResp.JSON201.AsOfframpAccount()
    fmt.Printf("Off-ramp account created!\n")
    fmt.Printf("Send USDC to: %s (on %s)\n", account.CryptoAddress, account.SourceNetworkId)

    // When customer sends USDC to this address:
    // 1. Dakota detects the deposit
    // 2. Converts USDC to USD
    // 3. Initiates ACH transfer to the bank account
    // 4. You receive webhook notifications at each step
}

func ptr[T any](v T) *T { return &v }

func getDestinationID(dest *gen.DestinationResponseUnion) gen.KSUID {
    if d, err := dest.AsFiatUSDestinationResponse(); err == nil {
        return d.Id
    }
    return ""
}
```

## Complete Flow: On-Ramp (USD → Crypto)

Accept USD bank transfers and deliver stablecoins to customer wallets.

```go
// Step 1: Create customer (same as off-ramp)
// Step 2: Create recipient
// Step 3: Create a crypto destination (where stablecoins will be sent)
cryptoDestResp, err := client.CheckResponse(
    c.Raw().CreateDestinationWithResponse(ctx, recipientID, gen.DestinationRequest{
        FromCryptoDestinationRequest: &gen.CryptoDestinationRequest{
            DestinationType: gen.CryptoDestinationRequestDestinationTypeCrypto,
            CryptoAddress:   "0x742d35Cc6634C0532925a3b844Bc9e7595f...",
            NetworkId:       gen.NetworkId("ethereum-mainnet"),
        },
    }),
)

// Step 4: Create an on-ramp account
// Dakota returns bank details where customer sends USD
onrampResp, err := client.CheckResponse(
    c.Raw().CreateAccountWithResponse(ctx, gen.AccountCreateRequest{
        FromOnrampAccountCreateRequest: &gen.OnrampAccountCreateRequest{
            AccountType:        gen.OnrampAccountCreateRequestAccountTypeOnramp,
            CustomerId:         customerID,
            DestinationId:      cryptoDestID,
            SourceAsset:        "USD",
            SourceRail:         gen.PaymentCapability("ach"),
            DestinationAsset:   "USDC",
            DestinationNetworkId: gen.NetworkId("ethereum-mainnet"),
        },
    }),
)

account, _ := onrampResp.JSON201.AsOnrampAccount()
fmt.Printf("Send USD to:\n")
fmt.Printf("  Bank: %s\n", *account.BankName)
fmt.Printf("  Routing: %s\n", *account.RoutingNumber)
fmt.Printf("  Account: %s\n", *account.AccountNumber)

// When customer sends USD:
// 1. Dakota receives the bank transfer
// 2. Converts USD to USDC
// 3. Sends USDC to the customer's wallet address
```

## One-Off Transactions

For single transactions without creating accounts:

```go
txResp, err := client.CheckResponse(
    c.Raw().CreateTransactionWithResponse(ctx, gen.OneOffTransactionRequest{
        CustomerId:         customerID,
        Amount:             "1000.00",
        SourceAsset:        "USDC",
        SourceNetworkId:    gen.NetworkId("ethereum-mainnet"),
        DestinationId:      destinationID,
        DestinationAsset:   "USD",
        DestinationPaymentRail: ptr(gen.PaymentCapability("ach")),
        PaymentReference:   ptr("Invoice #12345"),
    }),
)

tx := txResp.JSON201
fmt.Printf("Transaction created: %s\n", tx.Id)
fmt.Printf("Send %s USDC to: %s\n", *tx.SendAmount, tx.CryptoAddress)
fmt.Printf("Status: %s\n", tx.Status)
```

## Handling Webhooks

Dakota sends webhooks for all status changes. Set up a handler:

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/dakota-xyz/go-sdk/webhook"
    "github.com/dakota-xyz/go-sdk/webhook/idempotency"
)

func main() {
    handler, err := webhook.NewHandler(
        webhook.WithPublicKey("your_webhook_public_key_hex"),
        webhook.WithIdempotencyStore(idempotency.NewMemoryStore()),

        // Handle specific event types
        webhook.On(webhook.EventCustomerCreated, func(ctx context.Context, event webhook.Event) error {
            fmt.Printf("Customer created: %s\n", event.ID)
            return nil
        }),

        webhook.On(webhook.EventTransactionOneOffUpdated, func(ctx context.Context, event webhook.Event) error {
            fmt.Printf("Transaction %s updated\n", event.ID)
            // Check transaction status, update your records, notify user, etc.
            return nil
        }),

        // Catch-all for other events
        webhook.OnDefault(func(ctx context.Context, event webhook.Event) error {
            fmt.Printf("Event: %s (type: %s)\n", event.ID, event.Type)
            return nil
        }),
    )
    if err != nil {
        panic(err)
    }

    http.Handle("/webhooks/dakota", handler)
    http.ListenAndServe(":8080", nil)
}
```

## Pagination

Iterate through large collections:

```go
// Iterate all customers
it := c.CustomersIterator(nil)
for {
    customer, ok, err := it.Next(ctx)
    if err != nil {
        log.Fatal(err)
    }
    if !ok {
        break
    }
    fmt.Printf("%s: %s\n", customer.Id, customer.Name)
}

// Iterate transactions with filters
txIt := c.OneOffTransactionsIterator(&gen.ListTransactionsParams{
    CustomerId: &customerID,
    Status:     ptr(gen.OneOffTransactionStatusCompleted),
})
```

## Error Handling

```go
resp, err := client.CheckResponse(
    c.Raw().GetCustomerWithResponse(ctx, "invalid_id"),
)
if err != nil {
    var apiErr *client.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("API Error: %s (HTTP %d)\n", apiErr.Message, apiErr.StatusCode)
        fmt.Printf("Request ID: %s\n", apiErr.RequestID) // Include in support tickets

        if apiErr.Retryable {
            // Safe to retry (429, 503, etc.)
        }
    }
    return
}
```

## Environments

The SDK supports two main environments. **Sandbox is the default** for safe testing.

| Environment | URL | Use Case |
|-------------|-----|----------|
| **Sandbox** (default) | `https://api.platform.sandbox.dakota.xyz` | Testing & development |
| **Production** | `https://api.platform.dakota.xyz` | Live transactions with real money |

```go
// Sandbox (default) - safe for testing, no real money moves
c, _ := client.New(
    client.WithAPIKey("your_sandbox_api_key"),
)

// Production - real money, real transactions
c, _ := client.New(
    client.WithAPIKey("your_production_api_key"),
    client.WithEnvironment(client.EnvironmentProduction),
)
```

> **Note**: Sandbox and Production use different API keys. Make sure you're using the correct key for each environment.

## Configuration Options

```go
c, err := client.New(
    // Required: Authentication
    client.WithAPIKey("your_api_key"),

    // Environment (default: Sandbox)
    client.WithEnvironment(client.EnvironmentProduction),

    // Custom timeout (default: 15s)
    client.WithTimeout(30 * time.Second),

    // Custom retry policy
    client.WithRetryPolicy(client.RetryPolicy{
        MaxAttempts:    5,
        InitialBackoff: 100 * time.Millisecond,
        MaxBackoff:     5 * time.Second,
    }),

    // Structured logging
    client.WithLogger(slog.Default()),
)
```

## Supported Networks

| Network | Production | Sandbox |
|---------|------------|---------|
| Ethereum | `ethereum-mainnet` | `ethereum-sepolia` |
| Polygon | `polygon-mainnet` | `polygon-amoy` |
| Arbitrum | `arbitrum-mainnet` | `arbitrum-sepolia` |
| Base | `base-mainnet` | `base-sepolia` |
| Optimism | `optimism-mainnet` | — |
| Solana | `solana-mainnet` | `solana-devnet` |

## Packages

| Package | Description |
|---------|-------------|
| `github.com/dakota-xyz/go-sdk/client` | API client with retries, auth, pagination |
| `github.com/dakota-xyz/go-sdk/client/gen` | Generated types from OpenAPI spec |
| `github.com/dakota-xyz/go-sdk/webhook` | Webhook signature verification & handling |
| `github.com/dakota-xyz/go-sdk/errors` | Structured error types |
| `github.com/dakota-xyz/go-sdk/log` | Logging abstraction |

## Import Aliases

Avoid collisions with standard library:

```go
import (
    sdkerrors "github.com/dakota-xyz/go-sdk/errors"
    sdklog "github.com/dakota-xyz/go-sdk/log"
)
```

## Regenerating the Client

The OpenAPI spec lives at `client/gen/openapi.yaml`. To regenerate:

```bash
make generate-client
# or
./scripts/generate-client.sh
```

## Resources

- [Dakota Documentation](https://docs.dakota.xyz)
- [API Reference](https://docs.dakota.xyz/api-reference)
- [Common Flows](https://docs.dakota.xyz/documentation/common-flows)
