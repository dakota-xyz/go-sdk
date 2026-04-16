# Changelog

All notable changes to the Dakota Go SDK are documented in this file.

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
