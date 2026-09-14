package types

// ---------------------------------------------------------------------------
// Payout destinations
// ---------------------------------------------------------------------------

// FeePayoutDestinationUpdatedData is the event payload for
// [webhook.EventFeePayoutDestinationUpdated] ("fee_payout_destination.updated").
//
// Type is the destination kind — "usdc_wallet" today, developer-fee payouts
// being crypto-only. The event carries nothing else; read the destination
// itself with GET /fee-payout-destination.
type FeePayoutDestinationUpdatedData struct {
	Type string `json:"type"`
}

// RDPayoutDestinationUpdatedData is the event payload for
// [webhook.EventRDPayoutDestinationUpdated] ("rd_payout_destination.updated"),
// emitted when a client sets or replaces the wallet its RD marketing fee is
// sent to.
//
// A destination has no history of its own — a PUT replaces the row — so this
// event is the only record that it changed, which is why it carries the
// address, what it replaced, and who did it. PreviousAddress is the empty
// string on a first registration. UpdatedBy is the EMAIL of the dashboard
// user who made the change — personal data, so treat a logged payload
// accordingly — and the empty string when the change is not attributable to a
// user. Both keys are always present.
type RDPayoutDestinationUpdatedData struct {
	WalletAddress   string `json:"wallet_address"`
	PreviousAddress string `json:"previous_address"`
	UpdatedBy       string `json:"updated_by"`
}
