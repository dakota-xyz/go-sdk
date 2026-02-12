package log

import (
	"context"
	"log/slog"
)

// Logger is the structured logging interface for the SDK.
type Logger interface {
	Debug(ctx context.Context, msg string, attrs ...slog.Attr)
	Info(ctx context.Context, msg string, attrs ...slog.Attr)
	Warn(ctx context.Context, msg string, attrs ...slog.Attr)
	Error(ctx context.Context, msg string, attrs ...slog.Attr)
	With(attrs ...slog.Attr) Logger
}

// String returns a slog.Attr for a string value.
func String(key, value string) slog.Attr {
	return slog.String(key, value)
}

// Int returns a slog.Attr for an int value.
func Int(key string, value int) slog.Attr {
	return slog.Int(key, value)
}

// Int64 returns a slog.Attr for an int64 value.
func Int64(key string, value int64) slog.Attr {
	return slog.Int64(key, value)
}

// Bool returns a slog.Attr for a bool value.
func Bool(key string, value bool) slog.Attr {
	return slog.Bool(key, value)
}

// Err returns a slog.Attr for an error value.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	return slog.String("error", err.Error())
}

// Option configures a Logger.
type Option func(*slogLogger)

// WithLevel sets the minimum log level.
func WithLevel(level slog.Level) Option {
	return func(l *slogLogger) {
		if l.level == nil {
			l.level = &slog.LevelVar{}
			l.logger = slog.New(
				levelHandler{
					handler: l.logger.Handler(),
					level:   l.level,
				},
			)
		}
		l.level.Set(level)
	}
}

type levelHandler struct {
	handler slog.Handler
	level   *slog.LevelVar
}

func (h levelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.level != nil && level < h.level.Level() {
		return false
	}
	return h.handler.Enabled(ctx, level)
}

func (h levelHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.handler.Handle(ctx, r)
}

func (h levelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return levelHandler{
		handler: h.handler.WithAttrs(attrs),
		level:   h.level,
	}
}

func (h levelHandler) WithGroup(name string) slog.Handler {
	return levelHandler{
		handler: h.handler.WithGroup(name),
		level:   h.level,
	}
}

// slogLogger is the default Logger backed by log/slog.
type slogLogger struct {
	logger *slog.Logger
	level  *slog.LevelVar
}

func attrsToArgs(attrs []slog.Attr) []any {
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return args
}

func (l *slogLogger) Debug(
	ctx context.Context,
	msg string,
	attrs ...slog.Attr,
) {
	l.logger.LogAttrs(ctx, slog.LevelDebug, msg, attrs...)
}

func (l *slogLogger) Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.logger.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

func (l *slogLogger) Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.logger.LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
}

func (l *slogLogger) Error(
	ctx context.Context,
	msg string,
	attrs ...slog.Attr,
) {
	l.logger.LogAttrs(ctx, slog.LevelError, msg, attrs...)
}

func (l *slogLogger) With(attrs ...slog.Attr) Logger {
	args := attrsToArgs(attrs)
	return &slogLogger{
		logger: l.logger.With(args...),
		level:  l.level,
	}
}

// New creates a new Logger backed by the default slog logger.
func New(opts ...Option) Logger {
	level := &slog.LevelVar{}
	level.Set(slog.LevelInfo)

	l := &slogLogger{
		logger: slog.New(levelHandler{
			handler: slog.Default().Handler(),
			level:   level,
		}),
		level: level,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// FromSlog wraps an existing *slog.Logger.
func FromSlog(logger *slog.Logger) Logger {
	if logger == nil {
		return New()
	}

	return &slogLogger{
		logger: logger,
	}
}

// nopLogger silently discards all log output.
type nopLogger struct{}

func (nopLogger) Debug(_ context.Context, _ string, _ ...slog.Attr) {}
func (nopLogger) Info(_ context.Context, _ string, _ ...slog.Attr)  {}
func (nopLogger) Warn(_ context.Context, _ string, _ ...slog.Attr)  {}
func (nopLogger) Error(_ context.Context, _ string, _ ...slog.Attr) {}
func (n nopLogger) With(_ ...slog.Attr) Logger                      { return n }

// Nop returns a Logger that discards all output. Useful for tests.
func Nop() Logger {
	return nopLogger{}
}
