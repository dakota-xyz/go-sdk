# Dakota Go SDK - Enhanced Platform Context for AI Agents

This document provides deep technical context about the Dakota Platform that the SDK integrates with. This information helps AI agents understand the full scope of operations when assisting with SDK development.

## Platform Architecture Overview

Dakota Platform is a backend-for-backends financial services system built in Go, designed with enterprise-grade stability and API-first principles. The platform follows these architectural patterns:

### Core Design Principles
1. **Stability First** (DK:P Priority #1): Financial services require maximum availability and API consistency
2. **API Externalizability** (DK:EXT): All internal interfaces designed as if they'll be public
3. **Minimum Viable Complexity** (DK:MVP): Generic, composable components for adaptability
4. **Comprehensibility** (DK:C): Code should be understandable by less experienced developers

### Technology Stack
- **Language**: Go 1.22+
- **Database**: PostgreSQL (with migrations)
- **Cache/Idempotency**: Redis (24-hour idempotency keys)
- **API**: RESTful HTTP with OpenAPI 3.0 specification
- **Authentication**: API keys with X-Api-Key header
- **Webhook Security**: Ed25519 signatures (not ECDSA/RSA)

## Platform Services Deep Dive

### 1. Orchestrator Service
The central coordination service that manages complex multi-step financial operations:

```
internal/platform/orchestrator.go
internal/platform/orchestrator_*.go
```

**Key Responsibilities:**
- Transaction routing decisions based on cost/speed optimization
- Webhook ingestion from payment providers (Bridge, Circle, Lead, BVNK, Privy)
- State machine management for composite transactions
- Fund movement notifications to downstream systems

**Example Flow: Fiat to Stablecoin**
1. Accept fiat deposit via Lead Bank
2. Auto-convert dollars to DKUSD via Bridge
3. Move DKUSD to destination crypto account
4. Emit webhooks at each state transition

### 2. KYB/KYC Service
Unified interface abstracting multiple verification providers:

```
internal/platform/kyb/
internal/platform/compliance/
```

**Providers Abstracted:**
- Unit21 (compliance/AML)
- BVNK (onboarding)
- Custom verification flows

**Key Features:**
- Document collection and verification
- Risk scoring and compliance checks
- Verification link generation with expiry
- Status tracking (pending → approved/rejected)

### 3. Payment Rails Integration

The platform integrates with multiple payment providers, each with specific capabilities:

#### Bridge (Circle's Stablecoin Platform)
- **Purpose**: USDC/DKUSD minting and burning
- **Operations**: Stablecoin issuance, redemption, transfers
- **Networks**: Ethereum, Polygon, Solana, etc.

#### Lead Bank
- **Purpose**: Traditional banking rails
- **Operations**: ACH transfers, wire transfers, fiat accounts
- **Use Case**: Fiat on/off ramps

#### BVNK
- **Purpose**: European payment processing
- **Operations**: SEPA transfers, UK Faster Payments
- **Currencies**: EUR, GBP support

#### Privy
- **Purpose**: Embedded wallet infrastructure
- **Operations**: Wallet creation, key management
- **Use Case**: Simplified crypto custody

### 4. Auto Accounts System

Auto accounts are Dakota's intelligent fund routing system:

```go
type AutoAccount struct {
    ID                string
    CustomerID        string
    Enabled           bool
    AccountType       string // "ach", "wire", "crypto"
    BankAccount       *BankAccount
    Crypto            *CryptoRouteInfo
    DestinationID     string
    OutputAsset       Asset
    RoutingPreference string // "cost_optimized", "speed_optimized"
}
```

**Intelligent Routing Logic:**
- Monitor incoming transactions automatically
- Route based on preferences (cost vs speed)
- Support multiple output assets and destinations
- Handle complex routing rules and exceptions

### 5. Transaction State Machine

Transactions follow a strict state progression:

```
pending → processing → completed
         ↘          ↙
            failed
```

**Transaction Types:**
- `one_off`: Manual, user-initiated transactions
- `auto`: Triggered by auto account rules
- `internal`: Platform-managed transfers

**Failure Handling:**
- Detailed `failure_reason` for debugging
- Automatic retry logic for transient failures
- Exception events for manual intervention

## Webhook Event Flow Architecture

### Event Generation Pipeline
1. **Provider Webhook Receipt** → Platform receives webhook from payment provider
2. **Processing & Validation** → Platform validates, processes, updates state
3. **Event Generation** → Platform generates normalized Dakota events
4. **Event Dispatch** → Events sent to customer webhook endpoints
5. **SDK Processing** → Customer's SDK handler processes events

### Event Normalization
Platform normalizes provider-specific data into consistent Dakota events:

```
Provider Event → Platform Processing → Dakota Event
Bridge: payment.completed → transaction.auto.updated (status: completed)
Lead: ach.received → transaction.auto.created
BVNK: payment.settled → transaction.one_off.updated
```

### Critical Event Sequences

#### Customer Onboarding Flow
1. `customer.created` - Initial customer record
2. `customer.kyb_link.created` - Verification link generated
3. `customer.kyb_status.created` - KYB process initiated
4. `customer.kyb_status.updated` - Status changes (pending → approved/rejected)
5. `customer.updated` - Customer status activated

#### Auto Account Transaction Flow
1. `auto_account.created` - Account configured
2. `transaction.auto.created` - Incoming funds detected
3. `transaction.auto.updated` (processing) - Routing initiated
4. `transaction.auto.updated` (completed/failed) - Final state

#### Exception Handling Flow
1. `exception.created` - Anomaly detected
2. Manual intervention or automatic resolution
3. `exception.cleared` - Issue resolved

## Platform API Patterns

### Request Requirements
```http
POST /v1/transactions
X-Api-Key: sk_live_xxx
X-Idempotency-Key: 123e4567-e89b-12d3-a456-426614174000
Content-Type: application/json
```

### Pagination Pattern
```http
GET /v1/transactions?limit=10&starting_after=txn_xxx
```

Response includes:
- `data`: Array of results
- `has_more`: Boolean for additional pages
- `next_cursor`: Token for next page

### Error Response Format
```json
{
  "error": {
    "code": "invalid_request",
    "message": "The request was invalid",
    "details": {
      "field": "amount",
      "reason": "must be positive"
    }
  }
}
```

## Security Considerations

### Webhook Security Chain
1. **Ed25519 Signatures**: Cryptographic proof of authenticity
2. **Timestamp Validation**: 5-minute replay protection window
3. **Idempotency**: Exactly-once processing guarantee
4. **TLS**: Encrypted transport required
5. **IP Allowlisting**: Optional additional security

### API Security
1. **API Key Rotation**: Regular key rotation supported
2. **Rate Limiting**: Protection against abuse
3. **Audit Logging**: Complete API activity tracking
4. **Permission Scoping**: Granular access controls

## Platform-Specific Business Logic

### CRI (Customer Reference ID)
- External identifier linking platform entities to customer systems
- Used for reconciliation and reporting
- Searchable across all platform entities
- Format: Customer-defined string (max 255 chars)

### Asset Management
```go
type Asset struct {
    Symbol   string // "DKUSD", "USDC", "USD"
    Network  string // "ethereum", "polygon", "ach"
    Decimals int    // 6 for USDC, 2 for USD
}
```

### Amount Handling
- All amounts in smallest unit (cents for USD, wei for ETH)
- Consistent decimal handling across providers
- Automatic conversion for display purposes

### Provider-Specific Quirks
- **Bridge**: Requires pre-funding for minting operations
- **Lead**: ACH has T+2 settlement, wires are same-day
- **BVNK**: Requires separate EUR/GBP accounts
- **Privy**: Wallet addresses are deterministic

## Integration Testing Patterns

### Webhook Testing Best Practices
1. **Use Real Provider Payloads**: Mock data often misses edge cases
2. **Test Idempotency**: Send same event multiple times
3. **Simulate Failures**: Test timeout, signature failures
4. **Out-of-Order Events**: Events may arrive non-sequentially
5. **Load Testing**: Ensure handling of webhook bursts

### Environment-Specific Configurations
```
Production:  https://api.platform.dakota.xyz
Sandbox:     https://api.platform.sandbox.dakota.xyz
Development: https://api.platform.dev.dakota.xyz
```

## Common Integration Scenarios

### Scenario 1: Merchant Payment Processing
```
1. Customer creates auto_account for payment collection
2. End-user sends payment to auto account
3. Platform detects incoming transaction
4. Auto-routes to merchant's preferred destination
5. Webhooks notify of each state change
```

### Scenario 2: Cross-Border Remittance
```
1. Sender deposits USD via ACH (Lead)
2. Platform converts to USDC (Bridge)
3. USDC sent to recipient's wallet
4. Recipient off-ramps to local currency (BVNK)
```

### Scenario 3: Treasury Management
```
1. Configure multiple auto accounts for different purposes
2. Set routing rules based on amount thresholds
3. Automatic sweep to high-yield accounts
4. Exception handling for large transactions
```

## Performance Characteristics

### Latency Expectations
- Webhook delivery: < 100ms after event
- API response time: p95 < 500ms
- Transaction processing: Varies by provider
  - ACH: T+2 business days
  - Wire: Same business day
  - Crypto: 1-30 minutes depending on network

### Scale Capabilities
- Webhook processing: 10,000 events/second
- API requests: 1,000 requests/second per customer
- Idempotency store: 24-hour retention
- Event replay: Last 30 days available

## Debugging and Troubleshooting

### Common Issues and Solutions

1. **Webhook Signature Failures**
   - Verify public key is correctly configured
   - Check system time synchronization
   - Ensure raw payload bytes are used for verification

2. **Duplicate Event Processing**
   - Implement idempotency store
   - Check for race conditions in handler
   - Verify atomic reservation logic

3. **Missing Events**
   - Check webhook endpoint accessibility
   - Verify event subscriptions in platform
   - Review webhook retry logs

4. **Transaction Failures**
   - Inspect `failure_reason` field
   - Check provider-specific requirements
   - Verify sufficient balance/limits

### Monitoring Recommendations
- Track webhook processing latency
- Monitor idempotency cache hit rate
- Alert on signature validation failures
- Track event type distribution
- Monitor handler error rates

## SDK Development Guidelines

When developing SDK features:

1. **Prioritize Webhook Handling**: This is 90% of SDK usage
2. **Maintain Type Safety**: Use generics for event data parsing
3. **Support All Providers**: Don't assume single provider
4. **Handle Edge Cases**: Network issues, timeouts, retries
5. **Provide Clear Examples**: Real-world scenarios
6. **Document Provider Differences**: Each has unique requirements
7. **Test with Production-Like Data**: Sandbox isn't always accurate
8. **Consider Backwards Compatibility**: Platform evolves, maintain compatibility

## Future Platform Enhancements

Expected platform evolutions that may impact SDK:

1. **Additional Payment Providers**: New rails and networks
2. **Enhanced Routing Intelligence**: ML-based optimization
3. **Expanded Asset Support**: More cryptocurrencies and fiat
4. **Advanced Compliance Features**: Enhanced KYC/AML
5. **Real-time Analytics**: Streaming transaction insights
6. **Programmable Routing Rules**: Customer-defined logic

## Questions for AI Agents to Consider

When assisting with SDK development, consider:

1. How does this change affect webhook processing?
2. Does this support all payment provider scenarios?
3. Is idempotency properly handled?
4. Are we maintaining backward compatibility?
5. How will this work with auto accounts?
6. What happens during provider outages?
7. Is the error handling comprehensive?
8. Are we following Go best practices?
9. Is the implementation performant at scale?
10. Does this align with platform architecture?

## Contact for Platform Team

For platform-specific questions beyond SDK scope:
- Primary: Adam Train (Platform Lead)
- Webhook System: Platform Orchestrator Team
- Provider Integration: Platform Providers Team
- Compliance/KYB: Platform Compliance Team

Remember: The SDK is the public interface to a complex financial platform. Every decision impacts real money movement. Prioritize security, reliability, and clarity above all else.