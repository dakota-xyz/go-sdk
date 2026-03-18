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
type APIKeyData struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Name   string `json:"name"`
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
type CustomerData struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	Status         string   `json:"status"`
	Type           string   `json:"type"`
	ReferenceID    *string  `json:"reference_id,omitempty"`
	PhoneNumber    *string  `json:"phone_number,omitempty"`
	DateOfBirth    *string  `json:"date_of_birth,omitempty"`
	SSNLastFour    *string  `json:"ssn_last_four,omitempty"`
	IncorporatedOn *string  `json:"incorporated_on,omitempty"`
	Address        *Address `json:"address,omitempty"`
}

// ---------------------------------------------------------------------------
// KYB
// ---------------------------------------------------------------------------

// KYBStatusData is the event payload for customer.kyb_status.created and
// customer.kyb_status.updated events.
type KYBStatusData struct {
	CustomerID string  `json:"customer_id"`
	Status     string  `json:"status"`
	Reason     *string `json:"reason,omitempty"`
}

// KYBLinkData is the event payload for customer.kyb_link.created and
// customer.kyb_link.updated events.
type KYBLinkData struct {
	CustomerID string `json:"customer_id"`
	Link       string `json:"link"`
	ExpiresAt  int64  `json:"expires_at"`
}

// KYBApplicationSubmittedData is the event payload for
// customer.kyb_application.submitted events.
type KYBApplicationSubmittedData struct {
	CustomerID string `json:"customer_id"`
	Type       string `json:"type"`
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
	ID                 string         `json:"id"`
	AutoAccountID      string         `json:"auto_account_id"`
	DestinationID      string         `json:"destination_id"`
	Type               string         `json:"type"`
	ProviderID         string         `json:"provider_id"`
	ProviderExternalID string         `json:"provider_external_id"`
	ProviderStatus     string         `json:"provider_status"`
	Status             string         `json:"status"`
	CreatedAt          int64          `json:"created_at"`
	UpdatedAt          int64          `json:"updated_at"`
	FailureReason      *string        `json:"failure_reason,omitempty"`
	CompletedAt        *int64         `json:"completed_at,omitempty"`
	Receipt            *Receipt       `json:"receipt,omitempty"`
	CryptoDetails      *CryptoDetails `json:"crypto_details,omitempty"`
	SenderDetails      *SenderDetails `json:"sender_details,omitempty"`
}

// OneOffTransactionData is the event payload for transaction.one_off.created
// and transaction.one_off.updated events.
type OneOffTransactionData struct {
	ID                  string         `json:"id"`
	CustomerID          string         `json:"customer_id"`
	DestinationID       string         `json:"destination_id"`
	ProviderID          string         `json:"provider_id"`
	ProviderExternalID  string         `json:"provider_external_id"`
	ProviderStatus      string         `json:"provider_status"`
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
	ID          string       `json:"id"`
	CustomerID  string       `json:"customer_id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	BankAccount *BankAccount `json:"bank_account,omitempty"`
}

// RecipientUpdatedData is the event payload for recipient.updated events.
type RecipientUpdatedData struct {
	ID          string       `json:"id"`
	CustomerID  string       `json:"customer_id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	BankAccount *BankAccount `json:"bank_account,omitempty"`
}

// RecipientDeletedData is the event payload for recipient.deleted events.
type RecipientDeletedData struct {
	ID string `json:"id"`
}

// ---------------------------------------------------------------------------
// Destination
// ---------------------------------------------------------------------------

// DestinationData is the event payload for destination.created events.
type DestinationData struct {
	ID          string       `json:"id"`
	CustomerID  string       `json:"customer_id"`
	RecipientID string       `json:"recipient_id"`
	Currency    string       `json:"currency"`
	BankAccount *BankAccount `json:"bank_account,omitempty"`
}

// DestinationDeletedData is the event payload for destination.deleted events.
type DestinationDeletedData struct {
	ID string `json:"id"`
}

// ---------------------------------------------------------------------------
// Target
// ---------------------------------------------------------------------------

// TargetCreatedData is the event payload for target.created events.
type TargetCreatedData struct {
	ID            string `json:"id"`
	AutoAccountID string `json:"auto_account_id"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	Frequency     string `json:"frequency"`
}

// TargetUpdatedData is the event payload for target.updated events.
type TargetUpdatedData struct {
	ID            string `json:"id"`
	AutoAccountID string `json:"auto_account_id"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	Frequency     string `json:"frequency"`
}

// TargetDeletedData is the event payload for target.deleted events.
type TargetDeletedData struct {
	ID string `json:"id"`
}

// ---------------------------------------------------------------------------
// Exception
// ---------------------------------------------------------------------------

// ExceptionData is the event payload for exception.created events.
type ExceptionData struct {
	ID            string  `json:"id"`
	AutoAccountID string  `json:"auto_account_id"`
	TransactionID *string `json:"transaction_id,omitempty"`
	Type          string  `json:"type"`
	Message       string  `json:"message"`
	CreatedAt     int64   `json:"created_at"`
}

// ExceptionClearedData is the event payload for exception.cleared events.
type ExceptionClearedData struct {
	ID            string `json:"id"`
	AutoAccountID string `json:"auto_account_id"`
	ClearedAt     int64  `json:"cleared_at"`
}

// ---------------------------------------------------------------------------
// BVNK
// ---------------------------------------------------------------------------

// BVNKOnboardingData is the event payload for bvnk.onboarding.created and
// bvnk.onboarding.updated events.
type BVNKOnboardingData struct {
	ID         string  `json:"id"`
	CustomerID string  `json:"customer_id"`
	Status     string  `json:"status"`
	Reason     *string `json:"reason,omitempty"`
}
