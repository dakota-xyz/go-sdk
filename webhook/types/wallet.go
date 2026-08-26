package types

// WalletEventData is the event payload for wallet.created and wallet.updated
// events.
type WalletEventData struct {
	Wallet       WalletContent        `json:"wallet"`
	SignerGroups []SignerGroupContent `json:"signer_groups"`
	Policies     []PolicyContent      `json:"policies"`
}

// WalletContent describes a wallet within a wallet event.
type WalletContent struct {
	ID         string  `json:"id"`
	ClientID   string  `json:"client_id"`
	Family     string  `json:"family"`
	Address    string  `json:"address"`
	Name       string  `json:"name"`
	CreatedAt  int64   `json:"created_at"`
	UpdatedAt  int64   `json:"updated_at"`
	CustomerID *string `json:"customer_id,omitempty"`
	DeletedAt  *int64  `json:"deleted_at,omitempty"`
}

// SignerGroupContent describes a signer group within a wallet event.
type SignerGroupContent struct {
	ID       string              `json:"id"`
	ClientID string              `json:"client_id"`
	Name     string              `json:"name"`
	Members  []SignerGroupMember `json:"members,omitempty"`
}

// SignerGroupMember describes a member of a signer group.
type SignerGroupMember struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	KeyType   string `json:"key_type"`
}

// PolicyContent describes a policy within a wallet event. It also stands
// alone as the entire top-level payload of wallet.policy.created and
// wallet.policy.updated.
type PolicyContent struct {
	ID            string              `json:"id"`
	ClientID      string              `json:"client_id"`
	SignerGroupID string              `json:"signer_group_id"`
	Name          string              `json:"name"`
	Description   *string             `json:"description,omitempty"`
	Rules         []PolicyRuleContent `json:"rules,omitempty"`
}

// PolicyRuleContent describes a rule within a policy.
type PolicyRuleContent struct {
	ID         string                `json:"id"`
	PolicyID   string                `json:"policy_id"`
	RuleType   string                `json:"rule_type"`
	Action     string                `json:"action"`
	CreatedAt  int64                 `json:"created_at"`
	Definition *PolicyRuleDefinition `json:"definition,omitempty"`
}

// PolicyRuleDefinition contains the specific rule configuration.
type PolicyRuleDefinition struct {
	ApprovalThreshold *ApprovalThreshold `json:"approval_threshold,omitempty"`
	AmountThreshold   *AmountThreshold   `json:"amount_threshold,omitempty"`
	AddressList       *AddressList       `json:"address_list,omitempty"`
}

// ApprovalThreshold defines a quorum-based approval rule.
type ApprovalThreshold struct {
	Threshold   int     `json:"threshold"`
	Description *string `json:"description,omitempty"`
}

// AmountThreshold defines an amount-based approval rule.
//
// MinAmount is a whole number of the asset's smallest currency unit, sent as a
// JSON number rather than a decimal string. Divide by the asset's scale before
// displaying it.
type AmountThreshold struct {
	MinAmount int64 `json:"min_amount"`
	Threshold int   `json:"threshold"`
	Asset     Asset `json:"asset"`
}

// AddressList defines an address allowlist rule.
type AddressList struct {
	Addresses []string `json:"addresses"`
}

// WalletDepositData is the event payload for wallet.deposit events.
type WalletDepositData struct {
	WalletID        string       `json:"wallet_id"`
	TransactionHash string       `json:"transaction_hash"`
	Sender          string       `json:"sender"`
	Recipient       string       `json:"recipient"`
	Amount          string       `json:"amount"`
	ChainID         *string      `json:"chain_id,omitempty"`
	Asset           *WalletAsset `json:"asset,omitempty"`
}

// WalletAsset describes a token or native asset on a specific network.
type WalletAsset struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	NetworkID       string `json:"network_id"`
	ContractAddress string `json:"contract_address"`
	Decimals        int    `json:"decimals"`
	TokenStandard   string `json:"token_standard"`
}

// WalletTransactionData is the event payload for wallet.transaction.created
// and wallet.transaction.updated events.
type WalletTransactionData struct {
	ID          string                  `json:"id"`
	WalletID    string                  `json:"wallet_id"`
	Status      string                  `json:"status"`
	ExternalID  *string                 `json:"external_id,omitempty"`
	Hash        *string                 `json:"hash,omitempty"`
	ConfirmedAt *int64                  `json:"confirmed_at,omitempty"`
	Intent      WalletTransactionIntent `json:"intent"`
}

// WalletTransactionIntent describes the intent of a wallet transaction.
type WalletTransactionIntent struct {
	WalletID       string                     `json:"wallet_id"`
	Caip2          string                     `json:"caip2"`
	IdempotencyKey string                     `json:"idempotency_key"`
	Operation      WalletTransactionOperation `json:"operation"`
}

// WalletTransactionOperation describes a single operation within a wallet
// transaction.
type WalletTransactionOperation struct {
	Kind    string  `json:"kind"`
	From    string  `json:"from"`
	To      string  `json:"to"`
	Amount  string  `json:"amount"`
	AssetID string  `json:"asset_id"`
	Data    *string `json:"data,omitempty"`
}
