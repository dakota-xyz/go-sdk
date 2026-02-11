// Package log provides a structured logging abstraction for the Dakota
// Go SDK, built on [log/slog].
//
// The [Logger] interface is intentionally minimal and compatible with slog's
// attribute model. Use [New] for a default slog-backed logger, [FromSlog] to
// wrap an existing [slog.Logger], or [Nop] for a silent logger in tests.
//
//	logger := log.New()
//	logger.Info(ctx, "webhook received", log.String("event_id", id))
package log
