package webhook

import (
	"math"
	"strconv"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
)

// DefaultTimestampTolerance is the default maximum age for a webhook timestamp.
const DefaultTimestampTolerance = 5 * time.Minute

// ValidateTimestamp checks that the timestamp string is a valid unix timestamp
// and is within the given tolerance of the current time.
func ValidateTimestamp(timestampStr string, tolerance time.Duration) error {
	return ValidateTimestampAt(timestampStr, tolerance, time.Now())
}

// ValidateTimestampAt checks the timestamp against a specific reference time.
// This is useful for testing.
func ValidateTimestampAt(
	timestampStr string,
	tolerance time.Duration,
	now time.Time,
) error {
	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return errors.Wrap(
			errors.CodeSignatureExpired,
			"invalid timestamp format",
			err,
		)
	}

	webhookTime := time.Unix(ts, 0)
	diff := math.Abs(float64(now.Unix() - webhookTime.Unix()))

	if diff > tolerance.Seconds() {
		return errors.New(
			errors.CodeSignatureExpired,
			"timestamp outside tolerance window",
		)
	}

	return nil
}
