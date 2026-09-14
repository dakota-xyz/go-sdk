package types_test

import (
	"testing"

	"github.com/dakota-xyz/go-sdk/webhook"
	"github.com/dakota-xyz/go-sdk/webhook/types"
)

// Fixtures shaped from the platform's emitters
// (pkg/core/events/event_fee_payout_destination_updated.go,
// event_rd_payout_destination_updated.go).

func TestEventDataAs_FeePayoutDestinationUpdatedData(t *testing.T) {
	data := decodeEvent[types.FeePayoutDestinationUpdatedData](
		t, "evt_fee_payout_updated", webhook.EventFeePayoutDestinationUpdated, `{"type":"usdc_wallet"}`,
	)
	if data.Type != "usdc_wallet" {
		t.Errorf("Type = %q, want usdc_wallet", data.Type)
	}
}

func TestEventDataAs_RDPayoutDestinationUpdatedData(t *testing.T) {
	// A first registration: previous_address and updated_by are present and
	// empty, not absent — the emitter writes every key unconditionally.
	first := `{"wallet_address":"0x1234567890123456789012345678901234567890","previous_address":"","updated_by":""}`
	firstData := decodeEvent[types.RDPayoutDestinationUpdatedData](
		t, "evt_rd_payout_first", webhook.EventRDPayoutDestinationUpdated, first,
	)
	if firstData.WalletAddress != "0x1234567890123456789012345678901234567890" || firstData.PreviousAddress != "" {
		t.Fatalf("unexpected decode: %+v", firstData)
	}

	replaced := `{"wallet_address":"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd","previous_address":"0x1234567890123456789012345678901234567890","updated_by":"usr_1"}`
	replacedData := decodeEvent[types.RDPayoutDestinationUpdatedData](
		t, "evt_rd_payout_replaced", webhook.EventRDPayoutDestinationUpdated, replaced,
	)
	if replacedData.PreviousAddress != "0x1234567890123456789012345678901234567890" || replacedData.UpdatedBy != "usr_1" {
		t.Errorf("unexpected decode: %+v", replacedData)
	}
}
