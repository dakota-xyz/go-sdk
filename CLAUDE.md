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
All methods follow: `c.Raw().<Method>WithResponse(ctx, params)`

Example:
```go
resp, err := client.CheckResponse(
    c.Raw().CreateCustomerWithResponse(ctx, gen.CustomerCreateRequest{...}),
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
  └── Recipient
      └── Destination (bank or crypto wallet)
          └── Account (onramp/offramp/swap)
```

## Regenerating Client

If OpenAPI spec changes:
```bash
make generate-client
```

## Testing

```bash
go test ./...
```
