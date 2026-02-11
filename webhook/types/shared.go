// Package types provides typed Go structs for all Dakota Platform webhook
// event data payloads. Use these with [webhook.EventDataAs] to unmarshal
// event data into concrete types instead of working with raw JSON.
package types

// Address represents a physical address.
type Address struct {
	Street1    string  `json:"street1"`
	City       string  `json:"city"`
	Country    string  `json:"country"`
	Street2    *string `json:"street2,omitempty"`
	Street3    *string `json:"street3,omitempty"`
	Region     *string `json:"region,omitempty"`
	PostalCode *string `json:"postal_code,omitempty"`
}

// USBankDetails contains US-specific bank account details.
type USBankDetails struct {
	AccountType       string  `json:"account_type"`
	AccountNumber     string  `json:"account_number"`
	RoutingNumber     string  `json:"routing_number"`
	WireRoutingNumber *string `json:"wire_routing_number,omitempty"`
}

// BankAccount represents a bank account with optional regional details.
type BankAccount struct {
	AccountHolderName    string         `json:"account_holder_name"`
	BankAddress          *Address       `json:"bank_address,omitempty"`
	AccountHolderAddress *Address       `json:"account_holder_address,omitempty"`
	USDetails            *USBankDetails `json:"us_details,omitempty"`
	BIC                  *string        `json:"bic,omitempty"`
	IBAN                 *string        `json:"iban,omitempty"`
}

// CryptoRouteInfo describes a crypto routing destination.
type CryptoRouteInfo struct {
	NetworkID string `json:"network_id"`
	Address   string `json:"address"`
}

// Asset identifies a financial asset.
type Asset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Receipt contains the financial breakdown of a transaction.
type Receipt struct {
	InputCurrency   string  `json:"input_currency"`
	OutputCurrency  string  `json:"output_currency"`
	InitialAmount   string  `json:"initial_amount"`
	SubtotalAmount  string  `json:"subtotal_amount"`
	ConvertedAmount string  `json:"converted_amount"`
	OutgoingAmount  string  `json:"outgoing_amount"`
	ExternalFee     string  `json:"external_fee"`
	ClientFee       string  `json:"client_fee"`
	DakotaFee       string  `json:"dakota_fee"`
	ExchangeRate    string  `json:"exchange_rate"`
	GasFee          *string `json:"gas_fee,omitempty"`
	IMAD            *string `json:"imad,omitempty"`
}

// CryptoDetails contains blockchain-specific transaction metadata.
type CryptoDetails struct {
	SourceNetworkID      *string `json:"source_network_id,omitempty"`
	SourceAddress        *string `json:"source_address,omitempty"`
	DestinationNetworkID *string `json:"destination_network_id,omitempty"`
	DestinationAddress   *string `json:"destination_address,omitempty"`
	TxHash               *string `json:"tx_hash,omitempty"`
	DepositTxHash        *string `json:"deposit_tx_hash,omitempty"`
	LogIndex             *int    `json:"log_index,omitempty"`
	LogInnerIndex        *int    `json:"log_inner_index,omitempty"`
}

// SenderDetails contains information about the sender of a transaction.
type SenderDetails struct {
	SenderType              *string `json:"sender_type,omitempty"`
	SenderAccountHolderName *string `json:"sender_account_holder_name,omitempty"`
	SenderBankName          *string `json:"sender_bank_name,omitempty"`
	SenderRoutingNumber     *string `json:"sender_routing_number,omitempty"`
	SenderAccountNumber     *string `json:"sender_account_number,omitempty"`
	SenderWireRoutingNumber *string `json:"sender_wire_routing_number,omitempty"`
	SenderAccountType       *string `json:"sender_account_type,omitempty"`
	SenderIBAN              *string `json:"sender_iban,omitempty"`
	SenderBIC               *string `json:"sender_bic,omitempty"`
	SenderWalletAddress     *string `json:"sender_wallet_address,omitempty"`
	SenderNetwork           *string `json:"sender_network,omitempty"`
}
