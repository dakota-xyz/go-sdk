// Generates go-sdk client/testdata/governance_intents.json from policy-engine's
// own intent.CanonicalIntentBytes. It sits under testdata, outside the go-sdk
// build, because policy-engine is a private module; README.md says how to run it.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dakota-xyz/policy-engine/pkg/intent"
	"github.com/dakota-xyz/policy-engine/proto/gen/endorsement"
)

type input struct {
	SignerGroupID           string   `json:"signer_group_id,omitempty"`
	SignerGroupVersion      int64    `json:"signer_group_version"`
	IdempotencyKey          string   `json:"idempotency_key"`
	MemberKeys              []string `json:"member_keys,omitempty"`
	GovernanceThreshold     *int32   `json:"governance_threshold,omitempty"`
	GovernanceSignerGroupID *string  `json:"governance_signer_group_id,omitempty"`
	RecipientID             string   `json:"recipient_id,omitempty"`
	NetworkID               string   `json:"network_id,omitempty"`
	Address                 string   `json:"address,omitempty"`
}

type vec struct {
	Name   string `json:"name"`
	Intent string `json:"intent"`
	Input  input  `json:"input"`
	Want   string `json:"want"`
}

func i32(v int32) *int32   { return &v }
func str(v string) *string { return &v }

const (
	spki1 = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEMbuCXgirK7R0SD7rp7e/YLTYbw0ceCI/WA31TtTcZ//y3CNkqgXkKdwHBa3N4S5BI+8pSx1NLYnyfihfmELCgw=="
	spki2 = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEyrrHNsQMwUxGIWthOdpPCAwRwJ2b0vY0rD8m8b6Yw3cQ2F3n0h1m5k7e9P1w8b0gYt7c9xZ4vQ2a1s5d6f7g8h=="
	cose  = "pQECAyYgASFYIONPBSOpT1b1GhA42/UncK14mua0wlAqa3rifDVE+dNIIlggkoBPQr6PDtfwTEhVgmvlrYauaafB3MLQ3AIGo7OEvYo="
	grp   = "3MF5pRlAIRFYkOAd3TstqUKMOpp"
	grp2  = "3MF5pokUeizBfGkQF0N4qhUgRve"
	key1  = "047caa77-8cec-4e96-994c-a12b655231de"
	key2  = "6d41757e-fcb9-4ecb-ac1b-b8a6eed5d027"
)

func main() {
	var out []vec
	add := func(name string, in input, pe any, kind string) {
		b, err := intent.CanonicalIntentBytes(pe)
		if err != nil {
			panic(err)
		}
		out = append(out, vec{Name: name, Intent: kind, Input: in, Want: string(b)})
	}
	addS := func(name string, in input) {
		add(name, in, &endorsement.AddSignersToGroupIntent{SignerGroupId: in.SignerGroupID, NewMemberKeys: in.MemberKeys, IdempotencyKey: in.IdempotencyKey, SignerGroupVersion: in.SignerGroupVersion}, "add_signers_to_group")
	}
	rm := func(name string, in input) {
		add(name, in, &endorsement.RemoveSignersFromGroupIntent{GroupId: in.SignerGroupID, MembersToRemove: in.MemberKeys, IdempotencyKey: in.IdempotencyKey, SignerGroupVersion: in.SignerGroupVersion}, "remove_signers_from_group")
	}
	gov := func(name string, in input) {
		add(name, in, &endorsement.UpdateSignerGroupGovernanceIntent{SignerGroupId: in.SignerGroupID, GovernanceThreshold: in.GovernanceThreshold, GovernanceSignerGroupId: in.GovernanceSignerGroupID, IdempotencyKey: in.IdempotencyKey, SignerGroupVersion: in.SignerGroupVersion}, "update_signer_group_governance")
	}
	dest := func(name string, in input) {
		add(name, in, &endorsement.CreateCryptoDestinationIntent{RecipientId: in.RecipientID, NetworkId: in.NetworkID, Address: in.Address, SignerGroupId: in.SignerGroupID, IdempotencyKey: in.IdempotencyKey}, "create_crypto_destination")
	}

	addS("add, ES256 key, version 0", input{SignerGroupID: grp, SignerGroupVersion: 0, IdempotencyKey: key1, MemberKeys: []string{spki1}})
	addS("add, passkey (COSE) key, version 7", input{SignerGroupID: grp, SignerGroupVersion: 7, IdempotencyKey: key2, MemberKeys: []string{cose}})
	addS("add, policy-engine pinned case", input{SignerGroupID: "sg-1", SignerGroupVersion: 3, IdempotencyKey: "6f1c2d3e-0000-4000-8000-000000000001", MemberKeys: []string{"pk-1"}})
	rm("remove one, version 1", input{SignerGroupID: grp, SignerGroupVersion: 1, IdempotencyKey: key1, MemberKeys: []string{spki1}})
	rm("remove two, order kept (not sorted)", input{SignerGroupID: grp2, SignerGroupVersion: 12, IdempotencyKey: key2, MemberKeys: []string{spki2, cose, spki1}})
	rm("remove, version 0", input{SignerGroupID: grp2, SignerGroupVersion: 0, IdempotencyKey: key1, MemberKeys: []string{cose}})
	gov("governance, both set", input{SignerGroupID: grp, SignerGroupVersion: 3, IdempotencyKey: key1, GovernanceThreshold: i32(2), GovernanceSignerGroupID: str(grp2)})
	gov("governance, threshold only", input{SignerGroupID: grp, SignerGroupVersion: 4, IdempotencyKey: key2, GovernanceThreshold: i32(1)})
	gov("governance, governing group only", input{SignerGroupID: grp, SignerGroupVersion: 5, IdempotencyKey: key1, GovernanceSignerGroupID: str(grp2)})
	gov("governance, both cleared, version 0", input{SignerGroupID: grp, SignerGroupVersion: 0, IdempotencyKey: key2})
	gov("governance, large version", input{SignerGroupID: grp, SignerGroupVersion: 4294967296, IdempotencyKey: key1, GovernanceThreshold: i32(3)})
	dest("crypto destination, EVM mixed-case address", input{RecipientID: "3MF9wAqM2HQHe7eiQ1OU1ovUuWt", NetworkID: "ethereum-mainnet", Address: "0x52908400098527886E0F7030069857D2E4169EE7", SignerGroupID: grp, IdempotencyKey: key1})
	dest("crypto destination, characters JSON escapes (<>& and non-ASCII)", input{RecipientID: "rec<1>", NetworkID: "ethereum-mainnet", Address: "0xé&<>\u2028", SignerGroupID: grp, IdempotencyKey: key1})
	rm("remove, characters JSON escapes in keys", input{SignerGroupID: grp, SignerGroupVersion: 2, IdempotencyKey: key2, MemberKeys: []string{"pk<&>", "clé"}})
	dest("crypto destination, Solana address", input{RecipientID: "3MF9wAqM2HQHe7eiQ1OU1ovUuWt", NetworkID: "solana-mainnet", Address: "7EcDhSYGxXyscszYEp35KHN8vvw3svAuLKTzXwCFLtV", SignerGroupID: grp2, IdempotencyKey: key2})

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"source":  "policy-engine pkg/intent.CanonicalIntentBytes",
		"vectors": out,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
