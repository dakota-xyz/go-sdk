package webhook

import (
	"encoding/json"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
)

// ParseEvent unmarshals a JSON payload into an Event.
func ParseEvent(payload []byte) (Event, error) {
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return Event{}, errors.Wrap(
			errors.CodeMalformedPayload,
			"failed to parse event JSON",
			err,
		)
	}

	if event.ID == "" {
		return Event{}, errors.New(
			errors.CodeMalformedPayload,
			"event missing id field",
		)
	}

	if event.Type == "" {
		return Event{}, errors.New(
			errors.CodeMalformedPayload,
			"event missing type field",
		)
	}

	return event, nil
}

// ConstructOption configures ConstructEvent behavior.
type ConstructOption func(*constructConfig)

type constructConfig struct {
	tolerance       time.Duration
	ignoreTolerance bool
}

// WithTolerance sets a custom timestamp tolerance.
func WithTolerance(d time.Duration) ConstructOption {
	return func(c *constructConfig) {
		c.tolerance = d
	}
}

// IgnoringTolerance disables timestamp validation entirely.
func IgnoringTolerance() ConstructOption {
	return func(c *constructConfig) {
		c.ignoreTolerance = true
	}
}

// ConstructEvent verifies the signature, validates the timestamp, and parses
// the event in a single call. This is the recommended all-in-one function for
// processing incoming webhooks.
func ConstructEvent(
	payload []byte,
	signatureHeader string,
	timestampHeader string,
	publicKeyHex string,
	opts ...ConstructOption,
) (Event, error) {
	cfg := constructConfig{
		tolerance: DefaultTimestampTolerance,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if err := VerifySignature(
		payload,
		signatureHeader,
		timestampHeader,
		publicKeyHex,
	); err != nil {
		return Event{}, err
	}

	if !cfg.ignoreTolerance {
		if err := ValidateTimestamp(
			timestampHeader,
			cfg.tolerance,
		); err != nil {
			return Event{}, err
		}
	}

	return ParseEvent(payload)
}
