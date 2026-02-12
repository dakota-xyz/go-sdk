# Dakota Go SDK - Implementation Guide for AI Agents

This guide provides detailed implementation examples and patterns for AI agents assisting with Dakota Go SDK development.

## Complete Webhook Implementation Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"

    "github.com/dakota-xyz/go-sdk/webhook"
    "github.com/dakota-xyz/go-sdk/webhook/idempotency"
    "github.com/dakota-xyz/go-sdk/webhook/types"
    sdkerrors "github.com/dakota-xyz/go-sdk/errors"
    sdklog "github.com/dakota-xyz/go-sdk/log"
)

func main() {
    // Get public key from environment
    publicKey := os.Getenv("DAKOTA_WEBHOOK_PUBLIC_KEY")
    if publicKey == "" {
        log.Fatal("DAKOTA_WEBHOOK_PUBLIC_KEY environment variable is required")
    }

    // Create idempotency store
    store := idempotency.NewMemoryStore()

    // Create webhook handler with all event handlers
    handler, err := webhook.NewHandler(
        webhook.WithPublicKey(publicKey),
        webhook.WithIdempotencyStore(store),
        webhook.WithAckPolicy(webhook.AckOnSuccess),

        // Customer events
        webhook.OnEvent("customer.created", handleCustomerCreated),
        webhook.OnEvent("customer.updated", handleCustomerUpdated),
        webhook.OnEvent("customer.kyb_status.updated", handleKYBStatusUpdate),

        // Transaction events
        webhook.OnEvent("transaction.auto.created", handleAutoTransaction),
        webhook.OnEvent("transaction.auto.updated", handleTransactionUpdate),

        // Auto account events
        webhook.OnEvent("auto_account.created", handleAutoAccountCreated),

        // Catch-all for unhandled events
        webhook.OnDefault(handleDefaultEvent),
    )
    if err != nil {
        log.Fatalf("Failed to create webhook handler: %v", err)
    }

    // Start HTTP server
    http.Handle("/webhook", handler)
    log.Println("Webhook handler listening on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}

func handleCustomerCreated(ctx context.Context, event webhook.Event) error {
    customer, err := webhook.EventDataAs[types.CustomerData](event)
    if err != nil {
        return fmt.Errorf("failed to parse customer data: %w", err)
    }

    log.Printf("New customer created: ID=%s, Name=%s, Email=%s",
        customer.ID, customer.Name, customer.Email)

    // Implement your business logic here
    // e.g., create internal user account, send welcome email, etc.

    return nil
}

func handleCustomerUpdated(ctx context.Context, event webhook.Event) error {
    customer, err := webhook.EventDataAs[types.CustomerData](event)
    if err != nil {
        return fmt.Errorf("failed to parse customer data: %w", err)
    }

    log.Printf("Customer updated: ID=%s, Status=%s", customer.ID, customer.Status)

    // Update internal records

    return nil
}

func handleKYBStatusUpdate(ctx context.Context, event webhook.Event) error {
    kybStatus, err := webhook.EventDataAs[types.KYBStatusData](event)
    if err != nil {
        return fmt.Errorf("failed to parse KYB status: %w", err)
    }

    switch kybStatus.Status {
    case "approved":
        log.Printf("KYB approved for customer %s", kybStatus.CustomerID)
        // Enable customer features
    case "rejected":
        log.Printf("KYB rejected for customer %s: %v",
            kybStatus.CustomerID, kybStatus.Reason)
        // Notify customer, restrict features
    case "pending":
        log.Printf("KYB pending for customer %s", kybStatus.CustomerID)
        // Wait for further updates
    }

    return nil
}

func handleAutoTransaction(ctx context.Context, event webhook.Event) error {
    tx, err := webhook.EventDataAs[types.AutoTransactionData](event)
    if err != nil {
        return fmt.Errorf("failed to parse transaction: %w", err)
    }

    log.Printf("Auto transaction created: ID=%s, Status=%s, AutoAccountID=%s",
        tx.ID, tx.Status, tx.AutoAccountID)

    // Track transaction in your system

    return nil
}

func handleTransactionUpdate(ctx context.Context, event webhook.Event) error {
    tx, err := webhook.EventDataAs[types.AutoTransactionData](event)
    if err != nil {
        return fmt.Errorf("failed to parse transaction: %w", err)
    }

    switch tx.Status {
    case "completed":
        log.Printf("Transaction completed: ID=%s", tx.ID)
        if tx.Receipt != nil {
            log.Printf("Amount: %s %s", tx.Receipt.Amount, tx.Receipt.Currency)
        }
        // Credit user account, update balances

    case "failed":
        log.Printf("Transaction failed: ID=%s, Reason=%v", tx.ID, tx.FailureReason)
        // Handle failed transaction, notify user

    case "processing":
        log.Printf("Transaction processing: ID=%s", tx.ID)
        // Update UI status
    }

    return nil
}

func handleAutoAccountCreated(ctx context.Context, event webhook.Event) error {
    account, err := webhook.EventDataAs[types.AutoAccountData](event)
    if err != nil {
        return fmt.Errorf("failed to parse auto account: %w", err)
    }

    log.Printf("Auto account created: ID=%s, CustomerID=%s, Type=%s",
        account.ID, account.CustomerID, account.AccountType)

    return nil
}

func handleDefaultEvent(ctx context.Context, event webhook.Event) error {
    log.Printf("Received unhandled event: Type=%s, ID=%s", event.Type, event.ID)
    // Log for monitoring, but don't fail
    return nil
}
```

## Custom Idempotency Store Implementation

```go
package customstore

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    "github.com/dakota-xyz/go-sdk/webhook/idempotency"
)

// PostgreSQLStore implements idempotency.Store using PostgreSQL
type PostgreSQLStore struct {
    db *sql.DB
}

func NewPostgreSQLStore(db *sql.DB) (idempotency.Store, error) {
    // Create table if not exists
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS webhook_events (
            event_id VARCHAR(255) PRIMARY KEY,
            status VARCHAR(50) NOT NULL,
            created_at TIMESTAMP NOT NULL,
            updated_at TIMESTAMP NOT NULL
        )
    `)
    if err != nil {
        return nil, fmt.Errorf("failed to create table: %w", err)
    }

    return &PostgreSQLStore{db: db}, nil
}

func (s *PostgreSQLStore) Acquire(ctx context.Context, eventID string) error {
    // Try to insert with 'processing' status
    // This will fail if event already exists (idempotency)
    _, err := s.db.ExecContext(ctx, `
        INSERT INTO webhook_events (event_id, status, created_at, updated_at)
        VALUES ($1, 'processing', NOW(), NOW())
    `, eventID)

    if err != nil {
        // Check if it's a duplicate key error
        if isUniqueViolation(err) {
            return idempotency.ErrDuplicateEvent
        }
        return fmt.Errorf("failed to acquire event: %w", err)
    }

    return nil
}

func (s *PostgreSQLStore) Commit(ctx context.Context, eventID string) error {
    // Mark as successfully processed
    _, err := s.db.ExecContext(ctx, `
        UPDATE webhook_events
        SET status = 'completed', updated_at = NOW()
        WHERE event_id = $1 AND status = 'processing'
    `, eventID)

    if err != nil {
        return fmt.Errorf("failed to commit event: %w", err)
    }

    return nil
}

func (s *PostgreSQLStore) Release(ctx context.Context, eventID string) error {
    // Remove the processing lock on failure
    _, err := s.db.ExecContext(ctx, `
        DELETE FROM webhook_events
        WHERE event_id = $1 AND status = 'processing'
    `, eventID)

    if err != nil {
        return fmt.Errorf("failed to release event: %w", err)
    }

    return nil
}

func isUniqueViolation(err error) bool {
    // PostgreSQL specific error checking
    // Implement based on your database driver
    return false
}
```

## Testing Webhook Handlers

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/dakota-xyz/go-sdk/webhook"
    "github.com/dakota-xyz/go-sdk/webhook/types"
)

func TestWebhookHandler(t *testing.T) {
    // Generate test keys (in production, use real keys)
    publicKey := "your-test-public-key-hex"
    privateKey := "your-test-private-key-hex"

    // Create test handler
    var receivedEvent webhook.Event
    handler, err := webhook.NewHandler(
        webhook.WithPublicKey(publicKey),
        webhook.OnDefault(func(ctx context.Context, event webhook.Event) error {
            receivedEvent = event
            return nil
        }),
    )
    if err != nil {
        t.Fatalf("Failed to create handler: %v", err)
    }

    // Create test event
    event := webhook.Event{
        ID:        "test_event_123",
        Type:      "customer.created",
        Data: types.CustomerData{
            ID:    "cust_123",
            Name:  "Test Customer",
            Email: "test@example.com",
        },
        Timestamp: time.Now().Unix(),
    }

    // Marshal event
    payload, err := json.Marshal(event)
    if err != nil {
        t.Fatalf("Failed to marshal event: %v", err)
    }

    // Create request with signature
    req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))

    // In production, Dakota Platform adds these headers
    signature := generateTestSignature(payload, privateKey)
    req.Header.Set(webhook.SignatureHeader, signature)
    req.Header.Set(webhook.TimestampHeader, fmt.Sprintf("%d", time.Now().Unix()))

    // Test the handler
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    // Check response
    if rr.Code != http.StatusOK {
        t.Errorf("Handler returned wrong status code: got %v want %v",
            rr.Code, http.StatusOK)
    }

    // Verify event was processed
    if receivedEvent.ID != event.ID {
        t.Errorf("Event not processed correctly")
    }
}

func TestIdempotency(t *testing.T) {
    publicKey := "your-test-public-key-hex"

    processCount := 0
    handler, err := webhook.NewHandler(
        webhook.WithPublicKey(publicKey),
        webhook.WithIdempotencyStore(idempotency.NewMemoryStore()),
        webhook.OnDefault(func(ctx context.Context, event webhook.Event) error {
            processCount++
            return nil
        }),
    )
    if err != nil {
        t.Fatalf("Failed to create handler: %v", err)
    }

    // Send same event twice
    event := createTestEvent()

    // First request
    sendWebhook(t, handler, event)

    // Second request with same event ID
    sendWebhook(t, handler, event)

    // Should only process once
    if processCount != 1 {
        t.Errorf("Event processed %d times, expected 1", processCount)
    }
}
```

## Error Handling Patterns

```go
package main

import (
    "context"
    "errors"
    "log"
    "time"

    sdkerrors "github.com/dakota-xyz/go-sdk/errors"
    "github.com/dakota-xyz/go-sdk/webhook"
)

// RetryableError indicates an error that should trigger a retry
type RetryableError struct {
    Err error
}

func (e RetryableError) Error() string {
    return e.Err.Error()
}

// WebhookProcessor with retry logic
type WebhookProcessor struct {
    maxRetries int
    retryDelay time.Duration
}

func (p *WebhookProcessor) ProcessWithRetry(ctx context.Context, event webhook.Event) error {
    var lastErr error

    for attempt := 0; attempt <= p.maxRetries; attempt++ {
        if attempt > 0 {
            log.Printf("Retrying webhook processing, attempt %d/%d", attempt, p.maxRetries)
            time.Sleep(p.retryDelay * time.Duration(attempt))
        }

        err := p.processEvent(ctx, event)
        if err == nil {
            return nil
        }

        lastErr = err

        // Check if error is retryable
        var retryableErr RetryableError
        if !errors.As(err, &retryableErr) {
            // Non-retryable error, fail immediately
            return err
        }
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (p *WebhookProcessor) processEvent(ctx context.Context, event webhook.Event) error {
    switch event.Type {
    case "transaction.auto.created":
        return p.handleTransaction(ctx, event)
    default:
        // Unknown event type is not retryable
        return fmt.Errorf("unknown event type: %s", event.Type)
    }
}

func (p *WebhookProcessor) handleTransaction(ctx context.Context, event webhook.Event) error {
    tx, err := webhook.EventDataAs[types.AutoTransactionData](event)
    if err != nil {
        // Parsing error is not retryable
        return fmt.Errorf("failed to parse transaction: %w", err)
    }

    // Simulate database operation
    err = p.saveTransactionToDB(ctx, tx)
    if err != nil {
        // Database errors might be retryable
        if isTemporaryError(err) {
            return RetryableError{Err: err}
        }
        return err
    }

    return nil
}

func isTemporaryError(err error) bool {
    // Check if error is temporary (network, timeout, etc.)
    // Implementation depends on your database driver
    return false
}
```

## Monitoring and Observability

```go
package monitoring

import (
    "context"
    "time"

    "github.com/dakota-xyz/go-sdk/webhook"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    webhooksReceived = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "dakota_webhooks_received_total",
            Help: "Total number of webhooks received",
        },
        []string{"event_type"},
    )

    webhooksProcessed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "dakota_webhooks_processed_total",
            Help: "Total number of webhooks successfully processed",
        },
        []string{"event_type"},
    )

    webhooksFailed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "dakota_webhooks_failed_total",
            Help: "Total number of webhooks that failed processing",
        },
        []string{"event_type", "error_type"},
    )

    webhookProcessingDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "dakota_webhook_processing_duration_seconds",
            Help:    "Duration of webhook processing",
            Buckets: prometheus.DefBuckets,
        },
        []string{"event_type"},
    )
)

// InstrumentedHandler wraps webhook handlers with metrics
func InstrumentedHandler(eventType string, handler webhook.EventHandler) webhook.EventHandler {
    return func(ctx context.Context, event webhook.Event) error {
        webhooksReceived.WithLabelValues(eventType).Inc()

        start := time.Now()
        err := handler(ctx, event)
        duration := time.Since(start).Seconds()

        webhookProcessingDuration.WithLabelValues(eventType).Observe(duration)

        if err != nil {
            errorType := "unknown"
            if sdkerrors.Is(err, sdkerrors.ErrInvalidSignature) {
                errorType = "invalid_signature"
            } else if sdkerrors.Is(err, sdkerrors.ErrExpiredTimestamp) {
                errorType = "expired_timestamp"
            }

            webhooksFailed.WithLabelValues(eventType, errorType).Inc()
            return err
        }

        webhooksProcessed.WithLabelValues(eventType).Inc()
        return nil
    }
}

// Usage
func SetupHandlers() (*webhook.Handler, error) {
    return webhook.NewHandler(
        webhook.WithPublicKey(publicKey),
        webhook.OnEvent("customer.created",
            InstrumentedHandler("customer.created", handleCustomerCreated)),
        webhook.OnEvent("transaction.auto.created",
            InstrumentedHandler("transaction.auto.created", handleTransaction)),
    )
}
```

## Advanced Webhook Listener Configuration

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/dakota-xyz/go-sdk/webhook"
)

func RunWebhookListener(ctx context.Context) error {
    publicKey := os.Getenv("DAKOTA_WEBHOOK_PUBLIC_KEY")

    // Create listener with advanced configuration
    listener, err := webhook.NewListener(
        webhook.WithAddr(":8080"),
        webhook.WithHandlerOptions(
            webhook.WithPublicKey(publicKey),
            webhook.WithChannel(100), // Buffer size for async processing
            webhook.WithIdempotencyStore(createIdempotencyStore()),
            webhook.WithAckPolicy(webhook.AckOnSuccess),
        ),
    )
    if err != nil {
        return fmt.Errorf("failed to create listener: %w", err)
    }

    // Start listener in background
    errChan := make(chan error, 1)
    go func() {
        if err := listener.Start(ctx); err != nil {
            errChan <- err
        }
    }()

    log.Printf("Webhook listener started on %s", listener.Addr())

    // Process events from channel
    eventProcessor := NewEventProcessor()
    go eventProcessor.ProcessEvents(ctx, listener.Events())

    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    select {
    case <-sigChan:
        log.Println("Shutdown signal received")
    case err := <-errChan:
        return fmt.Errorf("listener error: %w", err)
    case <-ctx.Done():
        log.Println("Context cancelled")
    }

    // Graceful shutdown
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    return listener.Shutdown(shutdownCtx)
}

type EventProcessor struct {
    workers int
}

func NewEventProcessor() *EventProcessor {
    return &EventProcessor{
        workers: 10, // Number of concurrent workers
    }
}

func (p *EventProcessor) ProcessEvents(ctx context.Context, events <-chan webhook.Event) {
    for i := 0; i < p.workers; i++ {
        go p.worker(ctx, events)
    }
}

func (p *EventProcessor) worker(ctx context.Context, events <-chan webhook.Event) {
    for {
        select {
        case event, ok := <-events:
            if !ok {
                return // Channel closed
            }

            if err := p.processEvent(ctx, event); err != nil {
                log.Printf("Failed to process event %s: %v", event.ID, err)
            }

        case <-ctx.Done():
            return
        }
    }
}

func (p *EventProcessor) processEvent(ctx context.Context, event webhook.Event) error {
    log.Printf("Processing event: Type=%s, ID=%s", event.Type, event.ID)

    // Implement your business logic here

    return nil
}
```

## Common Troubleshooting Patterns

```go
package troubleshooting

// DiagnosticHandler helps debug webhook issues
func DiagnosticHandler(ctx context.Context, event webhook.Event) error {
    log.Printf("=== WEBHOOK DIAGNOSTIC ===")
    log.Printf("Event ID: %s", event.ID)
    log.Printf("Event Type: %s", event.Type)
    log.Printf("Timestamp: %d", event.Timestamp)

    // Log raw data for inspection
    data, _ := json.MarshalIndent(event.Data, "", "  ")
    log.Printf("Event Data:\n%s", string(data))

    // Check timestamp validity
    eventTime := time.Unix(event.Timestamp, 0)
    age := time.Since(eventTime)
    if age > 5*time.Minute {
        log.Printf("WARNING: Event is %v old (may be rejected)", age)
    }

    log.Printf("=== END DIAGNOSTIC ===")
    return nil
}

// VerifyConfiguration checks SDK setup
func VerifyConfiguration() error {
    // Check environment variables
    requiredEnvVars := []string{
        "DAKOTA_WEBHOOK_PUBLIC_KEY",
        "DAKOTA_API_KEY",
        "DAKOTA_API_URL",
    }

    for _, envVar := range requiredEnvVars {
        if os.Getenv(envVar) == "" {
            return fmt.Errorf("missing required environment variable: %s", envVar)
        }
    }

    // Test webhook handler creation
    publicKey := os.Getenv("DAKOTA_WEBHOOK_PUBLIC_KEY")
    _, err := webhook.NewHandler(
        webhook.WithPublicKey(publicKey),
    )
    if err != nil {
        return fmt.Errorf("failed to create webhook handler: %w", err)
    }

    log.Println("Configuration verified successfully")
    return nil
}
```

## Notes for AI Agents

When implementing webhook handlers:

1. **Always start with signature verification** - Security is paramount
2. **Implement idempotency from day one** - Critical for production
3. **Use structured logging** - Essential for debugging
4. **Handle all event types gracefully** - Use OnDefault for unknown events
5. **Test with real webhook payloads** - Mock data often misses edge cases
6. **Monitor processing times** - Webhook timeouts can cause retries
7. **Implement graceful shutdown** - Don't lose events during deploys
8. **Use appropriate error types** - Distinguish between retryable and fatal errors
9. **Document event flows** - Complex async flows need clear documentation
10. **Consider event ordering** - Events may arrive out of order

Remember: This SDK handles financial data. Every webhook could represent real money moving. Accuracy, reliability, and security are non-negotiable.