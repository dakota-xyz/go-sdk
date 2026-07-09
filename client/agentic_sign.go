package client

// §8 mandate signing. Approving (or cancelling) a mandate is not a bare API
// call: it requires a cryptographic signature over the mandate's canonical
// bytes, proving the action came from a holder of the registered key. This file
// reproduces the platform's exact signed payload (RFC 8785 / JCS canonical JSON)
// and a P-256 signing/verification path, so partners never hand-roll JCS+ECDSA.
//
// The canonical bytes a signature covers — matched byte-for-byte with the
// platform's MandatePayload — are JCS JSON of:
//
//	{action, id, bound_signer, rule, valid_from, valid_until}
//
// signed as base64( ASN.1( ECDSA-P256( SHA-256(payload) ))), with the public
// key as base64 PKIX (SPKI).

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

// MandateAction is the §8 verb a signature authorizes. It is part of the signed
// payload so an approval and a cancellation can never be each other's replay: a
// signed cancel must not be byte-identical to a signed approve.
type MandateAction string

const (
	// MandateActionApprove authorizes a pending mandate → active.
	MandateActionApprove MandateAction = "approve"
	// MandateActionCancel authorizes a mandate's cancellation (pending → rejected,
	// or active → revoked; the outcome follows the mandate's current status).
	MandateActionCancel MandateAction = "cancel"
)

// Signer produces the §8 signature the platform verifies for a mandate
// approve/cancel. The caller holds the private key; the SDK never sees it.
//
// Implement this over your own key custody (HSM/KMS) for production. For
// sandbox and tests, P256Signer is a ready in-memory implementation.
type Signer interface {
	// PublicKeyBase64 returns the base64 PKIX (SPKI) P-256 public key — the exact
	// form the platform registered for this signer and verifies signatures
	// against.
	PublicKeyBase64() string
	// Sign returns a base64 ASN.1 ECDSA signature over SHA-256(payload).
	Sign(payload []byte) (string, error)
}

// MandateSignPayload reproduces, byte-for-byte, the canonical bytes the platform
// signs for one §8 action on a mandate. Build it straight from the mandate as
// returned by GetMandate/ListMandates — no server state is needed, and field
// order is irrelevant (JCS sorts). The result is what a Signer signs.
//
// Experimental: agentic payments is an alpha surface (x-alpha, flag-gated on
// the platform) and may change without a major-version bump.
func MandateSignPayload(m gen.Mandate, action MandateAction) ([]byte, error) {
	switch {
	case m.Id == nil || *m.Id == "":
		return nil, errors.New("mandate has no id")
	case m.BoundSignerId == nil || *m.BoundSignerId == "":
		return nil, errors.New("mandate has no bound_signer_id")
	case m.Rule == nil:
		return nil, errors.New("mandate has no rule")
	}
	// valid_from / valid_until are always present in the signed payload (even when
	// zero); the wire omits a zero value, so a nil pointer canonicalizes to 0 —
	// matching the platform, which signs the int64 directly.
	return canonicalJSON(map[string]any{
		"action":       string(action),
		"id":           *m.Id,
		"bound_signer": *m.BoundSignerId,
		"rule":         *m.Rule,
		"valid_from":   derefInt64(m.ValidFrom),
		"valid_until":  derefInt64(m.ValidUntil),
	})
}

// canonicalJSON is RFC 8785 (JCS): json.Marshal then jsoncanonicalizer.Transform.
// This is identical to the platform's enc.C14nJSON, so a payload built here
// matches the platform's bytes exactly.
func canonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	canonical, err := jsoncanonicalizer.Transform(raw)
	if err != nil {
		return nil, fmt.Errorf("canonicalize (JCS): %w", err)
	}
	return canonical, nil
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// AttachGroupPayload is the canonical bytes a recognized wallet signer endorses
// to attach a signer group to a wallet — JCS JSON of
// {type:"attach_group_to_wallet", wallet_id, group_id, idempotency_key}. The
// idempotency_key is part of the signed bytes, so reuse the SAME key on the
// attach request. Used in agent provisioning (the group attach the customer
// endorses).
func AttachGroupPayload(walletID, groupID, idempotencyKey string) ([]byte, error) {
	return canonicalJSON(map[string]any{
		"type":            "attach_group_to_wallet",
		"wallet_id":       walletID,
		"group_id":        groupID,
		"idempotency_key": idempotencyKey,
	})
}

// AttachPolicyPayload is the canonical bytes a recognized wallet signer endorses
// to attach a policy to a wallet — JCS JSON of
// {type:"attach_policy_to_wallet", wallet_id, policy_id, idempotency_key}.
func AttachPolicyPayload(walletID, policyID, idempotencyKey string) ([]byte, error) {
	return canonicalJSON(map[string]any{
		"type":            "attach_policy_to_wallet",
		"wallet_id":       walletID,
		"policy_id":       policyID,
		"idempotency_key": idempotencyKey,
	})
}

// The remaining endorsed-intent payloads complete the wallet/policy mutation
// surface alongside the attach helpers above. Each reproduces, byte-for-byte,
// the canonical map the platform's policy engine rebuilds to verify the
// endorsement (its intent reconstruction), so a signature produced here verifies
// server-side. As with the attach payloads, the idempotency_key is part of the
// signed bytes — reuse the SAME key on the corresponding request.

// DetachGroupPayload is the canonical bytes a recognized wallet signer endorses
// to remove a signer group from a wallet — the inverse of AttachGroupPayload.
// JCS JSON of {type:"detach_group_from_wallet", wallet_id, group_id, idempotency_key}.
func DetachGroupPayload(walletID, groupID, idempotencyKey string) ([]byte, error) {
	return canonicalJSON(map[string]any{
		"type":            "detach_group_from_wallet",
		"wallet_id":       walletID,
		"group_id":        groupID,
		"idempotency_key": idempotencyKey,
	})
}

// DetachPolicyPayload is the canonical bytes a recognized wallet signer endorses
// to remove a policy from a wallet — the inverse of AttachPolicyPayload.
// JCS JSON of {type:"detach_policy_from_wallet", wallet_id, policy_id, idempotency_key}.
func DetachPolicyPayload(walletID, policyID, idempotencyKey string) ([]byte, error) {
	return canonicalJSON(map[string]any{
		"type":            "detach_policy_from_wallet",
		"wallet_id":       walletID,
		"policy_id":       policyID,
		"idempotency_key": idempotencyKey,
	})
}

// DeletePolicyPayload is the canonical bytes a member of the policy's governing
// signer group endorses to delete a policy — JCS JSON of
// {type:"delete_policy", policy_id, idempotency_key}.
func DeletePolicyPayload(policyID, idempotencyKey string) ([]byte, error) {
	return canonicalJSON(map[string]any{
		"type":            "delete_policy",
		"policy_id":       policyID,
		"idempotency_key": idempotencyKey,
	})
}

// RemovePolicyRulePayload is the canonical bytes a member of the policy's
// governing signer group endorses to remove a rule from a policy — JCS JSON of
// {type:"remove_policy_rule", policy_id, rule_id, idempotency_key}.
func RemovePolicyRulePayload(policyID, ruleID, idempotencyKey string) ([]byte, error) {
	return canonicalJSON(map[string]any{
		"type":            "remove_policy_rule",
		"policy_id":       policyID,
		"rule_id":         ruleID,
		"idempotency_key": idempotencyKey,
	})
}

// UpdatePolicyRulePayload is the canonical bytes a member of the policy's
// governing signer group endorses to update a rule's definition — JCS JSON of
// {type:"update_policy_rule", policy_id, rule_id, updated_definition, idempotency_key}.
// updatedDefinition is the rule definition as a JSON STRING; it is signed
// opaquely, exactly as sent on the update request.
func UpdatePolicyRulePayload(policyID, ruleID, updatedDefinition, idempotencyKey string) ([]byte, error) {
	return canonicalJSON(map[string]any{
		"type":               "update_policy_rule",
		"policy_id":          policyID,
		"rule_id":            ruleID,
		"updated_definition": updatedDefinition,
		"idempotency_key":    idempotencyKey,
	})
}

// AddPolicyRulePayload is the canonical bytes a member of the policy's governing
// signer group endorses to add a rule to a policy — JCS JSON of
// {type:"add_policy_rule", action, rule_type, policy_id, definition, idempotency_key}.
// action (e.g. "allow"), ruleType (e.g. "amount_threshold"), and definition (the
// rule-type-specific config, in API format) must match the values sent on the
// add request, or the endorsement won't verify against it.
func AddPolicyRulePayload(policyID, action, ruleType string, definition map[string]any, idempotencyKey string) ([]byte, error) {
	return canonicalJSON(map[string]any{
		"type":            "add_policy_rule",
		"action":          action,
		"rule_type":       ruleType,
		"policy_id":       policyID,
		"definition":      definition,
		"idempotency_key": idempotencyKey,
	})
}

// ---------------------------------------------------------------------------
// P256Signer — default in-memory signer (sandbox / tests)
// ---------------------------------------------------------------------------

// P256Signer is a Signer holding a P-256 (ES256) private key in memory.
// Convenient for sandbox and tests; for production, implement Signer over your
// own key custody so the private key never lives in the process.
type P256Signer struct {
	priv *ecdsa.PrivateKey
}

// NewP256Signer generates a fresh P-256 keypair.
func NewP256Signer() (*P256Signer, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate P-256 key: %w", err)
	}
	return &P256Signer{priv: priv}, nil
}

// P256SignerFromKey adopts an existing P-256 private key (e.g. one you persisted
// for a hosted agent's registered signer).
func P256SignerFromKey(priv *ecdsa.PrivateKey) (*P256Signer, error) {
	if priv == nil || priv.Curve != elliptic.P256() {
		return nil, errors.New("a P-256 private key is required (ES256)")
	}
	return &P256Signer{priv: priv}, nil
}

// PublicKeyBase64 returns the base64 PKIX (SPKI) public key — pass this as the
// signer's registered public key at agent creation / mandate approval.
func (s *P256Signer) PublicKeyBase64() string {
	der, _ := x509.MarshalPKIXPublicKey(&s.priv.PublicKey) // P-256 PKIX marshalling does not error
	return base64.StdEncoding.EncodeToString(der)
}

// Sign returns base64( ASN.1 ECDSA signature over SHA-256(payload) ).
func (s *P256Signer) Sign(payload []byte) (string, error) {
	h := sha256.Sum256(payload)
	sig, err := ecdsa.SignASN1(rand.Reader, s.priv, h[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifyMandateSignature checks a base64 ASN.1 signature over SHA-256 of the
// canonical payload against a base64 PKIX P-256 public key — the platform's
// exact verification. Exposed so callers (and tests) can self-check a signature
// before submitting it.
func VerifyMandateSignature(publicKeyBase64 string, payload []byte, signatureBase64 string) error {
	der, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok || ecPub.Curve != elliptic.P256() {
		return errors.New("not a P-256 public key (ES256 requires P-256)")
	}
	sig, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	h := sha256.Sum256(payload)
	if !ecdsa.VerifyASN1(ecPub, h[:], sig) {
		return errors.New("signature verification failed")
	}
	return nil
}
