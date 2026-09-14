package types

// ---------------------------------------------------------------------------
// Capability status
// ---------------------------------------------------------------------------

// CapabilityRequirement is one outstanding requirement gating a capability,
// keyed by an opaque join key — a terms id for a terms_acceptance requirement,
// a document type for a document one — and never by a partner identifier.
//
// Severity "required" blocks submission or unlock; "requested" pre-empts an
// RFI and does not block. Version and URL are present only when the platform
// has them.
type CapabilityRequirement struct {
	Type     string `json:"type"`
	Key      string `json:"key"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Version  string `json:"version,omitempty"`
	URL      string `json:"url,omitempty"`
}

// CustomerCapabilityStatusUpdatedData is the event payload for
// [webhook.EventCustomerCapabilityStatusUpdated]
// ("customer.capability_status.updated"), emitted when a customer's standing
// for a capability (rail) changes.
//
// Status is one of "available", "enabling", "action_required" or
// "unavailable" today; treat the set as open. Requirements lists what is still
// OUTSTANDING; the platform sends an empty array, never omits the key, when
// nothing is owed.
type CustomerCapabilityStatusUpdatedData struct {
	CustomerID   string                  `json:"customer_id"`
	Capability   string                  `json:"capability"`
	Status       string                  `json:"status"`
	Requirements []CapabilityRequirement `json:"requirements"`
}

// ---------------------------------------------------------------------------
// Request for information (RFI)
// ---------------------------------------------------------------------------

// RFIRequirementEntity identifies the party a document belongs to. ID is set
// only for an individual; the business and EDD entities are unique per
// application, so naming the kind is enough to locate them.
type RFIRequirementEntity struct {
	// Kind is "business", "individual" or "edd".
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}

// RFIRequirement is one thing a customer must supply to resolve a request for
// information. Switch on Type:
//
//   - "document": exactly one of DocumentType (a specific kind of document) or
//     Purpose (a compliance requirement several document types could satisfy)
//     is set. Status says whether something is already on file — "on_file"
//     means replace it, "missing" means send it for the first time. Entity is
//     omitted when the platform could not name the party.
//   - "field": Path names the application field to correct.
//   - "question": Key and Prompt carry a reviewer's free-text question. When
//     RequireDocument is true the answer MUST come with an upload; a text-only
//     answer leaves the RFI unresolved, so this is part of the contract and
//     not a hint.
type RFIRequirement struct {
	Type string `json:"type"`

	DocumentType string                `json:"document_type,omitempty"`
	Purpose      string                `json:"purpose,omitempty"`
	Entity       *RFIRequirementEntity `json:"entity,omitempty"`
	Status       string                `json:"status,omitempty"`

	Path string `json:"path,omitempty"`

	Key             string `json:"key,omitempty"`
	Prompt          string `json:"prompt,omitempty"`
	RequireDocument bool   `json:"require_document,omitempty"`
}

// CustomerRFIRequestedData is the event payload for
// [webhook.EventCustomerRFIRequested] ("customer.rfi.requested"), emitted when
// a reviewer opens a request for information against a customer's application.
//
// The client that onboarded the customer owns the relationship, so this event
// is how it learns the customer is blocked and what is still owed.
//
// The resubmission link is intentionally ABSENT: it embeds an access token, and
// a webhook body comes to rest in logs, traces and retry buffers where a
// credential must not. Read the link from the customer resource with your API
// key.
//
// MessageID is the correlation handle: dedupe redeliveries on it, pair a later
// customer.rfi.responded with the request it answers, and tell a
// re-notification from a genuinely new request. It is omitted when the
// platform did not record one.
type CustomerRFIRequestedData struct {
	CustomerID    string           `json:"customer_id"`
	ApplicationID string           `json:"application_id"`
	MessageID     string           `json:"message_id,omitempty"`
	RequestedAt   int64            `json:"requested_at"`
	Requirements  []RFIRequirement `json:"requirements"`
}

// CustomerRFIRespondedData is the event payload for
// [webhook.EventCustomerRFIResponded] ("customer.rfi.responded"): the requested
// information arrived and the application is back in review. Use it to stop
// chasing your customer.
type CustomerRFIRespondedData struct {
	CustomerID    string `json:"customer_id"`
	ApplicationID string `json:"application_id"`
	RespondedAt   int64  `json:"responded_at"`
}

// CustomerApplicationWithdrawnData is the event payload for
// [webhook.EventCustomerApplicationWithdrawn]
// ("customer.application.withdrawn"). Withdrawal is terminal: onboarding the
// customer again requires a new application.
//
// Reason is omitted when no reason was recorded.
type CustomerApplicationWithdrawnData struct {
	CustomerID    string `json:"customer_id"`
	ApplicationID string `json:"application_id"`
	Reason        string `json:"reason,omitempty"`
}
