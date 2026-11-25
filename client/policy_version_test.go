package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

// stalePolicyVersionBody is the platform's 409 for a stale policy_version on
// PUT/DELETE /policies/{policy_id}/wallets/{wallet_id}, as its public spec
// gives it (platform#2673, ENG-6512).
const stalePolicyVersionBody = `{"type":"https://docs.dakota.xyz/api-reference/errors#stale-policy-version","title":"Stale Policy Version","status":409,"detail":"The policy changed since the signatures were made. Read the policy's version and sign again with a new idempotency_key.","instance":"https://api.platform.dakota.xyz/policies/example-id/wallets/example-id","request_id":"req_01hzy6y7v8w9x0y1z2a3b4c5d6","reason":"stale_policy_version","code":"stale_policy_version"}`

// TestPolicyAttachDetach_PolicyVersionOnTheWire sends a signed attach and
// detach through the generated client: the intent carries policy_version only
// when it is set (so it matches what was signed), and the platform's 409 maps
// to ErrStalePolicyVersion.
func TestPolicyAttachDetach_PolicyVersionOnTheWire(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// The handler runs on the server's goroutine: it records what it saw
	// under a lock and answers, and the test goroutine asserts afterwards.
	var (
		mu  sync.Mutex
		got []map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body) // a bad body fails the assertions below
		mu.Lock()
		got = append(got, body)
		mu.Unlock()
		intent, _ := body["intent"].(map[string]any)
		if _, ok := intent["policy_version"]; ok {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, stalePolicyVersionBody)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)

	version := int32(3)
	params := &gen.UpsertPolicyWalletRelationshipParams{XIdempotencyKey: uuid.New()}

	var attach gen.EndorsedRequest
	require.NoError(t, attach.Intent.FromAttachPolicyToWalletIntent(gen.AttachPolicyToWalletIntent{
		Type: gen.AttachPolicyToWalletIntentTypeAttachPolicyToWallet, WalletId: "wal-1", PolicyId: "pol-1",
		IdempotencyKey: params.XIdempotencyKey.String(), PolicyVersion: &version,
	}))
	attach.Signatures = []string{"sig"}
	_, err = CheckResponse(c.Raw().UpsertPolicyWalletRelationshipWithResponse(ctx, "pol-1", "wal-1", params, attach))
	require.ErrorIs(t, err, ErrStalePolicyVersion)
	require.False(t, errors.Is(err, ErrStaleSignerGroupVersion))
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusConflict, apiErr.StatusCode)

	var detach gen.EndorsedRequest
	require.NoError(t, detach.Intent.FromDetachPolicyFromWalletIntent(gen.DetachPolicyFromWalletIntent{
		Type: gen.DetachPolicyFromWalletIntentTypeDetachPolicyFromWallet, WalletId: "wal-1", PolicyId: "pol-1",
		IdempotencyKey: params.XIdempotencyKey.String(),
	}))
	detach.Signatures = []string{"sig"}
	_, err = CheckResponse(c.Raw().DeletePolicyWalletRelationshipWithResponse(ctx, "pol-1", "wal-1",
		&gen.DeletePolicyWalletRelationshipParams{XIdempotencyKey: params.XIdempotencyKey}, detach))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, got, 2)
	attachSent, ok := got[0]["intent"].(map[string]any)
	require.True(t, ok, "attach body has no intent object: %v", got[0])
	detachSent, ok := got[1]["intent"].(map[string]any)
	require.True(t, ok, "detach body has no intent object: %v", got[1])
	require.Equal(t, float64(3), attachSent["policy_version"])
	require.NotContains(t, detachSent, "policy_version", "an intent without a version must not send the key")
}

// TestPolicyIntents_SentIntentMatchesSignedPayload: the caller gives the
// version twice, once to WithPolicyVersion (the signed bytes) and once as the
// generated intent's PolicyVersion (what is sent). The platform verifies the
// signature over the intent it receives, so the canonical form of the sent
// intent must be exactly the bytes the payload helper produced. This pins it
// for both intents, with and without a version, so a change to either side
// (the helper or the generated struct) fails here instead of at the platform.
func TestPolicyIntents_SentIntentMatchesSignedPayload(t *testing.T) {
	t.Parallel()
	version := int32(5)
	for _, tc := range []struct {
		name    string
		payload func() ([]byte, error)
		intent  any
	}{
		{"attach with version",
			func() ([]byte, error) {
				return AttachPolicyPayload("wal-1", "pol-1", "key-1", WithPolicyVersion(version))
			},
			gen.AttachPolicyToWalletIntent{Type: gen.AttachPolicyToWalletIntentTypeAttachPolicyToWallet, WalletId: "wal-1", PolicyId: "pol-1", IdempotencyKey: "key-1", PolicyVersion: &version}},
		{"attach without version",
			func() ([]byte, error) { return AttachPolicyPayload("wal-1", "pol-1", "key-1") },
			gen.AttachPolicyToWalletIntent{Type: gen.AttachPolicyToWalletIntentTypeAttachPolicyToWallet, WalletId: "wal-1", PolicyId: "pol-1", IdempotencyKey: "key-1"}},
		{"detach with version",
			func() ([]byte, error) {
				return DetachPolicyPayload("wal-1", "pol-1", "key-1", WithPolicyVersion(version))
			},
			gen.DetachPolicyFromWalletIntent{Type: gen.DetachPolicyFromWalletIntentTypeDetachPolicyFromWallet, WalletId: "wal-1", PolicyId: "pol-1", IdempotencyKey: "key-1", PolicyVersion: &version}},
		{"detach without version",
			func() ([]byte, error) { return DetachPolicyPayload("wal-1", "pol-1", "key-1") },
			gen.DetachPolicyFromWalletIntent{Type: gen.DetachPolicyFromWalletIntentTypeDetachPolicyFromWallet, WalletId: "wal-1", PolicyId: "pol-1", IdempotencyKey: "key-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			signed, err := tc.payload()
			require.NoError(t, err)
			sent, err := json.Marshal(tc.intent)
			require.NoError(t, err)
			sentCanonical, err := jsoncanonicalizer.Transform(sent)
			require.NoError(t, err)
			require.Equal(t, string(signed), string(sentCanonical))
		})
	}
}

func TestErrStalePolicyVersionMatching(t *testing.T) {
	t.Parallel()
	require.True(t, errors.Is(&APIError{StatusCode: 409, Code: "stale-policy-version"}, ErrStalePolicyVersion))
	require.False(t, errors.Is(&APIError{StatusCode: 409, Code: "stale-signer-group-version"}, ErrStalePolicyVersion))
	require.False(t, errors.Is(&APIError{StatusCode: 400, Code: "stale_policy_version"}, ErrStalePolicyVersion))
}
