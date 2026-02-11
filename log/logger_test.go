package log_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/dakota-xyz/go-sdk/log"
)

func TestNew(t *testing.T) {
	logger := log.New()
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// Should not panic.
	logger.Info(context.Background(), "test message")
}

func TestFromSlog(t *testing.T) {
	var buf bytes.Buffer
	slogger := slog.New(
		slog.NewTextHandler(
			&buf,
			&slog.HandlerOptions{Level: slog.LevelDebug},
		),
	)

	logger := log.FromSlog(slogger)
	logger.Info(context.Background(), "hello", log.String("key", "value"))

	output := buf.String()
	if !strings.Contains(output, "hello") {
		t.Errorf("expected log output to contain 'hello', got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("expected log output to contain 'key=value', got: %s", output)
	}
}

func TestNop(t *testing.T) {
	logger := log.Nop()

	// Should not panic.
	ctx := context.Background()
	logger.Debug(ctx, "debug")
	logger.Info(ctx, "info")
	logger.Warn(ctx, "warn")
	logger.Error(ctx, "error")

	withLogger := logger.With(log.String("k", "v"))
	withLogger.Info(ctx, "still nop")
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	slogger := slog.New(
		slog.NewTextHandler(
			&buf,
			&slog.HandlerOptions{Level: slog.LevelDebug},
		),
	)

	logger := log.FromSlog(slogger).With(log.String("component", "test"))
	logger.Info(context.Background(), "with test")

	output := buf.String()
	if !strings.Contains(output, "component=test") {
		t.Errorf("expected 'component=test' in output, got: %s", output)
	}
}

func TestAttrHelpers(t *testing.T) {
	tests := []struct {
		name string
		attr slog.Attr
		key  string
	}{
		{"String", log.String("k", "v"), "k"},
		{"Int", log.Int("n", 42), "n"},
		{"Int64", log.Int64("n64", 100), "n64"},
		{"Bool", log.Bool("flag", true), "flag"},
		{"Err", log.Err(nil), ""},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if tt.key != "" && tt.attr.Key != tt.key {
					t.Errorf("got key %q, want %q", tt.attr.Key, tt.key)
				}
			},
		)
	}
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	slogger := slog.New(
		slog.NewTextHandler(
			&buf,
			&slog.HandlerOptions{Level: slog.LevelDebug},
		),
	)
	logger := log.FromSlog(slogger)

	ctx := context.Background()

	logger.Debug(ctx, "debug msg")
	if !strings.Contains(buf.String(), "debug msg") {
		t.Error("expected debug message in output")
	}

	buf.Reset()
	logger.Warn(ctx, "warn msg")
	if !strings.Contains(buf.String(), "warn msg") {
		t.Error("expected warn message in output")
	}

	buf.Reset()
	logger.Error(ctx, "error msg")
	if !strings.Contains(buf.String(), "error msg") {
		t.Error("expected error message in output")
	}
}
