# Changelog

All notable changes to the Dakota Go SDK are documented in this file.

## [Unreleased]

### Added — agentic payments catch-up (alpha)

Regenerated `client/gen` against the current platform spec, bringing the Go
SDK level with three weeks of platform work (17 operations, 22 schemas).
Everything generated is reachable via `c.Raw()`; the items below also got a
hand-written surface.

- **Turn blockers.** `ConversationTurn.Blockers` / `HasBlockers` carry
  machine-actionable reasons a drafting turn could not complete, for the
  CLIENT APPLICATION rather than the customer — `Reply` says the same thing
  in prose, which software cannot branch on. They **accompany proposals
  rather than replacing them**, and routinely do: the common case is a payee
  who does not exist yet, where the turn proposes creating them *and*
  reports that the limit will not reach them. Codes today are
  `mandate_does_not_cover_payee` and `no_mandate`; switch on `Code` and
  ignore ones you do not recognize.
- **Mandate versions.** `MandateAmendSignPayload(m, version, rule)` produces
  the canonical bytes for an amend — the approve/cancel bytes plus a
  `version` key, which is what stops a signature collected for v2 from being
  replayed to append v3. `MandateSignPayload` now REFUSES the amend verb, so
  a versionless amend payload cannot be constructed. Byte-exact against the
  platform's own `MandateAmendPayload`. The endpoints
  (`AmendMandateWithResponse`, `ListMandateVersionsWithResponse`,
  `GetMandateBudgetWithResponse`) are on `c.Raw()`.
- **`rejected_input` no longer poisons the transcript.** The spec's fourth
  `conversation_status` means the message was refused WHOLESALE and must
  not be added to the conversation history. `AgentConversation` now rolls
  the refused user turn back and records no assistant turn, so the
  transcript is byte-identical to before the call — while the caller still
  gets the reply explaining what to resend. Left in, the refused message
  was re-transmitted on every later turn, corrupting the conversation from
  that point on. `warned` and `blocked` are unaffected: those turns
  happened and stay in the transcript.
- **Conversation options.** `WithTimezone` resolves "tomorrow" and "10 am" in
  the customer's IANA zone instead of UTC. `WithClientPolicy` sets the
  per-turn vocabulary override. Both are resent on every turn, since the
  endpoint is stateless — and both must be passed again to
  `ResumeAgentConversation`, which restores the transcript, not the options.
- **Client policy registration.** `GET`/`PUT /agentic-policy` via
  `c.Raw().GetClientAgenticPolicyWithResponse` /
  `UpdateClientAgenticPolicyWithResponse`. Prefer registering once over the
  per-turn override: forgetting the override fails SILENTLY, with the agent
  narrating in platform nouns again and nothing erroring. Registration is a
  full replace; `{}` clears it.
- **Developer fee per payout type.** `CreateInstructionsRequest.DeveloperFee`
  declares `swap_bps` and `offramp_bps` independently.
- Also generated: proposals progress, per-payment network selection,
  Persona share-token imports, customer capabilities / re-engagement /
  delete, and the fee payout destination.

### Changed

- `NewAgentConversation` and `ResumeAgentConversation` are variadic
  (`...ConversationOption`). Existing calls compile unchanged.
- `ConversationTurn.ConversationStatus` is converted from the now-typed
  generated enum; it remains a `string` on the SDK surface.
- `GetSignerGroupWithResponse` gained a params argument upstream
  (`include_removed`); internal call sites pass `nil`.

### Added

- **Agentic payments (alpha).** Hosted payment-agent client surface: wallet
  endorsement helpers (`AttachUserToWallet` / `DetachUserFromWallet`), multi-turn
  proposal drafting (`AgentConversation`), and the §8 mandate-signing kit
  (`Signer`, `P256Signer`, `MandateSignPayload`, `VerifyMandateSignature`). See
  `client/example_test.go` and the README "Agentic Payments (Alpha)" section.
  This surface is `x-alpha`/flag-gated and may change without a major bump.
- **Customer insight (alpha).** Read-only account-insight operations generated
  into `client/gen`, reachable via `c.Raw()`: `GetCustomerInsightsWithResponse`
  (the deterministic report — snapshot, `insights[]`, `suggestions[]`, each item
  carrying typed `evidence`) and `ChatCustomerInsightsWithResponse` (the advisory
  chat, `{messages[]} → {reply, conversation_status}`). Also `x-alpha`/flag-gated.

### Changed

- Regenerated `client/gen` against the renamed platform spec (ENG-2701): the
  payment-agent resource is `/payment-agents` (was `/agents`), the path/body id
  is `payment_agent_id` (was `agent_id`), operations are `…PaymentAgent`, schemas
  are `PaymentAgent…`, and the single-value `type` field is dropped.

### Fixed

- **`types.KYBStatusData` now matches the payload the platform actually sends.**
  The struct decoded `status` and `reason`, but `customer.kyb_status.created`
  and `customer.kyb_status.updated` carry `kyb_status` and an optional
  `reason_code`. Every real KYB webhook therefore unmarshalled into an empty
  `Status` and a nil `Reason`. `Status` is retagged to `kyb_status`, and
  `Reason` is renamed to `ReasonCode` (tag `reason_code`). The rename is
  source-breaking for anything that referenced `.Reason`, but that field never
  held a value on any released version, so no working code can regress.
- **`types.KYBLinkData` now matches the payload the platform actually sends.**
  The struct decoded a single `link` string and a required `expires_at`, but
  `customer.kyb_link.created` and `customer.kyb_link.updated` carry
  `link_type`, `url`, and `status`, with `expires_at` present only when the
  link actually expires. Every real KYB link webhook therefore unmarshalled
  into an empty `Link` and a zero `ExpiresAt` indistinguishable from "expires
  at the Unix epoch". `Link` is removed in favor of `LinkType`, `URL`, and
  `Status`, and `ExpiresAt` becomes `*int64` so a genuinely absent expiry
  decodes to `nil` instead of `0`. The removal of `Link` is source-breaking,
  but that field never held a value on any released version, so no working
  code can regress.
- **`types.KYBApplicationSubmittedData` now matches the payload the platform
  actually sends.** The struct decoded a bare `type` string, but
  `customer.kyb_application.submitted` carries `application_id` and
  `application_type` — there is no `type` key. `Type` is removed in favor of
  `ApplicationID` and `ApplicationType`. The removal of `Type` is
  source-breaking, but that field never held a value on any released
  version, so no working code can regress.
- **The six `Provider*` fields on `types.AutoTransactionData` and
  `types.OneOffTransactionData` are removed.** The platform's outbound
  webhook sanitizer strips every `provider_`-prefixed key before a webhook
  is ever delivered, so `ProviderID`, `ProviderExternalID`, and
  `ProviderStatus` on both structs could never be populated by a real
  payload. Removing them is source-breaking for anything that referenced
  those fields, but they never held a value on any released version, so no
  working code can regress.
- **`types.BVNKOnboardingData` is reduced to its one documented field.** The
  struct declared `id`, `status`, and `reason`, but the platform has no
  emitter for `bvnk.onboarding.created` or `bvnk.onboarding.updated` today —
  those event types exist only in the published public event-type enum,
  with a documented example payload of `{customer_id}`. The struct now
  matches that example and is retained (rather than removed) because the
  event type remains part of the public contract. `ID`, `Status`, and
  `Reason` never held a value on any released version, so no working code
  can regress.

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
