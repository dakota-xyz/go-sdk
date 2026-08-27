package types

// ---------------------------------------------------------------------------
// User
// ---------------------------------------------------------------------------

// UserData is the event payload for user.created and user.updated events.
type UserData struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// UserDeletedData is the event payload for user.deleted events.
type UserDeletedData struct {
	UserID string `json:"user_id"`
}

// ---------------------------------------------------------------------------
// API Key
// ---------------------------------------------------------------------------

// APIKeyData is the event payload for api_key.created events.
//
// Last6 is the last six characters of the key, the only fragment that is safe
// to display and the one clients use to identify a key in a list. The event
// also carries the key's hash, which the SDK deliberately does not surface.
type APIKeyData struct {
	ID    string `json:"id"`
	Last6 string `json:"last_6"`
}

// APIKeyDeletedData is the event payload for api_key.deleted events.
type APIKeyDeletedData struct {
	ID string `json:"id"`
}

// ---------------------------------------------------------------------------
// Customer
// ---------------------------------------------------------------------------

// CustomerData is the event payload for customer.created and customer.updated
// events.
//
// ExternalID is the client's own identifier for the customer, present only when
// one was supplied. ImportReference appears only on customer.created, and only
// for token-sharing imports.
type CustomerData struct {
	CustomerID      string           `json:"customer_id"`
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	ExternalID      *string          `json:"external_id,omitempty"`
	ImportReference *ImportReference `json:"import_reference,omitempty"`
}

// ImportReference identifies the external record a customer was imported from.
//
// Reference is the client's OWN submitted token — the value they already hold —
// so it can be correlated back to the record they submitted. It is not
// guaranteed unique across events: a retried import can emit a second
// customer.created carrying the same reference. Treat it as a correlation hint,
// not a primary key.
type ImportReference struct {
	Source    string `json:"source"`
	Reference string `json:"reference"`
}

// ---------------------------------------------------------------------------
// KYB
// ---------------------------------------------------------------------------

// KYBStatusData is the event payload for customer.kyb_status.created and
// customer.kyb_status.updated events.
//
// ReasonCode is only ever populated on customer.kyb_status.updated, and only
// for status changes driven by the Proof-of-Address flow (for example
// "pending_proof_of_address", "proof_of_address_approved",
// "proof_of_address_rejected"). It is nil on every other KYB status change.
type KYBStatusData struct {
	CustomerID string  `json:"customer_id"`
	Status     string  `json:"kyb_status"`
	ReasonCode *string `json:"reason_code,omitempty"`
}

// KYBLinkData is the event payload for customer.kyb_link.created and
// customer.kyb_link.updated events.
//
// ExpiresAt is nil whenever the link has no recorded expiry — always check for
// nil before dereferencing. A Persona-backed link only carries an expiry once
// the underlying inquiry has one, so an absent expires_at is the ordinary case
// rather than the exception; legacy TOS links never carry one.
type KYBLinkData struct {
	CustomerID string `json:"customer_id"`
	LinkType   string `json:"link_type"`
	URL        string `json:"url"`
	Status     string `json:"status"`
	ExpiresAt  *int64 `json:"expires_at,omitempty"`
}

// KYBApplicationSubmittedData is the event payload for
// customer.kyb_application.submitted events.
type KYBApplicationSubmittedData struct {
	CustomerID      string `json:"customer_id"`
	ApplicationID   string `json:"application_id"`
	ApplicationType string `json:"application_type"`
}

// ---------------------------------------------------------------------------
// Auto Account
// ---------------------------------------------------------------------------

// AutoAccountData is the event payload for auto_account.created and
// auto_account.updated events.
type AutoAccountData struct {
	ID                string           `json:"id"`
	CustomerID        string           `json:"customer_id"`
	Enabled           bool             `json:"enabled"`
	AccountType       string           `json:"account_type"`
	BankAccount       *BankAccount     `json:"bank_account,omitempty"`
	Crypto            *CryptoRouteInfo `json:"crypto,omitempty"`
	DestinationID     *string          `json:"destination_id,omitempty"`
	OutputAsset       *Asset           `json:"output_asset,omitempty"`
	RoutingPreference *string          `json:"routing_preference,omitempty"`
}

// AutoAccountDeletedData is the event payload for auto_account.deleted events.
type AutoAccountDeletedData struct {
	ID string `json:"id"`
}

// ---------------------------------------------------------------------------
// Transaction
// ---------------------------------------------------------------------------

// AutoTransactionData is the event payload for transaction.auto.created and
// transaction.auto.updated events.
type AutoTransactionData struct {
	ID            string         `json:"id"`
	AutoAccountID string         `json:"auto_account_id"`
	DestinationID string         `json:"destination_id"`
	Type          string         `json:"type"`
	Status        string         `json:"status"`
	CreatedAt     int64          `json:"created_at"`
	UpdatedAt     int64          `json:"updated_at"`
	FailureReason *string        `json:"failure_reason,omitempty"`
	CompletedAt   *int64         `json:"completed_at,omitempty"`
	Receipt       *Receipt       `json:"receipt,omitempty"`
	CryptoDetails *CryptoDetails `json:"crypto_details,omitempty"`
	SenderDetails *SenderDetails `json:"sender_details,omitempty"`
}

// OneOffTransactionData is the event payload for transaction.one_off.created
// and transaction.one_off.updated events.
type OneOffTransactionData struct {
	ID                  string         `json:"id"`
	CustomerID          string         `json:"customer_id"`
	DestinationID       string         `json:"destination_id"`
	SourceAsset         string         `json:"source_asset"`
	SourceNetworkID     string         `json:"source_network_id"`
	DestinationAmount   string         `json:"destination_amount"`
	DestinationCurrency string         `json:"destination_currency"`
	Status              string         `json:"status"`
	CreatedAt           int64          `json:"created_at"`
	UpdatedAt           int64          `json:"updated_at"`
	FailureReason       *string        `json:"failure_reason,omitempty"`
	PaymentReference    *string        `json:"payment_reference,omitempty"`
	TemporaryAddress    *string        `json:"temporary_address,omitempty"`
	SendAmount          *SendAmount    `json:"send_amount,omitempty"`
	CompletedAt         *int64         `json:"completed_at,omitempty"`
	Receipt             *Receipt       `json:"receipt,omitempty"`
	CryptoDetails       *CryptoDetails `json:"crypto_details,omitempty"`
	SenderDetails       *SenderDetails `json:"sender_details,omitempty"`
}

// SendAmount describes an amount in a specific currency.
type SendAmount struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// ---------------------------------------------------------------------------
// Recipient
// ---------------------------------------------------------------------------

// RecipientData is the event payload for recipient.created events.
type RecipientData struct {
	ID         string   `json:"id"`
	CustomerID string   `json:"customer_id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Address    *Address `json:"address,omitempty"`
}

// RecipientUpdatedData is the event payload for recipient.updated events.
//
// Unlike recipient.created and recipient.deleted, this event carries no
// customer_id. Resolve the owning customer from ID, or hold onto the
// customer_id delivered with recipient.created.
type RecipientUpdatedData struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Address *Address `json:"address,omitempty"`
}

// RecipientDeletedData is the event payload for recipient.deleted events.
type RecipientDeletedData struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
}

// ---------------------------------------------------------------------------
// Destination
// ---------------------------------------------------------------------------

// DestinationData is the event payload for destination.created events.
//
// Exactly one of Crypto or BankAccount is populated, per Type.
type DestinationData struct {
	ID          string           `json:"id"`
	RecipientID string           `json:"recipient_id"`
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Crypto      *CryptoRouteInfo `json:"crypto,omitempty"`
	BankAccount *BankAccount     `json:"bank_account,omitempty"`
}

// DestinationDeletedData is the event payload for destination.deleted events.
type DestinationDeletedData struct {
	ID          string `json:"id"`
	RecipientID string `json:"recipient_id"`
}

// ---------------------------------------------------------------------------
// Target
// ---------------------------------------------------------------------------

// TargetCreatedData is the event payload for target.created events.
//
// A target is a registered webhook endpoint. Global reports whether the
// endpoint receives every event type.
//
// EventTypes lists the subscribed types. The platform omits the key unless the
// target is both non-global and subscribed to at least one type, so an empty
// EventTypes does not imply a global target: read Global for that. Its order is
// not meaningful — the platform builds the list by ranging a map — so compare
// it as a set and never equality-diff two payloads on it.
//
// Note the endpoint URL arrives as `url` on target.created but as `target_url`
// on target.updated and target.deleted, which is why these are three types
// rather than one.
type TargetCreatedData struct {
	TargetID   string   `json:"target_id"`
	URL        string   `json:"url"`
	Global     bool     `json:"global"`
	EventTypes []string `json:"event_types,omitempty"`
}

// TargetUpdatedData is the event payload for target.updated events. Global and
// EventTypes carry the same caveats as on [TargetCreatedData].
type TargetUpdatedData struct {
	TargetID   string   `json:"target_id"`
	TargetURL  string   `json:"target_url"`
	Global     bool     `json:"global"`
	EventTypes []string `json:"event_types,omitempty"`
}

// TargetDeletedData is the event payload for target.deleted events.
type TargetDeletedData struct {
	TargetID  string `json:"target_id"`
	TargetURL string `json:"target_url"`
}

// ---------------------------------------------------------------------------
// Exception
// ---------------------------------------------------------------------------

// ExceptionData is the event payload for exception.created events.
//
// CustomerID is present only for an exception raised against a customer.
// Content is an open-ended, per-Type detail blob; it has no fixed schema, so it
// stays a map rather than pretending to a struct.
//
// Neither this event nor exception.cleared carries a timestamp of its own — use
// the enclosing envelope's Created.
type ExceptionData struct {
	ExceptionID string         `json:"exception_id"`
	Type        string         `json:"type"`
	CustomerID  *string        `json:"customer_id,omitempty"`
	Content     map[string]any `json:"exception_content,omitempty"`
}

// ExceptionClearedData is the event payload for exception.cleared events.
type ExceptionClearedData struct {
	ExceptionID string  `json:"exception_id"`
	Type        string  `json:"type"`
	CustomerID  *string `json:"customer_id,omitempty"`
}
