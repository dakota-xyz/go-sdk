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

## Agentic Payments (Beta)

> ⚠️ **Beta.** The hosted payment-agent surface is `x-beta` and flag-gated on the platform (endpoints return `404` unless enabled for your key). The SDK helpers below may change — or be removed — without a major-version bump. Not recommended for production.

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

The full agentic surface (`/payment-agents`, `/mandates`, `/instructions`, proposals) is always reachable via `c.Raw()`. The read-only **customer insight** report — `c.Raw().GetCustomerInsightsWithResponse(ctx, customerID)` — is reachable the same way, and so is its whole-book companion:

```go
// One deterministic report over ALL your customers' agentic activity: KPI
// metrics with previous-window values, daily series for charts, and a
// per-customer roll-up sorted worst-first. Every filter is optional and only
// narrows; the response shape never changes. Multi-value filters are slices
// and go on the wire comma-separated.
report, err := client.CheckResponse(c.Raw().GetClientInsightsWithResponse(ctx, &gen.GetClientInsightsParams{
    Severity:   &[]string{"critical", "warn"},
    WindowDays: ptr(30), // 1–90, default 14
}))
for _, row := range report.JSON200.Customers {
    if row.ItemCounts.Critical == 0 {
        continue
    }
    // Name and TotalUsd are optional, so they are pointers.
    if row.TotalUsd != nil {
        fmt.Println(row.CustomerId, *row.TotalUsd)
    } else {
        fmt.Println(row.CustomerId, "(balance index not configured)")
    }
}
```

Items reuse the customer report's `InsightItem`, plus `CustomerId` (nil on cross-customer aggregates) and `Responsibility` (`payment_ops` / `compliance` — a grouping label for routing, not ownership). The report scans at most 100 customers per request; `Snapshot.Customers.Scanned < Total` says it was computed over a prefix of the book.

### Blockers: for your application, not your customer

A turn can report machine-actionable reasons it could not complete. `Reply` explains the same thing in prose, which software cannot branch on.

**Blockers accompany proposals — they do not replace them.** The common case is a payee who does not exist yet: the turn proposes creating them *and* reports that the limit will not reach them, because you have to do both, in that order.

```go
for _, b := range turn.Blockers {
    switch b.Code {
    case "mandate_does_not_cover_payee":
        // Actionable: amend the limit to ADD this payee as a target. That
        // changes nothing else, so it can never raise the limit.
        openLimitEditor(b.MandateId, b.PayeeName)
    case "no_mandate":
        // Nothing to amend — the customer must establish a limit first.
        openLimitCreation()
    default:
        // Ignore codes you do not know; new ones are added over time.
    }
}
```

### Speaking your product's language

Register your vocabulary once and every drafting turn uses it — the agent then says "recipient" and "spending limit" instead of "destination" and "mandate":

```go
_, err := c.Raw().UpdateClientAgenticPolicyWithResponse(ctx, nil, gen.AgenticClientPolicy{
    PayeeModel: ptr(gen.AgenticClientPolicyPayeeModel("flat")),
    Labels:     &map[string]string{"limit": "spending limit", "payee": "recipient", "limit_unit": "USD"},
})
```

Registration is a **full replace**, not a merge — `{}` clears it, and a change takes effect on the next turn. The policy belongs to the client rather than to a request, so a drafting turn and the accept that follows it are always judged by the same rules.

### Timezones

The proposals endpoint is stateless, so the customer's zone must ride on every turn. `WithTimezone` does that for you:

```go
conv := c.NewAgentConversation(agentID, client.WithTimezone("America/Los_Angeles"))
```

Without it, "tomorrow" and "10 am" resolve as UTC. Pass it again to `ResumeAgentConversation` — the transcript carries the messages, not the options.

### Amending a mandate

An amend appends a new signed version WITHOUT resetting the spend already made in the current window: usage accrues to the mandate, so an agent that has spent 9,000 of a 10,000 monthly cap and is amended to 20,000 has 11,000 left — not 20,000.

```go
next := *m.Rule
next.MaxAmountInWindow = ptr("20000")

payload, err := client.MandateAmendSignPayload(m, *m.Version+1, next)
// sign `payload`, then submit via c.Raw().AmendMandateWithResponse
```

The payload commits to the version it creates, so a signature collected for v2 can never be replayed to append v3. The amend endpoint takes the rule **verbatim** — it must already be canonical (`window` present, `targets` as recipient ids, `asset` uppercase), and `MandateSignPayload` refuses the amend verb outright.

## Signer-group governance (signed member changes)

A signer group with governance settings changes its members or its governance only with signatures from its governance: the members of its governing group (`governance_signer_group_id`, else the group itself), at least `effective_governance_threshold` of them. A client with governance enforcement on needs signatures for every group. Without them the platform answers `400 endorsement_required`.

Each change is signed over a canonical intent that names the group's current `version` and the request's `X-Idempotency-Key`. The SDK builds that intent byte for byte as the platform verifies it, signs it, and sends the request with the same key and version:

```go
// Signers: an ES256 key you hold (P256Signer, or your HSM/KMS behind client.Signer)...
viaKey := client.ES256IntentSigner(signer)

// ...and/or a passkey. The SDK calls your function with the challenge to sign;
// pass it to navigator.credentials.get unchanged (it is the canonical intent
// itself, not a hash) and return the PublicKeyCredential JSON (credential.toJSON()).
viaPasskey := client.WebAuthnSigner(func(ctx context.Context, challenge []byte) ([]byte, error) {
    return askBrowserForAssertion(ctx, challenge) // your front end
})

group, err := c.AddSignerToGroupSigned(ctx, groupID, newMemberPublicKey, viaPasskey)
group, err = c.RemoveSignersFromGroupSigned(ctx, groupID, []string{memberPublicKey}, viaKey)

two := int32(2)
group, err = c.UpdateSignerGroupGovernanceSigned(ctx, groupID,
    client.GovernanceSettings{GovernanceThreshold: &two}, // a nil field clears that setting
    viaKey)
```

Pass as many signers as the group's quorum. Keep the governing keys and passkeys away from the service that holds your API key: governance exists so that an API key alone cannot change who controls a wallet.

`GovernanceSettings` replaces the group's whole governance, and a nil field clears that setting. So that a threshold-only update cannot silently drop a governing group, the SDK refuses to clear a setting the group has now unless you set `AllowClearing: true`. When the signatures come from different people at different times, prepare the change, collect the signatures, then submit. The prepared change marshals to JSON, so you can store it in the meantime:

```go
intent, err := c.PrepareRemoveSignersFromGroup(ctx, groupID, []string{memberPublicKey})
// Give intent.Canonical to each signer (for a passkey, it is the challenge;
// client.WebAuthnChallenge(intent.Canonical) is its base64url form).
sig1, err := client.ES256IntentSigner(alice).SignIntent(ctx, intent.Canonical)
sig2, err := client.EncodeWebAuthnAssertion(intent.Canonical, bobsAssertionJSON)
group, err := c.SubmitGovernanceIntent(ctx, intent, []string{sig1, sig2})
```

A signer UI should show the person what they are about to sign; `intent.Canonical` is the exact JSON. `Sign` and `SubmitGovernanceIntent` refuse an intent whose `Canonical` does not match its fields, and `SubmitGovernanceIntent` refuses fewer signatures than the group's quorum.

The idempotency key is part of what is signed, and the platform remembers the outcome per key. So a governance refusal (stale version, too few or bad signatures) is final for that intent: prepare and sign again. If a submit fails without any response, resubmit the same intent and signatures rather than calling a `*Signed` helper again (each call mints a new key).

Governance refusals are typed:

```go
switch {
case errors.Is(err, client.ErrStaleSignerGroupVersion):
    // The group changed since the signatures were made: prepare and sign again.
    // (The SDK does not re-sign on its own: for a passkey that is a new prompt.)
case errors.Is(err, client.ErrInsufficientEndorsements):
    // Too few governing members signed.
case errors.Is(err, client.ErrBelowThreshold):
    // The removal would leave the group below a threshold it must keep.
case errors.Is(err, client.ErrEndorsementRequired):
    // Sent without signatures.
}
```

To build the canonical bytes yourself, use `AddSignersToGroupIntent`, `RemoveSignersFromGroupIntent` and `UpdateSignerGroupGovernanceIntent`. The idempotency key you sign must be the lowercase UUID you send as `X-Idempotency-Key`; the builders refuse anything else.

A signed crypto destination has a builder but no one-call helper yet. Build, sign and send it through the raw client with the same key:

```go
key := uuid.New()
network := gen.NetworkId("ethereum-mainnet")
intent, err := client.CreateCryptoDestinationIntent(recipientID, string(network), address, signerGroupID, key.String())
sig, err := client.ES256IntentSigner(signer).SignIntent(ctx, intent)

var body gen.CreateDestinationJSONRequestBody
err = body.FromCryptoDestinationRequest(gen.CryptoDestinationRequest{
    DestinationType: gen.CryptoDestinationRequestDestinationTypeCrypto,
    Name:            "Treasury",
    CryptoAddress:   address,  // exactly as signed
    NetworkId:       &network, // required with signatures
    SignerGroupId:   &signerGroupID,
    Signatures:      &[]string{sig},
})
resp, err := client.CheckResponse(c.Raw().CreateDestinationWithResponse(ctx, recipientID,
    &gen.CreateDestinationParams{XIdempotencyKey: key}, body))
``` A runnable sandbox walk-through is in [`cmd/signer-group-governance`](cmd/signer-group-governance).

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
    "github.com/dakota-xyz/go-sdk/webhook/types"
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

        // A reviewer needs more from one of your customers. The payload lists
        // what is owed — documents by type or purpose, fields, questions —
        // but never the resubmission link: it embeds a credential. Read the
        // link from the customer resource with your API key.
        webhook.On(webhook.EventCustomerRFIRequested, func(ctx context.Context, event webhook.Event) error {
            rfi, err := webhook.EventDataAs[types.CustomerRFIRequestedData](event)
            if err != nil {
                return err
            }
            // rfi.MessageID dedupes redeliveries and pairs the later
            // customer.rfi.responded with this request.
            for _, req := range rfi.Requirements {
                fmt.Printf("customer %s owes a %s\n", rfi.CustomerID, req.Type)
            }
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

// Iterate transactions with filters. The iterator always requests the
// one_off family, so filtering by customer alone is safe here — a raw
// GET /transactions with customer_id and no transaction_type would return
// that customer's auto-account transactions instead.
txIt := c.OneOffTransactionsIterator(&gen.ListTransactionsParams{
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
