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
package client
