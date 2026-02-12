# Dakota Go SDK - AI Agent Instructions

## Project Overview
You are working on the Dakota Go SDK, a public Go library that enables customers to integrate with the Dakota Platform for stablecoin and payment operations. The SDK's primary focus is secure webhook handling for real-time event notifications.

## Critical Context
- **This SDK is PUBLIC** - Used by external customers
- **The Platform is PRIVATE** - You cannot access platform code directly
- **Financial Systems** - Every webhook could represent real money moving
- **Security is Paramount** - Always verify signatures, implement idempotency

## Key Documentation Files
1. `AI_AGENT_CONTEXT.md` - Comprehensive SDK and platform overview
2. `AI_IMPLEMENTATION_GUIDE.md` - Detailed implementation examples
3. `AI_AGENT_PLATFORM_CONTEXT.md` - Deep platform architecture understanding
4. `README.md` - Basic SDK usage

## When Assisting Developers

### Always Prioritize:
1. **Webhook Integration** - This is 90% of SDK usage
2. **Security** - Signature verification, timestamp validation, idempotency
3. **Type Safety** - Use `EventDataAs[T]` for parsing events
4. **Error Handling** - Financial systems need robust error handling
5. **Backward Compatibility** - Never break existing integrations

### Common Tasks You'll Help With:

#### 1. Implementing Webhook Handlers
```go
handler, err := webhook.NewHandler(
    webhook.WithPublicKey(publicKey),
    webhook.WithIdempotencyStore(store),
    webhook.OnEvent("customer.created", handleCustomerCreated),
)
```

#### 2. Processing Specific Events
```go
func handleTransaction(ctx context.Context, event webhook.Event) error {
    tx, err := webhook.EventDataAs[types.AutoTransactionData](event)
    if err != nil {
        return fmt.Errorf("failed to parse: %w", err)
    }
    // Process transaction
    return nil
}
```

#### 3. Implementing Idempotency Stores
- Help create custom stores (PostgreSQL, MySQL, MongoDB)
- Ensure atomic reservation/commit/release operations
- Handle race conditions properly

#### 4. Debugging Webhook Issues
- Check signature verification first
- Verify timestamp synchronization
- Ensure idempotency is implemented
- Check for event ordering issues

## Package Structure
```
webhook/              # Core webhook handling
├── handler.go       # HTTP handler implementation
├── signature.go     # Ed25519 signature verification
├── listener.go      # Webhook listener/server
├── idempotency/     # Idempotency implementations
└── types/           # Event type definitions

errors/              # SDK-specific error types
log/                 # Logging utilities
```

## Import Conventions
Always use aliases to avoid stdlib conflicts:
```go
import (
    sdkerrors "github.com/dakota-xyz/go-sdk/errors"
    sdklog "github.com/dakota-xyz/go-sdk/log"
)
```

## Event Categories to Understand:
1. **User & Auth**: user.created, api_key.created
2. **Customer/KYB**: customer.created, customer.kyb_status.updated
3. **Accounts**: auto_account.created, recipient.created
4. **Transactions**: transaction.auto.created/updated, transaction.one_off.created/updated
5. **Exceptions**: exception.created, exception.cleared

## Platform Providers Context:
- **Bridge**: USDC/stablecoin operations
- **Lead Bank**: ACH/Wire transfers
- **BVNK**: European payments (SEPA, UK)
- **Privy**: Wallet infrastructure

## Common Integration Patterns:

### Payment Collection
1. Create auto_account for receiving payments
2. Listen for transaction.auto.created events
3. Process transaction.auto.updated (completed) for confirmation
4. Handle transaction.auto.updated (failed) for errors

### KYB Verification
1. Create customer via API
2. Receive customer.kyb_link.created webhook
3. Monitor customer.kyb_status.updated for approval/rejection
4. Enable features based on KYB status

## Testing Guidance:
- Use real webhook payloads when possible
- Test idempotency by sending same event multiple times
- Simulate out-of-order event delivery
- Test signature failures and timestamp expiry
- Use race detection: `go test -race ./...`

## Security Checklist:
- [ ] Ed25519 signature verification implemented
- [ ] Timestamp validation (5-minute window)
- [ ] Idempotency store configured
- [ ] TLS/HTTPS only for webhook endpoints
- [ ] Error messages don't leak sensitive data
- [ ] Proper context cancellation handling

## Performance Considerations:
- Webhook handlers should respond quickly (< 5 seconds)
- Use async processing for heavy operations
- Implement proper connection pooling for idempotency stores
- Handle webhook bursts gracefully
- Monitor processing latency

## Common Mistakes to Avoid:
1. Not implementing idempotency (causes duplicate processing)
2. Using wrong signature algorithm (must be Ed25519, not RSA/ECDSA)
3. Not handling all event types (use OnDefault)
4. Blocking webhook handler with slow operations
5. Not preserving raw bytes for signature verification
6. Assuming event order (they can arrive out of sequence)

## When Users Ask About:

### "How do I handle webhook retries?"
- Implement idempotency store
- Use AckOnSuccess policy
- Return 2xx only on successful processing

### "Why is signature verification failing?"
- Check public key configuration
- Verify system time sync
- Ensure using raw payload bytes

### "How do I test webhooks locally?"
- Use webhook.NewListener for local server
- Generate test events with proper signatures
- Use ngrok or similar for external access

### "Can I filter which events I receive?"
- Platform sends all subscribed events
- Filter in your handler using event.Type
- Use specific OnEvent handlers for each type

## Remember:
- This SDK handles financial data - accuracy and reliability are critical
- Every webhook represents real money movement
- Security cannot be compromised
- Backward compatibility is essential
- Clear examples and documentation help adoption

## For Reference:
- Platform API docs: See openapi.yaml in platform repo
- Go version: 1.22+
- Signature algorithm: Ed25519 (not ECDSA or RSA)
- Idempotency window: 24 hours
- Webhook timeout window: 5 minutes