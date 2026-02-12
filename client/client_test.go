package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dakota-xyz/go-sdk/client"
	"github.com/dakota-xyz/go-sdk/client/gen"
	"github.com/dakota-xyz/go-sdk/log"
)

func TestNew_RequiresCredentials(t *testing.T) {
	_, err := client.New()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNew_DefaultsToSandbox(t *testing.T) {
	c, err := client.New(client.WithAPIKey("test_key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := c.BaseURL(), "https://api.platform.sandbox.dakota.xyz"; got != want {
		t.Fatalf("base URL = %q, want %q", got, want)
	}
}

func TestNew_WithBaseURLAndInjectedAPIKey(t *testing.T) {
	var gotAPIKey string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/customers" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAPIKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"meta":{"total_count":0,"has_more_after":false,"has_more_before":false}}`))
	}))
	defer ts.Close()

	c, err := client.New(
		client.WithBaseURL(ts.URL+"/"),
		client.WithAPIKey("super_secret"),
		client.WithTimeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := c.BaseURL(), ts.URL; got != want {
		t.Fatalf("base URL = %q, want %q", got, want)
	}

	resp, err := client.CheckResponse(c.Raw().ListCustomersWithResponse(context.Background(), nil))
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("expected JSON200 payload")
	}
	if gotAPIKey != "super_secret" {
		t.Fatalf("x-api-key = %q, want %q", gotAPIKey, "super_secret")
	}
}

func TestNew_ApplicationTokenAuthMode(t *testing.T) {
	_, err := client.New(
		client.WithAuthMode(client.AuthModeApplicationToken),
		client.WithAPIKey("unused"),
	)
	if err == nil {
		t.Fatal("expected error when application token is missing")
	}
}

func TestNew_ApplicationTokenAuthModeWorks(t *testing.T) {
	var gotToken string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Application-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"meta":{"total_count":0,"has_more_after":false,"has_more_before":false}}`))
	}))
	defer ts.Close()

	c, err := client.New(
		client.WithBaseURL(ts.URL),
		client.WithAuthMode(client.AuthModeApplicationToken),
		client.WithApplicationToken("app_token"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.CheckResponse(c.Raw().ListApplicationsWithResponse(context.Background(), nil))
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if gotToken != "app_token" {
		t.Fatalf("X-Application-Token = %q, want %q", gotToken, "app_token")
	}
}

func TestNew_InvalidConfigValidation(t *testing.T) {
	tests := []struct {
		name string
		opts []client.Option
	}{
		{
			name: "invalid base URL",
			opts: []client.Option{client.WithAPIKey("k"), client.WithBaseURL("://bad")},
		},
		{
			name: "invalid timeout",
			opts: []client.Option{client.WithAPIKey("k"), client.WithTimeout(0)},
		},
		{
			name: "invalid retry attempts",
			opts: []client.Option{client.WithAPIKey("k"), client.WithRetryPolicy(client.RetryPolicy{MaxAttempts: 0, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond})},
		},
		{
			name: "invalid retry backoff bounds",
			opts: []client.Option{client.WithAPIKey("k"), client.WithRetryPolicy(client.RetryPolicy{MaxAttempts: 2, InitialBackoff: 2 * time.Millisecond, MaxBackoff: time.Millisecond})},
		},
		{
			name: "api key mode requires api key",
			opts: []client.Option{client.WithAuthMode(client.AuthModeAPIKey), client.WithApplicationToken("app")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.New(tt.opts...)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestNew_PreservesProvidedHTTPClientTimeout(t *testing.T) {
	httpClient := &http.Client{Timeout: 123 * time.Millisecond}
	c, err := client.New(
		client.WithAPIKey("k"),
		client.WithHTTPClient(httpClient),
		client.WithTimeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := c.HTTPClient().Timeout, 123*time.Millisecond; got != want {
		t.Fatalf("http timeout = %v, want %v", got, want)
	}
}

func TestNew_WithUserAgent(t *testing.T) {
	var gotUserAgent string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"meta":{"total_count":0,"has_more_after":false,"has_more_before":false}}`))
	}))
	defer ts.Close()

	c, err := client.New(
		client.WithBaseURL(ts.URL),
		client.WithAPIKey("k"),
		client.WithUserAgent("dakota-test-agent"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.CheckResponse(c.Raw().ListCustomersWithResponse(context.Background(), nil))
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if gotUserAgent != "dakota-test-agent" {
		t.Fatalf("User-Agent = %q, want %q", gotUserAgent, "dakota-test-agent")
	}
}

func TestNew_AdditionalOptionsAndLoggerAccessor(t *testing.T) {
	customLogger := log.New()
	c, err := client.New(
		client.WithEnvironment(client.EnvironmentLocal),
		client.WithAPIKey("k"),
		client.WithLogger(customLogger),
		client.WithAutomaticIdempotency(false),
		client.WithIdempotencyKeyGenerator(func() (string, error) { return "custom", nil }),
		client.WithRedactedHeaders("X-Custom-Secret"),
		client.WithRequestEditor(func(_ context.Context, req *http.Request) error {
			req.Header.Set("X-Test-Editor", "1")
			return nil
		}),
		client.WithHeaderLogging(true, true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Logger() == nil {
		t.Fatal("expected logger accessor to return non-nil logger")
	}
}

func TestClient_IteratorHelpers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/applications", "/customers", "/events", "/customers/cus_1/transactions", "/customers/cus_1/recipients":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"total_count":0,"has_more_after":false,"has_more_before":false}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	c, err := client.New(
		client.WithBaseURL(ts.URL),
		client.WithAPIKey("k"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	iterators := []struct {
		name string
		next func(context.Context) (bool, error)
	}{
		{
			name: "applications",
			next: func(ctx context.Context) (bool, error) {
				_, ok, err := c.ApplicationsIterator(nil).Next(ctx)
				return ok, err
			},
		},
		{
			name: "customers",
			next: func(ctx context.Context) (bool, error) {
				_, ok, err := c.CustomersIterator(nil).Next(ctx)
				return ok, err
			},
		},
		{
			name: "transactions",
			next: func(ctx context.Context) (bool, error) {
				_, ok, err := c.TransactionsIterator(gen.KSUID("cus_1"), nil).Next(ctx)
				return ok, err
			},
		},
		{
			name: "recipients",
			next: func(ctx context.Context) (bool, error) {
				_, ok, err := c.RecipientsIterator(gen.KSUID("cus_1"), nil).Next(ctx)
				return ok, err
			},
		},
		{
			name: "events",
			next: func(ctx context.Context) (bool, error) {
				_, ok, err := c.EventsIterator(nil).Next(ctx)
				return ok, err
			},
		},
	}

	for _, it := range iterators {
		t.Run(it.name, func(t *testing.T) {
			ok, err := it.next(context.Background())
			if err != nil {
				t.Fatalf("unexpected iterator error: %v", err)
			}
			if ok {
				t.Fatal("expected no items")
			}
		})
	}
}
