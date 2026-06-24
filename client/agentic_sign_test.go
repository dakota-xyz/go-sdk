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
