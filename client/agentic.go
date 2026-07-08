package client

// High-level agentic helpers: collapse the multi-step, crypto-heavy hosted-agent
// flows into single calls with good defaults, so a partner integration stays
// short. The raw operations remain reachable via Raw().

import (
	"context"
	"fmt"

	"github.com/dakota-xyz/go-sdk/client/gen"
	"github.com/google/uuid"
)

// AttachUserToWallet grants a principal — a user OR an agent, both are just
// signers — permission to spend from a wallet by adding its signer to an EXISTING
// signer group on that wallet: the group whose attached policies should govern it.
//
// That membership change is the whole operation, and it is UNENDORSED: group
// membership is a client-admin action, not a wallet endorsement, so it needs NO
// signature — not from the wallet owner, not from the principal being added. From
// then on the principal's transactions are subject to exactly the policies bound to
// spendingGroupID, the same as any other member of that group. The per-payment
// spend bound remains the agentic mandate gate, not the policy.
//
// It deliberately reuses the wallet's pre-built policies and creates NO per-signer
// policy. Two policy-engine realities make a per-signer policy the wrong tool: a
// policy→wallet attach is endorsed by the policy's OWN signer group (so only that
// group's key could attach it), and an unconditional approval-threshold ALLOW
// policy inverts to a deny for any transaction its group didn't sign, so a second
// signer's policy would veto the first. Prepare the wallet's groups + policies in
// advance and point each principal at the group whose constraints you want it to
// inherit.
//
// The call is idempotent: if the signer is already a member, it is a no-op and
// returns alreadyMember=true. It errors if spendingGroupID is not attached to
// walletID, so a wrong group fails loudly instead of silently granting nothing.
//
// signerPublicKey is the principal's registered signer key (base64 PKIX) — e.g. the
// key returned by CreatePaymentAgent for a hosted agent.
//
// Experimental: agentic payments is an alpha surface (x-alpha, flag-gated on
// the platform) and may change without a major-version bump.
func (c *Client) AttachUserToWallet(ctx context.Context, walletID, signerPublicKey, spendingGroupID string) (alreadyMember bool, err error) {
	if walletID == "" || signerPublicKey == "" || spendingGroupID == "" {
		return false, fmt.Errorf("walletID, signerPublicKey and spendingGroupID are all required")
	}

	if err := c.requireGroupOnWallet(ctx, walletID, spendingGroupID); err != nil {
		return false, err
	}

	// Idempotency: skip the add if the signer's key is already a member.
	grp, err := CheckResponse(c.Raw().GetSignerGroupWithResponse(ctx, spendingGroupID))
	if err != nil {
		return false, fmt.Errorf("get signer group: %w", err)
	}
	if grp.JSON200 == nil {
		return false, fmt.Errorf("get signer group: empty response body")
	}
	for _, m := range grp.JSON200.Members {
		if m.PublicKey == signerPublicKey {
			return true, nil
		}
	}

	// Add the signer to the group (unendorsed). Granting spend permission is
	// security-relevant, so set an explicit x-idempotency-key rather than depend on
	// the transport: WithAutomaticIdempotency(false) would otherwise send this POST
	// without the required header and 400 (mirrors DetachUserFromWallet).
	idemKey, err := uuid.NewRandom()
	if err != nil {
		return false, fmt.Errorf("generate idempotency key: %w", err)
	}
	if _, err := CheckResponse(c.Raw().CreateSignerGroupSignerWithResponse(ctx, spendingGroupID, &gen.CreateSignerGroupSignerParams{
		XIdempotencyKey: idemKey,
	}, gen.CreateSignerGroupSignerRequest{
		MemberKey: signerPublicKey,
	})); err != nil {
		return false, fmt.Errorf("add signer to group: %w", err)
	}
	return false, nil
}

// DetachUserFromWallet revokes a principal's spend permission by removing its
// signer from spendingGroupID — the inverse of AttachUserToWallet, and likewise
// UNENDORSED (no signature).
//
// It is deliberately SCOPED to the one group you pass, NOT a sweep across every
// group on the wallet, for two reasons:
//   - signer groups are NOT wallet-exclusive — the same group can govern multiple
//     wallets — so removing a signer from a group revokes it on EVERY wallet that
//     group governs; passing one explicit group keeps that blast radius known
//     instead of multiplying it across all of the wallet's groups;
//   - a sweep cannot tell "added for this agent" from "belongs here for another
//     reason" (e.g. the human owner sitting in the admin group), so it can lock out
//     a legitimate member.
//
// Pass the same group you used to attach. Idempotent: if the signer isn't a member,
// it's a no-op (wasMember=false). Errors if spendingGroupID isn't attached to
// walletID.
//
// NOTE: this does not guard against removing a group's LAST member — emptying the
// group that governs a wallet can leave it with no authorized signer. The caller
// must avoid stranding the wallet. For a hosted agent, prefer the platform's
// RevokePaymentAgent (it destroys the agent's key, so the signer can authorize nothing
// even while still listed) and use this for membership hygiene.
func (c *Client) DetachUserFromWallet(ctx context.Context, walletID, signerPublicKey, spendingGroupID string) (wasMember bool, err error) {
	if walletID == "" || signerPublicKey == "" || spendingGroupID == "" {
		return false, fmt.Errorf("walletID, signerPublicKey and spendingGroupID are all required")
	}

	if err := c.requireGroupOnWallet(ctx, walletID, spendingGroupID); err != nil {
		return false, err
	}

	// Resolve the signer's public key → its resource id within the group: the delete
	// endpoint addresses members by id, not key. Absent ⇒ idempotent no-op.
	grp, err := CheckResponse(c.Raw().GetSignerGroupWithResponse(ctx, spendingGroupID))
	if err != nil {
		return false, fmt.Errorf("get signer group: %w", err)
	}
	if grp.JSON200 == nil {
		return false, fmt.Errorf("get signer group: empty response body")
	}
	signerID := ""
	for _, m := range grp.JSON200.Members {
		if m.PublicKey == signerPublicKey {
			signerID = m.Id
			break
		}
	}
	if signerID == "" {
		return false, nil
	}

	// The platform requires an x-idempotency-key on this DELETE. The idempotency
	// transport now injects one on DELETEs too, but set it explicitly here anyway:
	// revoking spend permission is security-relevant, so this path must not depend
	// on transport configuration (e.g. WithAutomaticIdempotency(false) would
	// otherwise let a detach silently 400 and leave the signer authorized).
	idemKey, err := uuid.NewRandom()
	if err != nil {
		return false, fmt.Errorf("generate idempotency key: %w", err)
	}
	if _, err := CheckResponse(c.Raw().DeleteSignerGroupSignerWithResponse(ctx, spendingGroupID, signerID, &gen.DeleteSignerGroupSignerParams{
		XIdempotencyKey: idemKey,
	})); err != nil {
		return false, fmt.Errorf("remove signer from group: %w", err)
	}
	return true, nil
}

// requireGroupOnWallet errors unless spendingGroupID is one of the signer groups
// attached to walletID — so attach/detach can't silently operate on a group that
// has nothing to do with the named wallet.
func (c *Client) requireGroupOnWallet(ctx context.Context, walletID, spendingGroupID string) error {
	groups, err := CheckResponse(c.Raw().GetSignerGroupsForWalletWithResponse(ctx, walletID))
	if err != nil {
		return fmt.Errorf("list wallet signer groups: %w", err)
	}
	if groups.JSON200 == nil {
		return fmt.Errorf("list wallet signer groups: empty response body")
	}
	for _, g := range *groups.JSON200 {
		if g.Id == spendingGroupID {
			return nil
		}
	}
	return fmt.Errorf("signer group %s is not attached to wallet %s", spendingGroupID, walletID)
}
