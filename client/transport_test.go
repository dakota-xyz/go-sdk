package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dakota-xyz/go-sdk/log"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestIdempotencyTransport_AddsKey(t *testing.T) {
	transport := &idempotencyTransport{
		generator: func() (string, error) { return "idem-key-123", nil },
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("x-idempotency-key"); got != "idem-key-123" {
				t.Fatalf("x-idempotency-key = %q, want %q", got, "idem-key-123")
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://example.com/customers", bytes.NewReader([]byte(`{"name":"Acme"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
}

func TestIdempotencyTransport_UsesContextKey(t *testing.T) {
	transport := &idempotencyTransport{
		generator: func() (string, error) { return "generated-key", nil },
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("x-idempotency-key"); got != "ctx-key" {
				t.Fatalf("x-idempotency-key = %q, want %q", got, "ctx-key")
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://example.com/customers", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req = req.WithContext(WithIdempotencyKey(req.Context(), "ctx-key"))

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
}

func TestIdempotencyTransport_DoesNotAddForGet(t *testing.T) {
	transport := &idempotencyTransport{
		generator: func() (string, error) { return "generated-key", nil },
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("x-idempotency-key"); got != "" {
				t.Fatalf("x-idempotency-key = %q, want empty", got)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com/customers", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
}

// The platform requires an x-idempotency-key on mutating PUT/PATCH/DELETE
// operations too (their generated *Params carry the field), so the transport
// injects one for every mutating method — not just POST.
func TestIdempotencyTransport_AddsKeyForPutPatchDelete(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			transport := &idempotencyTransport{
				generator: func() (string, error) { return "idem-key-123", nil },
				next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if got := req.Header.Get("x-idempotency-key"); got != "idem-key-123" {
						t.Fatalf("x-idempotency-key = %q, want %q", got, "idem-key-123")
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
				}),
			}

			req, err := http.NewRequest(method, "https://example.com/policies/pol_1", http.NoBody)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}

			if _, err := transport.RoundTrip(req); err != nil {
				t.Fatalf("RoundTrip error: %v", err)
			}
		})
	}
}

func TestRequestEditorWithIdempotencyKey(t *testing.T) {
	editor := RequestEditorWithIdempotencyKey("manual-key")
	req, err := http.NewRequest(http.MethodPost, "https://example.com/customers", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err := editor(context.Background(), req); err != nil {
		t.Fatalf("editor error: %v", err)
	}
	if got := req.Header.Get("x-idempotency-key"); got != "manual-key" {
		t.Fatalf("x-idempotency-key = %q, want %q", got, "manual-key")
	}
}

func TestRetryTransport_RetriesPostWithIdempotency(t *testing.T) {
	attempts := 0
	transport := &retryTransport{
		policy: RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond},
		logger: log.Nop(),
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return &http.Response{StatusCode: http.StatusInternalServerError, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("fail"))}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://example.com/customers", bytes.NewReader([]byte(`{"name":"Acme"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req = req.WithContext(context.Background())
	req.Header.Set("x-idempotency-key", "idem-123")

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestRetryTransport_DoesNotRetryPostWithoutIdempotency(t *testing.T) {
	attempts := 0
	transport := &retryTransport{
		policy: RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond},
		logger: log.Nop(),
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return &http.Response{StatusCode: http.StatusInternalServerError, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("fail"))}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://example.com/customers", bytes.NewReader([]byte(`{"name":"Acme"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	_, _ = transport.RoundTrip(req)
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryTransport_NonReplayableBody(t *testing.T) {
	attempts := 0
	transport := &retryTransport{
		policy: RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond},
		logger: log.Nop(),
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return &http.Response{StatusCode: http.StatusInternalServerError, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("fail"))}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://example.com/customers", io.NopCloser(strings.NewReader("body")))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("x-idempotency-key", "idem-123")
	req.GetBody = nil // force non-replayable

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryTransport_ContextCanceledDuringBackoff(t *testing.T) {
	attempts := 0
	transport := &retryTransport{
		policy: RetryPolicy{MaxAttempts: 3, InitialBackoff: 200 * time.Millisecond, MaxBackoff: 200 * time.Millisecond},
		logger: log.Nop(),
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("fail"))}, nil
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/customers", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err = transport.RoundTrip(req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryAfterDelay_Seconds(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "2")

	d, ok := retryAfterDelay(resp)
	if !ok {
		t.Fatal("expected retry-after delay")
	}
	if d != 2*time.Second {
		t.Fatalf("delay = %v, want %v", d, 2*time.Second)
	}
}

func TestRetryAfterDelay_HTTPDate(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	target := time.Now().Add(2 * time.Second).UTC()
	resp.Header.Set("Retry-After", target.Format(http.TimeFormat))

	d, ok := retryAfterDelay(resp)
	if !ok {
		t.Fatal("expected retry-after delay")
	}
	if d <= 0 || d > 3*time.Second {
		t.Fatalf("delay = %v, expected in (0,3s]", d)
	}
}

func TestRetryAfterDelay_Invalid(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "not-a-date")

	if _, ok := retryAfterDelay(resp); ok {
		t.Fatal("expected invalid retry-after to be ignored")
	}
}

func TestCloneRequestWithBody_GetBodyError(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/test", io.NopCloser(strings.NewReader("payload")))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return nil, fmt.Errorf("boom")
	}

	_, err = cloneRequestWithBody(req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthTransport_AutoApplicationsUsesAppToken(t *testing.T) {
	transport := &authTransport{
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("X-Application-Token"); got != "app_token" {
				t.Fatalf("X-Application-Token = %q, want %q", got, "app_token")
			}
			if got := req.Header.Get("x-api-key"); got != "" {
				t.Fatalf("x-api-key = %q, want empty", got)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
		apiKey:           "api_key",
		applicationToken: "app_token",
		authMode:         AuthModeAuto,
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com/applications/abc", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
}

func TestAuthTransport_APIKeyMode(t *testing.T) {
	transport := &authTransport{
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("x-api-key"); got != "api_key" {
				t.Fatalf("x-api-key = %q, want %q", got, "api_key")
			}
			if got := req.Header.Get("X-Application-Token"); got != "" {
				t.Fatalf("X-Application-Token = %q, want empty", got)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
		apiKey:   "api_key",
		authMode: AuthModeAPIKey,
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com/customers", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
}

func TestAuthTransport_PreservesExistingAuthHeaders(t *testing.T) {
	transport := &authTransport{
		next: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("x-api-key"); got != "already-set" {
				t.Fatalf("x-api-key = %q, want %q", got, "already-set")
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
		apiKey:           "api_key",
		applicationToken: "app_token",
		authMode:         AuthModeAuto,
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com/customers", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("x-api-key", "already-set")

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
}

func TestSanitizeHeaders_RedactsSensitive(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-api-key", "secret_api_key")
	headers.Set("Authorization", "Bearer super-secret")
	headers.Set("X-Custom", "safe")
	headers.Set("X-Extra-Secret", "hide-me")

	sanitized := sanitizeHeaders(headers, map[string]struct{}{"x-custom": {}})

	if got := sanitized["x-api-key"]; got != redactedValue && got != "" {
		t.Fatalf("x-api-key not redacted: %q", got)
	}
	if got := sanitized["Authorization"]; got != redactedValue {
		t.Fatalf("Authorization not redacted: %q", got)
	}
	if got := sanitized["X-Extra-Secret"]; got != redactedValue {
		t.Fatalf("X-Extra-Secret not redacted: %q", got)
	}
	if got := sanitized["X-Custom"]; got != redactedValue {
		t.Fatalf("custom redaction not applied: %q", got)
	}
}

func TestGenerateUUIDv4_Format(t *testing.T) {
	value, err := generateUUIDv4()
	if err != nil {
		t.Fatalf("generateUUIDv4 error: %v", err)
	}
	parts := strings.Split(value, "-")
	if len(parts) != 5 {
		t.Fatalf("uuid = %q, expected 5 parts", value)
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		t.Fatalf("uuid = %q, unexpected segment lengths", value)
	}
	if parts[2][0] != '4' {
		t.Fatalf("uuid = %q, expected version 4", value)
	}
}

func TestIsRetryableStatus(t *testing.T) {
	for _, status := range []int{
		http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		if !isRetryableStatus(status) {
			t.Fatalf("expected status %d to be retryable", status)
		}
	}
	if isRetryableStatus(http.StatusBadRequest) {
		t.Fatal("expected 400 to be non-retryable")
	}
}
