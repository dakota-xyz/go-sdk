package client_test

import (
	"context"
	"fmt"

	"github.com/dakota-xyz/go-sdk/client"
	"github.com/dakota-xyz/go-sdk/client/gen"
)

// Agentic payments is an ALPHA surface (x-alpha, flag-gated on the platform);
// these helpers may change without a major-version bump.

// Draft payments from natural language: the agent turns a prompt into a
// reviewable action series that you accept through the instructions flow.
func ExampleClient_NewAgentConversation() {
	c, err := client.New(client.WithAPIKey("sk_test_your_key"))
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	conv := c.NewAgentConversation("agt_123")
	turn, err := conv.Send(ctx, "Pay Alice 100 USDC on base-mainnet every month until December")
	if err != nil {
		panic(err)
	}
	if turn.HasProposals {
		fmt.Printf("agent drafted %d proposal(s); accept them via the instructions flow\n", len(turn.Proposals))
	} else {
		fmt.Println("agent needs more detail:", turn.Reply)
	}
}

// Endorse a payment agent onto a wallet: add its signer to the wallet's
// spending group so it can spend from that wallet. Idempotent.
func ExampleClient_AttachUserToWallet() {
	c, err := client.New(client.WithAPIKey("sk_test_your_key"))
	if err != nil {
		panic(err)
	}

	alreadyMember, err := c.AttachUserToWallet(
		context.Background(),
		"wlt_123",              // wallet id
		"BASE64_SIGNER_PUBKEY", // agent.SignerPublicKey
		"grp_spend",            // the wallet's spending group
	)
	if err != nil {
		panic(err)
	}
	fmt.Println("already a member:", alreadyMember)
}

// Sign a §8 mandate approval. The caller holds the key; the SDK never does.
// P256Signer is the in-memory default for sandbox/tests — implement Signer over
// your own HSM/KMS in production.
func ExampleMandateSignPayload() {
	signer, err := client.NewP256Signer()
	if err != nil {
		panic(err)
	}

	// mandate is returned by c.Raw().GetMandateWithResponse / ListMandates.
	var mandate gen.Mandate

	// Reproduce the exact canonical bytes the platform verifies, then sign them.
	payload, err := client.MandateSignPayload(mandate, client.MandateActionApprove)
	if err != nil {
		panic(err)
	}
	sig, err := signer.Sign(payload)
	if err != nil {
		panic(err)
	}

	// The signature verifies against the signer's registered public key.
	if err := client.VerifyMandateSignature(signer.PublicKeyBase64(), payload, sig); err != nil {
		panic(err)
	}
	fmt.Println("mandate approval signed")
}
