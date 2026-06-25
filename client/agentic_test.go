package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAuthorizeAgentOnWalletWithDefaultPolicy verifies the one-call spend
// authorization against a fake platform: it must create a solo signer group with
// the agent's key, create a DEFAULT allow policy, and perform the two
// customer-endorsed attaches whose signatures verify byte-exactly over the
// canonical attach payloads (idempotency key consistent across payload, intent,
// and header).
func TestAuthorizeAgentOnWalletWithDefaultPolicy(t *testing.T) {
	t.Parallel()

	walletSigner, err := NewP256Signer() // the recognized wallet signer (endorser)
	require.NoError(t, err)
	agentSigner, err := NewP256Signer() // only its public key matters here
	require.NoError(t, err)
	agentPub := agentSigner.PublicKeyBase64()

	var (
		groupCreate, policyCreate map[string]any
		endorsed                  = map[string]struct {
			Signatures []string       `json:"signatures"`
			Intent     map[string]any `json:"intent"`
		}{}
		idemHeader = map[string]string{}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/signer-groups":
			_ = json.NewDecoder(r.Body).Decode(&groupCreate)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"grp_1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/policies":
			_ = json.NewDecoder(r.Body).Decode(&policyCreate)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"pol_1"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/wallets/wlt_1/signer-groups/grp_1":
			var b struct {
				Signatures []string       `json:"signatures"`
				Intent     map[string]any `json:"intent"`
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			endorsed["group"] = b
			idemHeader["group"] = r.Header.Get("X-Idempotency-Key")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPut && r.URL.Path == "/policies/pol_1/wallets/wlt_1":
			var b struct {
				Signatures []string       `json:"signatures"`
				Intent     map[string]any `json:"intent"`
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			endorsed["policy"] = b
			idemHeader["policy"] = r.Header.Get("X-Idempotency-Key")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)

	auth, err := c.AuthorizeAgentOnWalletWithDefaultPolicy(context.Background(), "wlt_1", "Billpay", agentPub, walletSigner)
	require.NoError(t, err)
	require.Equal(t, "grp_1", auth.SignerGroupID)
	require.Equal(t, "pol_1", auth.PolicyID)

	// Group created holding the agent's signer key.
	require.Equal(t, []any{agentPub}, groupCreate["member_keys"])

	// Default policy carries exactly one ALLOW approval-threshold rule.
	rules, ok := policyCreate["rules"].([]any)
	require.True(t, ok)
	require.Len(t, rules, 1)
	rule := rules[0].(map[string]any)
	require.Equal(t, "allow", rule["action"])
	require.Equal(t, "approval_threshold", rule["rule_type"])

	// Each endorsed attach: signature verifies byte-exactly over the canonical
	// payload, and the idempotency key is consistent across intent + header.
	gIdem, _ := endorsed["group"].Intent["idempotency_key"].(string)
	require.NotEmpty(t, gIdem)
	require.Equal(t, gIdem, idemHeader["group"])
	require.Len(t, endorsed["group"].Signatures, 1)
	gPayload, err := AttachGroupPayload("wlt_1", "grp_1", gIdem)
	require.NoError(t, err)
	require.NoError(t, VerifyMandateSignature(walletSigner.PublicKeyBase64(), gPayload, endorsed["group"].Signatures[0]),
		"group attach endorsement must verify over the canonical payload")

	pIdem, _ := endorsed["policy"].Intent["idempotency_key"].(string)
	require.NotEmpty(t, pIdem)
	require.Equal(t, pIdem, idemHeader["policy"])
	require.Len(t, endorsed["policy"].Signatures, 1)
	pPayload, err := AttachPolicyPayload("wlt_1", "pol_1", pIdem)
	require.NoError(t, err)
	require.NoError(t, VerifyMandateSignature(walletSigner.PublicKeyBase64(), pPayload, endorsed["policy"].Signatures[0]),
		"policy attach endorsement must verify over the canonical payload")
}
