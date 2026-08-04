# Claude Code Instructions for Dakota Go SDK

## Project Overview

This is the official Go SDK for Dakota Platform - a stablecoin payments infrastructure.

**Main capabilities:**
- On-ramp (USD → crypto)
- Off-ramp (crypto → USD)
- Swap (crypto ↔ crypto)
- Non-custodial wallets

## Code Organization

```
client/              # Main package users import
├── client.go        # Client struct, New(), options
├── environment.go   # Sandbox/Production URLs
├── errors.go        # APIError, TransportError
├── pagination.go    # Iterator helpers
├── parsers.go       # SDK-friendly model converters
└── gen/             # Generated from OpenAPI (don't edit manually)
    ├── client.gen.go
    └── openapi.yaml

webhook/             # Webhook signature verification
errors/              # SDK error types
log/                 # Logging abstraction
```

## When Helping Users

### For Integration Questions
1. Read `README.md` for examples
2. Read `AGENTS.md` for API reference
3. Check `client/gen/client.gen.go` for available methods

### For API Method Questions
Most create/update methods follow: `c.Raw().<Method>WithResponse(ctx, params, request)`

**Important**: The `params` argument is for idempotency keys and can be `nil`.

Example:
```go
resp, err := client.CheckResponse(
    c.Raw().CreateCustomerWithResponse(ctx, nil, gen.CustomerCreateRequest{...}),
)
```

### For Environment Questions
- Default: Sandbox (`https://api.platform.sandbox.dakota.xyz`)
- Production: Add `client.WithEnvironment(client.EnvironmentProduction)`

### For Error Handling
Always check for `*client.APIError` and `*client.TransportError`.

## Resource Dependency Chain

```
Customer (needs KYB approval first)
  └── Recipient (needs Address for fiat destinations)
      └── Destination (bank or crypto wallet)
          └── Account (onramp/offramp/swap)
```

## Critical Implementation Notes

### Union Types
- **DestinationRequestUnion**: Use `FromFiatUSDestinationRequest()` or `FromCryptoDestinationRequest()` to set
- **DestinationResponseUnion**: Use `AsFiatUSDestinationResponse()` or `AsCryptoDestinationResponse()` to extract
- **AccountResponse**: NOT a union type - access fields directly

### Common Field Name Mistakes
- Bank routing: Use `AbaRoutingNumber` NOT `RoutingNumber`
- Bank accounts also require `BankName` field
- Off-ramp accounts require both `Capabilities` AND `Rail` fields
- ListAccounts requires `AccountType` in params (not optional)

## Regenerating Client

If OpenAPI spec changes:
```bash
make generate-client
```

### Syncing the spec — the platform's public spec can be stale

`client/gen/openapi.yaml` is synced from the platform's
`openapi.public.yaml`, which the platform GENERATES from its internal
`openapi.yaml` (`make openapi-public`). That generation is a manual step,
so the public file can lag the routes the server actually serves — the
internal `openapi.yaml` is what `internal/api/oapi/routes.go` is
generated from, and is therefore authoritative.

This has already bitten the TypeScript SDK: it shipped
`/clients/{client_id}/agentic-policy` from a stale public spec while the
server served `/agentic-policy`, so every call 404'd (fixed in ts-sdk
2.2.1). **After any sync, if a path looks wrong, check the platform's
internal `openapi.yaml` and `routes.go` before trusting the public
file.** Deliberate deviations are pinned by `TestSpecGuards` in
`client/spec_guards_test.go` — add a case there whenever you hand-correct
the spec, so a later sync fails loudly instead of shipping.

## Testing

```bash
go test ./...
```
