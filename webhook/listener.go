package webhook

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/log"
)

// ListenerOption configures a Listener.
type ListenerOption func(*listenerConfig)

type listenerConfig struct {
	addr         string
	path         string
	readTimeout  time.Duration
	writeTimeout time.Duration
	logger       log.Logger
	handlerOpts  []HandlerOption
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

// WithWriteTimeout sets the HTTP server write timeout.
func WithWriteTimeout(d time.Duration) ListenerOption {
	return func(c *listenerConfig) {
		c.writeTimeout = d
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
	handler *Handler
	server  *http.Server
	logger  log.Logger
}

// NewListener creates a new webhook Listener.
func NewListener(opts ...ListenerOption) (*Listener, error) {
	cfg := &listenerConfig{
		addr:         ":8080",
		path:         "/webhook",
		readTimeout:  30 * time.Second,
		writeTimeout: 30 * time.Second,
		logger:       log.Nop(),
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

	handler, err := NewHandler(cfg.handlerOpts...)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.path, handler)

	server := &http.Server{
		Addr:         cfg.addr,
		Handler:      mux,
		ReadTimeout:  cfg.readTimeout,
		WriteTimeout: cfg.writeTimeout,
	}

	return &Listener{
		handler: handler,
		server:  server,
		logger:  cfg.logger,
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
	errCh := make(chan error, 1)

	go func() {
		l.logger.Info(
			ctx,
			"webhook listener starting",
			log.String("addr", l.server.Addr),
		)
		if err := l.server.ListenAndServe(); err != nil && !errors.Is(
			err,
			http.ErrServerClosed,
		) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		l.logger.Info(ctx, "webhook listener shutting down")
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()
		return l.server.Shutdown(shutdownCtx)
	}
}

// Addr returns the listener's address. This is useful when using port 0
// for dynamic port allocation. Returns empty string if the server has no
// listener.
func (l *Listener) Addr() net.Addr {
	return nil
}

// Close shuts down the listener and closes the event channel.
func (l *Listener) Close() {
	l.server.Close()
	l.handler.Close()
}
