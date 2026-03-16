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
c.Raw().CreateTransactionWithResponse(ctx, gen.OneOffTransactionRequest{
    CustomerId:       customerID,
    Amount:           "1000.00",
    SourceAsset:      "USDC",
    SourceNetworkId:  gen.NetworkId("ethereum-mainnet"),
    DestinationId:    destinationID,
    DestinationAsset: "USD",
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
- `CreateCustomerWithResponse(ctx, request)`
- `ListCustomersWithResponse(ctx, params)`
- `GetCustomerWithResponse(ctx, customerID)`

### Recipients
- `CreateRecipientWithResponse(ctx, customerID, request)`
- `ListRecipientsWithResponse(ctx, customerID, params)`
- `GetRecipientWithResponse(ctx, recipientID)`
- `UpdateRecipientWithResponse(ctx, recipientID, request)`

### Destinations
- `CreateDestinationWithResponse(ctx, recipientID, request)`
- `ListDestinationsWithResponse(ctx, recipientID, params)`

### Accounts (On-ramp/Off-ramp/Swap)
- `CreateAccountWithResponse(ctx, request)`
- `ListAccountsWithResponse(ctx, params)`
- `GetAccountWithResponse(ctx, accountID)`
- `UpdateAccountWithResponse(ctx, accountID, request)`

### Transactions (One-Off)
- `CreateTransactionWithResponse(ctx, request)`
- `ListTransactionsWithResponse(ctx, params)`
- `GetTransactionWithResponse(ctx, transactionID)`
- `CreateTransactionCancellationWithResponse(ctx, transactionID)`

### Auto Transactions (Recurring)
- `ListAutoTransactionsWithResponse(ctx, params)`
- `GetAutoTransactionWithResponse(ctx, transactionID)`

### Wallets
- `CreateWalletWithResponse(ctx, request)`
- `GetWalletBalancesWithResponse(ctx, walletID)`
- `CreateWalletTransactionWithResponse(ctx, walletID, request)`

### Signer Groups (Multi-sig)
- `CreateSignerGroupWithResponse(ctx, request)`
- `ListSignerGroupsWithResponse(ctx, params)`
- `GetSignerGroupWithResponse(ctx, signerGroupID)`
- `GetSignerGroupsForWalletWithResponse(ctx, walletID)`
- `CreateSignerWithResponse(ctx, request)`
- `DeleteSignerWithResponse(ctx, signerID)`
- `CreateSignerGroupSignerWithResponse(ctx, signerGroupID, request)`
- `DeleteSignerGroupSignerWithResponse(ctx, signerGroupID, signerID)`
- `UpsertWalletSignerGroupRelationshipWithResponse(ctx, walletID, request)`
- `DeleteWalletSignerGroupRelationshipWithResponse(ctx, walletID, request)`

### Policies (Transaction Rules)
- `CreatePolicyWithResponse(ctx, request)`
- `ListPoliciesWithResponse(ctx, params)`
- `GetPolicyWithResponse(ctx, policyID)`
- `DeletePolicyWithResponse(ctx, policyID, request)`
- `AddPolicyRuleWithResponse(ctx, policyID, request)`
- `UpdatePolicyRuleWithResponse(ctx, policyID, ruleID, request)`
- `DeletePolicyRuleWithResponse(ctx, policyID, request)`
- `UpsertPolicyWalletRelationshipWithResponse(ctx, policyID, request)`
- `DeletePolicyWalletRelationshipWithResponse(ctx, policyID, request)`

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
- `CreateWebhookTargetWithResponse(ctx, request)`
- `ListWebhookTargetsWithResponse(ctx, params)`
- `GetWebhookTargetWithResponse(ctx, targetID)`
- `UpdateWebhookTargetWithResponse(ctx, targetID, request)`
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
- `SimulateInboundWithResponse(ctx, request)` - Simulate deposits/payments
- `SimulateOnboardingWithResponse(ctx, request)` - Simulate KYB status changes
- `ListSandboxScenariosWithResponse(ctx, params)`
- `GetSimulationWithResponse(ctx, simulationID)`
- `AdvanceSimulationWithResponse(ctx, simulationID, request)`

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

## Type Hints for Union Types

Some responses use union types. Extract with `As*` methods:

```go
// Account responses
account, err := resp.JSON201.AsOfframpAccount()
account, err := resp.JSON201.AsOnrampAccount()
account, err := resp.JSON201.AsSwapAccount()

// Destination responses
dest, err := resp.JSON201.AsFiatUSDestinationResponse()
dest, err := resp.JSON201.AsCryptoDestinationResponse()
```

## Common Mistakes to Avoid

1. **Don't skip CheckResponse** - Always wrap API calls with `client.CheckResponse()`
2. **Don't forget KYB** - Customers need `kyb_status: "active"` before creating accounts
3. **Don't mix environments** - Sandbox and Production have different API keys
4. **Don't ignore webhooks** - Use webhooks to track transaction status changes
