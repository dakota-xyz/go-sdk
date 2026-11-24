package webhook_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/log"
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
		"WithPublicKey listed again in WithPublicKeys": {
			webhook.WithPublicKey(oldKey.hex),
			webhook.WithPublicKeys(oldKey.hex, newKey.hex),
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

// The documented change for existing WithPublicKey handlers: a present but
// failing X-Dakota-Webhook-Signatures is rejected even when
// X-Webhook-Signature is valid.
func TestHandler_WithPublicKeyNoFallbackWhenSignaturesHeaderFails(t *testing.T) {
	key, stranger := newRotationKey(t), newRotationKey(t)
	handler, err := webhook.NewHandler(webhook.WithPublicKey(key.hex))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	if got := serve(t, handler, rotationRequest(t, rotationBody, &key, stranger)); got != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", got, http.StatusUnauthorized)
	}
}

// logRecord is one log line written through the handler's logger.
type logRecord struct {
	level slog.Level
	text  string // the message followed by every attribute, as key=value
}

// recordingHandler is a slog.Handler that keeps every record it handles,
// with any attributes added through With.
type recordingHandler struct {
	mu      *sync.Mutex
	records *[]logRecord
	attrs   []slog.Attr
}

func (h recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h recordingHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	for _, a := range h.attrs {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
	}
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, logRecord{level: r.Level, text: b.String()})
	return nil
}

func (h recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return h
}
func (h recordingHandler) WithGroup(string) slog.Handler { return h }

// recordLogs returns a handler option that logs to a recorder, and a function
// returning what was logged so far.
func recordLogs() (webhook.HandlerOption, func() []logRecord) {
	var mu sync.Mutex
	var records []logRecord
	logger := log.FromSlog(slog.New(recordingHandler{mu: &mu, records: &records}))
	return webhook.WithLogger(logger), func() []logRecord {
		mu.Lock()
		defer mu.Unlock()
		return append([]logRecord(nil), records...)
	}
}

func warnings(records []logRecord) []string {
	var out []string
	for _, r := range records {
		if r.level == slog.LevelWarn {
			out = append(out, r.text)
		}
	}
	return out
}

// Once WithPublicKeys has a key, a key given only through WithPublicKey is not
// trusted: an integrator who removes a retired key from the list stops
// trusting it even if an old WithPublicKey setting still names it.
func TestHandler_WithPublicKeysOverridesWithPublicKey(t *testing.T) {
	retired, current := newRotationKey(t), newRotationKey(t)
	orders := map[string][]webhook.HandlerOption{
		"WithPublicKey first":  {webhook.WithPublicKey(retired.hex), webhook.WithPublicKeys(current.hex)},
		"WithPublicKeys first": {webhook.WithPublicKeys(current.hex), webhook.WithPublicKey(retired.hex)},
	}
	for name, opts := range orders {
		t.Run(name, func(t *testing.T) {
			testWithPublicKeysOverrides(t, retired, current, opts)
		})
	}
}

func testWithPublicKeysOverrides(
	t *testing.T,
	retired, current rotationKey,
	opts []webhook.HandlerOption,
) {
	logOpt, logs := recordLogs()
	handler, err := webhook.NewHandler(append(opts, logOpt)...)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	want := "Dakota webhook handler: ignoring the WithPublicKey option (key id " +
		retired.id + ") because WithPublicKeys is set; " +
		"add that key to WithPublicKeys if you still need it"
	// Logged by NewHandler, before any delivery.
	if got := warnings(logs()); len(got) != 1 || got[0] != want {
		t.Fatalf("warnings after NewHandler:\n got %q\nwant [%q]", got, want)
	}

	deliveries := []struct {
		name    string
		legacy  *rotationKey
		signers []rotationKey
		want    int
	}{
		{"retired key, X-Webhook-Signature only", &retired, nil, http.StatusUnauthorized},
		{"retired key, X-Dakota-Webhook-Signatures only", nil, []rotationKey{retired}, http.StatusUnauthorized},
		{"retired key, both headers", &retired, []rotationKey{retired}, http.StatusUnauthorized},
		{"current key, X-Webhook-Signature only", &current, nil, http.StatusOK},
		{"current key, X-Dakota-Webhook-Signatures only", nil, []rotationKey{current}, http.StatusOK},
	}
	for _, d := range deliveries {
		t.Run(d.name, func(t *testing.T) {
			req := rotationRequest(t, rotationBody, d.legacy, d.signers...)
			if got := serve(t, handler, req); got != d.want {
				t.Errorf("got status %d, want %d", got, d.want)
			}
		})
	}

	got := warnings(logs())
	configWarnings := 0
	for _, w := range got {
		if strings.Contains(w, retired.hex) {
			t.Errorf("a log line contains the full WithPublicKey key: %q", w)
		}
		if strings.HasPrefix(w, "Dakota webhook handler:") {
			configWarnings++
			if w != want {
				t.Errorf("warning:\n got %q\nwant %q", w, want)
			}
		}
	}
	if configWarnings != 1 {
		t.Errorf("got %d configuration warnings, want exactly 1: %q", configWarnings, got)
	}
}

// A WithPublicKey key that WithPublicKeys also lists is still trusted, and
// nothing is ignored, so there is nothing to warn about. Neither is there
// without WithPublicKey.
func TestHandler_NoConfigWarning(t *testing.T) {
	key, other := newRotationKey(t), newRotationKey(t)
	configs := map[string][]webhook.HandlerOption{
		"WithPublicKey listed in WithPublicKeys": {
			webhook.WithPublicKey(key.hex),
			webhook.WithPublicKeys(other.hex + ", " + strings.ToUpper(key.hex)),
		},
		"WithPublicKeys only":                     {webhook.WithPublicKeys(key.hex, other.hex)},
		"WithPublicKey only":                      {webhook.WithPublicKey(key.hex)},
		"WithPublicKey plus empty WithPublicKeys": {webhook.WithPublicKey(key.hex), webhook.WithPublicKeys("")},
	}
	for name, opts := range configs {
		t.Run(name, func(t *testing.T) {
			logOpt, logs := recordLogs()
			handler, err := webhook.NewHandler(append(opts, logOpt)...)
			if err != nil {
				t.Fatalf("NewHandler: %v", err)
			}
			if got := serve(t, handler, rotationRequest(t, rotationBody, &key, key)); got != http.StatusOK {
				t.Errorf("got status %d, want %d", got, http.StatusOK)
			}
			if got := warnings(logs()); len(got) != 0 {
				t.Errorf("got warnings %q, want none", got)
			}
		})
	}
}

// An empty WithPublicKeys (an unset environment variable) leaves
// WithPublicKey in charge, exactly as before: its key is trusted through both
// headers and no other key is.
func TestHandler_EmptyWithPublicKeysFallsBackToWithPublicKey(t *testing.T) {
	key, stranger := newRotationKey(t), newRotationKey(t)
	for _, list := range []string{"", " , "} {
		handler, err := webhook.NewHandler(
			webhook.WithPublicKey(key.hex),
			webhook.WithPublicKeys(list),
		)
		if err != nil {
			t.Fatalf("list %q: NewHandler: %v", list, err)
		}
		cases := []struct {
			name    string
			legacy  *rotationKey
			signers []rotationKey
			want    int
		}{
			{"X-Webhook-Signature", &key, nil, http.StatusOK},
			{"X-Dakota-Webhook-Signatures", nil, []rotationKey{key}, http.StatusOK},
			{"another key", &stranger, []rotationKey{stranger}, http.StatusUnauthorized},
		}
		for _, c := range cases {
			req := rotationRequest(t, rotationBody, c.legacy, c.signers...)
			if got := serve(t, handler, req); got != c.want {
				t.Errorf("list %q, %s: got status %d, want %d", list, c.name, got, c.want)
			}
		}
	}
}

// A malformed WithPublicKey key is ignored, with a warning that never quotes
// it, when WithPublicKeys has a key, as dakota-ts-sdk does. On its own it
// still fails NewHandler.
func TestHandler_MalformedWithPublicKey(t *testing.T) {
	key := newRotationKey(t)
	// Looks like a private key pasted by mistake: 64 bytes of hex.
	malformed := strings.Repeat("ab", 64)

	t.Run("with WithPublicKeys", func(t *testing.T) {
		logOpt, logs := recordLogs()
		handler, err := webhook.NewHandler(
			webhook.WithPublicKey(malformed),
			webhook.WithPublicKeys(key.hex),
			logOpt,
		)
		if err != nil {
			t.Fatalf("NewHandler: %v", err)
		}
		if got := serve(t, handler, rotationRequest(t, rotationBody, &key, key)); got != http.StatusOK {
			t.Errorf("got status %d, want %d", got, http.StatusOK)
		}
		want := "Dakota webhook handler: ignoring the WithPublicKey option " +
			"(the WithPublicKey key is not 64 hex characters); " +
			"verifying with the 1 key(s) in WithPublicKeys"
		got := warnings(logs())
		if len(got) != 1 || got[0] != want {
			t.Errorf("warnings:\n got %q\nwant [%q]", got, want)
		}
		for _, w := range got {
			if strings.Contains(w, "abab") {
				t.Errorf("warning quotes the WithPublicKey value: %q", w)
			}
		}
	})

	t.Run("not hex, with WithPublicKeys", func(t *testing.T) {
		logOpt, logs := recordLogs()
		_, err := webhook.NewHandler(
			webhook.WithPublicKey("zz-secret-zz"),
			webhook.WithPublicKeys(key.hex),
			logOpt,
		)
		if err != nil {
			t.Fatalf("NewHandler: %v", err)
		}
		got := warnings(logs())
		if len(got) != 1 || strings.Contains(got[0], "zz") || strings.Contains(got[0], "secret") {
			t.Errorf("want one warning that does not quote the value, got %q", got)
		}
	})

	for name, opts := range map[string][]webhook.HandlerOption{
		"alone":                     {webhook.WithPublicKey(malformed)},
		"with empty WithPublicKeys": {webhook.WithPublicKey(malformed), webhook.WithPublicKeys("")},
		"not hex, with empty list":  {webhook.WithPublicKey("not-hex"), webhook.WithPublicKeys()},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := webhook.NewHandler(opts...)
			if !errors.Is(err, errors.ErrInvalidConfig) {
				t.Errorf("got %v, want ErrInvalidConfig", err)
			}
		})
	}
}

// A configuration that NewHandler rejects logs nothing about ignoring
// WithPublicKey: there is no handler for the warning to describe.
func TestHandler_NoConfigWarningWhenNewHandlerFails(t *testing.T) {
	retired, current := newRotationKey(t), newRotationKey(t)
	logOpt, logs := recordLogs()
	_, err := webhook.NewHandler(
		webhook.WithPublicKey(retired.hex),
		webhook.WithPublicKeys(current.hex),
		webhook.WithChannel(1),
		webhook.OnDefault(func(context.Context, webhook.Event) error { return nil }),
		logOpt,
	)
	if !errors.Is(err, errors.ErrInvalidConfig) {
		t.Fatalf("got %v, want ErrInvalidConfig", err)
	}
	if got := warnings(logs()); len(got) != 0 {
		t.Errorf("got warnings %q, want none", got)
	}
}
