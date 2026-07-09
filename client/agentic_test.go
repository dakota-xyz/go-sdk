package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeMember struct{ id, key string }

// newAgenticServer fakes the endpoints AttachUserToWallet / DetachUserFromWallet
// touch. *posted captures the member_key sent to the add endpoint; *deletedID
// captures the signer id sent to the delete endpoint (both empty if never called).
func newAgenticServer(t *testing.T, walletGroups []string, members []fakeMember, posted, deletedID *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/wallets/wlt_1/signer-groups":
			groups := make([]map[string]any, 0, len(walletGroups))
			for _, id := range walletGroups {
				groups = append(groups, map[string]any{"id": id})
			}
			_ = json.NewEncoder(w).Encode(groups)
		case r.Method == http.MethodGet && r.URL.Path == "/signer-groups/spend_grp":
			ms := make([]map[string]any, 0, len(members))
			for _, m := range members {
				ms = append(ms, map[string]any{"id": m.id, "public_key": m.key})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "spend_grp", "members": ms})
		case r.Method == http.MethodPost && r.URL.Path == "/signer-groups/spend_grp/signers":
			var b struct {
				MemberKey string `json:"member_key"`
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			*posted = b.MemberKey
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/signer-groups/spend_grp/signers/"):
			// The real platform requires an idempotency key on this DELETE; reject
			// without one so a missing key is a test failure, not a silent pass.
			if r.Header.Get("x-idempotency-key") == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"x-idempotency-key is required"}`))
				return
			}
			*deletedID = strings.TrimPrefix(r.URL.Path, "/signer-groups/spend_grp/signers/")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"spend_grp"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestAttachUserToWallet: adds the signer to an existing wallet group (no signature,
// no policy creation), idempotent when already a member, errors when the group
// isn't attached to the wallet.
func TestAttachUserToWallet(t *testing.T) {
	t.Parallel()
	signer, err := NewP256Signer()
	require.NoError(t, err)
	pub := signer.PublicKeyBase64()

	t.Run("adds signer to the group", func(t *testing.T) {
		var posted, deleted string
		srv := newAgenticServer(t, []string{"spend_grp"}, []fakeMember{{"sig_other", "someoneelse"}}, &posted, &deleted)
		defer srv.Close()
		c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
		require.NoError(t, err)

		already, err := c.AttachUserToWallet(context.Background(), "wlt_1", pub, "spend_grp")
		require.NoError(t, err)
		require.False(t, already)
		require.Equal(t, pub, posted, "signer key added to the group")
	})

	t.Run("idempotent when already a member", func(t *testing.T) {
		var posted, deleted string
		srv := newAgenticServer(t, []string{"spend_grp"}, []fakeMember{{"sig_self", pub}}, &posted, &deleted)
		defer srv.Close()
		c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
		require.NoError(t, err)

		already, err := c.AttachUserToWallet(context.Background(), "wlt_1", pub, "spend_grp")
		require.NoError(t, err)
		require.True(t, already)
		require.Empty(t, posted, "no add when already a member")
	})

	t.Run("errors when group not on wallet", func(t *testing.T) {
		var posted, deleted string
		srv := newAgenticServer(t, []string{"other_grp"}, nil, &posted, &deleted)
		defer srv.Close()
		c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
		require.NoError(t, err)

		_, err = c.AttachUserToWallet(context.Background(), "wlt_1", pub, "spend_grp")
		require.Error(t, err)
		require.Empty(t, posted)
	})
}

// TestAttachUserToWalletSuppliedIdempotencyKey: a caller-supplied idempotency key
// is what reaches the wire, so retrying the whole call (e.g. after a crash) dedupes
// server-side instead of sending a fresh key each attempt.
func TestAttachUserToWalletSuppliedIdempotencyKey(t *testing.T) {
	t.Parallel()
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/wallets/wlt_1/signer-groups":
			_, _ = w.Write([]byte(`[{"id":"spend_grp"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/signer-groups/spend_grp":
			_, _ = w.Write([]byte(`{"id":"spend_grp","members":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/signer-groups/spend_grp/signers":
			gotKey = r.Header.Get("x-idempotency-key")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)

	key := uuid.New()
	_, err = c.AttachUserToWallet(context.Background(), "wlt_1", "some_signer_key", "spend_grp", key)
	require.NoError(t, err)
	require.Equal(t, key.String(), gotKey, "the caller-supplied idempotency key must reach the wire")
}

// TestDetachUserFromWallet: removes the signer from the named group (resolving its
// key→id), idempotent when not a member, errors when the group isn't on the wallet.
func TestDetachUserFromWallet(t *testing.T) {
	t.Parallel()
	signer, err := NewP256Signer()
	require.NoError(t, err)
	pub := signer.PublicKeyBase64()

	t.Run("removes signer from the group", func(t *testing.T) {
		var posted, deleted string
		srv := newAgenticServer(t, []string{"spend_grp"}, []fakeMember{{"sig_self", pub}, {"sig_other", "someoneelse"}}, &posted, &deleted)
		defer srv.Close()
		c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
		require.NoError(t, err)

		was, err := c.DetachUserFromWallet(context.Background(), "wlt_1", pub, "spend_grp")
		require.NoError(t, err)
		require.True(t, was)
		require.Equal(t, "sig_self", deleted, "removed by the signer's resource id, not its key")
	})

	t.Run("idempotent when not a member", func(t *testing.T) {
		var posted, deleted string
		srv := newAgenticServer(t, []string{"spend_grp"}, []fakeMember{{"sig_other", "someoneelse"}}, &posted, &deleted)
		defer srv.Close()
		c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
		require.NoError(t, err)

		was, err := c.DetachUserFromWallet(context.Background(), "wlt_1", pub, "spend_grp")
		require.NoError(t, err)
		require.False(t, was)
		require.Empty(t, deleted, "no delete when not a member")
	})

	t.Run("errors when group not on wallet", func(t *testing.T) {
		var posted, deleted string
		srv := newAgenticServer(t, []string{"other_grp"}, nil, &posted, &deleted)
		defer srv.Close()
		c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
		require.NoError(t, err)

		_, err = c.DetachUserFromWallet(context.Background(), "wlt_1", pub, "spend_grp")
		require.Error(t, err)
		require.Empty(t, deleted)
	})
}
