package webhook_test

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/webhook"
	"github.com/dakota-xyz/go-sdk/webhook/idempotency"
)

type testHarness struct {
	pub    ed25519.PublicKey
	priv   ed25519.PrivateKey
	pubHex string
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}
	return &testHarness{
		pub:    pub,
		priv:   priv,
		pubHex: hex.EncodeToString(pub),
	}
}

func (h *testHarness) signedRequest(
	t *testing.T,
	payload string,
) *http.Request {
	t.Helper()
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sig := webhook.ComputeSignature(timestamp, []byte(payload), h.priv)

	req := httptest.NewRequest(
		http.MethodPost,
		"/webhook",
		strings.NewReader(payload),
	)
	req.Header.Set(webhook.SignatureHeader, sig)
	req.Header.Set(webhook.TimestampHeader, timestamp)
	return req
}

func TestNewHandler_RequiresPublicKey(t *testing.T) {
	_, err := webhook.NewHandler()
	if err == nil {
		t.Fatal("expected error without public key")
	}
	if !errors.Is(err, errors.ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got: %v", err)
	}
}

func TestNewHandler_InvalidPublicKey(t *testing.T) {
	_, err := webhook.NewHandler(webhook.WithPublicKey("not-valid-hex"))
	if err == nil {
		t.Fatal("expected error with invalid public key")
	}
}

func TestNewHandler_RejectsMixedModes(t *testing.T) {
	h := newTestHarness(t)
	_, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithChannel(10),
		webhook.On(
			webhook.EventCustomerCreated,
			func(_ context.Context, _ webhook.Event) error {
				return nil
			},
		),
	)
	if err == nil {
		t.Fatal("expected error for mixed delivery modes")
	}
	if !errors.Is(err, errors.ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got: %v", err)
	}
}

func TestNewHandler_RejectsInvalidConfig(t *testing.T) {
	h := newTestHarness(t)

	tests := []struct {
		name string
		opts []webhook.HandlerOption
	}{
		{
			name: "negative channel size",
			opts: []webhook.HandlerOption{
				webhook.WithPublicKey(h.pubHex),
				webhook.WithChannel(-1),
			},
		},
		{
			name: "non-positive payload size",
			opts: []webhook.HandlerOption{
				webhook.WithPublicKey(h.pubHex),
				webhook.WithMaxPayloadSize(0),
			},
		},
		{
			name: "non-positive tolerance",
			opts: []webhook.HandlerOption{
				webhook.WithPublicKey(h.pubHex),
				webhook.WithHandlerTolerance(0),
			},
		},
		{
			name: "invalid ack policy",
			opts: []webhook.HandlerOption{
				webhook.WithPublicKey(h.pubHex),
				webhook.WithAckPolicy(webhook.AckPolicy(99)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				_, err := webhook.NewHandler(tt.opts...)
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

func TestHandler_RejectsNonPOST(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(webhook.WithPublicKey(h.pubHex))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf(
			"got status %d, want %d",
			rec.Code,
			http.StatusMethodNotAllowed,
		)
	}
}

func TestHandler_MissingHeaders(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(webhook.WithPublicKey(h.pubHex))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name   string
		setSig bool
		setTs  bool
	}{
		{"missing both", false, false},
		{"missing signature", false, true},
		{"missing timestamp", true, false},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				req := httptest.NewRequest(
					http.MethodPost,
					"/webhook",
					strings.NewReader(`{}`),
				)
				if tt.setSig {
					req.Header.Set(webhook.SignatureHeader, "dGVzdA==")
				}
				if tt.setTs {
					req.Header.Set(webhook.TimestampHeader, "1700000000")
				}

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusBadRequest {
					t.Errorf(
						"got status %d, want %d",
						rec.Code,
						http.StatusBadRequest,
					)
				}
			},
		)
	}
}

func TestHandler_InvalidSignature(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(webhook.WithPublicKey(h.pubHex))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{"id":"evt_1","type":"customer.created","data":{}}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/webhook",
		strings.NewReader(payload),
	)
	req.Header.Set(
		webhook.SignatureHeader,
		"aW52YWxpZHNpZ25hdHVyZWludmFsaWRzaWduYXR1cmVpbnZhbGlkc2lnbmF0dXJlaW52YWxpZHNpZ25hdHVyZQ==",
	)
	req.Header.Set(
		webhook.TimestampHeader,
		fmt.Sprintf("%d", time.Now().Unix()),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandler_CallbackDelivery(t *testing.T) {
	h := newTestHarness(t)
	var received atomic.Int32

	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.On(
			webhook.EventCustomerCreated,
			func(_ context.Context, event webhook.Event) error {
				received.Add(1)
				if event.ID != "evt_1" {
					t.Errorf("got ID %q, want %q", event.ID, "evt_1")
				}
				return nil
			},
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{"id":"evt_1","type":"customer.created","data":{"name":"Acme"}}`
	req := h.signedRequest(t, payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if received.Load() != 1 {
		t.Errorf("callback invoked %d times, want 1", received.Load())
	}
}

func TestHandler_DefaultCallback(t *testing.T) {
	h := newTestHarness(t)
	var received atomic.Int32

	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.OnDefault(
			func(_ context.Context, _ webhook.Event) error {
				received.Add(1)
				return nil
			},
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{"id":"evt_1","type":"customer.created","data":{}}`
	req := h.signedRequest(t, payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if received.Load() != 1 {
		t.Errorf("default callback invoked %d times, want 1", received.Load())
	}
}

func TestHandler_ChannelDelivery(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithChannel(10),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer handler.Close()

	payload := `{"id":"evt_1","type":"customer.created","data":{}}`
	req := h.signedRequest(t, payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	select {
	case event := <-handler.Events():
		if event.ID != "evt_1" {
			t.Errorf("got ID %q, want %q", event.ID, "evt_1")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHandler_ChannelNilWhenCallbackMode(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.OnDefault(
			func(
				_ context.Context,
				_ webhook.Event,
			) error {
				return nil
			},
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if handler.Events() != nil {
		t.Error("expected nil Events() in callback mode")
	}
}

func TestHandler_IdempotencyStore(t *testing.T) {
	h := newTestHarness(t)
	var callCount atomic.Int32

	store := idempotency.NewMemoryStore()
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithIdempotencyStore(store),
		webhook.OnDefault(
			func(_ context.Context, _ webhook.Event) error {
				callCount.Add(1)
				return nil
			},
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{"id":"evt_1","type":"customer.created","data":{}}`

	// First request.
	req1 := h.signedRequest(t, payload)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf(
			"first request: got status %d, want %d",
			rec1.Code,
			http.StatusOK,
		)
	}

	// Second request (duplicate).
	req2 := h.signedRequest(t, payload)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf(
			"second request: got status %d, want %d",
			rec2.Code,
			http.StatusOK,
		)
	}

	if callCount.Load() != 1 {
		t.Errorf(
			"callback invoked %d times, want 1 (duplicate should be skipped)",
			callCount.Load(),
		)
	}
}

func TestHandler_ConcurrentDuplicateSingleProcess(t *testing.T) {
	h := newTestHarness(t)
	var callCount atomic.Int32

	store := idempotency.NewMemoryStore()
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithIdempotencyStore(store),
		webhook.OnDefault(
			func(_ context.Context, _ webhook.Event) error {
				callCount.Add(1)
				time.Sleep(50 * time.Millisecond)
				return nil
			},
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{"id":"evt_concurrent","type":"customer.created","data":{}}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sig := webhook.ComputeSignature(timestamp, []byte(payload), h.priv)

	const requests = 20
	var wg sync.WaitGroup
	statuses := make(chan int, requests)

	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(
				http.MethodPost,
				"/webhook",
				strings.NewReader(payload),
			)
			req.Header.Set(webhook.SignatureHeader, sig)
			req.Header.Set(webhook.TimestampHeader, timestamp)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			statuses <- rec.Code
		}()
	}

	wg.Wait()
	close(statuses)

	for code := range statuses {
		if code != http.StatusOK {
			t.Fatalf("got status %d, want %d", code, http.StatusOK)
		}
	}

	if callCount.Load() != 1 {
		t.Fatalf("callback invoked %d times, want 1", callCount.Load())
	}
}

func TestHandler_IdempotencyMarkedAfterSuccessfulProcessing(t *testing.T) {
	h := newTestHarness(t)
	var callCount atomic.Int32

	store := idempotency.NewMemoryStore()
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithIdempotencyStore(store),
		webhook.OnDefault(
			func(_ context.Context, _ webhook.Event) error {
				if callCount.Add(1) == 1 {
					return fmt.Errorf("transient failure")
				}
				return nil
			},
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{"id":"evt_1","type":"customer.created","data":{}}`

	req1 := h.signedRequest(t, payload)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"first request: got status %d, want %d",
			rec1.Code,
			http.StatusServiceUnavailable,
		)
	}

	req2 := h.signedRequest(t, payload)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf(
			"second request: got status %d, want %d",
			rec2.Code,
			http.StatusOK,
		)
	}

	req3 := h.signedRequest(t, payload)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf(
			"third request: got status %d, want %d",
			rec3.Code,
			http.StatusOK,
		)
	}

	if callCount.Load() != 2 {
		t.Fatalf("callback invoked %d times, want 2", callCount.Load())
	}
}

func TestHandler_PayloadTooLarge(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithMaxPayloadSize(10),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{"id":"evt_1","type":"customer.created","data":{"key":"value that makes this too long"}}`
	req := h.signedRequest(t, payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf(
			"got status %d, want %d",
			rec.Code,
			http.StatusRequestEntityTooLarge,
		)
	}
}

func TestHandler_BadJSON(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(webhook.WithPublicKey(h.pubHex))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{not json}`
	req := h.signedRequest(t, payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandler_CallbackError(t *testing.T) {
	h := newTestHarness(t)

	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.OnDefault(
			func(_ context.Context, _ webhook.Event) error {
				return fmt.Errorf("processing failed")
			},
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{"id":"evt_1","type":"customer.created","data":{}}`
	req := h.signedRequest(t, payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf(
			"got status %d, want %d",
			rec.Code,
			http.StatusServiceUnavailable,
		)
	}
}

func TestHandler_CallbackError_AckAlways(t *testing.T) {
	h := newTestHarness(t)

	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithAckPolicy(webhook.AckAlways),
		webhook.OnDefault(
			func(_ context.Context, _ webhook.Event) error {
				return fmt.Errorf("processing failed")
			},
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{"id":"evt_1","type":"customer.created","data":{}}`
	req := h.signedRequest(t, payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_ChannelFull(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithChannel(0), // unbuffered
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer handler.Close()

	// No one reading from the channel - should not block.
	payload := `{"id":"evt_1","type":"customer.created","data":{}}`
	req := h.signedRequest(t, payload)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-done:
		// Good - didn't block.
	case <-time.After(2 * time.Second):
		t.Fatal("ServeHTTP blocked on full channel")
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf(
			"got status %d, want %d",
			rec.Code,
			http.StatusServiceUnavailable,
		)
	}
}

func TestHandler_ChannelFull_AckAlways(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithAckPolicy(webhook.AckAlways),
		webhook.WithChannel(0), // unbuffered
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer handler.Close()

	payload := `{"id":"evt_1","type":"customer.created","data":{}}`
	req := h.signedRequest(t, payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_Close_IdempotentAndSafe(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithChannel(1),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.Close()
	handler.Close()

	payload := `{"id":"evt_1","type":"customer.created","data":{}}`
	req := h.signedRequest(t, payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf(
			"got status %d, want %d",
			rec.Code,
			http.StatusServiceUnavailable,
		)
	}
}

func TestHandler_ExpiredTimestamp(t *testing.T) {
	h := newTestHarness(t)
	handler, err := webhook.NewHandler(
		webhook.WithPublicKey(h.pubHex),
		webhook.WithHandlerTolerance(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	payload := `{"id":"evt_1","type":"customer.created","data":{}}`
	oldTimestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	sig := webhook.ComputeSignature(oldTimestamp, []byte(payload), h.priv)

	req := httptest.NewRequest(
		http.MethodPost,
		"/webhook",
		strings.NewReader(payload),
	)
	req.Header.Set(webhook.SignatureHeader, sig)
	req.Header.Set(webhook.TimestampHeader, oldTimestamp)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
