package webhook_test

import (
	"context"
	"testing"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/webhook"
)

func TestNewListener_RequiresHandlerOptions(t *testing.T) {
	_, err := webhook.NewListener()
	if err == nil {
		t.Fatal("expected error without handler options")
	}
	if !errors.Is(err, errors.ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got: %v", err)
	}
}

func TestNewListener_RequiresPublicKey(t *testing.T) {
	_, err := webhook.NewListener(
		webhook.WithHandlerOptions(
			webhook.WithChannel(10),
		),
	)
	if err == nil {
		t.Fatal("expected error without public key in handler options")
	}
}

func TestNewListener_ValidConfig(t *testing.T) {
	h := newTestHarness(t)
	listener, err := webhook.NewListener(
		webhook.WithAddr(":0"),
		webhook.WithPath("/test-webhook"),
		webhook.WithReadTimeout(10*time.Second),
		webhook.WithWriteTimeout(10*time.Second),
		webhook.WithHandlerOptions(
			webhook.WithPublicKey(h.pubHex),
			webhook.WithChannel(10),
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer listener.Close()

	if listener.Events() == nil {
		t.Error("expected non-nil Events() channel")
	}
}

func TestListener_StartAndShutdown(t *testing.T) {
	h := newTestHarness(t)
	listener, err := webhook.NewListener(
		webhook.WithAddr("127.0.0.1:0"),
		webhook.WithHandlerOptions(
			webhook.WithPublicKey(h.pubHex),
			webhook.WithChannel(10),
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		500*time.Millisecond,
	)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- listener.Start(ctx)
	}()

	// Wait for context cancellation + shutdown.
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error from Start: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}
