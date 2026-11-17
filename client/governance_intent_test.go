package client

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type governanceVector struct {
	Name   string `json:"name"`
	Intent string `json:"intent"`
	Input  struct {
		SignerGroupID           string   `json:"signer_group_id"`
		SignerGroupVersion      int64    `json:"signer_group_version"`
		IdempotencyKey          string   `json:"idempotency_key"`
		MemberKeys              []string `json:"member_keys"`
		GovernanceThreshold     *int32   `json:"governance_threshold"`
		GovernanceSignerGroupID *string  `json:"governance_signer_group_id"`
		RecipientID             string   `json:"recipient_id"`
		NetworkID               string   `json:"network_id"`
		Address                 string   `json:"address"`
	} `json:"input"`
	Want string `json:"want"`
}

// TestGovernanceIntentGoldenVectors proves each builder's bytes equal
// policy-engine's intent.CanonicalIntentBytes for the same intent. The vectors
// come from policy-engine itself (testdata/governance-vectors-generator), never
// from the SDK, so a drift on either side fails here.
func TestGovernanceIntentGoldenVectors(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/governance_intents.json")
	require.NoError(t, err)
	var file struct {
		Vectors []governanceVector `json:"vectors"`
	}
	require.NoError(t, json.Unmarshal(raw, &file))
	require.NotEmpty(t, file.Vectors)

	for _, v := range file.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			in := v.Input
			var got []byte
			var err error
			switch v.Intent {
			case IntentAddSignersToGroup:
				got, err = AddSignersToGroupIntent(in.SignerGroupID, in.SignerGroupVersion, in.IdempotencyKey, in.MemberKeys)
			case IntentRemoveSignersFromGroup:
				got, err = RemoveSignersFromGroupIntent(in.SignerGroupID, in.SignerGroupVersion, in.IdempotencyKey, in.MemberKeys)
			case IntentUpdateSignerGroupGovernance:
				got, err = UpdateSignerGroupGovernanceIntent(in.SignerGroupID, in.SignerGroupVersion, in.IdempotencyKey, GovernanceSettings{
					GovernanceThreshold:     in.GovernanceThreshold,
					GovernanceSignerGroupID: in.GovernanceSignerGroupID,
				})
			case IntentCreateCryptoDestination:
				got, err = CreateCryptoDestinationIntent(in.RecipientID, in.NetworkID, in.Address, in.SignerGroupID, in.IdempotencyKey)
			default:
				t.Fatalf("unknown intent %q in vectors", v.Intent)
			}
			require.NoError(t, err)
			require.Equal(t, v.Want, string(got))
		})
	}
}

// TestGovernanceIntentPolicyEnginePinned holds policy-engine's own pinned
// strings (pkg/intent/reconstruct_governance_test.go) whose idempotency key is
// a UUID, so the public builders accept them verbatim.
func TestGovernanceIntentPolicyEnginePinned(t *testing.T) {
	t.Parallel()
	got, err := AddSignersToGroupIntent("sg-1", 3, "6f1c2d3e-0000-4000-8000-000000000001", []string{"pk-1"})
	require.NoError(t, err)
	require.Equal(t,
		`{"idempotency_key":"6f1c2d3e-0000-4000-8000-000000000001","new_member_keys":["pk-1"],"signer_group_id":"sg-1","signer_group_version":3,"type":"add_signers_to_group"}`,
		string(got))
}

func TestGovernanceIntentRefusesBadInput(t *testing.T) {
	t.Parallel()
	const key = "047caa77-8cec-4e96-994c-a12b655231de"
	one := []string{"pk"}
	two := int32(2)
	zero := int32(0)
	empty := ""
	cases := map[string]func() ([]byte, error){
		// The platform refuses a key that is not a lowercase UUID, so a
		// signature over one could never be used.
		"uppercase key": func() ([]byte, error) {
			return AddSignersToGroupIntent("sg", 0, "047CAA77-8CEC-4E96-994C-A12B655231DE", one)
		},
		"non-UUID key":   func() ([]byte, error) { return AddSignersToGroupIntent("sg", 0, "k", one) },
		"braced UUID":    func() ([]byte, error) { return AddSignersToGroupIntent("sg", 0, "{"+key+"}", one) },
		"empty group":    func() ([]byte, error) { return AddSignersToGroupIntent("", 0, key, one) },
		"no member keys": func() ([]byte, error) { return AddSignersToGroupIntent("sg", 0, key, nil) },
		"empty member":   func() ([]byte, error) { return AddSignersToGroupIntent("sg", 0, key, []string{""}) },
		"negative version": func() ([]byte, error) {
			return RemoveSignersFromGroupIntent("sg", -1, key, one)
		},
		"remove nothing": func() ([]byte, error) { return RemoveSignersFromGroupIntent("sg", 0, key, []string{}) },
		"threshold 0": func() ([]byte, error) {
			return UpdateSignerGroupGovernanceIntent("sg", 0, key, GovernanceSettings{GovernanceThreshold: &zero})
		},
		// An empty governing group is not "clear it" (that is nil); refuse it
		// rather than sign a value the platform rejects.
		"empty governing group": func() ([]byte, error) {
			return UpdateSignerGroupGovernanceIntent("sg", 0, key, GovernanceSettings{GovernanceThreshold: &two, GovernanceSignerGroupID: &empty})
		},
		"destination without network": func() ([]byte, error) {
			return CreateCryptoDestinationIntent("rec", "", "0xabc", "sg", key)
		},
		"destination without address": func() ([]byte, error) {
			return CreateCryptoDestinationIntent("rec", "ethereum-mainnet", "", "sg", key)
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := build()
			require.Error(t, err)
			require.Nil(t, got)
		})
	}
}
