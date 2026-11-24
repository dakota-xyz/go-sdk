package webhook_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/webhook"
)

// rotationRequest builds a delivery as platform sends it. signers sign
// X-Dakota-Webhook-Signatures in order; legacy, if not nil, signs
// X-Webhook-Signature.
func rotationRequest(
	t *testing.T,
	body string,
	legacy *rotationKey,
	signers ...rotationKey,
) *http.Request {
	t.Helper()
	ts := fmt.Sprintf("%d", time.Now().Unix())
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set(webhook.TimestampHeader, ts)
	if legacy != nil {
		req.Header.Set(
			webhook.SignatureHeader,
			webhook.ComputeSignature(ts, []byte(body), legacy.priv),
		)
	}
	entries := make([]string, len(signers))
	for i, k := range signers {
		entries[i] = k.entry(ts, []byte(body))
	}
	if len(entries) > 0 {
		req.Header.Set(webhook.SignaturesHeader, strings.Join(entries, ", "))
	}
	return req
}

func serve(t *testing.T, handler http.Handler, req *http.Request) int {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code
}

const rotationBody = `{"id":"evt_1","type":"customer.created","data":{}}`

func TestHandler_RotationWithPublicKeys(t *testing.T) {
	oldKey, newKey := newRotationKey(t), newRotationKey(t)

	configs := map[string][]webhook.HandlerOption{
		"WithPublicKeys, two arguments": {webhook.WithPublicKeys(oldKey.hex, newKey.hex)},
		"WithPublicKeys, one list":      {webhook.WithPublicKeys(oldKey.hex + "," + newKey.hex)},
		"WithPublicKey plus WithPublicKeys": {
			webhook.WithPublicKey(oldKey.hex),
			webhook.WithPublicKeys(newKey.hex),
		},
	}
	deliveries := []struct {
		name    string
		legacy  *rotationKey
		signers []rotationKey
	}{
		{"legacy header with the old key", &oldKey, nil},
		{"legacy header with the new key", &newKey, nil},
		{"signatures header with the new key", nil, []rotationKey{newKey}},
		{"overlap: both headers, both keys", &oldKey, []rotationKey{oldKey, newKey}},
		{"cutover: both headers on the new key", &newKey, []rotationKey{newKey}},
	}

	for name, opts := range configs {
		handler, err := webhook.NewHandler(opts...)
		if err != nil {
			t.Fatalf("%s: NewHandler: %v", name, err)
		}
		for _, d := range deliveries {
			t.Run(name+"/"+d.name, func(t *testing.T) {
				req := rotationRequest(t, rotationBody, d.legacy, d.signers...)
				if got := serve(t, handler, req); got != http.StatusOK {
					t.Errorf("got status %d, want %d", got, http.StatusOK)
				}
			})
		}
	}
}

// A single-key handler built with WithPublicKey, as every integration is
// today, keeps working when platform adds X-Dakota-Webhook-Signatures, and
// accepts a delivery whose legacy header a proxy stripped.
func TestHandler_WithPublicKeyReadsSignaturesHeader(t *testing.T) {
	key := newRotationKey(t)
	handler, err := webhook.NewHandler(webhook.WithPublicKey(key.hex))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	if got := serve(t, handler, rotationRequest(t, rotationBody, &key, key)); got != http.StatusOK {
		t.Errorf("both headers: got status %d, want %d", got, http.StatusOK)
	}
	if got := serve(t, handler, rotationRequest(t, rotationBody, nil, key)); got != http.StatusOK {
		t.Errorf("signatures header only: got status %d, want %d", got, http.StatusOK)
	}
}

func TestHandler_SignaturesHeaderFailureIsUnauthorized(t *testing.T) {
	oldKey, newKey, stranger := newRotationKey(t), newRotationKey(t), newRotationKey(t)
	handler, err := webhook.NewHandler(webhook.WithPublicKeys(oldKey.hex))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	// After cutover, a receiver that only trusts the old key rejects.
	if got := serve(t, handler, rotationRequest(t, rotationBody, &newKey, newKey)); got != http.StatusUnauthorized {
		t.Errorf("cutover, old key only: got status %d, want %d", got, http.StatusUnauthorized)
	}
	// A failing signatures header is not rescued by a valid legacy header.
	if got := serve(t, handler, rotationRequest(t, rotationBody, &oldKey, stranger)); got != http.StatusUnauthorized {
		t.Errorf("untrusted signatures, valid legacy: got status %d, want %d", got, http.StatusUnauthorized)
	}
}

func TestNewHandler_PublicKeysConfig(t *testing.T) {
	key := newRotationKey(t)
	tests := map[string][]webhook.HandlerOption{
		"empty list (unset env var)": {webhook.WithPublicKeys("")},
		"no arguments":               {webhook.WithPublicKeys()},
		"bad key in list":            {webhook.WithPublicKeys(key.hex + ",not-hex")},
		"good single, bad list":      {webhook.WithPublicKey(key.hex), webhook.WithPublicKeys("not-hex")},
	}
	for name, opts := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := webhook.NewHandler(opts...)
			if !errors.Is(err, errors.ErrInvalidConfig) {
				t.Errorf("got %v, want ErrInvalidConfig", err)
			}
		})
	}
}

// A proxy may split X-Dakota-Webhook-Signatures across several lines.
func TestHandler_SplitSignaturesHeader(t *testing.T) {
	oldKey, newKey := newRotationKey(t), newRotationKey(t)
	handler, err := webhook.NewHandler(webhook.WithPublicKeys(oldKey.hex))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	ts := fmt.Sprintf("%d", time.Now().Unix())
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(rotationBody))
	req.Header.Set(webhook.TimestampHeader, ts)
	req.Header.Add(webhook.SignaturesHeader, newKey.entry(ts, []byte(rotationBody)))
	req.Header.Add(webhook.SignaturesHeader, oldKey.entry(ts, []byte(rotationBody)))
	if got := serve(t, handler, req); got != http.StatusOK {
		t.Errorf("got status %d, want %d", got, http.StatusOK)
	}
}

// Empty X-Dakota-Webhook-Signatures lines fall back to X-Webhook-Signature.
func TestHandler_EmptySignaturesLinesFallBack(t *testing.T) {
	key := newRotationKey(t)
	handler, err := webhook.NewHandler(webhook.WithPublicKeys(key.hex))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := rotationRequest(t, rotationBody, &key)
	req.Header.Add(webhook.SignaturesHeader, "")
	req.Header.Add(webhook.SignaturesHeader, " ")
	if got := serve(t, handler, req); got != http.StatusOK {
		t.Errorf("got status %d, want %d", got, http.StatusOK)
	}
}

func TestHandler_WithPublicKeysAccumulates(t *testing.T) {
	oldKey, newKey := newRotationKey(t), newRotationKey(t)
	handler, err := webhook.NewHandler(
		webhook.WithPublicKeys(oldKey.hex),
		webhook.WithPublicKeys(newKey.hex),
	)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	for _, k := range []rotationKey{oldKey, newKey} {
		if got := serve(t, handler, rotationRequest(t, rotationBody, nil, k)); got != http.StatusOK {
			t.Errorf("key %s: got status %d, want %d", k.id, got, http.StatusOK)
		}
	}
}

// Migration: keep WithPublicKey on the old variable and add WithPublicKeys
// on a new one that may not be set yet.
func TestHandler_WithPublicKeyPlusUnsetPublicKeys(t *testing.T) {
	key := newRotationKey(t)
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(key.hex),
		webhook.WithPublicKeys(""),
	)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	if got := serve(t, handler, rotationRequest(t, rotationBody, &key, key)); got != http.StatusOK {
		t.Errorf("got status %d, want %d", got, http.StatusOK)
	}
}
