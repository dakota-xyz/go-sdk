package webhook

import (
	"encoding/json"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
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
	EventCustomerKYBStatusUpdated          EventType = "customer.kyb_status.updated"
	EventCustomerKYBApplicationSubmitted EventType = "customer.kyb_application.submitted"

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

// AllEventTypes contains all known Dakota Platform webhook event types.
var AllEventTypes = []EventType{
	EventUserCreated,
	EventUserUpdated,
	EventUserDeleted,
	EventAPIKeyCreated,
	EventAPIKeyDeleted,
	EventCustomerCreated,
	EventCustomerUpdated,
	EventCustomerKYBLinkCreated,
	EventCustomerKYBLinkUpdated,
	EventCustomerKYBStatusCreated,
	EventCustomerKYBStatusUpdated,
	EventCustomerKYBApplicationSubmitted,
	EventAutoAccountCreated,
	EventAutoAccountUpdated,
	EventAutoAccountDeleted,
	EventTransactionAutoCreated,
	EventTransactionAutoUpdated,
	EventTransactionOneOffCreated,
	EventTransactionOneOffUpdated,
	EventRecipientCreated,
	EventRecipientUpdated,
	EventRecipientDeleted,
	EventDestinationCreated,
	EventDestinationDeleted,
	EventTargetCreated,
	EventTargetUpdated,
	EventTargetDeleted,
	EventExceptionCreated,
	EventExceptionCleared,
	EventBVNKOnboardingCreated,
	EventBVNKOnboardingUpdated,
	EventWalletCreated,
	EventWalletUpdated,
	EventWalletSignerGroupCreated,
	EventWalletSignerGroupUpdated,
	EventWalletPolicyCreated,
	EventWalletPolicyUpdated,
	EventWalletTransactionCreated,
	EventWalletTransactionUpdated,
	EventWalletDeposit,
}

// validEventTypes is the set of all known event types.
var validEventTypes = buildEventTypeSet(AllEventTypes)

func buildEventTypeSet(types []EventType) map[EventType]struct{} {
	m := make(map[EventType]struct{}, len(types))
	for _, t := range types {
		m[t] = struct{}{}
	}
	return m
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
	Type      EventType       `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

// Time returns the event timestamp as a time.Time.
func (e Event) Time() time.Time {
	return time.Unix(e.Timestamp, 0)
}

// DataAs unmarshals the event's Data field into the provided target.
func (e Event) DataAs(target any) error {
	return json.Unmarshal(e.Data, target)
}

// EventDataAs is a generic helper that unmarshals an event's Data field into
// the specified type. This avoids the need for a separate Unmarshal call.
func EventDataAs[T any](event Event) (T, error) {
	var result T
	if err := json.Unmarshal(event.Data, &result); err != nil {
		return result, errors.Wrap(
			errors.CodeMalformedPayload,
			"failed to unmarshal event data",
			err,
		)
	}
	return result, nil
}
