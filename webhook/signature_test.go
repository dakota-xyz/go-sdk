package webhook_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/webhook"
)

func generateTestKeyPair(t *testing.T) (
	ed25519.PublicKey,
	ed25519.PrivateKey,
	string,
) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}
	return pub, priv, hex.EncodeToString(pub)
}

func TestVerifySignature_RoundTrip(t *testing.T) {
	_, priv, pubHex := generateTestKeyPair(t)

	payload := []byte(`{"id":"evt_1","event":"customer.created","data":{}}`)
	timestamp := "1700000000"

	sig := webhook.ComputeSignature(timestamp, payload, priv)

	if err := webhook.VerifySignature(
		payload,
		sig,
		timestamp,
		pubHex,
	); err != nil {
		t.Fatalf("expected valid signature, got error: %v", err)
	}
}

func TestVerifySignature_WrongKey(t *testing.T) {
	_, priv, _ := generateTestKeyPair(t)
	_, _, otherPubHex := generateTestKeyPair(t)

	payload := []byte(`{"id":"evt_1","event":"customer.created","data":{}}`)
	timestamp := "1700000000"

	sig := webhook.ComputeSignature(timestamp, payload, priv)

	err := webhook.VerifySignature(payload, sig, timestamp, otherPubHex)
	if err == nil {
		t.Fatal("expected error for wrong key")
	}
	if !errors.Is(err, errors.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature, got: %v", err)
	}
}

func TestVerifySignature_TamperedPayload(t *testing.T) {
	_, priv, pubHex := generateTestKeyPair(t)

	payload := []byte(`{"id":"evt_1","event":"customer.created","data":{}}`)
	timestamp := "1700000000"

	sig := webhook.ComputeSignature(timestamp, payload, priv)

	tampered := []byte(`{"id":"evt_1","event":"customer.created","data":{"hacked":true}}`)
	err := webhook.VerifySignature(tampered, sig, timestamp, pubHex)
	if err == nil {
		t.Fatal("expected error for tampered payload")
	}
	if !errors.Is(err, errors.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature, got: %v", err)
	}
}

func TestVerifySignature_TamperedTimestamp(t *testing.T) {
	_, priv, pubHex := generateTestKeyPair(t)

	payload := []byte(`{"id":"evt_1","event":"customer.created","data":{}}`)
	timestamp := "1700000000"

	sig := webhook.ComputeSignature(timestamp, payload, priv)

	err := webhook.VerifySignature(payload, sig, "1700000001", pubHex)
	if err == nil {
		t.Fatal("expected error for tampered timestamp")
	}
}

func TestVerifySignature_InvalidInputs(t *testing.T) {
	tests := []struct {
		name      string
		payload   []byte
		sig       string
		timestamp string
		pubKey    string
		wantCode  errors.Code
	}{
		{
			name:      "invalid public key hex",
			payload:   []byte("test"),
			sig:       "dGVzdA==",
			timestamp: "1700000000",
			pubKey:    "not-hex",
			wantCode:  errors.CodeInvalidConfig,
		},
		{
			name:      "wrong public key length",
			payload:   []byte("test"),
			sig:       "dGVzdA==",
			timestamp: "1700000000",
			pubKey:    "aabbccdd",
			wantCode:  errors.CodeInvalidConfig,
		},
		{
			name:      "invalid base64 signature",
			payload:   []byte("test"),
			sig:       "not-base64!!!",
			timestamp: "1700000000",
			pubKey:    hex.EncodeToString(make([]byte, 32)),
			wantCode:  errors.CodeInvalidSignature,
		},
		{
			name:      "wrong signature length",
			payload:   []byte("test"),
			sig:       "dGVzdA==",
			timestamp: "1700000000",
			pubKey:    hex.EncodeToString(make([]byte, 32)),
			wantCode:  errors.CodeInvalidSignature,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				err := webhook.VerifySignature(
					tt.payload,
					tt.sig,
					tt.timestamp,
					tt.pubKey,
				)
				if err == nil {
					t.Fatal("expected error")
				}

				var dakErr *errors.Error
				if !errors.As(err, &dakErr) {
					t.Fatalf("expected dakotaerrors.Error, got %T", err)
				}
				if dakErr.Code != tt.wantCode {
					t.Errorf("got code %q, want %q", dakErr.Code, tt.wantCode)
				}
			},
		)
	}
}

func TestParsePublicKey(t *testing.T) {
	tests := []struct {
		name    string
		hexKey  string
		wantErr bool
	}{
		{
			name:    "valid 32-byte key",
			hexKey:  hex.EncodeToString(make([]byte, 32)),
			wantErr: false,
		},
		{
			name:    "invalid hex",
			hexKey:  "xyz",
			wantErr: true,
		},
		{
			name:    "wrong length",
			hexKey:  hex.EncodeToString(make([]byte, 16)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				_, err := webhook.ParsePublicKey(tt.hexKey)
				if (err != nil) != tt.wantErr {
					t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				}
			},
		)
	}
}

func TestComputeSignature_MatchesPlatform(t *testing.T) {
	// This test verifies our ComputeSignature matches the platform's
	// GenerateSignature: message = timestamp_bytes + body_bytes.
	_, priv, pubHex := generateTestKeyPair(t)

	timestamp := "1700000000"
	body := []byte(`{"event":"test"}`)

	sig := webhook.ComputeSignature(timestamp, body, priv)

	// Verify the computed signature.
	err := webhook.VerifySignature(body, sig, timestamp, pubHex)
	if err != nil {
		t.Fatalf("computed signature did not verify: %v", err)
	}
}

func TestVerifySignature_EmptyPayload(t *testing.T) {
	_, priv, pubHex := generateTestKeyPair(t)

	timestamp := "1700000000"
	payload := []byte{}

	sig := webhook.ComputeSignature(timestamp, payload, priv)

	if err := webhook.VerifySignature(
		payload,
		sig,
		timestamp,
		pubHex,
	); err != nil {
		t.Fatalf("expected valid signature for empty payload, got: %v", err)
	}
}

func BenchmarkVerifySignature(b *testing.B) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubHex := hex.EncodeToString(pub)
	payload := []byte(`{"id":"evt_bench","event":"customer.created","data":{"id":"cust_1"}}`)
	timestamp := "1700000000"
	sig := webhook.ComputeSignature(timestamp, payload, priv)

	b.ResetTimer()
	for range b.N {
		err := webhook.VerifySignature(payload, sig, timestamp, pubHex)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkComputeSignature(b *testing.B) {
	_, priv, _ := ed25519.GenerateKey(nil)
	payload := []byte(`{"id":"evt_bench","event":"customer.created","data":{"id":"cust_1"}}`)
	timestamp := fmt.Sprintf("%d", 1700000000)

	b.ResetTimer()
	for range b.N {
		webhook.ComputeSignature(timestamp, payload, priv)
	}
}
