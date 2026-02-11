// Package errors provides structured error types for the Dakota Go SDK.
//
// Each error carries a machine-readable [Code] that can be matched with
// [errors.Is]:
//
//	if errors.Is(err, errors.ErrInvalidSignature) {
//	    // handle invalid signature
//	}
//
// Use [New] and [Wrap] to create errors:
//
//	err := errors.New(errors.CodeMalformedPayload, "missing id field")
//	err := errors.Wrap(errors.CodeInternal, "store lookup", cause)
package errors
