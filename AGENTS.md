# Dakota Go SDK - Agent Instructions

This file helps AI coding agents (Claude Code, Cursor, Codex, Copilot, etc.) understand and integrate with the Dakota Go SDK.

## What is Dakota?

Dakota is a **stablecoin payments infrastructure** platform. It provides APIs for:

- **On-ramp**: USD bank transfer → USDC/USDT to blockchain wallet
- **Off-ramp**: USDC/USDT → USD to bank account (ACH/Wire)
- **Swap**: Exchange stablecoins across networks
- **Wallets**: Non-custodial multi-sig wallets

## SDK Structure

```
github.com/dakota-xyz/go-sdk/
├── client/           # Main API client
│   ├── client.go     # Client initialization & options
│   ├── environment.go # Sandbox/Production environments
│   ├── errors.go     # Error handling
│   ├── pagination.go # Iterator helpers
│   ├── parsers.go    # Model converters
│   └── gen/          # Auto-generated from OpenAPI
│       ├── client.gen.go  # Generated API methods
│       └── openapi.yaml   # OpenAPI spec (source of truth)
├── webhook/          # Webhook signature verification
└── errors/           # SDK error types
```

## Quick Integration Pattern

```go
import (
    "context"
    "github.com/dakota-xyz/go-sdk/client"
    "github.com/dakota-xyz/go-sdk/client/gen"
)

// 1. Create client (sandbox by default)
c, err := client.New(client.WithAPIKey("api_key"))

// 2. Make API calls using c.Raw().<MethodName>WithResponse()
resp, err := client.CheckResponse(
    c.Raw().ListCustomersWithResponse(ctx, nil),
)

// 3. Access response data
customers := resp.JSON200.Data
```

## Key Patterns

### API Call Pattern
Always use `client.CheckResponse()` wrapper for proper error handling:

```go
resp, err := client.CheckResponse(
    c.Raw().<MethodName>WithResponse(ctx, params),
)
if err != nil {
    // Handle error (APIError or TransportError)
}
// Success: use resp.JSON200, resp.JSON201, etc.
```

### Resource Hierarchy
Dakota follows this dependency chain for money movement:

```
Customer (must have kyb_status: "active")
    └── Recipient (payment destination entity)
        └── Destination (bank account or crypto wallet)
            └── Account (on-ramp, off-ramp, or swap)
```

### Common Flows

#### Off-Ramp (Crypto → USD)
1. `CreateCustomerWithResponse` → Get customer ID, wait for KYB approval
2. `CreateRecipientWithResponse` → Create recipient under customer
3. `CreateDestinationWithResponse` → Add bank account (fiat_us destination)
4. `CreateAccountWithResponse` → Create off-ramp account, get crypto deposit address
5. Customer sends USDC → Dakota converts and sends USD via ACH

#### On-Ramp (USD → Crypto)
1. Create customer, recipient (same as above)
2. `CreateDestinationWithResponse` → Add crypto wallet (crypto destination)
3. `CreateAccountWithResponse` → Create on-ramp account, get bank details
4. Customer sends USD → Dakota converts and sends USDC to wallet

#### One-Off Transaction
```go
c.Raw().CreateTransactionWithResponse(ctx, nil, gen.OneOffTransactionRequest{
    CustomerId:             customerID,
    Amount:                 "1000.00",
    SourceAsset:            "USDC",
    SourceNetworkId:        gen.NetworkId("ethereum-mainnet"),
    DestinationId:          destinationID,
    DestinationAsset:       "USD",
    DestinationPaymentRail: ptr(gen.PaymentCapability("ach")),  // Required for fiat destinations
})
```

## Environments

| Environment | Usage | Code |
|-------------|-------|------|
| Sandbox (default) | Testing | `client.New(client.WithAPIKey("key"))` |
| Production | Live | `client.New(client.WithAPIKey("key"), client.WithEnvironment(client.EnvironmentProduction))` |

## Available API Methods (84 Total)

All methods are on `c.Raw()` and follow pattern: `<Action><Resource>WithResponse(ctx, ...)`

### Customers
- `CreateCustomerWithResponse(ctx, params, request)` - params contains idempotency key
- `ListCustomersWithResponse(ctx, params)`
- `GetCustomerWithResponse(ctx, customerID)`

### Recipients
- `CreateRecipientWithResponse(ctx, customerID, params, request)` - params contains idempotency key
- `ListRecipientsWithResponse(ctx, customerID, params)`
- `GetRecipientWithResponse(ctx, recipientID)`
- `UpdateRecipientWithResponse(ctx, recipientID, params, request)`

### Destinations
- `CreateDestinationWithResponse(ctx, recipientID, params, request)` - params contains idempotency key
- `ListDestinationsWithResponse(ctx, recipientID, params)`

### Accounts (On-ramp/Off-ramp/Swap)
- `CreateAccountWithResponse(ctx, params, request)` - params contains idempotency key
- `ListAccountsWithResponse(ctx, params)` - **params.AccountType is REQUIRED**
- `GetAccountWithResponse(ctx, accountID)`
- `UpdateAccountWithResponse(ctx, accountID, params, request)`

### Transactions (One-Off)
- `CreateTransactionWithResponse(ctx, params, request)` - params contains idempotency key
- `ListOneOffTransactionsWithResponse(ctx, params)` - Note: method is ListOneOff*, not ListTransactions*
- `GetOneOffTransactionWithResponse(ctx, transactionID)` - Note: method is GetOneOff*
- `CancelOneOffTransactionWithResponse(ctx, transactionID)` - Note: method is CancelOneOff*

### Auto Transactions (Recurring)
- `ListAutoTransactionsWithResponse(ctx, params)`
- `GetAutoTransactionWithResponse(ctx, transactionID)`

### Wallets
- `CreateWalletWithResponse(ctx, params, request)`
- `GetWalletBalancesWithResponse(ctx, walletID)`
- `CreateWalletTransactionWithResponse(ctx, walletID, params, request)`

### Signer Groups (Multi-sig)
- `CreateSignerGroupWithResponse(ctx, params, request)`
- `ListSignerGroupsWithResponse(ctx, params)`
- `GetSignerGroupWithResponse(ctx, signerGroupID)`
- `GetSignerGroupsForWalletWithResponse(ctx, walletID)`
- `CreateSignerWithResponse(ctx, params, request)`
- `DeleteSignerWithResponse(ctx, signerID)`
- `CreateSignerGroupSignerWithResponse(ctx, signerGroupID, params, request)`
- `DeleteSignerGroupSignerWithResponse(ctx, signerGroupID, signerID)`
- `UpsertWalletSignerGroupRelationshipWithResponse(ctx, walletID, params, request)`
- `DeleteWalletSignerGroupRelationshipWithResponse(ctx, walletID, request)`

### Policies (Transaction Rules)
- `CreatePolicyWithResponse(ctx, params, request)`
- `ListPoliciesWithResponse(ctx, params)`
- `GetPolicyWithResponse(ctx, policyID)`
- `DeletePolicyWithResponse(ctx, policyID, params, request)`
- `AddPolicyRuleWithResponse(ctx, policyID, params, request)`
- `UpdatePolicyRuleWithResponse(ctx, policyID, ruleID, params, request)`
- `DeletePolicyRuleWithResponse(ctx, policyID, params, request)`
- `UpsertPolicyWalletRelationshipWithResponse(ctx, policyID, params, request)`
- `DeletePolicyWalletRelationshipWithResponse(ctx, policyID, params, request)`

### Applications (KYB/KYC)
- `ListApplicationsWithResponse(ctx, params)`
- `GetApplicationWithResponse(ctx, applicationID)`
- `CreateApplicationSubmissionWithResponse(ctx, applicationID)`
- `UpdateBusinessApplicationDetailsWithResponse(ctx, applicationID, request)`
- `UpdateIndividualApplicationDetailsWithResponse(ctx, applicationID, entityID, request)`
- `AddAssociatedIndividualWithResponse(ctx, applicationID, request)`
- `UpdateAssociatedIndividualWithResponse(ctx, applicationID, entityID, request)`
- `DeleteAssociatedIndividualWithResponse(ctx, applicationID, entityID)`
- `CreateOrUpdateEDDWithResponse(ctx, applicationID, request)`
- `GetEDDWithResponse(ctx, applicationID)`
- `SubmitAttestationWithResponse(ctx, applicationID, request)`

### Documents
- `ListDocumentsWithResponse(ctx, applicationID, params)`
- `DownloadDocumentWithResponse(ctx, applicationID, documentID)`
- `DeleteOnboardingDocumentWithResponse(ctx, applicationID, documentID)`
- `CreateApplicationDocumentWithResponse(ctx, applicationID, request)`
- `CreateApplicationDocumentUploadWithResponse(ctx, applicationID, request)`
- `CreateApplicationDocumentVerificationWithResponse(ctx, applicationID, documentID)`
- `UploadIndividualDocumentWithResponse(ctx, applicationID, entityID, request)`
- `CreateAssociatedIndividualDocumentUploadWithResponse(ctx, applicationID, entityID, request)`

### Webhooks
- `CreateWebhookTargetWithResponse(ctx, params, request)`
- `ListWebhookTargetsWithResponse(ctx, params)`
- `GetWebhookTargetWithResponse(ctx, targetID)`
- `UpdateWebhookTargetWithResponse(ctx, targetID, params, request)`
- `DeleteWebhookTargetWithResponse(ctx, targetID)`
- `ListWebhookHistoryWithResponse(ctx, targetID, params)`
- `GetWebhookDeliveryWithResponse(ctx, targetID, deliveryID)`
- `ReplayWebhookDeliveryWithResponse(ctx, targetID, deliveryID)`

### Events
- `ListEventsWithResponse(ctx, params)`

### API Keys
- `CreateApiKeyWithResponse(ctx)`
- `ListApiKeysWithResponse(ctx, params)`
- `DeleteApiKeyWithResponse(ctx, keyID)`
- `DeleteAllApiKeysWithResponse(ctx)`

### Users (Dashboard)
- `CreateClientUserWithResponse(ctx, request)`
- `ListClientUsersWithResponse(ctx, params)`
- `UpdateClientUserWithResponse(ctx, userID, request)`
- `DeleteClientUserWithResponse(ctx, userID)`

### Platform Info
- `GetCountriesWithResponse(ctx)`
- `GetSupportedNetworksWithResponse(ctx)`

### Sandbox Simulation
- `SimulateInboundWithResponse(ctx, request)` - Simulate ACH/Wire deposits (request needs SimulationId)
- `SimulateOnboardingWithResponse(ctx, request)` - Simulate KYB status changes (request needs SimulationId)
- `ListSandboxScenariosWithResponse(ctx, params)`
- `GetSimulationWithResponse(ctx, simulationID)`
- `AdvanceSimulationWithResponse(ctx, simulationID, request)`

**Note**: SimulateInbound and SimulateOnboarding require a `SimulationId` field (UUID string) in the request body.

## Pagination

Use built-in iterators for large collections:

```go
it := c.CustomersIterator(nil)
for {
    customer, ok, err := it.Next(ctx)
    if err != nil || !ok {
        break
    }
    // process customer
}
```

Available iterators:
- `CustomersIterator(params)`
- `ApplicationsIterator(params)`
- `OneOffTransactionsIterator(params)`
- `RecipientsIterator(customerID, params)`
- `EventsIterator(params)`

## Error Handling

```go
resp, err := client.CheckResponse(c.Raw().GetCustomerWithResponse(ctx, id))
if err != nil {
    var apiErr *client.APIError
    if errors.As(err, &apiErr) {
        // API error: apiErr.StatusCode, apiErr.Message, apiErr.Code
        // apiErr.Retryable indicates if safe to retry
    }
    var transportErr *client.TransportError
    if errors.As(err, &transportErr) {
        // Network/transport error
    }
}
```

## Webhook Handling

```go
import "github.com/dakota-xyz/go-sdk/webhook"

handler, _ := webhook.NewHandler(
    webhook.WithPublicKey("public_key_hex"),
    webhook.OnDefault(func(ctx context.Context, event webhook.Event) error {
        // Handle event
        return nil
    }),
)
http.Handle("/webhooks", handler)
```

## Supported Networks

| Network | Production ID | Sandbox ID |
|---------|--------------|------------|
| Ethereum | `ethereum-mainnet` | `ethereum-sepolia` |
| Polygon | `polygon-mainnet` | `polygon-amoy` |
| Arbitrum | `arbitrum-mainnet` | `arbitrum-sepolia` |
| Base | `base-mainnet` | `base-sepolia` |
| Solana | `solana-mainnet` | `solana-devnet` |

## Union Types

### Request Union Types (use From* methods to set)
```go
// Destination creation - use DestinationRequestUnion
destBody := gen.DestinationRequestUnion{}

// For bank accounts (fiat):
err = destBody.FromFiatUSDestinationRequest(gen.FiatUSDestinationRequest{
    Name:              "Bank Account",
    BankName:          "Chase Bank",  // Required
    AccountHolderName: "Acme Corp",
    AccountNumber:     "123456789",
    AbaRoutingNumber:  "021000021",   // Note: AbaRoutingNumber, NOT RoutingNumber
    AccountType:       gen.FiatUSDestinationRequestAccountTypeChecking,
})

// For crypto wallets:
networkID := gen.NetworkId("ethereum-mainnet")
err = destBody.FromCryptoDestinationRequest(gen.CryptoDestinationRequest{
    Name:          "Wallet",
    CryptoAddress: "0x...",
    NetworkId:     &networkID,
})
```

### Response Union Types (use As* methods to extract)
```go
// Destination responses - DestinationResponseUnion has As* methods
if resp.JSON201.Destination != nil {
    cryptoDest, err := resp.JSON201.Destination.AsCryptoDestinationResponse()
    fiatUSDest, err := resp.JSON201.Destination.AsFiatUSDestinationResponse()
    fiatIBANDest, err := resp.JSON201.Destination.AsFiatIBANDestinationResponse()
}

// Transaction list responses - use AsPaginatedOneOffTransactionResponse
txResp, err := resp.JSON200.AsPaginatedOneOffTransactionResponse()
```

### Account Responses (NOT a union type)
```go
// AccountResponse is a regular struct, NOT a union type
// Access fields directly - no As* methods needed
account := resp.JSON201
fmt.Println(account.AccountType)  // "onramp", "offramp", or "swap"
fmt.Println(account.Id)
if account.BankAccount != nil {
    fmt.Println(account.BankAccount.RoutingNumber)
}
if account.SourceCryptoAddress != nil {
    fmt.Println(*account.SourceCryptoAddress)
}
```

## Required Fields Reference

### Account Creation (AccountCreateRequest)
```go
gen.AccountCreateRequest{
    AccountType: gen.AccountTypeOnramp,  // Required: onramp, offramp, or swap

    // For on-ramp (USD → Crypto):
    CryptoDestinationId:  &cryptoDestID,   // Where crypto goes
    SourceAsset:          &sourceAsset,    // "USD"
    DestinationAsset:     &destAsset,      // "USDC"
    DestinationNetworkId: &destNetwork,    // gen.NetworkId("ethereum-mainnet")
    Capabilities:         &capabilities,   // gen.Capabilities{gen.PaymentCapability("ach")}

    // For off-ramp (Crypto → USD):
    FiatDestinationId: &fiatDestID,        // Where USD goes
    SourceAsset:       &sourceAsset,       // "USDC"
    DestinationAsset:  &destAsset,         // "USD"
    SourceNetworkId:   &sourceNetwork,     // gen.NetworkId("ethereum-mainnet")
    Capabilities:      &capabilities,      // gen.Capabilities{gen.PaymentCapability("ach")}
    Rail:              &rail,              // gen.PaymentCapability("ach") - Required for off-ramp
}
```

### Fiat Destination (FiatUSDestinationRequest)
```go
gen.FiatUSDestinationRequest{
    Name:              "Account Name",  // Required
    BankName:          "Chase Bank",    // Required
    AccountHolderName: "Acme Corp",     // Required
    AccountNumber:     "123456789",     // Required
    AbaRoutingNumber:  "021000021",     // Required - NOTE: AbaRoutingNumber, NOT RoutingNumber
    AccountType:       gen.FiatUSDestinationRequestAccountTypeChecking,  // Required
}
```

### Recipient for Fiat Destinations
Recipients that will have fiat destinations MUST include an Address:
```go
gen.RecipientRequest{
    Name: "Recipient Name",
    Address: &gen.Address{
        Street1: "123 Main St",
        City:    "New York",
        Country: "US",
        PostalCode: ptr("10001"),
        Region:     ptr("NY"),
    },
}
```

## Common Mistakes to Avoid

1. **Don't skip CheckResponse** - Always wrap API calls with `client.CheckResponse()`
2. **Don't forget KYB** - Customers need `kyb_status: "active"` before creating accounts
3. **Don't mix environments** - Sandbox and Production have different API keys
4. **Don't ignore webhooks** - Use webhooks to track transaction status changes
5. **Don't use wrong field names** - Use `AbaRoutingNumber` not `RoutingNumber` for US bank accounts
6. **Don't forget params** - Most create/update methods have a `params` argument (can be `nil`)
7. **Don't forget Capabilities** - Account creation requires `Capabilities` field
8. **Don't forget Rail for off-ramp** - Off-ramp accounts require `Rail` field
9. **Don't skip Address on Recipients** - Recipients for fiat destinations need an address
