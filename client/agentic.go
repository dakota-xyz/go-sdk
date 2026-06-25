package client

// High-level agentic helpers: collapse the multi-step, crypto-heavy hosted-agent
// flows into single calls with good defaults, so a partner integration stays
// short. The raw operations remain reachable via Raw().

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

// AgentWalletAuthorization is the result of AuthorizeAgentOnWalletWithDefaultPolicy:
// the signer group and policy created to let the agent spend from the wallet.
type AgentWalletAuthorization struct {
	// SignerGroupID is the solo group created for the agent's signer and attached to the wallet.
	SignerGroupID string
	// PolicyID is the default allow policy created for the group and attached to the wallet.
	PolicyID string
}

// AuthorizeAgentOnWalletWithDefaultPolicy grants a hosted agent permission to
// spend from a wallet, in one call. It:
//
//  1. creates a solo signer group holding the agent's signer key,
//  2. creates a DEFAULT no-cap "allow" policy for that group — any single member
//     signature passes; the real per-payment spend bounds are enforced by the
//     agentic mandate gate, NOT this policy, and
//  3. performs the two customer-endorsed attaches (group→wallet, policy→wallet),
//     each signed by walletSigner over the canonical attach payload.
//
// walletSigner must already be a signer RECOGNIZED on the wallet (e.g. the
// customer/owner whose key created it) — the platform verifies its endorsement.
// The SDK never holds the key: walletSigner does the signing.
//
// agentSignerPublicKey is the agent's signer public key (base64 PKIX) as returned
// by CreateAgent; name labels the created group and policy.
//
// After this returns, the agent's signer is recognized on the wallet and its
// approved mandates can move funds. To authorize the same agent on another
// wallet, call again (a fresh group + policy is created per wallet); for finer
// control, compose Raw().CreateSignerGroup / CreatePolicy with the endorsed
// attach steps yourself.
func (c *Client) AuthorizeAgentOnWalletWithDefaultPolicy(ctx context.Context, walletID, name, agentSignerPublicKey string, walletSigner Signer) (AgentWalletAuthorization, error) {
	var out AgentWalletAuthorization
	if walletSigner == nil {
		return out, fmt.Errorf("walletSigner is required (a recognized signer of the wallet endorses the attaches)")
	}

	// 1. Solo signer group holding the agent's signer key.
	grp, err := CheckResponse(c.Raw().CreateSignerGroupWithResponse(ctx, nil, gen.SignerGroupCreateRequest{
		Name:       name + " agent group",
		MemberKeys: []string{agentSignerPublicKey},
	}))
	if err != nil {
		return out, fmt.Errorf("create signer group: %w", err)
	}
	if grp.JSON201 == nil {
		return out, fmt.Errorf("create signer group: empty response body")
	}
	out.SignerGroupID = grp.JSON201.Id

	// 2. Default allow policy for the group. A rule-less policy never APPLIES
	// (policy-engine default-deny), so carry one explicit allow rule; the spend
	// limits live in the agentic mandate gate, not here.
	desc := "Default allow policy — per-payment spend bounds enforced by the agentic mandate gate"
	pol, err := CheckResponse(c.Raw().CreatePolicyWithResponse(ctx, nil, gen.CreatePolicyRequest{
		Name:          name + " agent allow policy",
		Description:   &desc,
		SignerGroupId: &out.SignerGroupID,
		Rules: &[]gen.CreatePolicyRuleRequest{{
			RuleType: gen.CreatePolicyRuleRequestRuleTypeApprovalThreshold,
			Action:   gen.CreatePolicyRuleRequestActionAllow,
			Definition: map[string]any{
				"threshold":   1,
				"description": "Any group member's signature allows",
			},
		}},
	}))
	if err != nil {
		return out, fmt.Errorf("create policy: %w", err)
	}
	if pol.JSON201 == nil {
		return out, fmt.Errorf("create policy: empty response body")
	}
	out.PolicyID = pol.JSON201.Id

	// 3. The two customer-endorsed attaches.
	if err := c.attachGroupToWallet(ctx, walletID, out.SignerGroupID, walletSigner); err != nil {
		return out, err
	}
	if err := c.attachPolicyToWallet(ctx, walletID, out.PolicyID, walletSigner); err != nil {
		return out, err
	}
	return out, nil
}

// attachGroupToWallet performs the customer-endorsed signer-group→wallet attach.
// The idempotency key is part of the SIGNED payload, so it must be the same value
// in the canonical bytes, the intent, and the request header.
func (c *Client) attachGroupToWallet(ctx context.Context, walletID, groupID string, signer Signer) error {
	idem := uuid.NewString()
	payload, err := AttachGroupPayload(walletID, groupID, idem)
	if err != nil {
		return fmt.Errorf("build group-attach payload: %w", err)
	}
	sig, err := signer.Sign(payload)
	if err != nil {
		return fmt.Errorf("sign group attach: %w", err)
	}
	var intent gen.EndorsedRequest_Intent
	if err := intent.FromAttachGroupToWalletIntent(gen.AttachGroupToWalletIntent{
		Type:           gen.AttachGroupToWalletIntentTypeAttachGroupToWallet,
		WalletId:       walletID,
		GroupId:        groupID,
		IdempotencyKey: idem,
	}); err != nil {
		return fmt.Errorf("build group-attach intent: %w", err)
	}
	if _, err := CheckResponse(c.Raw().UpsertWalletSignerGroupRelationshipWithResponse(
		ctx, walletID, groupID,
		&gen.UpsertWalletSignerGroupRelationshipParams{XIdempotencyKey: uuid.MustParse(idem)},
		gen.EndorsedRequest{Signatures: []string{sig}, Intent: intent},
	)); err != nil {
		return fmt.Errorf("attach group to wallet: %w", err)
	}
	return nil
}

// attachPolicyToWallet performs the customer-endorsed policy→wallet attach.
func (c *Client) attachPolicyToWallet(ctx context.Context, walletID, policyID string, signer Signer) error {
	idem := uuid.NewString()
	payload, err := AttachPolicyPayload(walletID, policyID, idem)
	if err != nil {
		return fmt.Errorf("build policy-attach payload: %w", err)
	}
	sig, err := signer.Sign(payload)
	if err != nil {
		return fmt.Errorf("sign policy attach: %w", err)
	}
	var intent gen.EndorsedRequest_Intent
	if err := intent.FromAttachPolicyToWalletIntent(gen.AttachPolicyToWalletIntent{
		Type:           gen.AttachPolicyToWalletIntentTypeAttachPolicyToWallet,
		WalletId:       walletID,
		PolicyId:       policyID,
		IdempotencyKey: idem,
	}); err != nil {
		return fmt.Errorf("build policy-attach intent: %w", err)
	}
	if _, err := CheckResponse(c.Raw().UpsertPolicyWalletRelationshipWithResponse(
		ctx, policyID, walletID,
		&gen.UpsertPolicyWalletRelationshipParams{XIdempotencyKey: uuid.MustParse(idem)},
		gen.EndorsedRequest{Signatures: []string{sig}, Intent: intent},
	)); err != nil {
		return fmt.Errorf("attach policy to wallet: %w", err)
	}
	return nil
}
