package client

import (
	"os"
	"strings"
	"testing"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

// TestTransactionStatusConstants pins generated status constants to the values
// they name. The names come from a positional x-enum-varnames list in the
// synced spec, so a sync that lets that list drift from its enum would rename
// constants without a compile error; this fails instead.
func TestTransactionStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"completed", string(gen.TransactionStatusCompleted), "completed"},
		{"returned", string(gen.TransactionStatusReturned), "returned"},
		{"legacy canceled spelling", string(gen.TransactionStatusCanceled), "canceled"},
		{"cancelled", string(gen.TransactionStatusCancelled), "cancelled"},
		{"paused credits", string(gen.TransactionStatusPausedCredits), "paused_credits"},
		{"Success label", string(gen.TransactionStatusLabelSuccess), "Success"},
		{"Waiting For Signature label", string(gen.TransactionStatusLabelWaitingForSignature), "Waiting For Signature"},
		{"completed code", string(gen.TransactionStatusCodeCompleted), "completed"},
		{"waiting for signature code", string(gen.TransactionStatusCodeWaitingForSignature), "waiting_for_signature"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("constant is %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// TestAutoTransactionsStatusFilterEnum pins that the GET /auto-transactions
// status filter documents every status the platform accepts for it. The filter
// has its own enum rather than a $ref, so a partial sync can drop values from
// it without changing the generated Go, which x-go-type pins.
func TestAutoTransactionsStatusFilterEnum(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("gen/openapi.yaml")
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	spec := string(raw)

	start := strings.Index(spec, "x-go-type: TransactionStatus\n")
	if start < 0 {
		t.Fatal("spec has no parameter pinned to TransactionStatus — did a sync drop the /auto-transactions status filter?")
	}
	block := spec[start:]
	if end := strings.Index(block, "- name:"); end >= 0 {
		block = block[:end]
	}

	for _, value := range []string{
		"completed", "cancelled", "canceled", "pending_return", "returned", "pending_reversal", "paused_credits",
	} {
		if !strings.Contains(block, "- "+value+"\n") {
			t.Errorf("the /auto-transactions status filter enum is missing %q, which the platform accepts", value)
		}
	}
}
