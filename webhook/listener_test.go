package webhook_test

import (
	"context"
	"strings"
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

func TestNewListener_InvalidConfig(t *testing.T) {
	h := newTestHarness(t)

	tests := []struct {
		name string
		opts []webhook.ListenerOption
	}{
		{
			name: "path must start with slash",
			opts: []webhook.ListenerOption{
				webhook.WithPath("webhook"),
			},
		},
		{
			name: "read timeout must be positive",
			opts: []webhook.ListenerOption{
				webhook.WithReadTimeout(0),
			},
		},
		{
			name: "write timeout must be positive",
			opts: []webhook.ListenerOption{
				webhook.WithWriteTimeout(0),
			},
		},
		{
			name: "read header timeout must be positive",
			opts: []webhook.ListenerOption{
				webhook.WithReadHeaderTimeout(0),
			},
		},
		{
			name: "idle timeout must be positive",
			opts: []webhook.ListenerOption{
				webhook.WithIdleTimeout(0),
			},
		},
		{
			name: "shutdown timeout must be positive",
			opts: []webhook.ListenerOption{
				webhook.WithShutdownTimeout(0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				opts := append(
					[]webhook.ListenerOption{
						webhook.WithHandlerOptions(webhook.WithPublicKey(h.pubHex)),
					},
					tt.opts...,
				)
				_, err := webhook.NewListener(opts...)
				if err == nil {
					t.Fatal("expected error")
				}
				if !errors.Is(err, errors.ErrInvalidConfig) {
					t.Errorf("expected ErrInvalidConfig, got: %v", err)
				}
			},
		)
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

func TestListener_AddrAfterStart(t *testing.T) {
	h := newTestHarness(t)
	listener, err := webhook.NewListener(
		webhook.WithAddr("127.0.0.1:0"),
		webhook.WithHandlerOptions(
			webhook.WithPublicKey(h.pubHex),
			webhook.WithChannel(1),
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- listener.Start(ctx)
	}()

	var addrStr string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		addr := listener.Addr()
		if addr != nil {
			addrStr = addr.String()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if addrStr == "" {
		t.Fatal("expected non-empty listener address after start")
	}
	if strings.HasSuffix(addrStr, ":0") {
		t.Fatalf("expected concrete bound port, got %q", addrStr)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error from Start: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after cancel")
	}
}
