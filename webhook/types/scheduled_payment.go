package types

// ScheduledPaymentFailedData is the event payload for
// [webhook.EventScheduledPaymentFailed] ("scheduled_payment.failed"), emitted
// when a scheduled payment flips to the failed terminal state.
//
// A successful scheduled fire already surfaces as wallet.transaction.created
// through the shared money path; failure is the actionable case (re-approve an
// expired mandate, fund the wallet, fix a destination), and FailureReason
// carries the humanized verdict while FailureCode is the stable machine code.
//
// PaymentAgentID, RecipientID, and DestinationID are omitted for schedules that
// have no such linkage (a signer-native schedule, or one paying a bare address);
// Address is always the crypto address the row pays.
type ScheduledPaymentFailedData struct {
	ScheduledPaymentID string `json:"scheduled_payment_id"`
	SignerID           string `json:"signer_id"`
	WalletID           string `json:"wallet_id"`
	Address            string `json:"address"`
	Amount             string `json:"amount"`
	Asset              string `json:"asset"`
	NetworkID          string `json:"network_id"`
	ScheduledAt        int64  `json:"scheduled_at"`
	FailureCode        string `json:"failure_code"`
	FailureReason      string `json:"failure_reason"`
	PaymentAgentID     string `json:"payment_agent_id,omitempty"`
	RecipientID        string `json:"recipient_id,omitempty"`
	DestinationID      string `json:"destination_id,omitempty"`
}
