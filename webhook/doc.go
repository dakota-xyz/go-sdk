// Package webhook provides production-ready handling of Dakota Platform webhook
// notifications.
//
// # Standalone Functions
//
// For maximum flexibility, use the standalone functions:
//
//   - [NewVerifier]: Verify against a list of trusted keys (key rotation)
//   - [VerifySignature]: Verify an Ed25519 webhook signature with one key
//   - [ValidateTimestamp]: Check timestamp freshness for replay protection
//   - [ParseEvent]: Parse a JSON payload into an [Event]
//   - [Verifier.ConstructEvent]: All-in-one verify + validate + parse, with
//     a list of trusted keys (recommended)
//   - [ConstructEvent]: All-in-one with one key and X-Webhook-Signature only
//
// # HTTP Handler
//
// [NewHandler] creates an [http.Handler] that handles the full verification
// pipeline and dispatches events via callbacks or a channel:
//
//	handler, err := webhook.NewHandler(
//	    webhook.WithPublicKeys(os.Getenv("DAKOTA_WEBHOOK_PUBLIC_KEYS")),
//	    webhook.WithAckPolicy(webhook.AckOnSuccess),
//	    webhook.On(webhook.EventCustomerCreated, handleCustomer),
//	    webhook.OnDefault(handleOther),
//	)
//	http.Handle("/webhook", handler)
//
// # Listener
//
// [NewListener] wraps the handler in a standalone HTTP server:
//
//	listener, err := webhook.NewListener(
//	    webhook.WithAddr(":8080"),
//	    webhook.WithHandlerOptions(
//	        webhook.WithPublicKeys(pubKeys),
//	        webhook.WithChannel(100),
//	    ),
//	)
//	go processEvents(listener.Events())
//	// GET /healthz is available for liveness checks.
//	listener.Start(ctx)
//
// # Signature Protocol
//
// Dakota Platform signs webhooks with Ed25519. The signed message is the
// concatenation of the timestamp string bytes and the payload bytes, with no
// delimiter. The timestamp is sent as a unix seconds string in
// X-Webhook-Timestamp. The X-Dakota-Webhook-Signatures header carries one
// "<key id>:<base64 signature>" entry per Dakota signing key, where the key id
// is the first 8 hex characters of the public key ([KeyID]); X-Webhook-Signature
// carries one base64 signature made with the current key.
//
// # Key Rotation
//
// Dakota rotates its signing key in phases: during the overlap
// X-Dakota-Webhook-Signatures carries the current and the new key; at cutover
// both headers switch to the new key; then the old key is retired. Trust a
// list of keys ([WithPublicKeys], [NewVerifier]), typically from a
// DAKOTA_WEBHOOK_PUBLIC_KEYS environment variable: add the new key when Dakota
// publishes it, and remove the old one as soon as Dakota retires it. The
// handler and [Verifier] check X-Dakota-Webhook-Signatures, each trusted key at
// most once, and fall back to X-Webhook-Signature only when that header is
// missing or empty.
package webhook
