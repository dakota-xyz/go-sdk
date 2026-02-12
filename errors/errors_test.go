package errors_test

import (
	"fmt"
	"testing"

	"github.com/dakota-xyz/go-sdk/errors"
)

func TestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *errors.Error
		expected string
	}{
		{
			name:     "without cause",
			err:      errors.New(errors.CodeInvalidSignature, "bad sig"),
			expected: "INVALID_SIGNATURE: bad sig",
		},
		{
			name: "with cause",
			err: errors.Wrap(
				errors.CodeMalformedPayload,
				"parse failed",
				fmt.Errorf("unexpected EOF"),
			),
			expected: "MALFORMED_PAYLOAD: parse failed: unexpected EOF",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if got := tt.err.Error(); got != tt.expected {
					t.Errorf("got %q, want %q", got, tt.expected)
				}
			},
		)
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := errors.Wrap(errors.CodeInternal, "wrapped", cause)

	if !errors.Is(err, cause) {
		t.Error("expected Unwrap to return the wrapped cause")
	}

	errNoCause := errors.New(errors.CodeInternal, "no cause")
	if errNoCause.Unwrap() != nil {
		t.Error("expected nil Unwrap when no cause")
	}
}

func TestError_Is(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{
			name:   "matches sentinel by code",
			err:    errors.New(errors.CodeInvalidSignature, "custom message"),
			target: errors.ErrInvalidSignature,
			want:   true,
		},
		{
			name: "wrapped error matches sentinel",
			err: errors.Wrap(
				errors.CodeSignatureExpired,
				"expired",
				fmt.Errorf("cause"),
			),
			target: errors.ErrSignatureExpired,
			want:   true,
		},
		{
			name:   "different codes do not match",
			err:    errors.New(errors.CodeInvalidSignature, "sig"),
			target: errors.ErrMalformedPayload,
			want:   false,
		},
		{
			name:   "non-Error target does not match",
			err:    errors.New(errors.CodeInternal, "internal"),
			target: fmt.Errorf("other error"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if got := errors.Is(tt.err, tt.target); got != tt.want {
					t.Errorf("errors.Is() = %v, want %v", got, tt.want)
				}
			},
		)
	}
}

func TestError_As(t *testing.T) {
	base := errors.Wrap(
		errors.CodeMalformedPayload,
		"decode event",
		fmt.Errorf("invalid JSON"),
	)

	joined := errors.Join(fmt.Errorf("extra context"), base)

	var extractedPtr *errors.Error
	if !errors.As(joined, &extractedPtr) {
		t.Fatal("expected errors.As to extract *errors.Error")
	}
	if extractedPtr.Code != errors.CodeMalformedPayload {
		t.Fatalf("got code %q, want %q", extractedPtr.Code, errors.CodeMalformedPayload)
	}

	// Value extraction should also work for value sentinels.
	sentinelWrapped := fmt.Errorf("outer: %w", errors.ErrInvalidSignature)
	var extractedValue errors.Error
	if !errors.As(sentinelWrapped, &extractedValue) {
		t.Fatal("expected errors.As to extract errors.Error value")
	}
	if extractedValue.Code != errors.CodeInvalidSignature {
		t.Fatalf("got code %q, want %q", extractedValue.Code, errors.CodeInvalidSignature)
	}
}

func TestSentinels(t *testing.T) {
	sentinels := []struct {
		name     string
		sentinel errors.Error
		code     errors.Code
	}{
		{
			"ErrInvalidSignature",
			errors.ErrInvalidSignature,
			errors.CodeInvalidSignature,
		},
		{
			"ErrSignatureExpired",
			errors.ErrSignatureExpired,
			errors.CodeSignatureExpired,
		},
		{
			"ErrPayloadTooLarge",
			errors.ErrPayloadTooLarge,
			errors.CodePayloadTooLarge,
		},
		{
			"ErrMalformedPayload",
			errors.ErrMalformedPayload,
			errors.CodeMalformedPayload,
		},
		{"ErrMissingHeader", errors.ErrMissingHeader, errors.CodeMissingHeader},
		{
			"ErrDuplicateEvent",
			errors.ErrDuplicateEvent,
			errors.CodeDuplicateEvent,
		},
		{"ErrInvalidConfig", errors.ErrInvalidConfig, errors.CodeInvalidConfig},
		{"ErrInternal", errors.ErrInternal, errors.CodeInternal},
	}

	for _, s := range sentinels {
		t.Run(
			s.name, func(t *testing.T) {
				if s.sentinel.Code != s.code {
					t.Errorf("got code %q, want %q", s.sentinel.Code, s.code)
				}
			},
		)
	}
}
