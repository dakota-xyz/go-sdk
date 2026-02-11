package errors

import (
	stderrors "errors"
	"fmt"
)

// Re-export stdlib errors functions so that importing this package does not
// require a separate import of the standard library errors package.
var (
	// Is reports whether any error in err's tree matches target.
	Is = stderrors.Is

	// As finds the first error in err's tree that matches target.
	As = stderrors.As

	// Unwrap returns the result of calling the Unwrap method on err.
	Unwrap = stderrors.Unwrap

	// Join returns an error that wraps the given errors.
	Join = stderrors.Join
)

// Code is a machine-readable error classification.
type Code string

const (
	CodeInvalidSignature Code = "INVALID_SIGNATURE"
	CodeSignatureExpired Code = "SIGNATURE_EXPIRED"
	CodePayloadTooLarge  Code = "PAYLOAD_TOO_LARGE"
	CodeMalformedPayload Code = "MALFORMED_PAYLOAD"
	CodeMissingHeader    Code = "MISSING_HEADER"
	CodeDuplicateEvent   Code = "DUPLICATE_EVENT"
	CodeInvalidConfig    Code = "INVALID_CONFIG"
	CodeInternal         Code = "INTERNAL"
)

// Error is a structured SDK error with a machine-readable code.
type Error struct {
	Code    Code
	Message string
	Err     error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %s", e.Code, e.Message, e.Err.Error())
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped cause.
func (e *Error) Unwrap() error {
	return e.Err
}

// Is reports whether the target matches this error's Code. This allows
// errors.Is(err, ErrInvalidSignature) to match any Error with
// CodeInvalidSignature.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// New creates a new Error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap creates a new Error wrapping an existing error.
func Wrap(code Code, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Sentinel errors for use with errors.Is().
var (
	ErrInvalidSignature = New(CodeInvalidSignature, "invalid signature")
	ErrSignatureExpired = New(CodeSignatureExpired, "signature expired")
	ErrPayloadTooLarge  = New(CodePayloadTooLarge, "payload too large")
	ErrMalformedPayload = New(CodeMalformedPayload, "malformed payload")
	ErrMissingHeader    = New(CodeMissingHeader, "missing header")
	ErrDuplicateEvent   = New(CodeDuplicateEvent, "duplicate event")
	ErrInvalidConfig    = New(CodeInvalidConfig, "invalid configuration")
	ErrInternal         = New(CodeInternal, "internal error")
)
