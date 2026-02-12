// Package webhook provides production-ready handling of Dakota Platform webhook
// notifications.
//
// # Standalone Functions
//
// For maximum flexibility, use the standalone functions:
//
//   - [VerifySignature]: Verify an Ed25519 webhook signature
//   - [ValidateTimestamp]: Check timestamp freshness for replay protection
//   - [ParseEvent]: Parse a JSON payload into an [Event]
//   - [ConstructEvent]: All-in-one verify + validate + parse
//
// # HTTP Handler
//
// [NewHandler] creates an [http.Handler] that handles the full verification
// pipeline and dispatches events via callbacks or a channel:
//
//	handler, err := webhook.NewHandler(
//	    webhook.WithPublicKey(os.Getenv("DAKOTA_WEBHOOK_PUBLIC_KEY")),
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
//	        webhook.WithPublicKey(pubKey),
//	        webhook.WithChannel(100),
//	    ),
//	)
//	go processEvents(listener.Events())
//	listener.Start(ctx)
//
// # Signature Protocol
//
// Dakota Platform signs webhooks with Ed25519. The signed message is the
// concatenation of the timestamp string bytes and the payload bytes, with no
// delimiter. The signature is sent base64-encoded in the X-Webhook-Signature
// header, and the timestamp as a unix seconds string in X-Webhook-Timestamp.
package webhook
