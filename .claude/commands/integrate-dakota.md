# Integrate Dakota SDK

Help the user integrate the Dakota Go SDK for stablecoin payments.

## Context

Dakota is a stablecoin payments platform providing:
- **On-ramp**: USD → USDC/USDT (bank to crypto)
- **Off-ramp**: USDC/USDT → USD (crypto to bank)
- **Swap**: Cross-chain stablecoin exchange
- **Wallets**: Non-custodial multi-sig wallets

## Integration Steps

When helping users integrate Dakota, follow this pattern:

### 1. Install the SDK
```bash
go get github.com/dakota-xyz/go-sdk
```

### 2. Initialize Client
```go
import (
    "github.com/dakota-xyz/go-sdk/client"
    "github.com/dakota-xyz/go-sdk/client/gen"
)

c, err := client.New(
    client.WithAPIKey("api_key"),
    // For production: client.WithEnvironment(client.EnvironmentProduction),
)
```

### 3. Follow Resource Hierarchy
```
Customer (KYB required) → Recipient → Destination → Account
```

### 4. API Call Pattern
Always use:
```go
resp, err := client.CheckResponse(
    c.Raw().<Method>WithResponse(ctx, params),
)
```

## Ask the User

1. What flow do they need? (on-ramp, off-ramp, swap, wallets, one-off transactions)
2. Are they using sandbox or production?
3. Do they need webhook handling?

## Key Files to Reference

- `client/client.go` - Client initialization
- `client/gen/client.gen.go` - All API methods
- `client/environment.go` - Environment configuration
- `README.md` - Full documentation with examples
- `AGENTS.md` - Comprehensive API reference

## Common Implementations

### Off-Ramp Flow
1. CreateCustomer → 2. CreateRecipient → 3. CreateDestination (bank) → 4. CreateAccount (offramp)

### On-Ramp Flow
1. CreateCustomer → 2. CreateRecipient → 3. CreateDestination (crypto) → 4. CreateAccount (onramp)

### One-Off Transaction
```go
c.Raw().CreateTransactionWithResponse(ctx, nil, gen.OneOffTransactionRequest{
    CustomerId:             customerID,
    Amount:                 "1000.00",
    SourceAsset:            "USDC",
    SourceNetworkId:        gen.NetworkId("ethereum-mainnet"),
    DestinationId:          destinationID,
    DestinationAsset:       "USD",
    DestinationPaymentRail: ptr(gen.PaymentCapability("ach")),  // Required for fiat
})

func ptr[T any](v T) *T { return &v }
```

### Webhook Handler
```go
handler, _ := webhook.NewHandler(
    webhook.WithPublicKey("key"),
    webhook.OnDefault(func(ctx context.Context, e webhook.Event) error {
        return nil
    }),
)
```

## Critical Notes

### Method Signatures
Most create/update methods require a `params` argument (can be nil):
`c.Raw().<Method>WithResponse(ctx, params, request)`

### Destination Creation (Union Types)
```go
destBody := gen.DestinationRequestUnion{}

// For bank account:
err = destBody.FromFiatUSDestinationRequest(gen.FiatUSDestinationRequest{
    Name:              "Bank Account",
    BankName:          "Chase",        // Required
    AccountHolderName: "Acme Corp",
    AccountNumber:     "123456789",
    AbaRoutingNumber:  "021000021",    // NOTE: AbaRoutingNumber, not RoutingNumber
    AccountType:       gen.FiatUSDestinationRequestAccountTypeChecking,
})

// For crypto wallet:
err = destBody.FromCryptoDestinationRequest(gen.CryptoDestinationRequest{...})
```

### Account Creation Required Fields
- Off-ramp needs both `Capabilities` AND `Rail` fields
- Recipients for fiat destinations need an `Address`
- ListAccounts requires `AccountType` in params
