# Changelog

All notable changes to the Dakota Go SDK are documented in this file.

## [Unreleased]

### Added

- **Agentic payments (alpha).** Hosted payment-agent client surface: wallet
  endorsement helpers (`AttachUserToWallet` / `DetachUserFromWallet`), multi-turn
  proposal drafting (`AgentConversation`), and the §8 mandate-signing kit
  (`Signer`, `P256Signer`, `MandateSignPayload`, `VerifyMandateSignature`). See
  `client/example_test.go` and the README "Agentic Payments (Alpha)" section.
  This surface is `x-alpha`/flag-gated and may change without a major bump.

### Changed

- Regenerated `client/gen` against the renamed platform spec (ENG-2701): the
  payment-agent resource is `/payment-agents` (was `/agents`), the path/body id
  is `payment_agent_id` (was `agent_id`), operations are `…PaymentAgent`, schemas
  are `PaymentAgent…`, and the single-value `type` field is dropped.

## [0.4.0] - 2026-06-17

### Summary

Sync against platform OpenAPI spec — nine platform commits since 0.3.0
(2026-05-11). Regenerated `client/gen/client.gen.go` from the latest
`openapi.public.yaml`. Adds 4 new operations, 2 new types, and field
additions across customers, transactions, wallet transactions, and
send-transaction intents.

### Added

#### Wallets read-only UI (ENG-1886/1887/1888/1889/1890/1891)
- `GetWalletWithResponse(ctx, walletId)` — fetch one wallet by id
  (`GET /wallets/{walletId}`).
- `GetPoliciesForWalletWithResponse(ctx, walletId)` — list policies
  attached to a wallet (`GET /wallets/{walletId}/policies`).
- `GetWalletsForPolicyWithResponse(ctx, policyId)` — list wallets
  attached to a policy (`GET /policies/{policyId}/wallets`).
- `GetWalletsForSignerGroupWithResponse(ctx, signerGroupId)` — list
  wallets attached to a signer group
  (`GET /signer-groups/{signerGroupId}/wallets`).

#### New generated types
- `AttachedPolicy` — slim `{id, name}` reference for policies attached
  to a wallet.
- `AttachedWallet` — slim `{id, name, family}` reference for wallets
  attached to a policy or signer group.

#### New optional fields

- **Customers (ENG-2454):** `Customer.IsSubClient *bool` +
  `CustomerCreateRequest.IsSubClient *bool`. Designate a customer as a
  sub-client at creation. A regular customer cannot be promoted to a
  sub-client afterwards; cannot be combined with `SubClientId`.
- **Transactions list (ENG-2368):** `ListTransactionsParams.WalletId *KSUID`
  + `ListTransactionsParams.Direction` (`in`, `out`). Both filter wallet
  transactions and require `TransactionType=wallet`.
- **Wallet transactions (ENG-2368):** `WalletTransaction.CreatedAt *int64`
  + `WalletTransaction.ConfirmedAt *int64` (Unix seconds; `ConfirmedAt`
  absent until on-chain confirmation).
- **Send transaction intent (ENG-1962):** `SendTransactionIntent.ContextDigest *string`
  — optional opaque SHA-256d digest of an upstream operator-meaningful
  context envelope. Policy-engine includes it in canonical hashing so
  the WebAuthn signature commits to it; the pre-image is persisted
  upstream for forensic / non-repudiation purposes.

### Changed

- Regenerated `client/gen/client.gen.go` from
  `~/Work/dakota/platform/openapi.public.yaml` (HEAD as of 2026-06-17).
- Wallet balance description tightened (ENG-2064): `total_amount_usd`
  and `amount_usd` are rounded DOWN to cents (truncated toward zero),
  never up, so the value never exceeds the holder's spendable balance.
  Description only — no schema impact.
- Webhook target URL validation now rejects loopback and private IPs
  in both sandbox and production (ENG-1997). Server-side enforcement
  only — the SDK's `WebhookTarget` types are unchanged.
- Transaction amount validation tightened at the platform endpoints
  (ENG-1959). Server-side only.
- Onboarding: original customer is now kept parented to Dakota on
  Provider (ENG-1227). Doc text on the application document upload
  endpoint about the rolling 7-day individual-PoA limit.
- Self-serve credit tiles: removed $5k and $10k presets (ENG-2008).

### Validated

- `go build ./...` clean.
- `go vet ./...` clean.
- `go test ./...` passes (`client`, `errors`, `log`, `webhook`,
  `webhook/idempotency`, `webhook/types`).

## [0.3.0] - 2026-05-11

### Summary

Sync the SDK's OpenAPI spec to the latest Dakota Platform API docs
(`mintlify-docs`). Regenerated `client/gen/client.gen.go` from the updated
spec — adds 2 new endpoints, 4 new schemas, and pulls in shape updates for
~18 schemas / 16 operations that already existed.

### Added

#### Customers
- `BulkImportFromSumsubTokensWithResponse()` — bulk-import customers from
  one or more Sumsub share tokens (`POST /customers/bulk-import-sumsub-tokens`).
  Returns per-token results so partial successes are observable.

#### Self-Serve Credits
- `GetSelfServeCreditsPricingWithResponse()` — fetch the caller's
  `ClientPricingConfig` (fee schedule: ACH / wire / SEPA / SWIFT / KYC / KYB
  + monthly minimum) (`GET /self-serve/credits/pricing`). Self-serve clients only.

#### New generated types
- `ClientPricingConfig`
- `FiatUSDestinationAddress`
- `InsufficientCreditsError`, `InsufficientCreditsErrorError`
- `SenderDetails`

### Changed

- Regenerated `client/gen/client.gen.go` from the latest `openapi.yaml`
  (mintlify-docs source of truth). Pulls in shape changes for `Application`,
  `AutoAccountTransaction`, `ClientUser`, `Customer`, `FiatIBANDestinationRequest`,
  `FiatIBANDestinationResponse`, `FiatUSDestinationRequest`, `KybLinkType`,
  `OneOffTransaction`, `OneOffTransactionRequest`, `OneOffTransactionStatus`,
  `PaymentCapability`, `Policy`, `SelfServeCreditsLedgerEntry`, `Signer`,
  `SignerCreateRequest`, `Transaction`, and `TransactionStatus`.

## [0.2.0] - 2026-04-17

### Summary

Full SDK audit and sync against the OpenAPI public spec. Regenerated the client from the updated specification, adding 10 new endpoints and all associated types.

### Added

#### Customers
- `UpdateCustomerSubClientWithResponse()` — associate or disassociate a customer with a sub-client (`PATCH /customers/{id}/sub-client`)
- `GetSubClientSummaryWithResponse()` — list all sub-clients with customer counts (`GET /customers/sub-client-summary`)
- `ListCustomersParams` now supports `SubClientId` and `IsSubClient` query filters

#### Recipients & Destinations
- `DeleteRecipientWithResponse()` — soft-delete a recipient (`DELETE /recipients/{id}`)
- `DeleteDestinationWithResponse()` — delete a destination (`DELETE /recipients/{id}/destinations/{id}`)

#### Accounts
- `DeleteAccountWithResponse()` — soft-delete an account (`DELETE /accounts/{id}`)

#### API Keys
- `CreateApiKeyForClientWithResponse()` — create API key for a specific client, admin only (`POST /api-keys/admin`)

#### Self-Serve Credits (new)
- `CreateSelfServeCreditsPurchaseWithResponse()` — create Stripe checkout session (`POST /self-serve/credits/purchase`)
- `GetSelfServeCreditsBalanceWithResponse()` — get current credit balance (`GET /self-serve/credits/balance`)
- `ListSelfServeCreditsLedgerWithResponse()` — list ledger entries (`GET /self-serve/credits/ledger`)
- `ListSelfServeCreditTiersWithResponse()` — list available purchase tiers (`GET /self-serve/credits/tiers`)

#### Types
- `UpdateCustomerSubClientRequest`, `SubClientSummary`
- `SelfServeCreditsPurchaseRequest`, `SelfServeCreditsPurchaseResponse`
- `SelfServeCreditsBalanceResponse`
- `SelfServeCreditsLedgerEntry`, `SelfServeCreditsLedgerResponse`
- `SelfServeCreditTier`, `SelfServeCreditTiersResponse`
- `Customer` model now includes `SubClientId`, `SubClientName`, `IsSubClient` fields
- `OneOffTransaction` now includes `ResourceType` field

### Fixed

- Event type count test updated from 39 to 40 (was already incorrect after `customer.kyb_application.submitted` was added)
- `OneOffTransaction` no longer includes a `FailureReason` field (removed from spec)
- Various response examples updated to match current spec

### Internal

- Synced `client/gen/openapi.yaml` with the platform's `openapi.public.yaml`
- Regenerated `client/gen/client.gen.go` via oapi-codegen v2.5.1
