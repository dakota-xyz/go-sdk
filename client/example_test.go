package client_test

import (
	"context"
	"fmt"

	"github.com/dakota-xyz/go-sdk/client"
	"github.com/dakota-xyz/go-sdk/client/gen"
)

// Agentic payments is a BETA surface (x-beta, flag-gated on the platform);
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

// Blockers are for your APPLICATION; Reply is for your customer. They ACCOMPANY
// proposals rather than replacing them — the common case is a payee who does not
// exist yet, where the turn proposes creating them AND reports that the limit
// will not reach them. You need both, in that order.
func ExampleConversationTurn_blockers() {
	c, err := client.New(client.WithAPIKey("sk_test_your_key"))
	if err != nil {
		panic(err)
	}

	turn, err := c.NewAgentConversation("agt_123").Send(context.Background(), "pay Priya 250 USDC on Fridays")
	if err != nil {
		panic(err)
	}

	// Accept the proposals FIRST — that is what gives a new payee an id.
	if turn.HasProposals {
		fmt.Printf("accept %d proposal(s) via the instructions flow\n", len(turn.Proposals))
	}

	for _, b := range turn.Blockers {
		switch b.Code {
		case "mandate_does_not_cover_payee":
			// Actionable: amend the limit to ADD this payee as a target. That
			// changes nothing else, so it can never raise the limit.
			fmt.Println("limit needs to cover this payee")
		case "no_mandate":
			// Nothing to amend — the customer must establish a limit first.
			fmt.Println("customer has no spending limit yet")
		default:
			// Ignore codes you do not know; new ones are added over time.
		}
	}
}

// Speak your product's language. Registration is the ONLY way to set a policy:
// it belongs to the client, not to a request, so every drafting turn and every
// accept resolve it from here and cannot disagree with one another.
func ExampleClient_Raw_registerAgenticPolicy() {
	c, err := client.New(client.WithAPIKey("sk_test_your_key"))
	if err != nil {
		panic(err)
	}

	payeeModel := gen.AgenticClientPolicyPayeeModel("flat")
	// A full REPLACE, not a merge: an omitted field means you no longer want it,
	// and an empty body clears the registration back to platform defaults.
	_, err = c.Raw().UpdateClientAgenticPolicyWithResponse(context.Background(), nil, gen.AgenticClientPolicy{
		PayeeModel: &payeeModel,
		Labels: &map[string]string{
			"limit":      "spending limit",
			"payee":      "recipient",
			"limit_unit": "USD",
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("policy registered")
}

// Amend a mandate: append a NEW signed version without resetting the spend
// already made in the current window. Usage accrues to the MANDATE, so an agent
// that has spent 9,000 of a 10,000 monthly cap and is amended to 20,000 has
// 11,000 left — not 20,000.
func ExampleMandateAmendSignPayload() {
	signer, err := client.NewP256Signer()
	if err != nil {
		panic(err)
	}

	// mandate is returned by c.Raw().GetMandateWithResponse.
	var mandate gen.Mandate
	mandate.Id = ptr("mandate_123")
	mandate.BoundSignerId = ptr("signer_123")
	mandate.Version = ptr(1)
	mandate.Rule = &gen.MandateRule{Asset: "USDC", TargetType: gen.MandateRuleTargetType("any")}

	// The NEW rule, complete and already canonical — the amend endpoint stores
	// and verifies it VERBATIM and never normalizes it.
	next := *mandate.Rule
	next.MaxAmountInWindow = ptr("20000")

	// The payload commits to the version it creates, so a signature for v2 can
	// never be replayed to append v3.
	payload, err := client.MandateAmendSignPayload(mandate, *mandate.Version+1, next)
	if err != nil {
		panic(err)
	}
	if _, err := signer.Sign(payload); err != nil {
		panic(err)
	}
	// Submit via c.Raw().AmendMandateWithResponse with the SAME rule.
	fmt.Println("amend signed")
}

func ptr[T any](v T) *T { return &v }
