package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	sdkerrors "github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/log"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

// AuthMode controls how authentication headers are injected.
type AuthMode string

const (
	// AuthModeAuto picks the best available auth header for the request.
	//
	// Priority:
	//  1. Existing auth headers already present on the request.
	//  2. X-Application-Token for /applications routes (if configured).
	//  3. x-api-key (if configured).
	//  4. X-Application-Token as fallback (if configured).
	AuthModeAuto AuthMode = "auto"
	// AuthModeAPIKey only injects x-api-key.
	AuthModeAPIKey AuthMode = "api_key"
	// AuthModeApplicationToken only injects X-Application-Token.
	AuthModeApplicationToken AuthMode = "application_token"
)

// RetryPolicy controls automatic retry behavior for outbound requests.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts including the initial call.
	MaxAttempts int
	// InitialBackoff is the first backoff duration before retrying.
	InitialBackoff time.Duration
	// MaxBackoff caps exponential backoff growth.
	MaxBackoff time.Duration
}

// DefaultRetryPolicy returns a conservative retry policy suitable for production.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
	}
}

// IdempotencyKeyGenerator creates an idempotency key for POST requests.
type IdempotencyKeyGenerator func() (string, error)

// Option configures an SDK API client.
type Option func(*config) error

type config struct {
	environment        Environment
	baseURL            string
	apiKey             string
	applicationToken   string
	authMode           AuthMode
	httpClient         *http.Client
	timeout            time.Duration
	retryPolicy        RetryPolicy
	logger             log.Logger
	userAgent          string
	autoIdempotency    bool
	idempotencyKeyGen  IdempotencyKeyGenerator
	redactedHeaders    map[string]struct{}
	requestEditors     []gen.RequestEditorFn
	logRequestHeaders  bool
	logResponseHeaders bool
}

func defaultConfig() config {
	redactions := make(map[string]struct{}, len(defaultSensitiveHeaders))
	for k := range defaultSensitiveHeaders {
		redactions[k] = struct{}{}
	}

	return config{
		environment:       EnvironmentSandbox,
		authMode:          AuthModeAuto,
		timeout:           15 * time.Second,
		retryPolicy:       DefaultRetryPolicy(),
		logger:            log.Nop(),
		userAgent:         "dakota-go-sdk",
		autoIdempotency:   true,
		idempotencyKeyGen: generateUUIDv4,
		redactedHeaders:   redactions,
	}
}

func (c *config) validate() error {
	if c.timeout <= 0 {
		return sdkerrors.New(
			sdkerrors.CodeInvalidConfig,
			"timeout must be greater than zero",
		)
	}
	if c.retryPolicy.MaxAttempts <= 0 {
		return sdkerrors.New(
			sdkerrors.CodeInvalidConfig,
			"retry max attempts must be greater than zero",
		)
	}
	if c.retryPolicy.InitialBackoff <= 0 {
		return sdkerrors.New(
			sdkerrors.CodeInvalidConfig,
			"retry initial backoff must be greater than zero",
		)
	}
	if c.retryPolicy.MaxBackoff <= 0 {
		return sdkerrors.New(
			sdkerrors.CodeInvalidConfig,
			"retry max backoff must be greater than zero",
		)
	}
	if c.retryPolicy.MaxBackoff < c.retryPolicy.InitialBackoff {
		return sdkerrors.New(
			sdkerrors.CodeInvalidConfig,
			"retry max backoff must be >= initial backoff",
		)
	}
	if c.logger == nil {
		c.logger = log.Nop()
	}
	if c.userAgent == "" {
		c.userAgent = "dakota-go-sdk"
	}
	if c.idempotencyKeyGen == nil {
		c.idempotencyKeyGen = generateUUIDv4
	}
	if c.redactedHeaders == nil {
		c.redactedHeaders = map[string]struct{}{}
	}

	switch c.authMode {
	case AuthModeAuto:
		if c.apiKey == "" && c.applicationToken == "" {
			return sdkerrors.New(
				sdkerrors.CodeInvalidConfig,
				"either API key or application token is required",
			)
		}
	case AuthModeAPIKey:
		if c.apiKey == "" {
			return sdkerrors.New(
				sdkerrors.CodeInvalidConfig,
				"auth mode api_key requires an API key",
			)
		}
	case AuthModeApplicationToken:
		if c.applicationToken == "" {
			return sdkerrors.New(
				sdkerrors.CodeInvalidConfig,
				"auth mode application_token requires an application token",
			)
		}
	default:
		return sdkerrors.New(
			sdkerrors.CodeInvalidConfig,
			fmt.Sprintf("invalid auth mode %q", c.authMode),
		)
	}

	if c.baseURL != "" {
		u, err := url.Parse(c.baseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return sdkerrors.New(
				sdkerrors.CodeInvalidConfig,
				"base URL must be a valid absolute URL",
			)
		}
	}

	return nil
}

func (c config) resolveBaseURL() (string, error) {
	if c.baseURL != "" {
		return strings.TrimRight(c.baseURL, "/"), nil
	}
	baseURL, err := c.environment.BaseURL()
	if err != nil {
		return "", sdkerrors.Wrap(
			sdkerrors.CodeInvalidConfig,
			"resolve environment base URL",
			err,
		)
	}
	return strings.TrimRight(baseURL, "/"), nil
}

// WithEnvironment configures the target environment.
func WithEnvironment(env Environment) Option {
	return func(c *config) error {
		c.environment = env
		return nil
	}
}

// WithBaseURL overrides environment-based URL resolution.
func WithBaseURL(baseURL string) Option {
	return func(c *config) error {
		c.baseURL = baseURL
		return nil
	}
}

// WithAPIKey configures x-api-key authentication.
func WithAPIKey(apiKey string) Option {
	return func(c *config) error {
		c.apiKey = apiKey
		return nil
	}
}

// WithApplicationToken configures X-Application-Token authentication.
func WithApplicationToken(token string) Option {
	return func(c *config) error {
		c.applicationToken = token
		return nil
	}
}

// WithAuthMode configures header injection behavior.
func WithAuthMode(mode AuthMode) Option {
	return func(c *config) error {
		c.authMode = mode
		return nil
	}
}

// WithHTTPClient sets a base HTTP client for outbound requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *config) error {
		c.httpClient = httpClient
		return nil
	}
}

// WithTimeout sets the outbound HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *config) error {
		c.timeout = timeout
		return nil
	}
}

// WithRetryPolicy configures automatic retries.
func WithRetryPolicy(policy RetryPolicy) Option {
	return func(c *config) error {
		c.retryPolicy = policy
		return nil
	}
}

// WithLogger configures structured logging for the client.
func WithLogger(logger log.Logger) Option {
	return func(c *config) error {
		c.logger = logger
		return nil
	}
}

// WithUserAgent sets the User-Agent header for outbound requests.
func WithUserAgent(userAgent string) Option {
	return func(c *config) error {
		c.userAgent = userAgent
		return nil
	}
}

// WithAutomaticIdempotency enables/disables automatic POST idempotency keys.
func WithAutomaticIdempotency(enabled bool) Option {
	return func(c *config) error {
		c.autoIdempotency = enabled
		return nil
	}
}

// WithIdempotencyKeyGenerator overrides automatic idempotency key generation.
func WithIdempotencyKeyGenerator(genFn IdempotencyKeyGenerator) Option {
	return func(c *config) error {
		c.idempotencyKeyGen = genFn
		return nil
	}
}

// WithRedactedHeaders adds additional headers to redact in client logs.
func WithRedactedHeaders(headerNames ...string) Option {
	return func(c *config) error {
		if c.redactedHeaders == nil {
			c.redactedHeaders = map[string]struct{}{}
		}
		for _, h := range headerNames {
			h = strings.ToLower(strings.TrimSpace(h))
			if h == "" {
				continue
			}
			c.redactedHeaders[h] = struct{}{}
		}
		return nil
	}
}

// WithRequestEditor adds a generated client request editor.
func WithRequestEditor(editor gen.RequestEditorFn) Option {
	return func(c *config) error {
		if editor != nil {
			c.requestEditors = append(c.requestEditors, editor)
		}
		return nil
	}
}

// WithHeaderLogging enables request/response header logging (with redaction).
func WithHeaderLogging(requestHeaders bool, responseHeaders bool) Option {
	return func(c *config) error {
		c.logRequestHeaders = requestHeaders
		c.logResponseHeaders = responseHeaders
		return nil
	}
}

// Client is the high-level Dakota Platform API client.
type Client struct {
	api        *gen.ClientWithResponses
	httpClient *http.Client
	baseURL    string
	logger     log.Logger
}

// New constructs a client with production-ready defaults.
func New(opts ...Option) (*Client, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	baseURL, err := cfg.resolveBaseURL()
	if err != nil {
		return nil, err
	}

	httpClient := buildHTTPClient(cfg)

	genOpts := []gen.ClientOption{gen.WithHTTPClient(httpClient)}
	for _, editor := range cfg.requestEditors {
		genOpts = append(genOpts, gen.WithRequestEditorFn(editor))
	}

	api, err := gen.NewClientWithResponses(baseURL, genOpts...)
	if err != nil {
		return nil, sdkerrors.Wrap(
			sdkerrors.CodeInternal,
			"create generated API client",
			err,
		)
	}

	return &Client{
		api:        api,
		httpClient: httpClient,
		baseURL:    baseURL,
		logger:     cfg.logger,
	}, nil
}

// Raw returns the typed generated OpenAPI client.
func (c *Client) Raw() *gen.ClientWithResponses {
	return c.api
}

// HTTPClient returns the underlying configured HTTP client.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// BaseURL returns the resolved API base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Logger returns the client logger.
func (c *Client) Logger() log.Logger {
	return c.logger
}

func buildHTTPClient(cfg config) *http.Client {
	baseClient := cfg.httpClient
	if baseClient == nil {
		baseClient = &http.Client{}
	}

	clientCopy := *baseClient
	if clientCopy.Timeout <= 0 {
		clientCopy.Timeout = cfg.timeout
	}

	baseRT := clientCopy.Transport
	if baseRT == nil {
		baseRT = http.DefaultTransport
	}

	clientCopy.Transport = buildTransport(baseRT, cfg)
	return &clientCopy
}
