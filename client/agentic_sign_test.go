package client

import (
	"testing"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

func sptr(s string) *string { return &s }
func i64ptr(i int64) *int64 { return &i }
func iptr(i int) *int       { return &i }

// goldenMandateApprovePayload is the platform's authoritative JCS-canonical
// approval payload for goldenMandate(), captured verbatim from the platform's
// enc.C14nJSON (= jsoncanonicalizer.Transform(json.Marshal(...))). The SDK MUST
// reproduce it byte-for-byte, or signatures it produces will not verify.
const goldenMandateApprovePayload = `{"action":"approve","bound_signer":"signer_2eFgHiJkLmNoPqRsTuVwXyZabcd","id":"mandate_2cDeFgHiJkLmNoPqRsTuVwXyZab","rule":{"asset":"USDC","max_count_per_target_in_window":1,"max_per_tx":"10","network_id":"base-sepolia","target_type":"recipient","targets":["recipient_2aBcDeFgHiJkLmNoPqRsTuVwXyZ"],"window":"MONTHLY"},"valid_from":0,"valid_until":1798675200}`

func goldenMandate() gen.Mandate {
	window := gen.MandateRuleWindow("MONTHLY")
	return gen.Mandate{
		Id:            sptr("mandate_2cDeFgHiJkLmNoPqRsTuVwXyZab"),
		BoundSignerId: sptr("signer_2eFgHiJkLmNoPqRsTuVwXyZabcd"),
		ValidFrom:     i64ptr(0),
		ValidUntil:    i64ptr(1798675200),
		Rule: &gen.MandateRule{
			TargetType:                gen.MandateRuleTargetType("recipient"),
			Targets:                   &[]string{"recipient_2aBcDeFgHiJkLmNoPqRsTuVwXyZ"},
			NetworkId:                 sptr("base-sepolia"),
			Asset:                     "USDC",
			MaxPerTx:                  sptr("10"),
			Window:                    &window,
			MaxCountPerTargetInWindow: iptr(1),
		},
	}
}

// TestMandateSignPayload_ByteExact proves the SDK reproduces the platform's
// canonical approval bytes exactly (the golden is the platform's own output).
func TestMandateSignPayload_ByteExact(t *testing.T) {
	got, err := MandateSignPayload(goldenMandate(), MandateActionApprove)
	if err != nil {
		t.Fatalf("MandateSignPayload: %v", err)
	}
	if string(got) != goldenMandateApprovePayload {
		t.Fatalf("payload mismatch with platform golden:\n got:  %s\n want: %s", got, goldenMandateApprovePayload)
	}
}

// TestMandateSignPayload_ZeroValidFromOmittedOnWire: the API omits valid_from
// when it is 0, so a nil pointer must still canonicalize to valid_from:0 —
// matching the platform, which always signs the int64.
func TestMandateSignPayload_ZeroValidFromOmittedOnWire(t *testing.T) {
	m := goldenMandate()
	m.ValidFrom = nil // exactly how the wire returns valid_from == 0
	got, err := MandateSignPayload(m, MandateActionApprove)
	if err != nil {
		t.Fatalf("MandateSignPayload: %v", err)
	}
	if string(got) != goldenMandateApprovePayload {
		t.Fatalf("nil valid_from must canonicalize to 0:\n got:  %s\n want: %s", got, goldenMandateApprovePayload)
	}
}

// TestMandateSign_Roundtrip: the default P-256 signer's signature verifies, and
// an approve signature does NOT verify a cancel payload — the action verb is
// part of the signed bytes, so the two can never be each other's replay.
func TestMandateSign_Roundtrip(t *testing.T) {
	signer, err := NewP256Signer()
	if err != nil {
		t.Fatalf("NewP256Signer: %v", err)
	}
	approve, err := MandateSignPayload(goldenMandate(), MandateActionApprove)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := signer.Sign(approve)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := VerifyMandateSignature(signer.PublicKeyBase64(), approve, sig); err != nil {
		t.Fatalf("self-signed signature must verify: %v", err)
	}
	cancel, err := MandateSignPayload(goldenMandate(), MandateActionCancel)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyMandateSignature(signer.PublicKeyBase64(), cancel, sig); err == nil {
		t.Fatal("approve signature must NOT verify a cancel payload (replay guard)")
	}
}

// TestMandateSignPayload_MissingFields: a mandate missing a required payload
// field is rejected, never silently producing a wrong-but-signable payload.
func TestMandateSignPayload_MissingFields(t *testing.T) {
	if _, err := MandateSignPayload(gen.Mandate{}, MandateActionApprove); err == nil {
		t.Fatal("want an error for a mandate with no id / bound_signer / rule")
	}
}

// TestAttachPayloads_ByteExact pins the JCS canonical form of the two wallet
// endorsement payloads (sorted keys, no whitespace) — the bytes the customer
// signs during agent provisioning. Matches Nimbus's proven attach payloads.
func TestAttachPayloads_ByteExact(t *testing.T) {
	group, err := AttachGroupPayload("wallet_W", "group_G", "idem_K")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(group), `{"group_id":"group_G","idempotency_key":"idem_K","type":"attach_group_to_wallet","wallet_id":"wallet_W"}`; got != want {
		t.Fatalf("attach-group payload:\n got:  %s\n want: %s", got, want)
	}
	policy, err := AttachPolicyPayload("wallet_W", "policy_P", "idem_K")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(policy), `{"idempotency_key":"idem_K","policy_id":"policy_P","type":"attach_policy_to_wallet","wallet_id":"wallet_W"}`; got != want {
		t.Fatalf("attach-policy payload:\n got:  %s\n want: %s", got, want)
	}
}

// TestEndorsementPayloads_ByteExact pins the JCS canonical form of the remaining
// endorsed-intent payloads — the bytes a recognized signer signs to detach a
// group/policy, delete a policy, or add/update/remove a policy rule. Each must
// match the platform policy engine's intent reconstruction byte-for-byte, or the
// endorsement won't verify server-side.
func TestEndorsementPayloads_ByteExact(t *testing.T) {
	cases := []struct {
		name string
		got  func() ([]byte, error)
		want string
	}{
		{"detach_group", func() ([]byte, error) { return DetachGroupPayload("wallet_W", "group_G", "idem_K") },
			`{"group_id":"group_G","idempotency_key":"idem_K","type":"detach_group_from_wallet","wallet_id":"wallet_W"}`},
		{"detach_policy", func() ([]byte, error) { return DetachPolicyPayload("wallet_W", "policy_P", "idem_K") },
			`{"idempotency_key":"idem_K","policy_id":"policy_P","type":"detach_policy_from_wallet","wallet_id":"wallet_W"}`},
		{"delete_policy", func() ([]byte, error) { return DeletePolicyPayload("policy_P", "idem_K") },
			`{"idempotency_key":"idem_K","policy_id":"policy_P","type":"delete_policy"}`},
		{"remove_policy_rule", func() ([]byte, error) { return RemovePolicyRulePayload("policy_P", "rule_R", "idem_K") },
			`{"idempotency_key":"idem_K","policy_id":"policy_P","rule_id":"rule_R","type":"remove_policy_rule"}`},
		{"update_policy_rule", func() ([]byte, error) { return UpdatePolicyRulePayload("policy_P", "rule_R", "def_str", "idem_K") },
			`{"idempotency_key":"idem_K","policy_id":"policy_P","rule_id":"rule_R","type":"update_policy_rule","updated_definition":"def_str"}`},
		{"add_policy_rule", func() ([]byte, error) {
			return AddPolicyRulePayload("policy_P", "allow", "amount_threshold", map[string]any{"threshold": "100"}, "idem_K")
		},
			`{"action":"allow","definition":{"threshold":"100"},"idempotency_key":"idem_K","policy_id":"policy_P","rule_type":"amount_threshold","type":"add_policy_rule"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.got()
			if err != nil {
				t.Fatal(err)
			}
			if got := string(b); got != tc.want {
				t.Fatalf("%s payload:\n got:  %s\n want: %s", tc.name, got, tc.want)
			}
		})
	}
}
