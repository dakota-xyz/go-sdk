# Dakota Go SDK - Claude Code Instructions

## What This SDK Does

The Dakota Go SDK is a lightweight library for receiving and processing webhook events from the Dakota Platform. The SDK's primary purpose is to help developers:
- Securely receive webhook notifications about financial events
- Validate webhook authenticity using Ed25519 signatures
- Ensure exactly-once processing with idempotency
- Parse events into type-safe Go structs

## Primary Use Case: Webhook Handling

90% of SDK usage involves setting up a webhook handler to receive events:

```go
import (
    "github.com/dakota-xyz/go-sdk/webhook"
    "github.com/dakota-xyz/go-sdk/webhook/idempotency"
    "github.com/dakota-xyz/go-sdk/webhook/types"
)

handler, err := webhook.NewHandler(
    webhook.WithPublicKey(publicKeyHex),
    webhook.WithIdempotencyStore(idempotency.NewMemoryStore()),
    webhook.WithAckPolicy(webhook.AckOnSuccess),
    webhook.OnEvent("customer.created", handleCustomerCreated),
    webhook.OnEvent("transaction.auto.updated", handleTransactionUpdate),
    webhook.OnDefault(handleUnknownEvent),
)

http.Handle("/webhook", handler)
```

## Package Structure

```
github.com/dakota-xyz/go-sdk/
├── webhook/         # Core webhook handling
│   ├── handler.go   # HTTP handler
│   ├── signature.go # Ed25519 verification
│   ├── listener.go  # Local webhook server
│   ├── idempotency/ # Exactly-once processing
│   └── types/       # Event type definitions
├── errors/          # SDK error types
└── log/            # Logging utilities
```

### Import Convention
Always use aliases to avoid stdlib conflicts:
```go
import (
    sdkerrors "github.com/dakota-xyz/go-sdk/errors"
    sdklog "github.com/dakota-xyz/go-sdk/log"
)
```

## Security Requirements

### ✅ Security Checklist
1. **Signature Verification**: All webhooks MUST be verified using Ed25519 (NOT RSA/ECDSA)
2. **Timestamp Validation**: Events older than 5 minutes are rejected (replay protection)
3. **Idempotency**: Implement store to prevent duplicate processing
4. **HTTPS Only**: Webhook endpoints must use TLS
5. **Raw Payload**: Use raw bytes for signature verification, not parsed JSON

### Signature Verification Flow
```go
event, err := webhook.ConstructEvent(
    payload,                                    // Raw request body bytes
    r.Header.Get(webhook.SignatureHeader),     // Ed25519 signature
    r.Header.Get(webhook.TimestampHeader),     // Unix timestamp
    publicKeyHex,                              // Your webhook public key
)
```

## Event Types Reference

### Core Event Categories
- **Customer Events**: `customer.created`, `customer.updated`, `customer.kyb_status.updated`
- **Transaction Events**: `transaction.auto.created`, `transaction.auto.updated`, `transaction.one_off.created`
- **Account Events**: `auto_account.created`, `auto_account.updated`, `auto_account.deleted`
- **Recipient Events**: `recipient.created`, `recipient.updated`, `recipient.deleted`
- **Exception Events**: `exception.created`, `exception.cleared`

### Event Structure
```go
type Event struct {
    ID        string      // Unique identifier (KSUID)
    Type      string      // Event type (e.g., "customer.created")
    Data      interface{} // Type-specific payload
    Timestamp int64       // Unix timestamp
}
```

## Common Implementation Patterns

### 1. Type-Safe Event Parsing
```go
func handleCustomerCreated(ctx context.Context, event webhook.Event) error {
    // Parse event data into specific type
    customer, err := webhook.EventDataAs[types.CustomerData](event)
    if err != nil {
        return fmt.Errorf("invalid customer data: %w", err)
    }

    // Process customer
    fmt.Printf("New customer: %s (%s)\n", customer.Name, customer.Email)
    return nil
}
```

### 2. Transaction Status Handling
```go
func handleTransactionUpdate(ctx context.Context, event webhook.Event) error {
    tx, err := webhook.EventDataAs[types.AutoTransactionData](event)
    if err != nil {
        return err
    }

    switch tx.Status {
    case "completed":
        // Transaction successful
        return processCompletedTransaction(tx)
    case "failed":
        // Transaction failed
        return handleFailedTransaction(tx, tx.FailureReason)
    case "processing":
        // Still in progress
        return updateTransactionStatus(tx)
    default:
        return fmt.Errorf("unknown status: %s", tx.Status)
    }
}
```

### 3. Custom Idempotency Store
```go
// Implement this interface for custom storage (PostgreSQL, Redis, etc.)
type Store interface {
    Acquire(ctx context.Context, eventID string) error
    Commit(ctx context.Context, eventID string) error
    Release(ctx context.Context, eventID string) error
}

// Example: Use in-memory store for development
store := idempotency.NewMemoryStore()

// Production: Implement database-backed store
store := NewPostgreSQLStore(db)
```

### 4. Acknowledgment Policies
```go
// AckOnSuccess (recommended): Only return 200 if processing succeeds
webhook.WithAckPolicy(webhook.AckOnSuccess)

// AckAlways: Always return 200 (handle retries yourself)
webhook.WithAckPolicy(webhook.AckAlways)
```

## Testing Webhook Handlers

### Local Testing with Listener
```go
func TestWebhookIntegration(t *testing.T) {
    listener, err := webhook.NewListener(
        webhook.WithAddr("127.0.0.1:0"),
        webhook.WithHandlerOptions(
            webhook.WithPublicKey(testPublicKey),
            webhook.WithChannel(100),
        ),
    )
    require.NoError(t, err)

    ctx := context.Background()
    go listener.Start(ctx)

    // Send test events to listener.Addr()
    // Process events from listener.Events() channel
}
```

### Unit Testing Handlers
```go
func TestCustomerHandler(t *testing.T) {
    event := webhook.Event{
        ID:   "test_123",
        Type: "customer.created",
        Data: types.CustomerData{
            ID:    "cust_123",
            Name:  "Test Customer",
            Email: "test@example.com",
        },
    }

    err := handleCustomerCreated(context.Background(), event)
    assert.NoError(t, err)
}
```

## Error Handling

### SDK Error Types
```go
import sdkerrors "github.com/dakota-xyz/go-sdk/errors"

// Check specific errors
if sdkerrors.Is(err, sdkerrors.ErrInvalidSignature) {
    // Signature verification failed - potential security issue
    return http.StatusUnauthorized
}

if sdkerrors.Is(err, sdkerrors.ErrExpiredTimestamp) {
    // Event too old - likely a replay attempt
    return http.StatusBadRequest
}

if sdkerrors.Is(err, sdkerrors.ErrDuplicateEvent) {
    // Already processed - return success (idempotent)
    return http.StatusOK
}
```

## Common Issues & Solutions

### Issue: Signature Verification Failing
**Solution**:
- Verify you're using the correct public key
- Ensure you're passing raw body bytes (not parsed JSON)
- Check system time synchronization
- Confirm using Ed25519 (not RSA/ECDSA)

### Issue: Duplicate Event Processing
**Solution**:
- Implement idempotency store
- Check for race conditions in handler
- Ensure atomic acquire/commit/release operations

### Issue: Handler Timeouts
**Solution**:
- Process events asynchronously
- Respond quickly, process in background
- Use goroutines for heavy operations
- Implement proper context cancellation

### Issue: Missing Events
**Solution**:
- Verify webhook endpoint is publicly accessible
- Check firewall/network settings
- Ensure handler returns 2xx status codes
- Review webhook retry logs

## Best Practices

1. **Always verify signatures** - Never skip this in production
2. **Implement idempotency** - Webhooks may be retried
3. **Handle async** - Don't block the webhook response
4. **Use structured logging** - Track event IDs for debugging
5. **Set timeouts** - Use context with timeout for operations
6. **Monitor performance** - Track processing time and success rates
7. **Graceful shutdown** - Properly drain in-flight requests
8. **Test thoroughly** - Include edge cases and failure scenarios

## Quick Reference

### Required Headers from Dakota Platform
- `X-Dakota-Signature`: Ed25519 signature (hex encoded)
- `X-Dakota-Timestamp`: Unix timestamp (seconds)

### Response Codes
- `200 OK`: Event processed successfully
- `400 Bad Request`: Invalid payload or expired timestamp
- `401 Unauthorized`: Signature verification failed
- `500 Internal Server Error`: Processing error (will retry)

### Environment Variables
```bash
DAKOTA_WEBHOOK_PUBLIC_KEY=your_public_key_hex
DAKOTA_API_KEY=sk_live_xxx                    # If using API calls
DAKOTA_API_URL=https://api.platform.dakota.xyz
```

## When Helping Developers

Focus on:
1. **Webhook implementation** - This is the core use case
2. **Security best practices** - Signatures and idempotency are critical
3. **Type safety** - Use generics for event parsing
4. **Error handling** - Financial systems need robustness
5. **Testing strategies** - Help write comprehensive tests

Avoid:
- Getting into platform internals (not relevant for SDK usage)
- Assuming specific payment providers or rails
- Making changes that break backward compatibility
- Skipping security measures for convenience

Remember: This SDK processes financial events. Security, reliability, and correctness are non-negotiable.