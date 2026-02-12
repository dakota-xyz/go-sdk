package webhook

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/log"
)

const healthzPath = "/healthz"

// ListenerOption configures a Listener.
type ListenerOption func(*listenerConfig)

type listenerConfig struct {
	addr              string
	path              string
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	shutdownTimeout   time.Duration
	logger            log.Logger
	handlerOpts       []HandlerOption
}

// WithAddr sets the listen address (default ":8080").
func WithAddr(addr string) ListenerOption {
	return func(c *listenerConfig) {
		c.addr = addr
	}
}

// WithPath sets the HTTP path for the webhook endpoint (default "/webhook").
func WithPath(path string) ListenerOption {
	return func(c *listenerConfig) {
		c.path = path
	}
}

// WithReadTimeout sets the HTTP server read timeout.
func WithReadTimeout(d time.Duration) ListenerOption {
	return func(c *listenerConfig) {
		c.readTimeout = d
	}
}

// WithReadHeaderTimeout sets the HTTP server read header timeout.
func WithReadHeaderTimeout(d time.Duration) ListenerOption {
	return func(c *listenerConfig) {
		c.readHeaderTimeout = d
	}
}

// WithWriteTimeout sets the HTTP server write timeout.
func WithWriteTimeout(d time.Duration) ListenerOption {
	return func(c *listenerConfig) {
		c.writeTimeout = d
	}
}

// WithIdleTimeout sets the HTTP server idle timeout.
func WithIdleTimeout(d time.Duration) ListenerOption {
	return func(c *listenerConfig) {
		c.idleTimeout = d
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout.
func WithShutdownTimeout(d time.Duration) ListenerOption {
	return func(c *listenerConfig) {
		c.shutdownTimeout = d
	}
}

// WithListenerLogger sets the logger for the listener.
func WithListenerLogger(l log.Logger) ListenerOption {
	return func(c *listenerConfig) {
		c.logger = l
	}
}

// WithHandlerOptions passes options through to the underlying Handler.
func WithHandlerOptions(opts ...HandlerOption) ListenerOption {
	return func(c *listenerConfig) {
		c.handlerOpts = append(c.handlerOpts, opts...)
	}
}

// Listener is a standalone HTTP server that receives webhook events.
type Listener struct {
	handler   *Handler
	server    *http.Server
	logger    log.Logger
	mu        sync.RWMutex
	listener  net.Listener
	closeOnce sync.Once
	shutdown  time.Duration
}

// NewListener creates a new webhook Listener.
func NewListener(opts ...ListenerOption) (*Listener, error) {
	cfg := &listenerConfig{
		addr:              ":8080",
		path:              "/webhook",
		readHeaderTimeout: 5 * time.Second,
		readTimeout:       30 * time.Second,
		writeTimeout:      30 * time.Second,
		idleTimeout:       60 * time.Second,
		shutdownTimeout:   10 * time.Second,
		logger:            log.Nop(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if len(cfg.handlerOpts) == 0 {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"handler options are required (at minimum WithPublicKey)",
		)
	}
	if !strings.HasPrefix(cfg.path, "/") {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"path must start with '/'",
		)
	}
	if cfg.path == healthzPath {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"webhook path conflicts with health endpoint path",
		)
	}
	if cfg.readHeaderTimeout <= 0 {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"read header timeout must be greater than zero",
		)
	}
	if cfg.readTimeout <= 0 {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"read timeout must be greater than zero",
		)
	}
	if cfg.writeTimeout <= 0 {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"write timeout must be greater than zero",
		)
	}
	if cfg.idleTimeout <= 0 {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"idle timeout must be greater than zero",
		)
	}
	if cfg.shutdownTimeout <= 0 {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"shutdown timeout must be greater than zero",
		)
	}
	if cfg.logger == nil {
		cfg.logger = log.Nop()
	}

	handler, err := NewHandler(cfg.handlerOpts...)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.path, handler)
	mux.HandleFunc(
		healthzPath,
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
		},
	)

	server := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.readHeaderTimeout,
		ReadTimeout:       cfg.readTimeout,
		WriteTimeout:      cfg.writeTimeout,
		IdleTimeout:       cfg.idleTimeout,
	}

	return &Listener{
		handler:  handler,
		server:   server,
		logger:   cfg.logger,
		shutdown: cfg.shutdownTimeout,
	}, nil
}

// Events returns the event channel from the underlying Handler. Returns nil
// if the handler is not in channel delivery mode.
func (l *Listener) Events() <-chan Event {
	return l.handler.Events()
}

// Start begins listening for webhooks. It blocks until the context is
// cancelled, then gracefully shuts down the server.
func (l *Listener) Start(ctx context.Context) error {
	l.mu.RLock()
	alreadyStarted := l.listener != nil
	l.mu.RUnlock()
	if alreadyStarted {
		return errors.New(errors.CodeInvalidConfig, "listener already started")
	}

	ln, err := net.Listen("tcp", l.server.Addr)
	if err != nil {
		return err
	}
	l.setListener(ln)

	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)
		defer l.setListener(nil)

		l.logger.Info(
			ctx,
			"webhook listener starting",
			log.String("addr", ln.Addr().String()),
		)
		if err := l.server.Serve(ln); err != nil && !errors.Is(
			err,
			http.ErrServerClosed,
		) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		l.logger.Info(ctx, "webhook listener shutting down")
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			l.shutdown,
		)
		defer cancel()
		if err := l.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		l.handler.Close()

		if err := <-errCh; err != nil {
			return err
		}
		return nil
	}
}

// Addr returns the listener's address. This is useful when using port 0
// for dynamic port allocation. Returns nil before Start has bound a listener.
func (l *Listener) Addr() net.Addr {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.listener == nil {
		return nil
	}
	return l.listener.Addr()
}

// Close forcefully shuts down the listener and closes the event channel.
// It is safe to call multiple times.
func (l *Listener) Close() {
	l.closeOnce.Do(
		func() {
			_ = l.server.Close()
			l.handler.Close()
		},
	)
}

func (l *Listener) setListener(ln net.Listener) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.listener = ln
}
