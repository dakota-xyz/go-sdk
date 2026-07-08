// Package client provides a production-ready Dakota Platform API client.
//
// The package wraps the generated OpenAPI client with SDK-level defaults:
//
//   - sandbox-first environment selection
//   - API key / application token header injection
//   - automatic idempotency keys for POST requests
//   - bounded retries with exponential backoff and Retry-After support
//   - structured logging with built-in secret redaction
//   - typed error mapping
//   - cursor pagination helpers (iterator + async stream)
//
// The generated client/types are available under client/gen.
//
// # Agentic payments (alpha)
//
// The hosted payment-agent surface — [Client.AttachUserToWallet],
// [Client.NewAgentConversation], the mandate-signing helpers ([Signer],
// [P256Signer], [MandateSignPayload]), and the raw /payment-agents, /mandates,
// and /instructions operations under client/gen — is an ALPHA feature. The
// endpoints are x-alpha and flag-gated on the platform (they return 404 unless
// enabled), and these helpers may change (or be removed) without a
// major-version bump. Not recommended for production.
package client
