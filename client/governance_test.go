package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeGovernedGroup is a signer group under governance, enforced the way the
// platform and policy-engine do: the version must match, the intent is rebuilt
// from the request (body + X-Idempotency-Key) and every signature must verify
// over it with a governing member's key, at least threshold distinct ones.
type fakeGovernedGroup struct {
	t         *testing.T
	mu        sync.Mutex
	id        string
	version   int64
	members   []string
	governors map[string]bool // ES256 public keys allowed to endorse
	threshold int
	settings  GovernanceSettings
	// seen records each mutating request's header key and body.
	seen []fakeRequest
}

type fakeRequest struct {
	method, path, key string
	body              map[string]any
}

var lowerUUIDRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (g *fakeGovernedGroup) groupJSON() map[string]any {
	ms := make([]map[string]any, 0, len(g.members))
	for i, k := range g.members {
		ms = append(ms, map[string]any{"id": "sig_" + string(rune('a'+i)), "public_key": k, "key_type": "ES256", "name": "m"})
	}
	return map[string]any{
		"id": g.id, "client_id": "cli", "name": "governed", "members": ms,
		"version": g.version, "effective_governance_threshold": g.threshold,
		"governance_threshold": g.settings.GovernanceThreshold, "governance_signer_group_id": g.settings.GovernanceSignerGroupID,
	}
}

func problem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":  "https://docs.dakota.xyz/api-reference/errors#" + regexp.MustCompile(`_`).ReplaceAllString(code, "-"),
		"title": "x", "status": status, "detail": detail, "code": code, "reason": code,
	})
}

func (g *fakeGovernedGroup) handler(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	base := "/signer-groups/" + g.id
	if r.Method == http.MethodGet && r.URL.Path == base {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(g.groupJSON())
		return
	}
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	key := r.Header.Get("X-Idempotency-Key")
	g.seen = append(g.seen, fakeRequest{r.Method, r.URL.Path, key, body})
	if !lowerUUIDRe.MatchString(key) {
		problem(w, 400, "invalid_request", "X-Idempotency-Key must be a lowercase UUID")
		return
	}
	sigs, _ := body["signatures"].([]any)
	if len(sigs) == 0 {
		problem(w, 400, "endorsement_required", "this group has governance settings")
		return
	}
	v, _ := body["signer_group_version"].(float64)
	if int64(v) != g.version {
		problem(w, 409, "stale_signer_group_version", "The signer group changed since the signatures were made.")
		return
	}

	var intent []byte
	var err error
	switch {
	case r.Method == http.MethodPost && r.URL.Path == base+"/signers":
		intent, err = AddSignersToGroupIntent(g.id, g.version, key, []string{body["member_key"].(string)})
	case r.Method == http.MethodPost && r.URL.Path == base+"/signer-removals":
		var rm []string
		for _, m := range body["members_to_remove"].([]any) {
			rm = append(rm, m.(string))
		}
		intent, err = RemoveSignersFromGroupIntent(g.id, g.version, key, rm)
	case r.Method == http.MethodPatch && r.URL.Path == base+"/governance":
		var s GovernanceSettings
		if t, ok := body["governance_threshold"].(float64); ok {
			n := int32(t)
			s.GovernanceThreshold = &n
		}
		if id, ok := body["governance_signer_group_id"].(string); ok {
			s.GovernanceSignerGroupID = &id
		}
		intent, err = UpdateSignerGroupGovernanceIntent(g.id, g.version, key, s)
	default:
		g.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(404)
		return
	}
	if err != nil {
		g.t.Errorf("fake could not rebuild the intent: %v", err)
		w.WriteHeader(500)
		return
	}

	endorsers := map[string]bool{}
	for _, s := range sigs {
		for pub := range g.governors {
			if VerifyMandateSignature(pub, intent, s.(string)) == nil {
				endorsers[pub] = true
			}
		}
	}
	if len(endorsers) < g.threshold {
		problem(w, 403, "insufficient_endorsements", "insufficient endorsements")
		return
	}

	switch r.URL.Path {
	case base + "/signers":
		g.members = append(g.members, body["member_key"].(string))
	case base + "/signer-removals":
		for _, m := range body["members_to_remove"].([]any) {
			g.members = slices.DeleteFunc(g.members, func(k string) bool { return k == m.(string) })
		}
		if len(g.members) == 0 {
			problem(w, 422, "below_threshold", "the removal would leave the group without members")
			return
		}
	case base + "/governance":
		g.settings = GovernanceSettings{}
		if t, ok := body["governance_threshold"].(float64); ok {
			n := int32(t)
			g.settings.GovernanceThreshold = &n
		}
		if id, ok := body["governance_signer_group_id"].(string); ok {
			g.settings.GovernanceSignerGroupID = &id
		}
	}
	g.version++
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(g.groupJSON())
}

func newGovernedGroup(t *testing.T, threshold int, governors ...*P256Signer) (*fakeGovernedGroup, *Client) {
	t.Helper()
	g := &fakeGovernedGroup{t: t, id: "3MF5pRlAIRFYkOAd3TstqUKMOpp", threshold: threshold, governors: map[string]bool{}}
	for _, s := range governors {
		g.governors[s.PublicKeyBase64()] = true
		g.members = append(g.members, s.PublicKeyBase64())
	}
	srv := httptest.NewServer(http.HandlerFunc(g.handler))
	t.Cleanup(srv.Close)
	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)
	return g, c
}

func newKeys(t *testing.T, n int) []*P256Signer {
	t.Helper()
	out := make([]*P256Signer, n)
	for i := range out {
		s, err := NewP256Signer()
		require.NoError(t, err)
		out[i] = s
	}
	return out
}

func TestSignedMembershipChanges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("add then remove, one endorser", func(t *testing.T) {
		keys := newKeys(t, 3)
		g, c := newGovernedGroup(t, 1, keys[0], keys[1])
		newMember := keys[2].PublicKeyBase64()

		grp, err := c.AddSignerToGroupSigned(ctx, g.id, newMember, ES256IntentSigner(keys[0]))
		require.NoError(t, err)
		require.Equal(t, int64(1), *grp.Version)
		require.Len(t, grp.Members, 3)

		grp, err = c.RemoveSignersFromGroupSigned(ctx, g.id, []string{newMember}, ES256IntentSigner(keys[1]))
		require.NoError(t, err)
		require.Equal(t, int64(2), *grp.Version)
		require.Len(t, grp.Members, 2)

		// The key in the signed intent is the header key, a fresh lowercase
		// UUID per change, and each request carried the version it signed.
		require.Len(t, g.seen, 2)
		require.NotEqual(t, g.seen[0].key, g.seen[1].key)
		require.EqualValues(t, 0, g.seen[0].body["signer_group_version"])
		require.EqualValues(t, 1, g.seen[1].body["signer_group_version"])
	})

	t.Run("quorum of two needs both signers", func(t *testing.T) {
		keys := newKeys(t, 3)
		g, c := newGovernedGroup(t, 2, keys[0], keys[1])

		// The same governor twice is two signatures but one endorser: the
		// SDK's count check passes and the platform refuses.
		_, err := c.AddSignerToGroupSigned(ctx, g.id, keys[2].PublicKeyBase64(), ES256IntentSigner(keys[0]), ES256IntentSigner(keys[0]))
		require.ErrorIs(t, err, ErrInsufficientEndorsements)

		grp, err := c.AddSignerToGroupSigned(ctx, g.id, keys[2].PublicKeyBase64(), ES256IntentSigner(keys[0]), ES256IntentSigner(keys[1]))
		require.NoError(t, err)
		require.Len(t, grp.Members, 3)
	})

	t.Run("signatures collected separately, then submitted", func(t *testing.T) {
		keys := newKeys(t, 3)
		g, c := newGovernedGroup(t, 2, keys[0], keys[1])

		intent, err := c.PrepareRemoveSignersFromGroup(ctx, g.id, []string{keys[1].PublicKeyBase64()})
		require.NoError(t, err)
		require.Equal(t, IntentRemoveSignersFromGroup, intent.Type)

		// The prepared intent survives a round trip through storage (e.g.
		// while waiting for the second person to sign).
		stored, err := json.Marshal(intent)
		require.NoError(t, err)
		var loaded GovernanceIntent
		require.NoError(t, json.Unmarshal(stored, &loaded))

		s1, err := loaded.Sign(ctx, ES256IntentSigner(keys[0]))
		require.NoError(t, err)
		s2, err := ES256IntentSigner(keys[1]).SignIntent(ctx, loaded.Canonical)
		require.NoError(t, err)

		grp, err := c.SubmitGovernanceIntent(ctx, &loaded, append(s1, s2))
		require.NoError(t, err)
		require.Len(t, grp.Members, 1)
		require.Equal(t, intent.IdempotencyKey, g.seen[0].key)
	})

	t.Run("a change in between makes the signatures stale", func(t *testing.T) {
		keys := newKeys(t, 4)
		g, c := newGovernedGroup(t, 1, keys[0], keys[1])

		intent, err := c.PrepareAddSignerToGroup(ctx, g.id, keys[2].PublicKeyBase64())
		require.NoError(t, err)
		sigs, err := intent.Sign(ctx, ES256IntentSigner(keys[0]))
		require.NoError(t, err)

		_, err = c.AddSignerToGroupSigned(ctx, g.id, keys[3].PublicKeyBase64(), ES256IntentSigner(keys[1]))
		require.NoError(t, err)

		_, err = c.SubmitGovernanceIntent(ctx, intent, sigs)
		require.ErrorIs(t, err, ErrStaleSignerGroupVersion)
		var apiErr *APIError
		require.ErrorAs(t, err, &apiErr)
		require.Equal(t, 409, apiErr.StatusCode)
	})

	t.Run("removing the last member is below threshold", func(t *testing.T) {
		keys := newKeys(t, 1)
		g, c := newGovernedGroup(t, 1, keys[0])
		_, err := c.RemoveSignersFromGroupSigned(ctx, g.id, []string{keys[0].PublicKeyBase64()}, ES256IntentSigner(keys[0]))
		require.ErrorIs(t, err, ErrBelowThreshold)
	})

	t.Run("governance update replaces the settings", func(t *testing.T) {
		keys := newKeys(t, 2)
		g, c := newGovernedGroup(t, 1, keys[0], keys[1])
		two := int32(2)

		grp, err := c.UpdateSignerGroupGovernanceSigned(ctx, g.id, GovernanceSettings{GovernanceThreshold: &two}, ES256IntentSigner(keys[0]))
		require.NoError(t, err)
		require.Equal(t, int32(2), *grp.GovernanceThreshold)
		require.Nil(t, grp.GovernanceSignerGroupId)
		// An unset setting is absent from the body, matching the signed intent.
		require.NotContains(t, g.seen[0].body, "governance_signer_group_id")

		grp, err = c.UpdateSignerGroupGovernanceSigned(ctx, g.id, GovernanceSettings{AllowClearing: true}, ES256IntentSigner(keys[0]), ES256IntentSigner(keys[1]))
		require.NoError(t, err)
		require.Nil(t, grp.GovernanceThreshold)
		require.NotContains(t, g.seen[1].body, "governance_threshold")
	})

	t.Run("a tampered prepared intent is refused before sending", func(t *testing.T) {
		keys := newKeys(t, 3)
		g, c := newGovernedGroup(t, 1, keys[0])
		intent, err := c.PrepareAddSignerToGroup(ctx, g.id, keys[1].PublicKeyBase64())
		require.NoError(t, err)
		sigs, err := intent.Sign(ctx, ES256IntentSigner(keys[0]))
		require.NoError(t, err)

		intent.MemberKeys = []string{keys[2].PublicKeyBase64()}
		_, err = c.SubmitGovernanceIntent(ctx, intent, sigs)
		require.Error(t, err)
		require.Empty(t, g.seen, "nothing may reach the platform")
	})

	t.Run("no signers is refused before sending", func(t *testing.T) {
		keys := newKeys(t, 2)
		g, c := newGovernedGroup(t, 1, keys[0])
		_, err := c.AddSignerToGroupSigned(ctx, g.id, keys[1].PublicKeyBase64())
		require.Error(t, err)
		require.Empty(t, g.seen)
	})
}

func TestGovernanceIntentGuards(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("Sign refuses canonical bytes that disagree with the fields", func(t *testing.T) {
		keys := newKeys(t, 3)
		g, c := newGovernedGroup(t, 1, keys[0])
		intent, err := c.PrepareAddSignerToGroup(ctx, g.id, keys[1].PublicKeyBase64())
		require.NoError(t, err)

		// A relay swaps the bytes the signers see for another change.
		other, err := AddSignersToGroupIntent(g.id, intent.SignerGroupVersion, intent.IdempotencyKey, []string{keys[2].PublicKeyBase64()})
		require.NoError(t, err)
		intent.Canonical = other
		_, err = intent.Sign(ctx, ES256IntentSigner(keys[0]))
		require.ErrorContains(t, err, "do not match its canonical bytes")
	})

	t.Run("Submit refuses fewer signatures than the quorum", func(t *testing.T) {
		keys := newKeys(t, 3)
		g, c := newGovernedGroup(t, 2, keys[0], keys[1])
		intent, err := c.PrepareAddSignerToGroup(ctx, g.id, keys[2].PublicKeyBase64())
		require.NoError(t, err)
		require.Equal(t, 2, intent.RequiredSignatures)
		sigs, err := intent.Sign(ctx, ES256IntentSigner(keys[0]))
		require.NoError(t, err)
		_, err = c.SubmitGovernanceIntent(ctx, intent, sigs)
		require.ErrorContains(t, err, "needs 2 signatures, got 1")
		require.Empty(t, g.seen, "nothing may reach the platform")
	})

	t.Run("Submit refuses a stored intent of another format", func(t *testing.T) {
		keys := newKeys(t, 2)
		g, c := newGovernedGroup(t, 1, keys[0])
		intent, err := c.PrepareAddSignerToGroup(ctx, g.id, keys[1].PublicKeyBase64())
		require.NoError(t, err)
		sigs, err := intent.Sign(ctx, ES256IntentSigner(keys[0]))
		require.NoError(t, err)
		intent.Format = 0
		_, err = c.SubmitGovernanceIntent(ctx, intent, sigs)
		require.ErrorContains(t, err, "prepare it again")
	})

	t.Run("a threshold-only update does not silently drop the governing group", func(t *testing.T) {
		keys := newKeys(t, 2)
		g, c := newGovernedGroup(t, 1, keys[0], keys[1])
		other := "3MF5pokUeizBfGkQF0N4qhUgRve"
		g.settings.GovernanceSignerGroupID = &other
		three := int32(1)

		_, err := c.UpdateSignerGroupGovernanceSigned(ctx, g.id, GovernanceSettings{GovernanceThreshold: &three}, ES256IntentSigner(keys[0]))
		require.ErrorContains(t, err, "would clear the group's governing group")
		require.Empty(t, g.seen)

		grp, err := c.UpdateSignerGroupGovernanceSigned(ctx, g.id, GovernanceSettings{GovernanceThreshold: &three, AllowClearing: true}, ES256IntentSigner(keys[0]))
		require.NoError(t, err)
		require.Nil(t, grp.GovernanceSignerGroupId)
	})

	t.Run("a group cannot name itself as its governing group", func(t *testing.T) {
		keys := newKeys(t, 1)
		g, c := newGovernedGroup(t, 1, keys[0])
		self := g.id
		_, err := c.UpdateSignerGroupGovernanceSigned(ctx, g.id, GovernanceSettings{GovernanceSignerGroupID: &self}, ES256IntentSigner(keys[0]))
		require.ErrorContains(t, err, "cannot name itself")
	})

	t.Run("bad input is refused before the group is read", func(t *testing.T) {
		keys := newKeys(t, 1)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}))
		defer srv.Close()
		c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
		require.NoError(t, err)
		_, err = c.AddSignerToGroupSigned(ctx, "g", " pk", ES256IntentSigner(keys[0]))
		require.ErrorContains(t, err, "whitespace")
		_, err = c.RemoveSignersFromGroupSigned(ctx, "", []string{"pk"}, ES256IntentSigner(keys[0]))
		require.ErrorContains(t, err, "signer group id is required")
	})

	for name, tc := range map[string]struct {
		status int
		body   string
		want   string
	}{
		"group not found":    {404, `{"type":"https://docs.dakota.xyz/api-reference/errors#not-found","title":"Signer Group not found","status":404}`, "read signer group"},
		"no version on read": {200, `{"id":"g","client_id":"c","name":"n","members":[]}`, "did not return the group's version"},
	} {
		t.Run("prepare fails: "+name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
			require.NoError(t, err)
			_, err = c.PrepareAddSignerToGroup(ctx, "g", "pk")
			require.ErrorContains(t, err, tc.want)
		})
	}
}

// unhashableErr is a comparable-looking error whose dynamic type cannot be a
// map key (it holds a slice), like validator-style multi-errors.
type unhashableErr struct{ fields []string }

func (u unhashableErr) Error() string { return "invalid" }

func TestAPIErrorIsMatching(t *testing.T) {
	t.Parallel()
	stale := &APIError{StatusCode: 409, Code: "stale-signer-group-version"}

	require.True(t, errors.Is(stale, ErrStaleSignerGroupVersion))
	require.False(t, errors.Is(stale, ErrInsufficientEndorsements))
	// Must not panic on an unhashable target, and must not match it.
	require.NotPanics(t, func() { require.False(t, errors.Is(stale, unhashableErr{[]string{"x"}})) })
	require.False(t, errors.Is(stale, io.EOF))
	// The code alone is not enough: the status must match too.
	require.False(t, errors.Is(&APIError{StatusCode: 400, Code: "stale_signer_group_version"}, ErrStaleSignerGroupVersion))
	// A body "code" is read when the type fragment is something else.
	require.True(t, errors.Is(&APIError{StatusCode: 403, Code: "forbidden", RawBody: []byte(`{"code":"insufficient_endorsements"}`)}, ErrInsufficientEndorsements))
	// An ordinary conflict is not a stale version.
	require.False(t, errors.Is(&APIError{StatusCode: 409, Code: "conflict", RawBody: []byte(`{"code":"conflict","reason":"stale_signer_group_version"}`)}, ErrStaleSignerGroupVersion))
	// Wrapped, as the helpers return it.
	require.ErrorIs(t, fmt.Errorf("add signer to group: %w", stale), ErrStaleSignerGroupVersion)
}

// TestGovernanceErrorsFromPlatformBodies maps the problem bodies the sandbox
// returned on 2026-11-17 (ENG-6464) to the typed errors.
func TestGovernanceErrorsFromPlatformBodies(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status int
		body   string
		want   error
	}{
		{400, `{"code":"endorsement_required","detail":"This signer group has governance settings, so its membership changes need signatures.","instance":"/signer-groups/3MF5pRlAIRFYkOAd3TstqUKMOpp/signers","reason":"endorsement_required","request_id":"3MF5pYTBJTR1rdhkX6hLO8XoxlD","status":400,"title":"Endorsement Required","type":"https://docs.dakota.xyz/api-reference/errors#endorsement-required"}`, ErrEndorsementRequired},
		{403, `{"type":"https://docs.dakota.xyz/api-reference/errors#insufficient-endorsements","title":"Insufficient Endorsements","status":403,"detail":"insufficient endorsements: have 1, need 2","instance":"/signer-groups/3MF5pokUeizBfGkQF0N4qhUgRve/signer-removals","request_id":"3MF5pnjey2Hg8YrCPfYRNUiCu1T","reason":"insufficient_endorsements","code":"insufficient_endorsements"}`, ErrInsufficientEndorsements},
		{409, `{"type":"https://docs.dakota.xyz/api-reference/errors#stale-signer-group-version","title":"Stale Signer Group Version","status":409,"detail":"The signer group changed since the signatures were made. Read the group's version and sign again.","instance":"/signer-groups/3MF5pRlAIRFYkOAd3TstqUKMOpp/signers","request_id":"3MF5piozymuwCnu8h4Ponbyj9Fm","reason":"stale_signer_group_version","code":"stale_signer_group_version"}`, ErrStaleSignerGroupVersion},
		{422, `{"type":"https://docs.dakota.xyz/api-reference/errors#below-threshold","title":"Below Threshold","status":422,"detail":"the removal would leave the group below its governance quorum","reason":"below_threshold","code":"below_threshold"}`, ErrBelowThreshold},
	}
	all := []error{ErrEndorsementRequired, ErrInsufficientEndorsements, ErrStaleSignerGroupVersion, ErrBelowThreshold}
	for _, tc := range cases {
		t.Run(tc.want.Error(), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"id":"g","client_id":"c","name":"n","members":[],"version":0}`))
					return
				}
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
			require.NoError(t, err)
			key, err := NewP256Signer()
			require.NoError(t, err)

			_, err = c.RemoveSignersFromGroupSigned(context.Background(), "g", []string{"pk"}, ES256IntentSigner(key))
			require.ErrorIs(t, err, tc.want)
			for _, other := range all {
				if other != tc.want {
					require.False(t, errors.Is(err, other), "also matched %v", other)
				}
			}
		})
	}
}
