package webhook_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/webhook"
)

func TestParseEvent(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantID  string
		wantErr bool
	}{
		{
			name:    "valid event",
			payload: `{"id":"evt_1","event":"customer.created","data":{"name":"Acme"},"created_at":"2024-01-15T10:45:00Z"}`,
			wantID:  "evt_1",
			wantErr: false,
		},
		{
			name:    "minimal event",
			payload: `{"id":"evt_2","event":"user.created","data":{}}`,
			wantID:  "evt_2",
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			payload: `{not json}`,
			wantErr: true,
		},
		{
			name:    "missing id",
			payload: `{"event":"customer.created","data":{}}`,
			wantErr: true,
		},
		{
			name:    "missing event type",
			payload: `{"id":"evt_1","data":{}}`,
			wantErr: true,
		},
		{
			name:    "empty payload",
			payload: ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				event, err := webhook.ParseEvent([]byte(tt.payload))
				if (err != nil) != tt.wantErr {
					t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				}
				if err != nil {
					if !errors.Is(err, errors.ErrMalformedPayload) {
						t.Errorf("expected ErrMalformedPayload, got: %v", err)
					}
					return
				}
				if event.ID != tt.wantID {
					t.Errorf("got ID %q, want %q", event.ID, tt.wantID)
				}
			},
		)
	}
}

func TestConstructEvent(t *testing.T) {
	_, priv, pubHex := generateTestKeyPair(t)

	now := time.Now()
	timestamp := fmt.Sprintf("%d", now.Unix())
	payload := []byte(`{"id":"evt_1","event":"customer.created","data":{"name":"Acme"},"created_at":"2024-01-15T10:45:00Z"}`)

	sig := webhook.ComputeSignature(timestamp, payload, priv)

	event, err := webhook.ConstructEvent(payload, sig, timestamp, pubHex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.ID != "evt_1" {
		t.Errorf("got ID %q, want %q", event.ID, "evt_1")
	}
	if event.Type != webhook.EventCustomerCreated {
		t.Errorf(
			"got Type %q, want %q",
			event.Type,
			webhook.EventCustomerCreated,
		)
	}
}

func TestConstructEvent_WithTolerance(t *testing.T) {
	_, priv, pubHex := generateTestKeyPair(t)

	// Timestamp 10 minutes ago.
	oldTimestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	payload := []byte(`{"id":"evt_1","event":"customer.created","data":{}}`)
	sig := webhook.ComputeSignature(oldTimestamp, payload, priv)

	// Default tolerance (5 min) should fail.
	_, err := webhook.ConstructEvent(payload, sig, oldTimestamp, pubHex)
	if err == nil {
		t.Fatal("expected error for old timestamp with default tolerance")
	}

	// Custom wider tolerance should succeed.
	event, err := webhook.ConstructEvent(
		payload,
		sig,
		oldTimestamp,
		pubHex,
		webhook.WithTolerance(15*time.Minute),
	)
	if err != nil {
		t.Fatalf("unexpected error with wider tolerance: %v", err)
	}
	if event.ID != "evt_1" {
		t.Errorf("got ID %q, want %q", event.ID, "evt_1")
	}
}

func TestConstructEvent_IgnoringTolerance(t *testing.T) {
	_, priv, pubHex := generateTestKeyPair(t)

	// Very old timestamp.
	oldTimestamp := "1000000000"
	payload := []byte(`{"id":"evt_1","event":"customer.created","data":{}}`)
	sig := webhook.ComputeSignature(oldTimestamp, payload, priv)

	event, err := webhook.ConstructEvent(
		payload,
		sig,
		oldTimestamp,
		pubHex,
		webhook.IgnoringTolerance(),
	)
	if err != nil {
		t.Fatalf("unexpected error with IgnoringTolerance: %v", err)
	}
	if event.ID != "evt_1" {
		t.Errorf("got ID %q, want %q", event.ID, "evt_1")
	}
}

func TestConstructEvent_InvalidSignature(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	pubHex := hex.EncodeToString(pub)

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	payload := []byte(`{"id":"evt_1","event":"customer.created","data":{}}`)

	_, err := webhook.ConstructEvent(payload, "dGVzdA==", timestamp, pubHex)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}
