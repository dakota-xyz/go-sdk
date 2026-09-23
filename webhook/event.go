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

	EventCustomerKYBLinkCreated          EventType = "customer.kyb_link.created"
	EventCustomerKYBLinkUpdated          EventType = "customer.kyb_link.updated"
	EventCustomerKYBStatusCreated        EventType = "customer.kyb_status.created"
	EventCustomerKYBStatusUpdated        EventType = "customer.kyb_status.updated"
	EventCustomerKYBApplicationSubmitted EventType = "customer.kyb_application.submitted"

	// EventCustomerCapabilityStatusUpdated is emitted when a customer's
	// standing for a capability (rail) changes. Its payload is
	// [types.CustomerCapabilityStatusUpdatedData]: the capability, the new
	// status, and the OUTSTANDING requirements, keyed by opaque join keys and
	// never by a partner name.
	EventCustomerCapabilityStatusUpdated EventType = "customer.capability_status.updated"

	// EventCustomerRFIRequested is emitted when a reviewer opens a request for
	// information against a customer's application. Its payload is
	// [types.CustomerRFIRequestedData]: what is still owed, but deliberately
	// NOT the resubmission link — that link embeds an access token, and a
	// webhook body comes to rest in logs, traces and retry buffers. Read the
	// link from the customer resource over your own authenticated channel.
	EventCustomerRFIRequested EventType = "customer.rfi.requested"
	// EventCustomerRFIResponded is emitted when the requested information
	// arrives and the application returns to review. Its payload is
	// [types.CustomerRFIRespondedData]. There is no rfi.resolved event: an
	// RFI ends in a decision, which customer.kyb_status.updated reports.
	EventCustomerRFIResponded EventType = "customer.rfi.responded"
	// EventCustomerApplicationWithdrawn is emitted when an onboarding
	// application is withdrawn, by the applicant or by the client on their
	// behalf. Withdrawal is terminal. Its payload is
	// [types.CustomerApplicationWithdrawnData].
	EventCustomerApplicationWithdrawn EventType = "customer.application.withdrawn"

	// EventFeePayoutDestinationUpdated is emitted when a client registers or
	// replaces its developer-fee payout destination. Its payload is
	// [types.FeePayoutDestinationUpdatedData].
	EventFeePayoutDestinationUpdated EventType = "fee_payout_destination.updated"
	// EventFeePayoutDestinationDeleted is emitted when that destination is
	// removed. Its payload is an empty object.
	EventFeePayoutDestinationDeleted EventType = "fee_payout_destination.deleted"
	// EventRDPayoutDestinationUpdated is emitted when a client sets or
	// replaces the wallet its RD marketing fee is sent to. Its payload is
	// [types.RDPayoutDestinationUpdatedData].
	EventRDPayoutDestinationUpdated EventType = "rd_payout_destination.updated"
	// EventDeploymentPayoutDestinationUpdated is emitted when a client sets or
	// replaces the wallet one NON-RD deployment's payouts are sent to. Its
	// payload is [types.DeploymentPayoutDestinationUpdatedData], which names
	// the deployment's asset and network — RD keeps its own event, so a
	// subscriber to that one never receives another deployment's wallet.
	EventDeploymentPayoutDestinationUpdated EventType = "deployment_payout_destination.updated"

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

	EventWalletCreated            EventType = "wallet.created"
	EventWalletUpdated            EventType = "wallet.updated"
	EventWalletSignerGroupCreated EventType = "wallet.signer_group.created"
	EventWalletSignerGroupUpdated EventType = "wallet.signer_group.updated"
	EventWalletPolicyCreated      EventType = "wallet.policy.created"
	EventWalletPolicyUpdated      EventType = "wallet.policy.updated"
	EventWalletTransactionCreated EventType = "wallet.transaction.created"
	EventWalletTransactionUpdated EventType = "wallet.transaction.updated"
	EventWalletDeposit            EventType = "wallet.deposit"

	// EventScheduledPaymentFailed is emitted when a scheduled payment flips to
	// the failed terminal state (the async agentic-payments actor's one silent
	// state change). Its payload is [types.ScheduledPaymentFailedData].
	EventScheduledPaymentFailed EventType = "scheduled_payment.failed"
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
	EventCustomerCapabilityStatusUpdated,
	EventCustomerRFIRequested,
	EventCustomerRFIResponded,
	EventCustomerApplicationWithdrawn,
	EventFeePayoutDestinationUpdated,
	EventFeePayoutDestinationDeleted,
	EventRDPayoutDestinationUpdated,
	EventDeploymentPayoutDestinationUpdated,
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
	EventWalletCreated,
	EventWalletUpdated,
	EventWalletSignerGroupCreated,
	EventWalletSignerGroupUpdated,
	EventWalletPolicyCreated,
	EventWalletPolicyUpdated,
	EventWalletTransactionCreated,
	EventWalletTransactionUpdated,
	EventWalletDeposit,
	EventScheduledPaymentFailed,
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

// Event represents a webhook event from Dakota Platform. It mirrors the
// platform's public event envelope: the resource the event is about lives under
// Data.Object (decode it with [Event.DataAs] or [EventDataAs]), and Created is
// the emission time in unix seconds.
type Event struct {
	ID         string         `json:"id"`
	Type       EventType      `json:"type"`
	Created    int64          `json:"created"`
	APIVersion string         `json:"api_version,omitempty"`
	Data       EventData      `json:"data"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Request    *EventRequest  `json:"request,omitempty"`
}

// EventData is the event payload container. Object holds the resource the event
// is about; PreviousAttributes, when present, holds the prior values of the
// fields that changed on an update event.
type EventData struct {
	Object             json.RawMessage `json:"object"`
	PreviousAttributes json.RawMessage `json:"previous_attributes,omitempty"`
}

// EventRequest captures the request context that produced the event, when known.
type EventRequest struct {
	ID             *string `json:"id,omitempty"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

// Time returns the event creation time.
func (e Event) Time() time.Time {
	return time.Unix(e.Created, 0)
}

// DataAs unmarshals the event's payload (Data.Object) into target.
func (e Event) DataAs(target any) error {
	return unmarshalObject(e.Data.Object, target)
}

// EventDataAs is a generic helper that unmarshals an event's payload
// (Data.Object) into the specified type, avoiding a separate Unmarshal call.
func EventDataAs[T any](event Event) (T, error) {
	var result T
	if err := unmarshalObject(event.Data.Object, &result); err != nil {
		return result, errors.Wrap(
			errors.CodeMalformedPayload,
			"failed to unmarshal event data",
			err,
		)
	}
	return result, nil
}

// unmarshalObject decodes a Data.Object payload into target, treating an absent
// object (null / empty) as an empty object so callers get a zero value instead
// of an "unexpected end of JSON input" error.
func unmarshalObject(object json.RawMessage, target any) error {
	if len(object) == 0 {
		object = json.RawMessage("{}")
	}
	return json.Unmarshal(object, target)
}
