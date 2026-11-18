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
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

// Typed governance errors. Match them with errors.Is on an error returned by
// the helpers below (or by CheckResponse on a raw call); errors.As to *APIError
// gives the detail and request id.
var (
	// ErrEndorsementRequired (400): the change was sent without signatures,
	// but the group has governance settings or the client has governance
	// enforcement on. Send it signed.
	ErrEndorsementRequired = &ProblemCodeError{Status: 400, Code: "endorsement_required", Message: "this change needs signatures from the group's governance"}
	// ErrInsufficientEndorsements (403): the signatures did not meet the
	// group's governance quorum (too few distinct governing members verified).
	ErrInsufficientEndorsements = &ProblemCodeError{Status: 403, Code: "insufficient_endorsements", Message: "the signatures do not meet the group's governance quorum"}
	// ErrStaleSignerGroupVersion (409): the group changed since the
	// signatures were made. Prepare again and re-sign.
	ErrStaleSignerGroupVersion = &ProblemCodeError{Status: 409, Code: "stale_signer_group_version", Message: "the group changed since the signatures were made; prepare and sign again"}
	// ErrBelowThreshold (422): the removal would leave the group below a
	// threshold it must keep (at least one member, its governance quorum, or
	// a governed group's threshold).
	ErrBelowThreshold = &ProblemCodeError{Status: 422, Code: "below_threshold", Message: "the removal would leave the group below a threshold it must keep"}
)

// GovernanceIntentFormat is the version of GovernanceIntent's JSON form. A
// stored intent in another format is refused: prepare it again.
const GovernanceIntentFormat = 1

// GovernanceIntent is a prepared, not yet submitted, signer-group change: the
// canonical bytes to sign and everything needed to send the matching request.
// It marshals to JSON (format GovernanceIntentFormat), so it can be stored
// while a quorum signs. An SDK upgrade between prepare and submit may require
// preparing again.
type GovernanceIntent struct {
	// Format is GovernanceIntentFormat.
	Format int `json:"format"`
	// Type is IntentAddSignersToGroup, IntentRemoveSignersFromGroup or
	// IntentUpdateSignerGroupGovernance.
	Type GovernanceIntentType `json:"type"`
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
	Settings GovernanceSettings `json:"settings,omitzero"`
	// RequiredSignatures is the group's effective_governance_threshold when
	// the change was prepared: the fewest signatures Submit will send.
	RequiredSignatures int `json:"required_signatures"`
	// Canonical is what every signer signs (for a passkey, the challenge).
	Canonical []byte `json:"canonical"`
}

// canonical rebuilds the intent's canonical bytes from its fields.
func (g *GovernanceIntent) canonical() ([]byte, error) {
	if g.Format != GovernanceIntentFormat {
		return nil, fmt.Errorf("governance intent format %d is not %d; prepare it again", g.Format, GovernanceIntentFormat)
	}
	switch g.Type {
	case IntentAddSignersToGroup:
		return AddSignersToGroupIntent(g.SignerGroupID, g.SignerGroupVersion, g.IdempotencyKey, g.MemberKeys)
	case IntentRemoveSignersFromGroup:
		return RemoveSignersFromGroupIntent(g.SignerGroupID, g.SignerGroupVersion, g.IdempotencyKey, g.MemberKeys)
	case IntentUpdateSignerGroupGovernance:
		return UpdateSignerGroupGovernanceIntent(g.SignerGroupID, g.SignerGroupVersion, g.IdempotencyKey, g.Settings)
	default:
		return nil, fmt.Errorf("unsupported governance intent type %q", g.Type)
	}
}

// verified returns the canonical bytes after checking that they are exactly
// what the intent's fields describe, so nobody signs or sends a Canonical that
// says something other than the fields (e.g. a stored intent edited by
// whoever relays it to the signers).
func (g *GovernanceIntent) verified() ([]byte, error) {
	if g == nil {
		return nil, errors.New("nil governance intent")
	}
	if len(g.Canonical) == 0 {
		return nil, errors.New("governance intent has no canonical bytes; create it with a Prepare* method")
	}
	rebuilt, err := g.canonical()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rebuilt, g.Canonical) {
		return nil, errors.New("governance intent fields do not match its canonical bytes; refusing to sign or send it (prepare again)")
	}
	return rebuilt, nil
}

// Sign collects one signature per signer over the intent, in signer order.
// It first checks that Canonical matches the intent's fields. A signer UI
// should show the person what they are signing, ideally parsed from
// Canonical itself.
func (g *GovernanceIntent) Sign(ctx context.Context, signers ...IntentSigner) ([]string, error) {
	canonical, err := g.verified()
	if err != nil {
		return nil, err
	}
	if len(signers) == 0 {
		return nil, errors.New("at least one signer is required")
	}
	sigs := make([]string, 0, len(signers))
	for i, s := range signers {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if s == nil {
			return nil, fmt.Errorf("signer %d is nil", i)
		}
		sig, err := s.SignIntent(ctx, bytes.Clone(canonical))
		if err != nil {
			return nil, fmt.Errorf("signer %d: %w", i, err)
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

// prepare reads the group and builds the intent around its current version
// and a fresh lowercase UUID key. check, when set, vets the change against the
// group's current state before anything is signed.
func (c *Client) prepare(ctx context.Context, g *GovernanceIntent, check func(*gen.SignerGroup) error) (*GovernanceIntent, error) {
	g.Format = GovernanceIntentFormat
	// Validate the caller's input before the network read: the version and
	// key are placeholders here.
	g.SignerGroupVersion, g.IdempotencyKey = 0, "00000000-0000-4000-8000-000000000000"
	if _, err := g.canonical(); err != nil {
		return nil, err
	}
	resp, err := CheckResponse(c.Raw().GetSignerGroupWithResponse(ctx, g.SignerGroupID, nil))
	if err != nil {
		return nil, fmt.Errorf("read signer group: %w", err)
	}
	grp := resp.JSON200
	if grp == nil {
		return nil, errors.New("read signer group: empty response body")
	}
	if grp.Version == nil {
		return nil, errors.New("read signer group: the platform did not return the group's version")
	}
	if check != nil {
		if err := check(grp); err != nil {
			return nil, err
		}
	}
	g.SignerGroupVersion = *grp.Version
	g.IdempotencyKey = uuid.NewString() // lowercase canonical form
	g.RequiredSignatures = 1
	if t := grp.EffectiveGovernanceThreshold; t != nil && *t > 1 {
		g.RequiredSignatures = int(*t)
	}
	if g.Canonical, err = g.canonical(); err != nil {
		return nil, err
	}
	return g, nil
}

// PrepareAddSignerToGroup prepares adding memberKey (a registered signer's
// public key) to the group: reads the group's version and builds the
// add_signers_to_group intent to sign.
func (c *Client) PrepareAddSignerToGroup(ctx context.Context, signerGroupID, memberKey string) (*GovernanceIntent, error) {
	return c.prepare(ctx, &GovernanceIntent{Type: IntentAddSignersToGroup, SignerGroupID: signerGroupID, MemberKeys: []string{memberKey}}, nil)
}

// PrepareRemoveSignersFromGroup prepares removing members (public keys as the
// group lists them, in the order given) from the group.
func (c *Client) PrepareRemoveSignersFromGroup(ctx context.Context, signerGroupID string, memberKeys []string) (*GovernanceIntent, error) {
	return c.prepare(ctx, &GovernanceIntent{Type: IntentRemoveSignersFromGroup, SignerGroupID: signerGroupID, MemberKeys: slices.Clone(memberKeys)}, nil)
}

// PrepareUpdateSignerGroupGovernance prepares replacing the group's governance
// settings. The group's CURRENT governance signs it.
//
// The update replaces both settings, and a nil one clears it. Because clearing
// the governing group or the threshold loosens who controls the group, this
// refuses to clear a setting the group has now unless settings.AllowClearing
// is set: to change only the threshold of a group governed by another group,
// pass both. A group cannot govern itself by id: leave
// GovernanceSignerGroupID nil for that.
func (c *Client) PrepareUpdateSignerGroupGovernance(ctx context.Context, signerGroupID string, settings GovernanceSettings) (*GovernanceIntent, error) {
	settings = settings.clone()
	if id := settings.GovernanceSignerGroupID; id != nil && *id == signerGroupID {
		return nil, errors.New("a group cannot name itself as its governing group; leave GovernanceSignerGroupID nil to let it govern itself")
	}
	check := func(grp *gen.SignerGroup) error {
		if settings.AllowClearing {
			return nil
		}
		if grp.GovernanceSignerGroupId != nil && settings.GovernanceSignerGroupID == nil {
			return fmt.Errorf("this would clear the group's governing group %s (the update replaces both settings); pass it to keep it, or set AllowClearing", *grp.GovernanceSignerGroupId)
		}
		if grp.GovernanceThreshold != nil && settings.GovernanceThreshold == nil {
			return fmt.Errorf("this would clear the group's governance threshold %d back to the default (the update replaces both settings); pass it to keep it, or set AllowClearing", *grp.GovernanceThreshold)
		}
		return nil
	}
	return c.prepare(ctx, &GovernanceIntent{Type: IntentUpdateSignerGroupGovernance, SignerGroupID: signerGroupID, Settings: settings}, check)
}

// SubmitGovernanceIntent sends a prepared change with its signatures. It
// refuses to send when the intent's Canonical does not match its fields, or
// with fewer signatures than the group's quorum at prepare time.
//
// Any refusal by the platform's governance check (bad or too few signatures,
// a stale version) is final for this intent: the idempotency key is signed
// into the intent and the platform remembers the outcome per key, so prepare
// and sign again. A transport failure (no response) is different: resubmit
// the SAME intent and signatures, which the platform deduplicates by key.
//
// Typed failures: ErrInsufficientEndorsements, ErrStaleSignerGroupVersion,
// ErrBelowThreshold (removals), ErrEndorsementRequired.
func (c *Client) SubmitGovernanceIntent(ctx context.Context, intent *GovernanceIntent, signatures []string) (*gen.SignerGroup, error) {
	if _, err := intent.verified(); err != nil {
		return nil, err
	}
	if len(signatures) == 0 {
		return nil, errors.New("at least one signature is required")
	}
	if slices.Contains(signatures, "") {
		return nil, errors.New("a signature is empty")
	}
	if len(signatures) < intent.RequiredSignatures {
		return nil, fmt.Errorf("the group needs %d signatures, got %d (sending fewer would use up this intent)", intent.RequiredSignatures, len(signatures))
	}
	// Send from a snapshot, so the request matches what was just checked.
	snap := *intent
	snap.MemberKeys = slices.Clone(intent.MemberKeys)
	snap.Settings = intent.Settings.clone()
	key, err := uuid.Parse(snap.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("idempotency key: %w", err)
	}
	sigs := slices.Clone(signatures)
	version := snap.SignerGroupVersion

	var group *gen.SignerGroup
	switch snap.Type {
	case IntentAddSignersToGroup:
		resp, err := CheckResponse(c.Raw().CreateSignerGroupSignerWithResponse(ctx, snap.SignerGroupID,
			&gen.CreateSignerGroupSignerParams{XIdempotencyKey: key},
			gen.CreateSignerGroupSignerRequest{MemberKey: snap.MemberKeys[0], Signatures: &sigs, SignerGroupVersion: &version}))
		if err != nil {
			return nil, fmt.Errorf("add signer to group: %w", err)
		}
		group = resp.JSON200
	case IntentRemoveSignersFromGroup:
		resp, err := CheckResponse(c.Raw().RemoveSignerGroupSignersWithResponse(ctx, snap.SignerGroupID,
			&gen.RemoveSignerGroupSignersParams{XIdempotencyKey: key},
			gen.RemoveSignerGroupSignersRequest{MembersToRemove: snap.MemberKeys, Signatures: sigs, SignerGroupVersion: version}))
		if err != nil {
			return nil, fmt.Errorf("remove signers from group: %w", err)
		}
		group = resp.JSON200
	case IntentUpdateSignerGroupGovernance:
		resp, err := CheckResponse(c.Raw().UpdateSignerGroupGovernanceWithResponse(ctx, snap.SignerGroupID,
			&gen.UpdateSignerGroupGovernanceParams{XIdempotencyKey: key},
			gen.UpdateSignerGroupGovernanceRequest{
				GovernanceThreshold:     snap.Settings.GovernanceThreshold,
				GovernanceSignerGroupId: snap.Settings.GovernanceSignerGroupID,
				Signatures:              sigs,
				SignerGroupVersion:      version,
			}))
		if err != nil {
			return nil, fmt.Errorf("update signer group governance: %w", err)
		}
		group = resp.JSON200
	default:
		return nil, fmt.Errorf("unsupported governance intent type %q", snap.Type)
	}
	if group == nil {
		return nil, fmt.Errorf("%s: empty response body", snap.Type)
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
	// Refuse before anyone is asked to sign (a passkey signer prompts a
	// person), not after every signature is collected.
	if len(signers) < intent.RequiredSignatures {
		return nil, fmt.Errorf("the group needs %d signatures, but only %d signer(s) were passed", intent.RequiredSignatures, len(signers))
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
//
// Each call mints a new idempotency key. If a call fails without a response,
// do not call it again blindly: read the group to see whether the change
// landed, or use Prepare*/SubmitGovernanceIntent and resubmit the same intent.
func (c *Client) AddSignerToGroupSigned(ctx context.Context, signerGroupID, memberKey string, signers ...IntentSigner) (*gen.SignerGroup, error) {
	return c.prepareSignSubmit(ctx, func() (*GovernanceIntent, error) {
		return c.PrepareAddSignerToGroup(ctx, signerGroupID, memberKey)
	}, signers)
}

// RemoveSignersFromGroupSigned removes memberKeys from the group with
// signatures from signers, in one call (POST /signer-groups/{id}/signer-removals).
// The retry note on AddSignerToGroupSigned applies.
func (c *Client) RemoveSignersFromGroupSigned(ctx context.Context, signerGroupID string, memberKeys []string, signers ...IntentSigner) (*gen.SignerGroup, error) {
	return c.prepareSignSubmit(ctx, func() (*GovernanceIntent, error) {
		return c.PrepareRemoveSignersFromGroup(ctx, signerGroupID, memberKeys)
	}, signers)
}

// UpdateSignerGroupGovernanceSigned replaces the group's governance settings
// with signatures from its current governance, in one call. See
// PrepareUpdateSignerGroupGovernance for how settings are replaced, and the
// retry note on AddSignerToGroupSigned.
func (c *Client) UpdateSignerGroupGovernanceSigned(ctx context.Context, signerGroupID string, settings GovernanceSettings, signers ...IntentSigner) (*gen.SignerGroup, error) {
	return c.prepareSignSubmit(ctx, func() (*GovernanceIntent, error) {
		return c.PrepareUpdateSignerGroupGovernance(ctx, signerGroupID, settings)
	}, signers)
}
