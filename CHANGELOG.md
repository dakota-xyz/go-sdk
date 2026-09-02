# Changelog

All notable changes to the Dakota Go SDK are documented in this file.

## [Unreleased]

### Changed — the agentic surface is BETA, not alpha (ENG-3168)

Platform promoted the whole agentic surface from alpha to beta. Nothing about
the wire contract moved: same paths, parameters, schemas, status codes and
response shapes. What changed is the maturity marker, and it reaches this SDK
three ways — the vendored spec's `x-alpha: true` is now `x-beta: true`, the 21
operation summaries read `(BETA)`, and the generated doc comments that quote
them followed. The hand-written notes in `README.md`, `client/doc.go` and the
`client/agentic*.go` helpers say beta too.

Beta is not a promise of stability. These helpers may still change, or be
removed, without a major-version bump, and the endpoints stay flag-gated on the
platform (`404` unless enabled for your key).

### Added — `AgenticProposal.PaymentAgentId`

Picked up by the same re-vendor. It names the payment agent a proposal was
drafted under, and is present on proposals returned by the drafting endpoint —
it echoes the agent you called. Ignored on input: the accept endpoint takes the
agent in its own `payment_agent_id` field. Optional, so nothing existing breaks.

### Changed — spec sync with platform main

`client/gen/openapi.yaml` is a copy of platform `openapi.public.yaml` at the
alpha-to-beta commit, replacing one that was 82 lines behind. Beyond the two
items above the delta is documentation only, and the corrections are worth
reading if you touch SWIFT:

- **A payment reference DOES reach a SWIFT payee**, carried as the wire's
  remittance information. The old text said SWIFT "carries no payment reference
  at all" and told you not to plan on one — that was wrong. Whether the payee
  sees it still depends on the receiving bank and any intermediaries, so it is
  not something to reconcile against, but it is delivered.
- **SWIFT reference formatting is tighter than documented**: 5-140 characters
  across at most 4 lines of 35, and only letters, numbers, spaces, commas and
  periods.
- **The self-serve SWIFT banking fee is 2500 ($25.00), not 4000.** Debited from
  the prepaid credit balance on outbound transfers only; deposits carry no
  banking fee.
- **International (SWIFT) wire returns carry no NACHA code**, and may return a
  reduced amount after intermediary fees. Intermediary and beneficiary banks can
  deduct en route; Dakota neither controls nor learns those deductions, and they
  do not appear in external fees — so the amount received can be lower than the
  amount sent.
- Onboarding submission is no longer `pending`-only: applicants may also submit
  in `request_for_information` and `compliance_review`, and Dakota admins in
  `admin_revision`.

### Removed

- **Insight chat (alpha).** `ChatCustomerInsightsWithResponse` and the
  `InsightChat{Message,Request,Response}` types are gone from `client/gen`:
  platform removed `POST /customers/{customer_id}/insights/chat` for the agentic
  BETA (ENG-3153), so the call had no server to reach. The deterministic report
  `GetCustomerInsightsWithResponse` is unaffected. Nothing here shipped in a
  tagged release — the operation existed only on `main` — and the conversational
  surface is expected to return in a reshaped form after the beta.

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
  the customer's IANA zone instead of UTC. It is resent on every turn, since
  the endpoint is stateless — and must be passed again to
  `ResumeAgentConversation`, which restores the transcript, not the options.
- **Client policy registration.** `GET`/`PUT /agentic-policy` via
  `c.Raw().GetClientAgenticPolicyWithResponse` /
  `UpdateClientAgenticPolicyWithResponse`. This is the ONLY way to set a
  policy: it belongs to the client, not to a request, so a drafting turn and
  the accept that follows it cannot be judged by different rules. Registration
  is a full replace; `{}` clears it.
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
- **Customer insight (alpha).** The read-only account-insight operation
  generated into `client/gen` and reachable via `c.Raw()`:
  `GetCustomerInsightsWithResponse` (the deterministic report — snapshot,
  `insights[]`, `suggestions[]`, each item carrying typed `evidence`). Also
  `x-alpha`/flag-gated. (This entry also announced a companion
  `ChatCustomerInsightsWithResponse`; see **Removed** above — it never reached
  a tagged release.)

### Changed

- Regenerated `client/gen` against the renamed platform spec (ENG-2701): the
  payment-agent resource is `/payment-agents` (was `/agents`), the path/body id
  is `payment_agent_id` (was `agent_id`), operations are `…PaymentAgent`, schemas
  are `PaymentAgent…`, and the single-value `type` field is dropped.

### Fixed

- **`types.AmountThreshold.MinAmount` is now `int64`, not `string`.** The
  platform emits a policy rule's `min_amount` as a whole number of the
  asset's smallest currency unit — a JSON number, as the bundled spec's
  example shows. Because `webhook.EventDataAs` is a strict
  `json.Unmarshal`, a `string` field here did not decode to an empty value
  the way the rest of this sweep did: it failed the *entire* event. Any
  `wallet.created`, `wallet.updated`, `wallet.policy.created`, or
  `wallet.policy.updated` carrying an `amount_threshold` rule returned
  `MALFORMED_PAYLOAD` and delivered nothing to the consumer. The type change
  is source-breaking for anything that referenced `.MinAmount` as a string,
  but no such reference can have been reading a real value, because no event
  containing the field ever decoded.
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
  decodes to `nil` instead of `0`. Both changes are source-breaking: `Link`
  is removed, and `time.Unix(data.ExpiresAt, 0)` no longer compiles against
  an `*int64`. Neither field ever held a usable value on any released
  version — `Link` was always empty and `ExpiresAt` was always either a real
  timestamp or a `0` indistinguishable from the Unix epoch — so no working
  code can regress. Note also that a nil `ExpiresAt` is the *common* case on
  `customer.kyb_link.created`, not the exception: check for nil before
  dereferencing.
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
  `ProviderStatus` on both structs could never be populated by a *delivered
  webhook payload* — the only thing these structs are for, and the only
  thing this package models. Removing them is source-breaking for anything
  that referenced those fields, but they never held a value on any released
  version, so no working code can regress.
- **`bvnk.onboarding.created`, `bvnk.onboarding.updated`, and
  `types.BVNKOnboardingData` are removed.** Dakota does not use BVNK as a
  provider — the platform has no emitter for these event types and never
  sends them, so no consumer of this SDK could ever have received one.
  (The event types remain in the platform's published public event-type
  enum for now; removing them there is tracked separately and out of scope
  for this SDK.) Removing `EventBVNKOnboardingCreated`,
  `EventBVNKOnboardingUpdated`, and `types.BVNKOnboardingData` is
  source-breaking for anything that referenced them, but since no delivered
  webhook could ever carry this shape, no running code can regress.
- **`types.CustomerData` now matches the payload the platform actually
  sends.** The struct tagged its primary key `id`, but `customer.created`
  and `customer.updated` carry `customer_id` — so the customer ID on every
  customer webhook decoded to `""`, the same silent-empty-identifier failure
  as the KYB structs above. `ReferenceID` was a mis-tag of the emitted
  `external_id`, and `Email`, `Status`, `PhoneNumber`, `DateOfBirth`,
  `SSNLastFour`, `IncorporatedOn`, and `Address` are not emitted on these
  events at all. `ID` becomes `CustomerID` (tag `customer_id`),
  `ReferenceID` becomes `ExternalID` (tag `external_id`), the seven
  never-populated fields are removed, and `ImportReference`
  (`{source, reference}`, present on `customer.created` for token-sharing
  imports) is added. The rename and removals are source-breaking, but none
  of those fields ever held a value on any released version, so no working
  code can regress.
- **`types.TargetCreatedData`, `types.TargetUpdatedData`, and
  `types.TargetDeletedData` described the wrong concept entirely.** The
  structs modeled a savings/payout target (`amount`, `currency`,
  `frequency`); a platform *target* is a registered webhook endpoint.
  `target.created` carries `target_id`, `url`, `global`, and an optional
  `event_types`; `target.updated` and `target.deleted` carry the same
  endpoint under `target_url` instead of `url`, which is why these remain
  three types rather than one. Every previous field is removed. This is
  source-breaking, but no field on any of the three ever held a value on any
  released version, so no working code can regress.
- **`types.ExceptionData` and `types.ExceptionClearedData` now match the
  payloads the platform actually sends.** Both events identify the exception
  as `exception_id`, not `id`, and carry a `type`; `exception.created` adds
  an optional `customer_id` and an open-ended `exception_content` blob.
  Neither event carries a timestamp of its own — the enclosing envelope's
  `Created` is the time. `AutoAccountID`, `TransactionID`, `Message`, and
  `CreatedAt`/`ClearedAt` are removed; `ExceptionID`, `CustomerID`, and
  `Content` are added. Five of the six fields on the first struct and all
  three on the second never held a value on any released version, so no
  working code can regress.
- **`types.RecipientData` and `types.RecipientUpdatedData` now match the
  payloads the platform actually sends.** `Type` and `BankAccount` are not
  emitted on either event, and the emitted `status` and `address` were not
  decodable. Both fields are removed in favor of `Status` and `Address`.
  Separately, **`recipient.updated` carries no `customer_id` at all**, so
  `RecipientUpdatedData.CustomerID` always decoded to `""`; it is removed
  rather than left to read as an empty string — resolve the owner from `ID`,
  or retain the `customer_id` delivered with `recipient.created`.
  `types.RecipientDeletedData` gains `CustomerID`, which that event always
  emits. The removals are source-breaking, but none of the removed fields
  ever held a value on any released version, so no working code can regress.
- **`types.DestinationData` now matches the payload the platform actually
  sends.** `destination.created` carries `id`, `recipient_id`, `name`,
  `type`, and exactly one of an optional `crypto` or `bank_account` block.
  `CustomerID` and `Currency` are not emitted and are removed; `Name`,
  `Type`, and `Crypto` are added. `types.DestinationDeletedData` gains
  `RecipientID`, which that event always emits. The removals are
  source-breaking, but neither field ever held a value on any released
  version, so no working code can regress.
- **`types.APIKeyData` now matches the payload the platform actually
  sends.** `api_key.created` carries `id` and `last_6`; `UserID` and `Name`
  are not emitted, and `last_6` — the one fragment of a key that is safe to
  display and the field clients use to identify a key in a list — was not
  decodable. `UserID` and `Name` are removed and `Last6` is added. The event
  also carries the key's `hash`, which is deliberately *not* surfaced on the
  SDK's typed payload. The removals are source-breaking, but neither field
  ever held a value on any released version, so no working code can regress.
- **`types.WalletTransactionIntent.Sponsor` is removed and
  `types.WalletTransactionOperation` gains `AssetID`.** No platform event
  ever emits a `sponsor` key, so the field was always `false` — a value a
  consumer could easily mistake for "gas is not sponsored". Conversely
  `asset_id` is emitted on every wallet transaction operation and was not
  modeled, so the asset silently decoded to `""`. The removal is
  source-breaking, but the field never held a meaningful value on any
  released version, so no working code can regress.
- **`types.OneOffTransactionData` gains `SenderDetails`.** The platform
  emits `sender_details` on one-off transactions (the crypto subset:
  `sender_wallet_address` and `sender_network`) and the sanitizer does not
  strip it, but only the auto-transaction struct modeled it, so the
  originating wallet was unreachable from a one-off payload. Additive only.

### Deliberately deferred

The sweep above corrects the field-level mismatches found against the
platform emitters. These emitted keys are knowingly *not* modeled yet, and
are additive whenever they land — no struct below decodes incorrectly
without them:

- `fiat_rail`, `return_code`, `return_reason`, `return_initiated_at`,
  `return_deadline`, `net_recovered_amount`, `reversal_reason`, and
  `reversal_initiated_at` on the transaction payloads (the return and
  reversal lifecycles).
- `developer_fee_bps` on `auto_account.created` / `auto_account.updated`.
- `hash` on `api_key.created`, which will stay unmodeled by choice.
- The gap between `webhook.AllEventTypes` and the set of event types the
  platform actually emits, which is tracked separately.

### Changed

- **A second regeneration of `client/gen`**, on top of the one described at the
  top of this section: the vendored spec was last synced 2026-08-04, and this
  brings it level with platform main again. `c.Raw()` and the `gen.` types are
  a documented part of this SDK's surface, so it carries source-breaking
  changes for code using them directly:
  - `PaginatedOneOffTransactionResponse.Meta`,
    `PaginatedWalletTransactionResponse.Meta` and
    `PaginatedCustomerTransactionResponse.Meta` are now
    `gen.TransactionListMeta` (was `gen.Meta`) — the same three pagination
    fields plus a required `transaction_type` naming the family the page
    lists. A whole-struct assignment between the two no longer compiles.
  - `ApplicationDocumentUploadUrlRequest.Country` is now `*string` (was
    `string`): some supporting documents, such as the generic `other` type,
    have no issuing country.
  - `gen.CreateProposalsRequest.ClientPolicy` and
    `gen.CreateInstructionsRequest.ClientPolicy` are gone — see **Removed**.
- **`OneOffTransactionsIterator` now rejects an explicit `TransactionType`
  other than `one_off`**, where it previously honored it silently. The iterator
  yields `gen.OneOffTransaction`, so asking it for the wallet or auto-account
  family was always incoherent — it just failed quietly, unmarshalling foreign
  rows into zeroed structs. The error surfaces from the first `Next` call.
  Reach the other families through `Raw().ListTransactionsWithResponse`. See
  **Fixed** for the silent-substitution half of this.
- **Five operations added**, reachable via `c.Raw()`:
  `GetLegalAcceptanceContextWithResponse`, `ListLegalDocumentsWithResponse`,
  `GetLegalDocumentWithResponse`, `ListRDMarketingFeeStatementsWithResponse`,
  `GetRDMarketingFeeStatementWithResponse`.

  These five also join `gen.ClientInterface` and
  `gen.ClientWithResponsesInterface` (185 methods each, now 190). `c.Raw()`
  returns the concrete `*gen.ClientWithResponses`, so most code is unaffected —
  but if you implement either interface, typically a generated mock, regenerate
  it or add the five methods.

### Fixed

- **`OneOffTransactionsIterator` could return another family's transactions.**
  `GET /transactions` serves three resource families from one path, and when
  `transaction_type` is omitted the server infers the family from the other
  filters — `customer_id` alone infers `auto_account`. The iterator did not
  name a family, so the obvious spelling of "this customer's one-off
  transactions":

  ```go
  c.OneOffTransactionsIterator(&gen.ListTransactionsParams{CustomerId: &id})
  ```

  returned that customer's **auto-account** transactions, which unmarshal into
  `gen.OneOffTransaction` with zeroed fields rather than erroring. Nothing
  surfaced the substitution, so a caller could not tell.

  The iterator now always sends `transaction_type=one_off`, and additionally
  verifies the family the response reports before yielding a page — a positive
  mismatch only, since an absent `transaction_type` means the response did not
  say rather than that it served the wrong family.

  **What changes for you:** if you filter by `customer_id` without naming a
  family, you were receiving auto-account rows and will now receive one-off
  rows. That is the correction, but it *is* a change in the data you get back.
  See also the related breaking change under **Changed**.

  This was live before the regen; the newly-synced spec is what made it
  visible, since responses now name the family they served.

  The README example at the "Iterate transactions with filters" heading taught
  exactly this trap, and additionally named a `gen.ListOneOffTransactionsParams`
  type that the generated client had already dropped by the time that example
  was written — so it never compiled as printed. Both corrected.

### Removed

- **`WithClientPolicy` — added and removed within this same unreleased cycle.**
  No tagged release ever carried it, so no released version can regress. It is
  called out only for anyone tracking an unreleased commit; if that is you, see
  the migration below.

  The platform removed `client_policy` from the
  `POST /payment-agents/{id}/proposals` and `POST /instructions` bodies
  deliberately — dropping the field from both
  `gen.CreateProposalsRequest` and `gen.CreateInstructionsRequest`: a policy is a
  property of the CLIENT, not of a request, and carrying one per request let a
  two-call conversation disagree with itself — a proposal drafted under one
  policy and accepted without it is judged by different rules, so a legal draft
  could be refused at the customer's approval click. The field is gone from the
  request schema, so the option could no longer do anything, and it failed
  *silently*: the agent simply narrated in platform's nouns again, with no
  error anywhere.

  If you are pinned to an unreleased commit that used it, register the policy
  once instead of passing it per conversation:

  ```go
  // before
  conv := c.NewAgentConversation(agentID, client.WithClientPolicy(policy))

  // after — once per client, at startup
  _, err := c.Raw().UpdateClientAgenticPolicyWithResponse(ctx, nil, policy)
  conv := c.NewAgentConversation(agentID)
  ```

  See `ExampleClient_Raw_registerAgenticPolicy`.

### Internal

- `OneOffTransactionsIterator` now copies the transactions page `Meta` field-by-field
  instead of by whole-struct assignment. Transaction list responses now carry their
  own `TransactionListMeta`, which is not assignable to `gen.Meta`; the keyed copy
  is what keeps the regeneration above compiling. Its `transaction_type` field is
  not surfaced on `Page` — see the comment at the assignment site. No behavior change.

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
