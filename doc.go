// Package dakota is the official Go SDK for Dakota Platform.
//
// The SDK provides production-ready components for receiving and verifying
// webhook notifications from Dakota Platform, including Ed25519 signature
// verification, timestamp validation, event parsing, and idempotent handling.
//
// # Packages
//
//   - webhook: Core webhook handling with signature verification, event parsing,
//     and HTTP handler/listener implementations.
//   - errors: Structured error types with machine-readable codes.
//   - log: Structured logging abstraction built on log/slog.
//   - webhook/idempotency: Event deduplication with pluggable store interface.
//
// # Quick Start
//
// The simplest way to handle webhooks is with [webhook.ConstructEvent]:
//
//	event, err := webhook.ConstructEvent(payload, sigHeader, tsHeader, publicKeyHex)
//	if err != nil {
//	    // handle verification failure
//	}
//	// process event
//
// For a full HTTP server, use [webhook.NewHandler] or [webhook.NewListener].
package dakota
