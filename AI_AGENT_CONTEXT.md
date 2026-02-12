# Dakota Go SDK - AI Agent Context

This document provides comprehensive context for AI agents working with the Dakota Go SDK. The SDK is the public interface for customers to integrate with the Dakota Platform, a financial services system for stablecoin operations.

## Overview

The Dakota Go SDK (`github.com/dakota-xyz/go-sdk`) is a Go library that enables secure integration with the Dakota Platform. The primary focus is on webhook handling for real-time event notifications from the Dakota Platform.

## What is Dakota Platform?

Dakota Platform is a backend-for-backends financial services system that provides:

### Core Capabilities
1. **Stablecoin Operations**: Minting and burning of digital assets (primarily DKUSD stablecoin)
2. **Payment Rails**: Bridging between crypto and fiat currencies through multiple payment networks
3. **KYB/KYC Services**: Unified interface for business and customer verification across multiple providers
4. **Transaction Routing**: Intelligent routing to optimize for cost or speed based on transaction characteristics
5. **Multi-Currency Support**: Handles both fiat (USD, EUR, etc.) and crypto assets across various blockchain networks

### Key Services
- **Issuance API**: Asset minting and burning operations
- **Onboarding API**: Know Your Business/Customer verification
- **On/Off Ramp API**: Fiat-to-crypto (onramp) and crypto-to-fiat (offramp) conversions
- **Recipients API**: Managing payment destinations for verified entities
- **Transactions API**: Viewing and managing transaction history
- **Auto Accounts**: Automated account management for recurring transactions
- **Webhooks**: Real-time event notifications for all platform operations

## SDK Architecture

### Package Structure
```
github.com/dakota-xyz/go-sdk/
├── errors/          # Structured error types with error codes
├── log/             # Logging utilities
├── webhook/         # Core webhook handling functionality
│   ├── idempotency/ # Idempotency store implementations
│   └── types/       # Event type definitions
```

### Import Conventions
```go
// Use aliases to avoid conflicts with stdlib
import (
    sdkerrors "github.com/dakota-xyz/go-sdk/errors"
    sdklog "github.com/dakota-xyz/go-sdk/log"
)
```

## Webhook System

The webhook system is the primary mechanism for receiving real-time updates from Dakota Platform.

### Security Features
1. **Ed25519 Signature Verification**: All webhooks are cryptographically signed
2. **Timestamp Validation**: Prevents replay attacks (5-minute window)
3. **Idempotency**: Ensures exactly-once processing of events
4. **Atomic Handler Execution**: Thread-safe event processing

### Event Categories

#### User & Authentication Events
- `user.created`: New user account created
- `user.updated`: User information modified
- `user.deleted`: User account removed
- `api_key.created`: New API key generated
- `api_key.deleted`: API key revoked

#### Customer Management Events
- `customer.created`: New customer onboarded
- `customer.updated`: Customer information changed
- `customer.kyb_status.created`: KYB verification initiated
- `customer.kyb_status.updated`: KYB status changed (approved/rejected/pending)
- `customer.kyb_link.created`: KYB verification link generated

#### Financial Account Events
- `auto_account.created`: Automated account established
- `auto_account.updated`: Account configuration changed
- `auto_account.deleted`: Account removed
- `recipient.created/updated/deleted`: Payment recipient management
- `destination.created/deleted`: Payment destination management

#### Transaction Events
- `transaction.auto.created`: Automated transaction initiated
- `transaction.auto.updated`: Transaction status changed
- `transaction.one_off.created`: Manual transaction initiated
- `transaction.one_off.updated`: Transaction status changed

#### Target & Exception Events
- `target.created/updated/deleted`: Transaction targets for auto accounts
- `exception.created`: System exception occurred
- `exception.cleared`: Exception resolved

#### Provider-Specific Events
- `bvnk.onboarding.created/updated`: BVNK provider onboarding status

### Event Data Structures

Each event contains:
```go
type Event struct {
    ID        string      // Unique event identifier (KSUID)
    Type      string      // Event type (e.g., "customer.created")
    Data      interface{} // Type-specific payload
    Timestamp int64       // Unix timestamp
}
```

### Transaction States

Transactions progress through these states:
- `pending`: Transaction created, awaiting processing
- `processing`: Transaction is being executed
- `completed`: Successfully completed
- `failed`: Transaction failed (check failure_reason)

### Customer/KYB States
- `pending`: Verification in progress
- `approved`: Successfully verified
- `rejected`: Verification failed (check reason field)
- `expired`: Verification link/session expired

## Implementation Patterns

### Basic Webhook Handler
```go
handler, err := webhook.NewHandler(
    webhook.WithPublicKey(publicKeyHex),
    webhook.OnDefault(func(ctx context.Context, event webhook.Event) error {
        // Process any event
        return nil
    }),
)
```

### Type-Safe Event Handling
```go
webhook.OnEvent("customer.created", func(ctx context.Context, event webhook.Event) error {
    customer, err := webhook.EventDataAs[types.CustomerData](event)
    if err != nil {
        return err
    }
    // Process customer data
    return nil
})
```

### Idempotency Store
```go
// In-memory store for development
store := idempotency.NewMemoryStore()

// Custom store must implement:
type Store interface {
    Acquire(ctx context.Context, eventID string) error  // Atomic reservation
    Commit(ctx context.Context, eventID string) error   // Mark as processed
    Release(ctx context.Context, eventID string) error  // Release on failure
}
```

### Acknowledgment Policies
- `AckOnSuccess` (default): Returns 2xx only on successful processing
- `AckAlways`: Always returns 2xx, even on processing failure

## Platform Integration Details

### API Authentication
All platform API calls require:
- `X-Api-Key`: API key for authentication
- `X-Idempotency-Key`: UUID v4 for POST/PUT/PATCH operations (24-hour validity)

### Platform Environments
- Production: `https://api.platform.dakota.xyz`
- Sandbox: `https://api.platform.sandbox.dakota.xyz`
- Development: `https://api.platform.dev.dakota.xyz`

### Key Platform Concepts

#### Auto Accounts
Automated accounts that:
- Monitor incoming transactions
- Automatically route funds based on configuration
- Support multiple output assets and destinations
- Can be configured for specific routing preferences

#### Destinations vs Recipients
- **Recipient**: A verified entity (person/business) that can receive funds
- **Destination**: A specific account/wallet for a recipient in a particular currency

#### Transaction Routing
Platform intelligently routes through:
- ACH/Wire for fiat transfers
- Bridge/Circle for stablecoin operations
- Multiple blockchain networks for crypto
- Optimizes based on speed, cost, and availability

#### CRI (Customer Reference ID)
External identifier linking platform entities to customer's internal systems.

## Error Handling

The SDK uses structured errors that support standard Go error patterns:
```go
if sdkerrors.Is(err, sdkerrors.ErrInvalidSignature) {
    // Handle signature validation failure
}
```

Common error types:
- `ErrInvalidSignature`: Webhook signature verification failed
- `ErrExpiredTimestamp`: Webhook timestamp outside valid window
- `ErrDuplicateEvent`: Event already processed (idempotency)
- `ErrMalformedPayload`: Invalid event data structure

## Development Best Practices

1. **Always verify webhook signatures** in production
2. **Implement idempotency** to handle webhook retries
3. **Use structured logging** with the provided sdklog package
4. **Handle events asynchronously** when possible for better performance
5. **Implement proper error handling** and retry logic
6. **Monitor webhook processing** latency and success rates
7. **Use type-safe event parsing** with EventDataAs function

## Testing Webhooks

### Local Development
```go
listener, err := webhook.NewListener(
    webhook.WithAddr("127.0.0.1:8080"),
    webhook.WithHandlerOptions(
        webhook.WithPublicKey(testPublicKey),
    ),
)
```

### Webhook Replay Protection
The SDK automatically validates timestamps to prevent replay attacks. Events older than 5 minutes are rejected.

## Common Integration Scenarios

### Processing Incoming Payments
1. Listen for `transaction.auto.created` events
2. Check transaction type and status
3. When status becomes `completed`, credit user account
4. Handle `failed` status for reconciliation

### KYB Verification Flow
1. Create customer via Platform API
2. Listen for `customer.kyb_link.created` event
3. Direct user to verification link
4. Monitor `customer.kyb_status.updated` for approval/rejection
5. Enable features based on KYB status

### Automated Fund Routing
1. Set up auto account with routing preferences
2. Listen for `transaction.auto.created` events
3. Monitor transaction progress through status updates
4. Handle exceptions via `exception.created` events

## Security Considerations

1. **Never expose webhook signing keys**
2. **Validate all webhook payloads** before processing
3. **Implement rate limiting** on webhook endpoints
4. **Log security events** (invalid signatures, replay attempts)
5. **Use TLS** for all webhook endpoints
6. **Implement proper access controls** on webhook configuration

## Debugging Tips

1. **Check webhook signatures** first when events aren't processing
2. **Verify timestamp synchronization** between systems
3. **Monitor idempotency store** for duplicate processing attempts
4. **Use structured logging** to trace event processing
5. **Implement health checks** for webhook endpoints
6. **Test with webhook replay** to ensure idempotency

## Additional Notes for AI Agents

When assisting developers with this SDK:

1. **Focus on webhook integration** - This is the primary use case
2. **Emphasize security** - Signature verification is critical
3. **Recommend idempotency** - Essential for production systems
4. **Suggest type safety** - Use EventDataAs for parsing
5. **Platform is private** - SDK is public interface, platform code not accessible
6. **Event-driven architecture** - Most integrations are reactive to webhooks
7. **Financial context** - This handles money, accuracy and security are paramount

## Version Compatibility

- SDK requires Go 1.22+
- Compatible with Dakota Platform API v1.0.0
- Uses Ed25519 signatures (not ECDSA or RSA)

## Support Resources

- SDK Repository: github.com/dakota-xyz/go-sdk
- Import packages with appropriate aliases to avoid stdlib conflicts
- Test thoroughly with both unit tests and integration tests
- Use race detection during testing: `go test -race ./...`