package client

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	sdkerrors "github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/log"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

const redactedValue = "***REDACTED***"

var defaultSensitiveHeaders = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"x-api-key":           {},
	"x-application-token": {},
	"cookie":              {},
	"set-cookie":          {},
}

func buildTransport(base http.RoundTripper, cfg config) http.RoundTripper {
	rt := base
	rt = &authTransport{
		next:             rt,
		apiKey:           cfg.apiKey,
		applicationToken: cfg.applicationToken,
		authMode:         cfg.authMode,
		userAgent:        cfg.userAgent,
	}
	rt = &retryTransport{
		next:   rt,
		policy: cfg.retryPolicy,
		logger: cfg.logger,
	}
	if cfg.autoIdempotency {
		rt = &idempotencyTransport{
			next:      rt,
			generator: cfg.idempotencyKeyGen,
		}
	}
	rt = &loggingTransport{
		next:               rt,
		logger:             cfg.logger,
		redactedHeaders:    cfg.redactedHeaders,
		logRequestHeaders:  cfg.logRequestHeaders,
		logResponseHeaders: cfg.logResponseHeaders,
	}

	return rt
}

type authTransport struct {
	next             http.RoundTripper
	apiKey           string
	applicationToken string
	authMode         AuthMode
	userAgent        string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := cloneRequestMetadata(req)

	if t.userAgent != "" && cloned.Header.Get("User-Agent") == "" {
		cloned.Header.Set("User-Agent", t.userAgent)
	}

	if cloned.Header.Get("x-api-key") == "" &&
		cloned.Header.Get("X-Application-Token") == "" {
		switch t.authMode {
		case AuthModeAPIKey:
			if t.apiKey != "" {
				cloned.Header.Set("x-api-key", t.apiKey)
			}
		case AuthModeApplicationToken:
			if t.applicationToken != "" {
				cloned.Header.Set("X-Application-Token", t.applicationToken)
			}
		case AuthModeAuto:
			if isApplicationsPath(cloned.URL.Path) && t.applicationToken != "" {
				cloned.Header.Set("X-Application-Token", t.applicationToken)
			} else if t.apiKey != "" {
				cloned.Header.Set("x-api-key", t.apiKey)
			} else if t.applicationToken != "" {
				cloned.Header.Set("X-Application-Token", t.applicationToken)
			}
		}
	}

	return t.next.RoundTrip(cloned)
}

func isApplicationsPath(path string) bool {
	return path == "/applications" || strings.HasPrefix(path, "/applications/")
}

// WithIdempotencyKey pins a specific idempotency key on a request context.
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, idempotencyKeyContextKey{}, key)
}

// RequestEditorWithIdempotencyKey applies a specific idempotency key.
func RequestEditorWithIdempotencyKey(key string) gen.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("x-idempotency-key", key)
		return nil
	}
}

type idempotencyKeyContextKey struct{}

// idempotencyTransport auto-injects an x-idempotency-key header on mutating
// requests (POST/PUT/PATCH/DELETE) that don't already carry one. The platform
// declares the header on create/update/delete operations across all four
// methods, so keying it per-method — rather than per-operation — covers them
// all; an operation that doesn't declare the header simply ignores an extra one.
type idempotencyTransport struct {
	next      http.RoundTripper
	generator IdempotencyKeyGenerator
}

func (t *idempotencyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !methodTakesIdempotencyKey(req.Method) {
		return t.next.RoundTrip(req)
	}
	if req.Header.Get("x-idempotency-key") != "" {
		return t.next.RoundTrip(req)
	}

	cloned := cloneRequestMetadata(req)
	if key, ok := req.Context().Value(idempotencyKeyContextKey{}).(string); ok && key != "" {
		cloned.Header.Set("x-idempotency-key", key)
		return t.next.RoundTrip(cloned)
	}

	key, err := t.generator()
	if err != nil {
		return nil, sdkerrors.Wrap(
			sdkerrors.CodeInternal,
			"generate idempotency key",
			err,
		)
	}
	cloned.Header.Set("x-idempotency-key", key)
	return t.next.RoundTrip(cloned)
}

// methodTakesIdempotencyKey reports whether the platform may require an
// x-idempotency-key on this HTTP method. The API declares the header on
// create/update/delete operations across POST, PUT, PATCH and DELETE (their
// generated *Params carry an XIdempotencyKey field); GET/HEAD and friends never
// take it. Injecting per-method is safe even for an operation that doesn't
// declare the header: the generated server binds only declared params, so an
// extra header is dropped — the same reason blind POST injection has always
// been safe.
func methodTakesIdempotencyKey(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

type retryTransport struct {
	next   http.RoundTripper
	policy RetryPolicy
	logger log.Logger
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(req)
	if !t.shouldRetry(req, resp, err) {
		return resp, err
	}

	if !requestIsReplayable(req) {
		return resp, err
	}

	if resp != nil && resp.Body != nil {
		drainAndClose(resp.Body)
	}

	for attempt := 2; attempt <= t.policy.MaxAttempts; attempt++ {
		delay := t.computeDelay(attempt-1, resp)
		if waitErr := sleepWithContext(req.Context(), delay); waitErr != nil {
			return nil, waitErr
		}

		retryReq, cloneErr := cloneRequestWithBody(req)
		if cloneErr != nil {
			return nil, cloneErr
		}

		t.logger.Warn(
			req.Context(),
			"retrying request",
			log.String("method", req.Method),
			log.String("path", req.URL.Path),
			log.Int("attempt", attempt),
			log.Int("max_attempts", t.policy.MaxAttempts),
			log.Int64("delay_ms", delay.Milliseconds()),
		)

		resp, err = t.next.RoundTrip(retryReq)
		if !t.shouldRetry(req, resp, err) || attempt == t.policy.MaxAttempts {
			return resp, err
		}
		if resp != nil && resp.Body != nil {
			drainAndClose(resp.Body)
		}
	}

	return resp, err
}

func (t *retryTransport) shouldRetry(
	req *http.Request,
	resp *http.Response,
	err error,
) bool {
	if t.policy.MaxAttempts <= 1 {
		return false
	}
	if !methodIsRetryable(req.Method, req.Header.Get("x-idempotency-key") != "") {
		return false
	}
	if err != nil {
		if req.Context().Err() != nil {
			return false
		}
		return true
	}
	if resp == nil {
		return false
	}

	return isRetryableStatus(resp.StatusCode)
}

func methodIsRetryable(method string, hasIdempotencyKey bool) bool {
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPut,
		http.MethodDelete,
		http.MethodOptions,
		http.MethodTrace:
		return true
	case http.MethodPost:
		return hasIdempotencyKey
	default:
		return false
	}
}

func requestIsReplayable(req *http.Request) bool {
	if req.Body == nil || req.Body == http.NoBody {
		return true
	}
	return req.GetBody != nil
}

func (t *retryTransport) computeDelay(retryNumber int, resp *http.Response) time.Duration {
	if d, ok := retryAfterDelay(resp); ok {
		if d > t.policy.MaxBackoff {
			return t.policy.MaxBackoff
		}
		return d
	}

	exponent := math.Pow(2, float64(retryNumber-1))
	backoff := time.Duration(float64(t.policy.InitialBackoff) * exponent)
	if backoff > t.policy.MaxBackoff {
		backoff = t.policy.MaxBackoff
	}
	return withJitter(backoff)
}

func retryAfterDelay(resp *http.Response) (time.Duration, bool) {
	if resp == nil {
		return 0, false
	}
	value := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if value == "" {
		return 0, false
	}

	seconds, err := strconv.Atoi(value)
	if err == nil {
		if seconds <= 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}

	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	delta := time.Until(when)
	if delta <= 0 {
		return 0, false
	}
	return delta, true
}

func withJitter(base time.Duration) time.Duration {
	fraction, err := cryptoRandomUnitFloat64()
	if err != nil {
		return base
	}
	factor := 0.8 + (fraction * 0.4)
	return time.Duration(float64(base) * factor)
}

func cryptoRandomUnitFloat64() (float64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	// Convert to IEEE754-compatible [0,1) fraction using 53 random bits.
	v := binary.BigEndian.Uint64(b[:]) >> 11
	return float64(v) / float64(uint64(1)<<53), nil
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func cloneRequestMetadata(req *http.Request) *http.Request {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	return cloned
}

func cloneRequestWithBody(req *http.Request) (*http.Request, error) {
	cloned := cloneRequestMetadata(req)
	if req.Body == nil || req.Body == http.NoBody {
		return cloned, nil
	}
	if req.GetBody == nil {
		return nil, sdkerrors.New(
			sdkerrors.CodeInternal,
			"request body is not replayable",
		)
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, sdkerrors.Wrap(
			sdkerrors.CodeInternal,
			"clone request body",
			err,
		)
	}
	cloned.Body = body
	return cloned, nil
}

func drainAndClose(rc io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(rc, 8192))
	_ = rc.Close()
}

type loggingTransport struct {
	next               http.RoundTripper
	logger             log.Logger
	redactedHeaders    map[string]struct{}
	logRequestHeaders  bool
	logResponseHeaders bool
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	startedAt := time.Now()
	resp, err := t.next.RoundTrip(req)
	duration := time.Since(startedAt)

	attrs := []slog.Attr{
		log.String("method", req.Method),
		log.String("path", req.URL.Path),
		log.Int64("duration_ms", duration.Milliseconds()),
	}

	if t.logRequestHeaders {
		attrs = append(
			attrs,
			slog.Any("request_headers", sanitizeHeaders(req.Header, t.redactedHeaders)),
		)
	}

	if err != nil {
		attrs = append(attrs, log.Err(err))
		t.logger.Error(req.Context(), "platform API request failed", attrs...)
		return nil, err
	}

	if resp != nil {
		attrs = append(attrs, log.Int("status_code", resp.StatusCode))
		if requestID := resp.Header.Get("X-Request-Id"); requestID != "" {
			attrs = append(attrs, log.String("request_id", requestID))
		}
		if t.logResponseHeaders {
			attrs = append(
				attrs,
				slog.Any("response_headers", sanitizeHeaders(resp.Header, t.redactedHeaders)),
			)
		}
	}

	if resp != nil && resp.StatusCode >= 500 {
		t.logger.Warn(req.Context(), "platform API request completed", attrs...)
		return resp, nil
	}

	t.logger.Info(req.Context(), "platform API request completed", attrs...)
	return resp, nil
}

func sanitizeHeaders(
	headers http.Header,
	additionalRedactions map[string]struct{},
) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}

	out := make(map[string]string, len(headers))
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if headerIsSensitive(lowerKey, additionalRedactions) {
			out[key] = redactedValue
			continue
		}
		out[key] = strings.Join(values, ",")
	}
	return out
}

func headerIsSensitive(
	key string,
	additionalRedactions map[string]struct{},
) bool {
	if _, ok := defaultSensitiveHeaders[key]; ok {
		return true
	}
	if _, ok := additionalRedactions[key]; ok {
		return true
	}
	return strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "authorization")
}

func generateUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	// RFC 4122 variant + version bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16],
	), nil
}
