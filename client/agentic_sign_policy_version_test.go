package client

import (
	"strings"
	"testing"
)

// TestPolicyPayloads_PolicyVersion pins the bytes a signer endorses for a
// policy attach or detach that carries the policy's version (ENG-6773). The
// expected strings are copied from policy-engine v0.0.48's own vectors
// (pkg/intent/reconstruct_policy_version_test.go), the reconstruction that
// verifies these signatures: without the version the bytes are the ones
// signed before ENG-6512, with it "policy_version" joins as a JSON integer.
func TestPolicyPayloads_PolicyVersion(t *testing.T) {
	cases := []struct {
		name string
		got  func() ([]byte, error)
		want string
	}{
		{"attach without version", func() ([]byte, error) { return AttachPolicyPayload("wal-1", "pol-1", "key-1") },
			`{"idempotency_key":"key-1","policy_id":"pol-1","type":"attach_policy_to_wallet","wallet_id":"wal-1"}`},
		{"attach with version", func() ([]byte, error) {
			return AttachPolicyPayload("wal-1", "pol-1", "key-1", WithPolicyVersion(3))
		},
			`{"idempotency_key":"key-1","policy_id":"pol-1","policy_version":3,"type":"attach_policy_to_wallet","wallet_id":"wal-1"}`},
		{"detach without version", func() ([]byte, error) { return DetachPolicyPayload("wal-1", "pol-1", "key-1") },
			`{"idempotency_key":"key-1","policy_id":"pol-1","type":"detach_policy_from_wallet","wallet_id":"wal-1"}`},
		{"detach with version", func() ([]byte, error) {
			return DetachPolicyPayload("wal-1", "pol-1", "key-1", WithPolicyVersion(7))
		},
			`{"idempotency_key":"key-1","policy_id":"pol-1","policy_version":7,"type":"detach_policy_from_wallet","wallet_id":"wal-1"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.got()
			if err != nil {
				t.Fatal(err)
			}
			if got := string(b); got != tc.want {
				t.Fatalf("payload:\n got:  %s\n want: %s", got, tc.want)
			}
		})
	}
}

// TestPolicyPayloads_PolicyVersionBelowOne: the platform refuses a version
// below 1 with 400, and policy versions start at 1, so the SDK refuses to
// build such a payload rather than have the caller sign it.
func TestPolicyPayloads_PolicyVersionBelowOne(t *testing.T) {
	for _, v := range []int32{0, -1} {
		if _, err := AttachPolicyPayload("wal-1", "pol-1", "key-1", WithPolicyVersion(v)); err == nil || !strings.Contains(err.Error(), "policy version") {
			t.Fatalf("attach with version %d: err = %v, want a policy version error", v, err)
		}
		if _, err := DetachPolicyPayload("wal-1", "pol-1", "key-1", WithPolicyVersion(v)); err == nil || !strings.Contains(err.Error(), "policy version") {
			t.Fatalf("detach with version %d: err = %v, want a policy version error", v, err)
		}
	}
}
