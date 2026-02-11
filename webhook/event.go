package webhook

import (
	"encoding/json"
	"time"
)

// EventType identifies the type of webhook event.
type EventType string

// Event types as defined by Dakota Platform.
const (
	EventUserCreated EventType = "user.created"
	EventUserUpdated EventType = "user.updated"
	EventUserDeleted EventType = "user.deleted"

	EventAPIKeyCreated EventType = "api_key.created"
	EventAPIKeyDeleted EventType = "api_key.deleted"

	EventCustomerCreated EventType = "customer.created"
	EventCustomerUpdated EventType = "customer.updated"

	EventCustomerKYBLinkCreated   EventType = "customer.kyb_link.created"
	EventCustomerKYBLinkUpdated   EventType = "customer.kyb_link.updated"
	EventCustomerKYBStatusCreated EventType = "customer.kyb_status.created"
	EventCustomerKYBStatusUpdated EventType = "customer.kyb_status.updated"

	EventAutoAccountCreated EventType = "auto_account.created"
	EventAutoAccountUpdated EventType = "auto_account.updated"
	EventAutoAccountDeleted EventType = "auto_account.deleted"

	EventTransactionAutoCreated   EventType = "transaction.auto.created"
	EventTransactionAutoUpdated   EventType = "transaction.auto.updated"
	EventTransactionOneOffCreated EventType = "transaction.one_off.created"
	EventTransactionOneOffUpdated EventType = "transaction.one_off.updated"

	EventRecipientCreated EventType = "recipient.created"
	EventRecipientUpdated EventType = "recipient.updated"
	EventRecipientDeleted EventType = "recipient.deleted"

	EventDestinationCreated EventType = "destination.created"
	EventDestinationDeleted EventType = "destination.deleted"

	EventTargetCreated EventType = "target.created"
	EventTargetUpdated EventType = "target.updated"
	EventTargetDeleted EventType = "target.deleted"

	EventExceptionCreated EventType = "exception.created"
	EventExceptionCleared EventType = "exception.cleared"

	EventBVNKOnboardingCreated EventType = "bvnk.onboarding.created"
	EventBVNKOnboardingUpdated EventType = "bvnk.onboarding.updated"

	EventWalletCreated            EventType = "wallet.created"
	EventWalletUpdated            EventType = "wallet.updated"
	EventWalletSignerGroupCreated EventType = "wallet.signer_group.created"
	EventWalletSignerGroupUpdated EventType = "wallet.signer_group.updated"
	EventWalletPolicyCreated      EventType = "wallet.policy.created"
	EventWalletPolicyUpdated      EventType = "wallet.policy.updated"
	EventWalletTransactionCreated EventType = "wallet.transaction.created"
	EventWalletTransactionUpdated EventType = "wallet.transaction.updated"
	EventWalletDeposit            EventType = "wallet.deposit"
)

// validEventTypes is the set of all known event types.
var validEventTypes = map[EventType]struct{}{
	EventUserCreated:              {},
	EventUserUpdated:              {},
	EventUserDeleted:              {},
	EventAPIKeyCreated:            {},
	EventAPIKeyDeleted:            {},
	EventCustomerCreated:          {},
	EventCustomerUpdated:          {},
	EventCustomerKYBLinkCreated:   {},
	EventCustomerKYBLinkUpdated:   {},
	EventCustomerKYBStatusCreated: {},
	EventCustomerKYBStatusUpdated: {},
	EventAutoAccountCreated:       {},
	EventAutoAccountUpdated:       {},
	EventAutoAccountDeleted:       {},
	EventTransactionAutoCreated:   {},
	EventTransactionAutoUpdated:   {},
	EventTransactionOneOffCreated: {},
	EventTransactionOneOffUpdated: {},
	EventRecipientCreated:         {},
	EventRecipientUpdated:         {},
	EventRecipientDeleted:         {},
	EventDestinationCreated:       {},
	EventDestinationDeleted:       {},
	EventTargetCreated:            {},
	EventTargetUpdated:            {},
	EventTargetDeleted:            {},
	EventExceptionCreated:         {},
	EventExceptionCleared:         {},
	EventBVNKOnboardingCreated:    {},
	EventBVNKOnboardingUpdated:    {},
	EventWalletCreated:            {},
	EventWalletUpdated:            {},
	EventWalletSignerGroupCreated: {},
	EventWalletSignerGroupUpdated: {},
	EventWalletPolicyCreated:      {},
	EventWalletPolicyUpdated:      {},
	EventWalletTransactionCreated: {},
	EventWalletTransactionUpdated: {},
	EventWalletDeposit:            {},
}

// String returns the string representation of the event type.
func (t EventType) String() string {
	return string(t)
}

// IsValid reports whether the event type is a known Dakota Platform event.
func (t EventType) IsValid() bool {
	_, ok := validEventTypes[t]
	return ok
}

// Event represents a webhook event from Dakota Platform.
type Event struct {
	ID        string          `json:"id"`
	Type      EventType       `json:"event"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

// DataAs unmarshals the event's Data field into the provided target.
func (e Event) DataAs(target any) error {
	return json.Unmarshal(e.Data, target)
}
