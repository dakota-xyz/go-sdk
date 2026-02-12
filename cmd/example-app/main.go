package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dakota-xyz/go-sdk/webhook"
	"github.com/dakota-xyz/go-sdk/webhook/types"
)

const (
	listenAddr              = "WH_ADDR"
	webhookPath             = "WH_PATH"
	webhookSigningPublicKey = "WH_PUB_KEY"
	readHeaderTimeout       = 5 * time.Second
	readTimeout             = 15 * time.Second
	writeTimeout            = 15 * time.Second
	idleTimeout             = 60 * time.Second
	shutdownTimeout         = 10 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("webhook app exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	h, err := newPlatformWebhookHandler(logger)
	if err != nil {
		return fmt.Errorf("create webhook handler: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle(webhookPath, h)

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info(
			"starting webhook server",
			slog.String("url", "http://"+listenAddr+webhookPath),
		)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(
			serveErr,
			http.ErrServerClosed,
		) {
			errCh <- serveErr
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()

		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("shutdown webhook server: %w", shutdownErr)
		}

		if serveErr := <-errCh; serveErr != nil {
			return fmt.Errorf("webhook server exited: %w", serveErr)
		}
		return nil
	case serveErr := <-errCh:
		if serveErr != nil {
			return fmt.Errorf("webhook server failed: %w", serveErr)
		}
		return nil
	}
}

func newPlatformWebhookHandler(logger *slog.Logger) (http.Handler, error) {
	eventHandler := func(ctx context.Context, event webhook.Event) error {
		return parseAndLogPlatformEvent(ctx, logger, event)
	}

	return webhook.NewHandler(
		webhook.WithPublicKey(webhookSigningPublicKey),
		webhook.On(webhook.EventTargetUpdated, eventHandler),
		webhook.On(webhook.EventTargetDeleted, eventHandler),
		webhook.On(webhook.EventTransactionAutoCreated, eventHandler),
		webhook.On(webhook.EventTransactionAutoUpdated, eventHandler),
		webhook.On(webhook.EventTransactionOneOffUpdated, eventHandler),
	)
}

func parseAndLogPlatformEvent(
	ctx context.Context,
	logger *slog.Logger,
	event webhook.Event,
) error {
	switch event.Type {
	case webhook.EventTargetUpdated:
		payload, err := webhook.EventDataAs[types.TargetUpdatedData](event)
		if err != nil {
			return fmt.Errorf("parse %s payload: %w", event.Type, err)
		}

		logger.InfoContext(
			ctx,
			"processed webhook event",
			slog.String("event_id", event.ID),
			slog.String("event_type", event.Type.String()),
			slog.String("target_id", payload.ID),
			slog.String("auto_account_id", payload.AutoAccountID),
			slog.String("amount", payload.Amount),
			slog.String("currency", payload.Currency),
			slog.String("frequency", payload.Frequency),
		)

	case webhook.EventTargetDeleted:
		payload, err := webhook.EventDataAs[types.TargetDeletedData](event)
		if err != nil {
			return fmt.Errorf("parse %s payload: %w", event.Type, err)
		}

		logger.InfoContext(
			ctx,
			"processed webhook event",
			slog.String("event_id", event.ID),
			slog.String("event_type", event.Type.String()),
			slog.String("target_id", payload.ID),
		)

	case webhook.EventTransactionAutoCreated, webhook.EventTransactionAutoUpdated:
		payload, err := webhook.EventDataAs[types.AutoTransactionData](event)
		if err != nil {
			return fmt.Errorf("parse %s payload: %w", event.Type, err)
		}

		logger.InfoContext(
			ctx,
			"processed webhook event",
			slog.String("event_id", event.ID),
			slog.String("event_type", event.Type.String()),
			slog.String("transaction_id", payload.ID),
			slog.String("auto_account_id", payload.AutoAccountID),
			slog.String("status", payload.Status),
			slog.String("provider_status", payload.ProviderStatus),
		)

	case webhook.EventTransactionOneOffUpdated:
		payload, err := webhook.EventDataAs[types.OneOffTransactionData](event)
		if err != nil {
			return fmt.Errorf("parse %s payload: %w", event.Type, err)
		}

		logger.InfoContext(
			ctx,
			"processed webhook event",
			slog.String("event_id", event.ID),
			slog.String("event_type", event.Type.String()),
			slog.String("transaction_id", payload.ID),
			slog.String("customer_id", payload.CustomerID),
			slog.String("status", payload.Status),
			slog.String("provider_status", payload.ProviderStatus),
		)

	default:
		return fmt.Errorf("unsupported event type: %s", event.Type)
	}

	return nil
}
