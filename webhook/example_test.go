package webhook_test

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dakota-xyz/go-sdk/idempotency"
	"github.com/dakota-xyz/go-sdk/webhook"
	"github.com/dakota-xyz/go-sdk/webhook/types"
)

func ExampleVerifySignature() {
	// In production, use the Dakota Platform public key for your environment.
	publicKeyHex := os.Getenv("DAKOTA_WEBHOOK_PUBLIC_KEY")

	payload := []byte(`{"id":"evt_1","type":"customer.created","data":{}}`)
	signatureB64 := "..." // from X-Webhook-Signature header
	timestampStr := "..." // from X-Webhook-Timestamp header

	if err := webhook.VerifySignature(payload, signatureB64, timestampStr, publicKeyHex); err != nil {
		log.Printf("signature verification failed: %v", err)
		return
	}
	fmt.Println("signature valid")
}

func ExampleConstructEvent() {
	// Generate a test key pair for this example.
	pub, priv, _ := ed25519.GenerateKey(nil)
	publicKeyHex := hex.EncodeToString(pub)

	// Simulate a webhook payload.
	payload := []byte(`{"id":"evt_123","type":"customer.created","data":{"name":"Acme"},"timestamp":1705315500}`)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := webhook.ComputeSignature(timestamp, payload, priv)

	// All-in-one: verify + validate timestamp + parse.
	event, err := webhook.ConstructEvent(payload, signature, timestamp, publicKeyHex)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Event: %s (type: %s)\n", event.ID, event.Type)
	// Output: Event: evt_123 (type: customer.created)
}

func ExampleNewHandler_callbacks() {
	publicKeyHex := os.Getenv("DAKOTA_WEBHOOK_PUBLIC_KEY")

	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(publicKeyHex),
		webhook.On(webhook.EventCustomerCreated, func(ctx context.Context, event webhook.Event) error {
			// Use EventDataAs for typed access to the event payload.
			customer, err := webhook.EventDataAs[types.CustomerData](event)
			if err != nil {
				return fmt.Errorf("parse customer data: %w", err)
			}
			fmt.Printf("New customer: %s (%s)\n", customer.Name, customer.ID)
			return nil
		}),
		webhook.On(webhook.EventTransactionAutoUpdated, func(ctx context.Context, event webhook.Event) error {
			txn, err := webhook.EventDataAs[types.AutoTransactionData](event)
			if err != nil {
				return fmt.Errorf("parse transaction data: %w", err)
			}
			fmt.Printf("Transaction %s status: %s\n", txn.ID, txn.Status)
			return nil
		}),
		webhook.OnDefault(func(ctx context.Context, event webhook.Event) error {
			fmt.Printf("Other event: %s (%s)\n", event.ID, event.Type)
			return nil
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/webhook", handler)
	// http.ListenAndServe(":8080", nil)
	_ = handler // silence unused warning in example
}

func ExampleNewHandler_channel() {
	publicKeyHex := os.Getenv("DAKOTA_WEBHOOK_PUBLIC_KEY")

	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(publicKeyHex),
		webhook.WithChannel(100),
		webhook.WithIdempotencyStore(idempotency.NewMemoryStore()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer handler.Close()

	// Process events from the channel.
	go func() {
		for event := range handler.Events() {
			fmt.Printf("Received: %s (%s)\n", event.ID, event.Type)
		}
	}()

	http.Handle("/webhook", handler)
	// http.ListenAndServe(":8080", nil)
	_ = handler // silence unused warning in example
}

func ExampleNewListener() {
	publicKeyHex := os.Getenv("DAKOTA_WEBHOOK_PUBLIC_KEY")

	listener, err := webhook.NewListener(
		webhook.WithAddr(":8080"),
		webhook.WithPath("/webhook"),
		webhook.WithHandlerOptions(
			webhook.WithPublicKey(publicKeyHex),
			webhook.WithChannel(100),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Process events in a goroutine.
	go func() {
		for event := range listener.Events() {
			fmt.Printf("Event: %s\n", event.ID)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Blocks until context is cancelled.
	if err := listener.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
