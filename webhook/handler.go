package webhook

import (
	"context"
	"crypto/ed25519"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/log"
	"github.com/dakota-xyz/go-sdk/webhook/idempotency"
)

const defaultMaxPayloadSize = 64 * 1024 // 64 KB

// EventHandler is a callback for processing a webhook event.
type EventHandler func(ctx context.Context, event Event) error

// HandlerOption configures a Handler.
type HandlerOption func(*handlerConfig)

type deliveryMode int

const (
	deliveryCallback deliveryMode = iota
	deliveryChannel
)

// AckPolicy controls when the handler acknowledges webhook delivery (2xx).
type AckPolicy int

const (
	// AckOnSuccess only acknowledges (2xx) when delivery succeeds.
	AckOnSuccess AckPolicy = iota

	// AckAlways always acknowledges (2xx), even if delivery fails.
	AckAlways
)

type handlerConfig struct {
	publicKeyHex   string
	tolerance      time.Duration
	maxPayloadSize int64
	ackPolicy      AckPolicy
	logger         log.Logger
	store          idempotency.Store
	handlers       map[EventType]EventHandler
	defaultHandler EventHandler
	channelBuf     int
	useChannel     bool
	hasCallbacks   bool
}

// WithPublicKey sets the hex-encoded Ed25519 public key used for signature
// verification. This option is required.
func WithPublicKey(hexKey string) HandlerOption {
	return func(c *handlerConfig) {
		c.publicKeyHex = hexKey
	}
}

// WithHandlerTolerance sets the timestamp tolerance for replay protection.
func WithHandlerTolerance(d time.Duration) HandlerOption {
	return func(c *handlerConfig) {
		c.tolerance = d
	}
}

// WithMaxPayloadSize sets the maximum request body size in bytes.
func WithMaxPayloadSize(n int64) HandlerOption {
	return func(c *handlerConfig) {
		c.maxPayloadSize = n
	}
}

// WithAckPolicy controls how webhook delivery is acknowledged.
func WithAckPolicy(policy AckPolicy) HandlerOption {
	return func(c *handlerConfig) {
		c.ackPolicy = policy
	}
}

// WithLogger sets the logger for the handler.
func WithLogger(l log.Logger) HandlerOption {
	return func(c *handlerConfig) {
		c.logger = l
	}
}

// WithIdempotencyStore sets the store for deduplicating events.
func WithIdempotencyStore(s idempotency.Store) HandlerOption {
	return func(c *handlerConfig) {
		c.store = s
	}
}

// On registers a callback for a specific event type.
func On(eventType EventType, handler EventHandler) HandlerOption {
	return func(c *handlerConfig) {
		c.handlers[eventType] = handler
		c.hasCallbacks = true
	}
}

// OnDefault registers a fallback callback for event types without a specific
// handler.
func OnDefault(handler EventHandler) HandlerOption {
	return func(c *handlerConfig) {
		c.defaultHandler = handler
		c.hasCallbacks = true
	}
}

// WithChannel configures the handler to deliver events via a channel instead
// of callbacks. Cannot be combined with On or OnDefault.
func WithChannel(bufferSize int) HandlerOption {
	return func(c *handlerConfig) {
		c.channelBuf = bufferSize
		c.useChannel = true
	}
}

// Handler is an http.Handler that verifies, parses, and dispatches webhook
// events.
type Handler struct {
	publicKey      ed25519.PublicKey
	tolerance      time.Duration
	maxPayloadSize int64
	ackPolicy      AckPolicy
	logger         log.Logger
	store          idempotency.Store
	mode           deliveryMode
	handlers       map[EventType]EventHandler
	defaultHandler EventHandler
	eventsMu       sync.RWMutex
	events         chan Event
	closeOnce      sync.Once
}

// NewHandler creates a new webhook Handler.
//
// Returns an error if the public key is missing or if both callback and channel
// delivery modes are configured.
func NewHandler(opts ...HandlerOption) (*Handler, error) {
	cfg := &handlerConfig{
		tolerance:      DefaultTimestampTolerance,
		maxPayloadSize: defaultMaxPayloadSize,
		ackPolicy:      AckOnSuccess,
		logger:         log.Nop(),
		handlers:       make(map[EventType]EventHandler),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.publicKeyHex == "" {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"public key is required",
		)
	}
	if cfg.tolerance <= 0 {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"timestamp tolerance must be greater than zero",
		)
	}
	if cfg.maxPayloadSize <= 0 {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"max payload size must be greater than zero",
		)
	}
	if cfg.ackPolicy != AckOnSuccess && cfg.ackPolicy != AckAlways {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"invalid ack policy",
		)
	}
	if cfg.logger == nil {
		cfg.logger = log.Nop()
	}

	// Parse and validate public key eagerly.
	pubKey, err := ParsePublicKey(cfg.publicKeyHex)
	if err != nil {
		return nil, err
	}

	if cfg.useChannel && cfg.hasCallbacks {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"cannot use both channel and callback delivery modes",
		)
	}
	if cfg.useChannel && cfg.channelBuf < 0 {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"channel buffer size must be >= 0",
		)
	}

	h := &Handler{
		publicKey:      pubKey,
		tolerance:      cfg.tolerance,
		maxPayloadSize: cfg.maxPayloadSize,
		ackPolicy:      cfg.ackPolicy,
		logger:         cfg.logger,
		store:          cfg.store,
		handlers:       cfg.handlers,
		defaultHandler: cfg.defaultHandler,
	}

	if cfg.useChannel {
		h.mode = deliveryChannel
		h.events = make(chan Event, cfg.channelBuf)
	} else {
		h.mode = deliveryCallback
	}

	return h, nil
}

// Events returns the event channel. Returns nil if the handler is not in
// channel delivery mode.
func (h *Handler) Events() <-chan Event {
	h.eventsMu.RLock()
	defer h.eventsMu.RUnlock()
	return h.events
}

// Close closes the event channel if in channel mode.
func (h *Handler) Close() {
	h.closeOnce.Do(
		func() {
			h.eventsMu.Lock()
			defer h.eventsMu.Unlock()
			if h.events != nil {
				close(h.events)
				h.events = nil
			}
		},
	)
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Reject non-POST.
	if r.Method != http.MethodPost {
		h.logger.Warn(
			ctx,
			"rejected non-POST request",
			log.String("method", r.Method),
		)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. Read body with size limit.
	body, err := io.ReadAll(io.LimitReader(r.Body, h.maxPayloadSize+1))
	if err != nil {
		h.logger.Error(ctx, "failed to read request body", log.Err(err))
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > h.maxPayloadSize {
		h.logger.Warn(
			ctx,
			"payload too large",
			log.Int64("size", int64(len(body))),
		)
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	// 3. Extract headers.
	sigHeader := r.Header.Get(SignatureHeader)
	if sigHeader == "" {
		h.logger.Warn(ctx, "missing signature header")
		http.Error(w, "missing signature header", http.StatusBadRequest)
		return
	}
	tsHeader := r.Header.Get(TimestampHeader)
	if tsHeader == "" {
		h.logger.Warn(ctx, "missing timestamp header")
		http.Error(w, "missing timestamp header", http.StatusBadRequest)
		return
	}

	// 4. Verify signature.
	if err := verifySignatureWithPublicKey(
		body,
		sigHeader,
		tsHeader,
		h.publicKey,
	); err != nil {
		h.logger.Warn(ctx, "signature verification failed", log.Err(err))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// 5. Validate timestamp.
	if err := ValidateTimestamp(tsHeader, h.tolerance); err != nil {
		h.logger.Warn(ctx, "timestamp validation failed", log.Err(err))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// 6. Parse event.
	event, err := ParseEvent(body)
	if err != nil {
		h.logger.Warn(ctx, "event parse failed", log.Err(err))
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// 7. Idempotency pre-check.
	if h.store != nil {
		acquired, err := h.store.Acquire(ctx, event.ID)
		if err != nil {
			h.logger.Error(ctx, "idempotency store acquire failed", log.Err(err))
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
			return
		}
		if !acquired {
			h.logger.Info(
				ctx,
				"duplicate event skipped",
				log.String("event_id", event.ID),
			)
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// 8. Dispatch.
	processed := true
	switch h.mode {
	case deliveryChannel:
		processed = h.deliverToChannel(ctx, event)
	case deliveryCallback:
		processed = h.deliverToCallback(ctx, event)
	}

	if !processed {
		if h.store != nil {
			if err := h.store.Release(ctx, event.ID); err != nil {
				h.logger.Error(
					ctx,
					"idempotency store release failed",
					log.String("event_id", event.ID),
					log.Err(err),
				)
			}
		}

		if h.ackPolicy == AckOnSuccess {
			http.Error(w, "delivery failed", http.StatusServiceUnavailable)
			return
		}

		h.logger.Warn(
			ctx,
			"delivery failed but request acknowledged due to ack policy",
			log.String("event_id", event.ID),
			log.String("event_type", string(event.Type)),
			log.String("ack_policy", "always"),
		)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 9. Mark event as seen after successful processing.
	if h.store != nil {
		if err := h.store.Commit(ctx, event.ID); err != nil {
			h.logger.Error(
				ctx,
				"idempotency store commit failed after event processing",
				log.String("event_id", event.ID),
				log.Err(err),
			)
		}
	}

	// 10. Return 200 OK.
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) deliverToChannel(ctx context.Context, event Event) bool {
	h.eventsMu.RLock()
	defer h.eventsMu.RUnlock()

	if h.events == nil {
		h.logger.Warn(
			ctx,
			"event channel unavailable, dropping event",
			log.String("event_id", event.ID),
			log.String("event_type", string(event.Type)),
		)
		return false
	}

	select {
	case h.events <- event:
		return true
	default:
		h.logger.Warn(
			ctx,
			"event channel full, dropping event",
			log.String("event_id", event.ID),
			log.String("event_type", string(event.Type)),
		)
		return false
	}
}

func (h *Handler) deliverToCallback(ctx context.Context, event Event) bool {
	handler, ok := h.handlers[event.Type]
	if !ok {
		handler = h.defaultHandler
	}
	if handler == nil {
		return true
	}

	if err := handler(ctx, event); err != nil {
		h.logger.Error(
			ctx,
			"event handler error",
			log.String("event_id", event.ID),
			log.String("event_type", string(event.Type)),
			log.Err(err),
		)
		return false
	}

	return true
}
