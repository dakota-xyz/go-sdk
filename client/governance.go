package client

// Signed signer-group membership and governance changes.
//
// A group with governance settings (and, for a client with governance
// enforcement on, every group) changes its members or its governance only with
// signatures from its governance: the members of its governing group
// (governance_signer_group_id, else the group itself), at least
// effective_governance_threshold distinct ones. The flow is always:
//
//  1. read the group's current version;
//  2. build the canonical intent with that version and a fresh lowercase UUID
//     idempotency key;
//  3. collect signatures over the intent (ES256 keys, passkeys, or both);
//  4. send the change with the same key as X-Idempotency-Key, the version and
//     the signatures.
//
// The *Signed helpers do all four in one call. When signatures come from
// different people at different times (a quorum), call Prepare*, hand the
// GovernanceIntent's Canonical bytes to each signer, and SubmitGovernanceIntent
// once enough signatures are in. A GovernanceIntent marshals to JSON, so it can
// be stored while signatures are collected.
//
// Any membership or governance change in between bumps the version and makes
// the signatures stale (ErrStaleSignerGroupVersion): prepare again and re-sign.
// The SDK never re-signs on its own, because re-signing with a passkey is a new
// user prompt.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

// Typed governance errors. Match them with errors.Is on an error returned by
// the helpers below (or by CheckResponse on a raw call); errors.As to *APIError
// gives the status, detail and request id.
var (
	// ErrEndorsementRequired (400): the change was sent without signatures,
	// but the group has governance settings or the client has governance
	// enforcement on. Send it signed.
	ErrEndorsementRequired = errors.New("endorsement_required: this change needs signatures from the group's governance")
	// ErrInsufficientEndorsements (403): the signatures did not meet the
	// group's governance quorum (too few distinct governing members verified).
	ErrInsufficientEndorsements = errors.New("insufficient_endorsements: the signatures do not meet the group's governance quorum")
	// ErrStaleSignerGroupVersion (409): the group changed since the
	// signatures were made. Read the group's version again and re-sign.
	ErrStaleSignerGroupVersion = errors.New("stale_signer_group_version: the group changed since the signatures were made; prepare and sign again")
	// ErrBelowThreshold (422): the removal would leave the group below a
	// threshold it must keep (at least one member, its governance quorum, or
	// a governed group's threshold).
	ErrBelowThreshold = errors.New("below_threshold: the removal would leave the group below a threshold it must keep")
)

var governanceErrorCodes = map[error]string{
	ErrEndorsementRequired:      "endorsement_required",
	ErrInsufficientEndorsements: "insufficient_endorsements",
	ErrStaleSignerGroupVersion:  "stale_signer_group_version",
	ErrBelowThreshold:           "below_threshold",
}

// Is lets errors.Is match an APIError against the typed governance errors
// (ErrStaleSignerGroupVersion and friends) by the platform's problem code.
func (e *APIError) Is(target error) bool {
	code, ok := governanceErrorCodes[target]
	if !ok || e == nil {
		return false
	}
	return slices.Contains(e.problemCodes(), code)
}

// problemCodes returns the error's machine codes in snake_case: Code (which
// for a problem+json body is the type URI's fragment, e.g.
// "stale-signer-group-version") and the body's "code" and "reason" fields.
func (e *APIError) problemCodes() []string {
	norm := func(s string) string { return strings.ReplaceAll(strings.ToLower(s), "-", "_") }
	codes := []string{norm(e.Code)}
	var body struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	if len(e.RawBody) > 0 && json.Unmarshal(e.RawBody, &body) == nil {
		codes = append(codes, norm(body.Code), norm(body.Reason))
	}
	return codes
}

// GovernanceIntent is a prepared, not yet submitted, signer-group change: the
// canonical bytes to sign and everything needed to send the matching request.
type GovernanceIntent struct {
	// Type is IntentAddSignersToGroup, IntentRemoveSignersFromGroup or
	// IntentUpdateSignerGroupGovernance.
	Type string `json:"type"`
	// SignerGroupID is the group being changed.
	SignerGroupID string `json:"signer_group_id"`
	// SignerGroupVersion is the group's version the signatures are made over.
	SignerGroupVersion int64 `json:"signer_group_version"`
	// IdempotencyKey is signed and sent as X-Idempotency-Key.
	IdempotencyKey string `json:"idempotency_key"`
	// MemberKeys is the member to add (one key) or the members to remove,
	// in the order signed and sent.
	MemberKeys []string `json:"member_keys,omitempty"`
	// Settings is the governance to set (update only).
	Settings GovernanceSettings `json:"settings"`
	// Canonical is what every signer signs (for a passkey, the challenge).
	Canonical []byte `json:"canonical"`
}

// canonical rebuilds the intent's canonical bytes from its fields.
func (g *GovernanceIntent) canonical() ([]byte, error) {
	switch g.Type {
	case IntentAddSignersToGroup:
		if len(g.MemberKeys) != 1 {
			return nil, fmt.Errorf("an add carries exactly one member key, got %d", len(g.MemberKeys))
		}
		return AddSignersToGroupIntent(g.SignerGroupID, g.SignerGroupVersion, g.IdempotencyKey, g.MemberKeys)
	case IntentRemoveSignersFromGroup:
		return RemoveSignersFromGroupIntent(g.SignerGroupID, g.SignerGroupVersion, g.IdempotencyKey, g.MemberKeys)
	case IntentUpdateSignerGroupGovernance:
		return UpdateSignerGroupGovernanceIntent(g.SignerGroupID, g.SignerGroupVersion, g.IdempotencyKey, g.Settings)
	default:
		return nil, fmt.Errorf("unsupported governance intent type %q", g.Type)
	}
}

// Sign collects one signature per signer over the intent, in signer order.
func (g *GovernanceIntent) Sign(ctx context.Context, signers ...IntentSigner) ([]string, error) {
	if len(signers) == 0 {
		return nil, errors.New("at least one signer is required")
	}
	if len(g.Canonical) == 0 {
		return nil, errors.New("governance intent has no canonical bytes; create it with a Prepare* method")
	}
	sigs := make([]string, 0, len(signers))
	for i, s := range signers {
		if s == nil {
			return nil, fmt.Errorf("signer %d is nil", i)
		}
		sig, err := s.SignIntent(ctx, g.Canonical)
		if err != nil {
			return nil, fmt.Errorf("signer %d: %w", i, err)
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

// prepare reads the group's current version and builds the intent around a
// fresh lowercase UUID key.
func (c *Client) prepare(ctx context.Context, g *GovernanceIntent) (*GovernanceIntent, error) {
	if g.SignerGroupID == "" {
		return nil, errors.New("signer group id is required")
	}
	resp, err := CheckResponse(c.Raw().GetSignerGroupWithResponse(ctx, g.SignerGroupID, nil))
	if err != nil {
		return nil, fmt.Errorf("read signer group version: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, errors.New("read signer group version: empty response body")
	}
	if resp.JSON200.Version == nil {
		return nil, errors.New("read signer group version: the platform did not return the group's version")
	}
	g.SignerGroupVersion = *resp.JSON200.Version
	g.IdempotencyKey = uuid.NewString() // lowercase canonical form
	if g.Canonical, err = g.canonical(); err != nil {
		return nil, err
	}
	return g, nil
}

// PrepareAddSignerToGroup prepares adding memberKey (a registered signer's
// public key) to the group: reads the group's version and builds the
// add_signers_to_group intent to sign.
func (c *Client) PrepareAddSignerToGroup(ctx context.Context, signerGroupID, memberKey string) (*GovernanceIntent, error) {
	return c.prepare(ctx, &GovernanceIntent{Type: IntentAddSignersToGroup, SignerGroupID: signerGroupID, MemberKeys: []string{memberKey}})
}

// PrepareRemoveSignersFromGroup prepares removing members (public keys as the
// group lists them, in the order given) from the group.
func (c *Client) PrepareRemoveSignersFromGroup(ctx context.Context, signerGroupID string, memberKeys []string) (*GovernanceIntent, error) {
	return c.prepare(ctx, &GovernanceIntent{Type: IntentRemoveSignersFromGroup, SignerGroupID: signerGroupID, MemberKeys: slices.Clone(memberKeys)})
}

// PrepareUpdateSignerGroupGovernance prepares replacing the group's governance
// settings (a nil field clears it; see GovernanceSettings). The group's
// CURRENT governance signs it.
func (c *Client) PrepareUpdateSignerGroupGovernance(ctx context.Context, signerGroupID string, settings GovernanceSettings) (*GovernanceIntent, error) {
	return c.prepare(ctx, &GovernanceIntent{Type: IntentUpdateSignerGroupGovernance, SignerGroupID: signerGroupID, Settings: settings})
}

// SubmitGovernanceIntent sends a prepared change with its signatures. It first
// rebuilds the canonical bytes from the intent's fields and refuses to send if
// they differ from intent.Canonical, so a request never goes out that does not
// match what was signed.
//
// Typed failures: ErrInsufficientEndorsements, ErrStaleSignerGroupVersion,
// ErrBelowThreshold (removals), ErrEndorsementRequired.
func (c *Client) SubmitGovernanceIntent(ctx context.Context, intent *GovernanceIntent, signatures []string) (*gen.SignerGroup, error) {
	if intent == nil {
		return nil, errors.New("nil governance intent")
	}
	if len(signatures) == 0 {
		return nil, errors.New("at least one signature is required")
	}
	rebuilt, err := intent.canonical()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rebuilt, intent.Canonical) {
		return nil, errors.New("governance intent fields do not match its signed canonical bytes; prepare and sign again")
	}
	key, err := uuid.Parse(intent.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("idempotency key: %w", err)
	}
	sigs := slices.Clone(signatures)
	version := intent.SignerGroupVersion

	var group *gen.SignerGroup
	switch intent.Type {
	case IntentAddSignersToGroup:
		resp, err := CheckResponse(c.Raw().CreateSignerGroupSignerWithResponse(ctx, intent.SignerGroupID,
			&gen.CreateSignerGroupSignerParams{XIdempotencyKey: key},
			gen.CreateSignerGroupSignerRequest{MemberKey: intent.MemberKeys[0], Signatures: &sigs, SignerGroupVersion: &version}))
		if err != nil {
			return nil, fmt.Errorf("add signer to group: %w", err)
		}
		group = resp.JSON200
	case IntentRemoveSignersFromGroup:
		resp, err := CheckResponse(c.Raw().RemoveSignerGroupSignersWithResponse(ctx, intent.SignerGroupID,
			&gen.RemoveSignerGroupSignersParams{XIdempotencyKey: key},
			gen.RemoveSignerGroupSignersRequest{MembersToRemove: slices.Clone(intent.MemberKeys), Signatures: sigs, SignerGroupVersion: version}))
		if err != nil {
			return nil, fmt.Errorf("remove signers from group: %w", err)
		}
		group = resp.JSON200
	case IntentUpdateSignerGroupGovernance:
		resp, err := CheckResponse(c.Raw().UpdateSignerGroupGovernanceWithResponse(ctx, intent.SignerGroupID,
			&gen.UpdateSignerGroupGovernanceParams{XIdempotencyKey: key},
			gen.UpdateSignerGroupGovernanceRequest{
				GovernanceThreshold:     intent.Settings.GovernanceThreshold,
				GovernanceSignerGroupId: intent.Settings.GovernanceSignerGroupID,
				Signatures:              sigs,
				SignerGroupVersion:      version,
			}))
		if err != nil {
			return nil, fmt.Errorf("update signer group governance: %w", err)
		}
		group = resp.JSON200
	}
	if group == nil {
		return nil, fmt.Errorf("%s: empty response body", intent.Type)
	}
	return group, nil
}

func (c *Client) prepareSignSubmit(ctx context.Context, prepare func() (*GovernanceIntent, error), signers []IntentSigner) (*gen.SignerGroup, error) {
	if len(signers) == 0 {
		return nil, errors.New("at least one signer is required (an unsigned change goes through the raw client)")
	}
	intent, err := prepare()
	if err != nil {
		return nil, err
	}
	sigs, err := intent.Sign(ctx, signers...)
	if err != nil {
		return nil, err
	}
	return c.SubmitGovernanceIntent(ctx, intent, sigs)
}

// AddSignerToGroupSigned adds memberKey to the group with signatures from
// signers, in one call: reads the version, builds and signs the intent, sends
// it. Pass as many signers as the group's effective_governance_threshold.
func (c *Client) AddSignerToGroupSigned(ctx context.Context, signerGroupID, memberKey string, signers ...IntentSigner) (*gen.SignerGroup, error) {
	return c.prepareSignSubmit(ctx, func() (*GovernanceIntent, error) {
		return c.PrepareAddSignerToGroup(ctx, signerGroupID, memberKey)
	}, signers)
}

// RemoveSignersFromGroupSigned removes memberKeys from the group with
// signatures from signers, in one call (POST /signer-groups/{id}/signer-removals).
func (c *Client) RemoveSignersFromGroupSigned(ctx context.Context, signerGroupID string, memberKeys []string, signers ...IntentSigner) (*gen.SignerGroup, error) {
	return c.prepareSignSubmit(ctx, func() (*GovernanceIntent, error) {
		return c.PrepareRemoveSignersFromGroup(ctx, signerGroupID, memberKeys)
	}, signers)
}

// UpdateSignerGroupGovernanceSigned replaces the group's governance settings
// with signatures from its current governance, in one call.
func (c *Client) UpdateSignerGroupGovernanceSigned(ctx context.Context, signerGroupID string, settings GovernanceSettings, signers ...IntentSigner) (*gen.SignerGroup, error) {
	return c.prepareSignSubmit(ctx, func() (*GovernanceIntent, error) {
		return c.PrepareUpdateSignerGroupGovernance(ctx, signerGroupID, settings)
	}, signers)
}
