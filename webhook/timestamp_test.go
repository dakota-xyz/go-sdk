package webhook_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/webhook"
)

func TestValidateTimestampAt(t *testing.T) {
	now := time.Unix(1700000000, 0)

	tests := []struct {
		name      string
		timestamp string
		tolerance time.Duration
		wantErr   bool
	}{
		{
			name:      "exact match",
			timestamp: "1700000000",
			tolerance: 5 * time.Minute,
			wantErr:   false,
		},
		{
			name:      "within tolerance past",
			timestamp: fmt.Sprintf("%d", now.Add(-4*time.Minute).Unix()),
			tolerance: 5 * time.Minute,
			wantErr:   false,
		},
		{
			name:      "within tolerance future",
			timestamp: fmt.Sprintf("%d", now.Add(4*time.Minute).Unix()),
			tolerance: 5 * time.Minute,
			wantErr:   false,
		},
		{
			name:      "at boundary",
			timestamp: fmt.Sprintf("%d", now.Add(-5*time.Minute).Unix()),
			tolerance: 5 * time.Minute,
			wantErr:   false,
		},
		{
			name:      "too old",
			timestamp: fmt.Sprintf("%d", now.Add(-6*time.Minute).Unix()),
			tolerance: 5 * time.Minute,
			wantErr:   true,
		},
		{
			name:      "too far in future",
			timestamp: fmt.Sprintf("%d", now.Add(6*time.Minute).Unix()),
			tolerance: 5 * time.Minute,
			wantErr:   true,
		},
		{
			name:      "invalid format",
			timestamp: "not-a-number",
			tolerance: 5 * time.Minute,
			wantErr:   true,
		},
		{
			name:      "empty string",
			timestamp: "",
			tolerance: 5 * time.Minute,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				err := webhook.ValidateTimestampAt(
					tt.timestamp,
					tt.tolerance,
					now,
				)
				if (err != nil) != tt.wantErr {
					t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				}
				if err != nil {
					if !errors.Is(err, errors.ErrSignatureExpired) {
						t.Errorf("expected ErrSignatureExpired, got: %v", err)
					}
				}
			},
		)
	}
}

func TestValidateTimestamp(t *testing.T) {
	// Use current time so it's always within tolerance.
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	if err := webhook.ValidateTimestamp(timestamp, 5*time.Minute); err != nil {
		t.Fatalf("expected no error for current timestamp, got: %v", err)
	}

	// Very old timestamp should fail.
	oldTs := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	if err := webhook.ValidateTimestamp(oldTs, 5*time.Minute); err == nil {
		t.Error("expected error for old timestamp")
	}
}
