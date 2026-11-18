package client

// Signer-group governance intents. A membership or governance change of a
// signer group with governance settings (or of any group, for a client with
// governance enforcement on) carries signatures from the group's governance
// over a canonical intent. Policy-engine rebuilds that intent from the request
// and verifies the signatures over it, so the bytes signed here must equal its
// reconstruction exactly: RFC 8785 (JCS) JSON, keys sorted, no whitespace.
//
// TestGovernanceIntentGoldenVectors pins every builder against bytes produced by
// policy-engine's own intent.CanonicalIntentBytes.
//
// Two values inside every intent trip integrators up, so the builders check
// them:
//   - idempotency_key is the request's X-Idempotency-Key verbatim, and the
//     platform accepts only a lowercase canonical UUID there. A signature over
//     any other spelling can never be used.
//   - signer_group_version is the group's current `version` (always present,
//     including 0). It binds the signatures to the membership the endorsers
//     saw: after any change they are refused with stale_signer_group_version.

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// GovernanceIntentType is an intent's "type" key.
type GovernanceIntentType string

// Intent types, as they appear in the canonical intent's "type" key.
const (
	IntentAddSignersToGroup           GovernanceIntentType = "add_signers_to_group"
	IntentRemoveSignersFromGroup      GovernanceIntentType = "remove_signers_from_group"
	IntentUpdateSignerGroupGovernance GovernanceIntentType = "update_signer_group_governance"
	IntentCreateCryptoDestination     GovernanceIntentType = "create_crypto_destination"
)

// GovernanceSettings is a signer group's complete governance state, as sent to
// PATCH /signer-groups/{id}/governance. The request REPLACES the group's
// governance: a nil field clears that setting (no threshold means the default
// quorum; no governing group means the group governs itself), and the signed
// intent omits exactly the fields left nil.
type GovernanceSettings struct {
	// GovernanceThreshold is the number of distinct endorsers a governance
	// change of the group needs (at least 1), or nil for the default quorum.
	GovernanceThreshold *int32 `json:"governance_threshold,omitempty"`
	// GovernanceSignerGroupID is another signer group of the same client whose
	// members endorse governance changes of this group, or nil when the group
	// governs itself.
	GovernanceSignerGroupID *string `json:"governance_signer_group_id,omitempty"`
	// AllowClearing lets PrepareUpdateSignerGroupGovernance clear a setting
	// the group has now (by leaving it nil). Without it, clearing is refused,
	// so a threshold-only update cannot silently drop the governing group.
	// It is not sent or signed.
	AllowClearing bool `json:"-"`
}

func (s GovernanceSettings) clone() GovernanceSettings {
	if s.GovernanceThreshold != nil {
		t := *s.GovernanceThreshold
		s.GovernanceThreshold = &t
	}
	if s.GovernanceSignerGroupID != nil {
		g := *s.GovernanceSignerGroupID
		s.GovernanceSignerGroupID = &g
	}
	return s
}

// lowercaseUUID is the only idempotency-key spelling the platform accepts.
var lowercaseUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func checkIdempotencyKey(key string) error {
	if !lowercaseUUID.MatchString(key) {
		return fmt.Errorf("idempotency key %q must be a lowercase UUID (the platform refuses any other spelling, so a signature over it could never be used)", key)
	}
	return nil
}

func checkCommon(signerGroupID string, signerGroupVersion int64, idempotencyKey string) error {
	if signerGroupID == "" {
		return errors.New("signer group id is required")
	}
	if signerGroupVersion < 0 {
		return fmt.Errorf("signer group version must be >= 0, got %d", signerGroupVersion)
	}
	return checkIdempotencyKey(idempotencyKey)
}

// maxMembershipKeys is the most keys one membership change may name; the
// platform refuses more.
const maxMembershipKeys = 100

func checkKeys(what string, keys []string) error {
	if len(keys) == 0 {
		return fmt.Errorf("at least one %s is required", what)
	}
	if len(keys) > maxMembershipKeys {
		return fmt.Errorf("at most %d keys per change, got %d (the platform refuses more)", maxMembershipKeys, len(keys))
	}
	seen := make(map[string]bool, len(keys))
	for i, k := range keys {
		if k == "" {
			return fmt.Errorf("%s %d is empty", what, i)
		}
		if seen[k] {
			return fmt.Errorf("%s %d repeats an earlier key (the platform refuses duplicates)", what, i)
		}
		seen[k] = true
		// The platform trims an added key before rebuilding the intent, so a
		// key signed with surrounding whitespace never verifies.
		if strings.TrimSpace(k) != k {
			return fmt.Errorf("%s %d has leading or trailing whitespace", what, i)
		}
	}
	return nil
}

// AddSignersToGroupIntent returns the canonical bytes the group's governance
// signs to add members:
//
//	{"idempotency_key":…,"new_member_keys":[…],"signer_group_id":…,"signer_group_version":…,"type":"add_signers_to_group"}
//
// POST /signer-groups/{id}/signers adds one member, and the platform rebuilds
// the intent with exactly that key, so newMemberKeys must hold exactly one key
// (its member_key).
func AddSignersToGroupIntent(signerGroupID string, signerGroupVersion int64, idempotencyKey string, newMemberKeys []string) ([]byte, error) {
	if err := checkCommon(signerGroupID, signerGroupVersion, idempotencyKey); err != nil {
		return nil, err
	}
	if err := checkKeys("new member key", newMemberKeys); err != nil {
		return nil, err
	}
	if len(newMemberKeys) != 1 {
		return nil, fmt.Errorf("an add carries exactly one member key (the endpoint adds one member), got %d", len(newMemberKeys))
	}
	return canonicalJSON(map[string]any{
		"type":                 IntentAddSignersToGroup,
		"signer_group_id":      signerGroupID,
		"new_member_keys":      newMemberKeys,
		"idempotency_key":      idempotencyKey,
		"signer_group_version": signerGroupVersion,
	})
}

// RemoveSignersFromGroupIntent returns the canonical bytes the group's
// governance signs to remove members:
//
//	{"group_id":…,"idempotency_key":…,"members_to_remove":[…],"signer_group_version":…,"type":"remove_signers_from_group"}
//
// membersToRemove are public keys as the group lists them, and their order is
// signed: send them in the same order on POST /signer-groups/{id}/signer-removals.
// Note the key is "group_id" here, not "signer_group_id".
func RemoveSignersFromGroupIntent(signerGroupID string, signerGroupVersion int64, idempotencyKey string, membersToRemove []string) ([]byte, error) {
	if err := checkCommon(signerGroupID, signerGroupVersion, idempotencyKey); err != nil {
		return nil, err
	}
	if err := checkKeys("member to remove", membersToRemove); err != nil {
		return nil, err
	}
	return canonicalJSON(map[string]any{
		"type":                 IntentRemoveSignersFromGroup,
		"group_id":             signerGroupID,
		"members_to_remove":    membersToRemove,
		"idempotency_key":      idempotencyKey,
		"signer_group_version": signerGroupVersion,
	})
}

// UpdateSignerGroupGovernanceIntent returns the canonical bytes the group's
// CURRENT governance signs to replace its governance settings:
//
//	{"governance_signer_group_id":…,"governance_threshold":…,"idempotency_key":…,"signer_group_id":…,"signer_group_version":…,"type":"update_signer_group_governance"}
//
// with each setting present only when non-nil in settings (see
// GovernanceSettings: nil clears it).
func UpdateSignerGroupGovernanceIntent(signerGroupID string, signerGroupVersion int64, idempotencyKey string, settings GovernanceSettings) ([]byte, error) {
	if err := checkCommon(signerGroupID, signerGroupVersion, idempotencyKey); err != nil {
		return nil, err
	}
	m := map[string]any{
		"type":                 IntentUpdateSignerGroupGovernance,
		"signer_group_id":      signerGroupID,
		"idempotency_key":      idempotencyKey,
		"signer_group_version": signerGroupVersion,
	}
	if t := settings.GovernanceThreshold; t != nil {
		if *t < 1 {
			return nil, fmt.Errorf("governance threshold must be >= 1 (use nil for the default quorum), got %d", *t)
		}
		m["governance_threshold"] = *t
	}
	if g := settings.GovernanceSignerGroupID; g != nil {
		if *g == "" {
			return nil, errors.New("governing signer group id is empty (use nil to let the group govern itself)")
		}
		m["governance_signer_group_id"] = *g
	}
	return canonicalJSON(m)
}

// CreateCryptoDestinationIntent returns the canonical bytes the governance of
// signerGroupID signs to create a crypto destination for a recipient:
//
//	{"address":…,"idempotency_key":…,"network_id":…,"recipient_id":…,"signer_group_id":…,"type":"create_crypto_destination"}
//
// address is signed verbatim (an EVM checksum's mixed case included), so send
// the exact same string as crypto_address. With signatures, network_id is
// required on the request, and signerGroupID must be attached to a wallet of the
// recipient's customer.
func CreateCryptoDestinationIntent(recipientID, networkID, address, signerGroupID, idempotencyKey string) ([]byte, error) {
	switch {
	case recipientID == "":
		return nil, errors.New("recipient id is required")
	case networkID == "":
		return nil, errors.New("network id is required (the signed intent names it)")
	case address == "":
		return nil, errors.New("address is required")
	case signerGroupID == "":
		return nil, errors.New("signer group id is required")
	}
	if err := checkIdempotencyKey(idempotencyKey); err != nil {
		return nil, err
	}
	return canonicalJSON(map[string]any{
		"type":            IntentCreateCryptoDestination,
		"recipient_id":    recipientID,
		"network_id":      networkID,
		"address":         address,
		"signer_group_id": signerGroupID,
		"idempotency_key": idempotencyKey,
	})
}
