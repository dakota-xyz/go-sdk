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
		l.level = level
	}
}

// slogLogger is the default Logger backed by log/slog.
type slogLogger struct {
	logger *slog.Logger
	level  slog.Level
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
	l := &slogLogger{
		logger: slog.Default(),
		level:  slog.LevelInfo,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// FromSlog wraps an existing *slog.Logger.
func FromSlog(logger *slog.Logger) Logger {
	return &slogLogger{
		logger: logger,
		level:  slog.LevelInfo,
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
