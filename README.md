# Dakota Go SDK

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://golang.org/)
[![Go Reference](https://pkg.go.dev/badge/github.com/dakota-xyz/go-sdk.svg)](https://pkg.go.dev/github.com/dakota-xyz/go-sdk)

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
        c.Raw().ListCustomersWithResponse(context.Background(), &gen.ListCustomersParams{}),
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
        c.Raw().CreateCustomerWithResponse(ctx, nil, gen.CustomerCreateRequest{
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
    // Include address for fiat destinations
    postalCode := "10001"
    region := "NY"
    recipientResp, err := client.CheckResponse(
        c.Raw().CreateRecipientWithResponse(ctx, customerID, nil, gen.RecipientRequest{
            Name: "Acme Treasury",
            Address: &gen.Address{
                Street1:    "123 Main Street",
                City:       "New York",
                Country:    "US",
                PostalCode: &postalCode,
                Region:     &region,
            },
        }),
    )
    if err != nil {
        log.Fatal(err)
    }

    recipientID := recipientResp.JSON201.Id
    fmt.Printf("Created recipient: %s\n", recipientID)

    // Step 3: Create a bank destination (where USD will be sent)
    // Use DestinationRequestUnion with FromFiatUSDestinationRequest method
    destBody := gen.DestinationRequestUnion{}
    err = destBody.FromFiatUSDestinationRequest(gen.FiatUSDestinationRequest{
        Name:             "Acme Bank Account",
        BankName:         "Chase Bank",
        AccountHolderName: "Acme Corporation",
        AccountNumber:    "123456789",
        AbaRoutingNumber: "021000021",  // Note: AbaRoutingNumber, not RoutingNumber
        AccountType:      gen.FiatUSDestinationRequestAccountTypeChecking,
    })
    if err != nil {
        log.Fatal(err)
    }

    destResp, err := client.CheckResponse(
        c.Raw().CreateDestinationWithResponse(ctx, recipientID, nil, destBody),
    )
    if err != nil {
        log.Fatal(err)
    }

    fiatDestID := destResp.JSON201.Id
    fmt.Printf("Created bank destination: %s\n", fiatDestID)

    // Step 4: Create an off-ramp account
    // Dakota returns a crypto address where customer sends USDC
    sourceAsset := "USDC"
    destAsset := "USD"
    sourceNetwork := "ethereum-mainnet"
    capabilities := gen.Capabilities{gen.PaymentCapability("ach")}
    rail := gen.PaymentCapability("ach")

    accountResp, err := client.CheckResponse(
        c.Raw().CreateAccountWithResponse(ctx, nil, gen.AccountCreateRequest{
            AccountType:       gen.AccountTypeOfframp,
            FiatDestinationId: &fiatDestID,
            SourceAsset:       &sourceAsset,
            DestinationAsset:  &destAsset,
            SourceNetworkId:   &sourceNetwork,
            Capabilities:      &capabilities,
            Rail:              &rail,
        }),
    )
    if err != nil {
        log.Fatal(err)
    }

    account := accountResp.JSON201
    fmt.Printf("Off-ramp account created! ID: %s\n", account.Id)
    if account.SourceCryptoAddress != nil {
        fmt.Printf("Send USDC to: %s\n", *account.SourceCryptoAddress)
    }

    // When customer sends USDC to this address:
    // 1. Dakota detects the deposit
    // 2. Converts USDC to USD
    // 3. Initiates ACH transfer to the bank account
    // 4. You receive webhook notifications at each step
}

func ptr[T any](v T) *T { return &v }
```

## Complete Flow: On-Ramp (USD → Crypto)

Accept USD bank transfers and deliver stablecoins to customer wallets.

```go
// Step 1: Create customer (same as off-ramp)
// Step 2: Create recipient (same as off-ramp)

// Step 3: Create a crypto destination (where stablecoins will be sent)
networkID := gen.NetworkId("ethereum-mainnet")
cryptoDestBody := gen.DestinationRequestUnion{}
err = cryptoDestBody.FromCryptoDestinationRequest(gen.CryptoDestinationRequest{
    Name:          "Customer Wallet",
    CryptoAddress: "0x742d35Cc6634C0532925a3b844Bc9e7595f...",
    NetworkId:     &networkID,
})
if err != nil {
    log.Fatal(err)
}

cryptoDestResp, err := client.CheckResponse(
    c.Raw().CreateDestinationWithResponse(ctx, recipientID, nil, cryptoDestBody),
)
if err != nil {
    log.Fatal(err)
}

cryptoDestID := cryptoDestResp.JSON201.Id

// Step 4: Create an on-ramp account
// Dakota returns bank details where customer sends USD
sourceAsset := "USD"
destAsset := "USDC"
destNetwork := gen.NetworkId("ethereum-mainnet")
capabilities := gen.Capabilities{gen.PaymentCapability("ach")}

onrampResp, err := client.CheckResponse(
    c.Raw().CreateAccountWithResponse(ctx, nil, gen.AccountCreateRequest{
        AccountType:          gen.AccountTypeOnramp,
        CryptoDestinationId:  &cryptoDestID,
        SourceAsset:          &sourceAsset,
        DestinationAsset:     &destAsset,
        DestinationNetworkId: &destNetwork,
        Capabilities:         &capabilities,
    }),
)
if err != nil {
    log.Fatal(err)
}

account := onrampResp.JSON201
fmt.Printf("On-ramp account created! ID: %s\n", account.Id)
if account.BankAccount != nil {
    fmt.Printf("Send USD to:\n")
    fmt.Printf("  Bank: %s\n", account.BankAccount.BankName)
    fmt.Printf("  Routing: %s\n", account.BankAccount.RoutingNumber)
    fmt.Printf("  Account: %s\n", account.BankAccount.AccountNumber)
}

// When customer sends USD:
// 1. Dakota receives the bank transfer
// 2. Converts USD to USDC
// 3. Sends USDC to the customer's wallet address
```

## One-Off Transactions

For single transactions without creating accounts:

```go
txResp, err := client.CheckResponse(
    c.Raw().CreateTransactionWithResponse(ctx, nil, gen.OneOffTransactionRequest{
        CustomerId:             customerID,
        Amount:                 "1000.00",
        SourceAsset:            "USDC",
        SourceNetworkId:        gen.NetworkId("ethereum-mainnet"),
        DestinationId:          destinationID,
        DestinationAsset:       "USD",
        DestinationPaymentRail: ptr(gen.PaymentCapability("ach")),
        PaymentReference:       ptr("Invoice #12345"),
    }),
)
if err != nil {
    log.Fatal(err)
}

tx := txResp.JSON201
fmt.Printf("Transaction created: %s\n", tx.Id)
fmt.Printf("Status: %s\n", tx.Status)
```

## Agentic Payments (Alpha)

> ⚠️ **Alpha.** The hosted payment-agent surface is `x-alpha` and flag-gated on the platform (endpoints return `404` unless enabled for your key). The SDK helpers below may change — or be removed — without a major-version bump. Not recommended for production.

A **payment agent** is a named, customer-scoped signer Dakota can drive: you provision it, endorse it onto a wallet, then it drafts and — once a **mandate** is signed — fires payments, bounded by that customer-approved mandate. Runnable snippets live in [`client/example_test.go`](client/example_test.go).

### 1. Provision an agent and endorse it onto a wallet

```go
// Create a hosted payment agent (Dakota custodies its signing key).
agentResp, err := client.CheckResponse(
    c.Raw().CreatePaymentAgentWithResponse(ctx, gen.CreatePaymentAgentRequest{
        CustomerId: customerID,
        Name:       "Bill Pay",
        Hosted:     true,
    }),
)
if err != nil {
    log.Fatal(err)
}
agent := agentResp.JSON201

// Grant it spend permission on a wallet by adding its signer to the wallet's
// spending group (idempotent) — the customer-endorsed attach.
if _, err := c.AttachUserToWallet(ctx, walletID, *agent.SignerPublicKey, spendingGroupID); err != nil {
    log.Fatal(err)
}
```

### 2. Draft payments from natural language

```go
conv := c.NewAgentConversation(*agent.Id)
turn, err := conv.Send(ctx, "Pay Alice 100 USDC on base-mainnet every month until December")
if err != nil {
    log.Fatal(err)
}
if turn.HasProposals {
    // Review turn.Proposals, then accept them through the instructions flow.
    fmt.Printf("agent drafted %d proposal(s)\n", len(turn.Proposals))
} else {
    fmt.Println("agent needs more detail:", turn.Reply)
}
```

### 3. Sign and approve a mandate (§8)

The caller holds the keys; the SDK never does. `P256Signer` is a ready in-memory signer for sandbox/tests — implement the `Signer` interface over your HSM/KMS in production.

```go
signer, _ := client.NewP256Signer() // or client.P256SignerFromKey(yourECDSAKey)

// `mandate` comes from c.Raw().GetMandateWithResponse / ListMandates.
payload, err := client.MandateSignPayload(mandate, client.MandateActionApprove)
if err != nil {
    log.Fatal(err)
}
sig, err := signer.Sign(payload)
if err != nil {
    log.Fatal(err)
}
// Submit `sig` via the mandate-approve endpoint (c.Raw()) to activate the mandate.
```

The full agentic surface (`/payment-agents`, `/mandates`, `/instructions`, proposals) is always reachable via `c.Raw()`.

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
txIt := c.OneOffTransactionsIterator(&gen.ListOneOffTransactionsParams{
    CustomerId: &customerID,
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

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Resources

- [Dakota Documentation](https://docs.dakota.xyz)
- [API Reference](https://docs.dakota.xyz/api-reference)
- [Common Flows](https://docs.dakota.xyz/documentation/common-flows)

## License

MIT License - see [LICENSE](LICENSE) for details.
