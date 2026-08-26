package types_test

import (
	"encoding/json"
	"testing"

	"github.com/dakota-xyz/go-sdk/webhook"
	"github.com/dakota-xyz/go-sdk/webhook/types"
)

func TestEventDataAs_UserData(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantUserID string
		wantEmail  string
	}{
		{
			name:       "user.created",
			payload:    `{"user_id":"usr_1","email":"alice@example.com"}`,
			wantUserID: "usr_1",
			wantEmail:  "alice@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				event := webhook.Event{
					ID:   "evt_1",
					Type: webhook.EventUserCreated,
					Data: webhook.EventData{Object: json.RawMessage(tt.payload)},
				}

				data, err := webhook.EventDataAs[types.UserData](event)
				if err != nil {
					t.Fatalf("EventDataAs error: %v", err)
				}
				if data.UserID != tt.wantUserID {
					t.Errorf("UserID = %q, want %q", data.UserID, tt.wantUserID)
				}
				if data.Email != tt.wantEmail {
					t.Errorf("Email = %q, want %q", data.Email, tt.wantEmail)
				}
			},
		)
	}
}

func TestEventDataAs_UserDeletedData(t *testing.T) {
	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventUserDeleted,
		Data: webhook.EventData{Object: json.RawMessage(`{"user_id":"usr_1"}`)},
	}

	data, err := webhook.EventDataAs[types.UserDeletedData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.UserID != "usr_1" {
		t.Errorf("UserID = %q, want %q", data.UserID, "usr_1")
	}
}

func TestEventDataAs_APIKeyData(t *testing.T) {
	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventAPIKeyCreated,
		Data: webhook.EventData{Object: json.RawMessage(`{"id":"key_1","user_id":"usr_1","name":"My Key"}`)},
	}

	data, err := webhook.EventDataAs[types.APIKeyData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ID != "key_1" {
		t.Errorf("ID = %q, want %q", data.ID, "key_1")
	}
	if data.UserID != "usr_1" {
		t.Errorf("UserID = %q, want %q", data.UserID, "usr_1")
	}
	if data.Name != "My Key" {
		t.Errorf("Name = %q, want %q", data.Name, "My Key")
	}
}

func TestEventDataAs_CustomerData(t *testing.T) {
	payload := `{
		"id":"cust_1",
		"name":"Acme Corp",
		"email":"admin@acme.com",
		"status":"active",
		"type":"business",
		"address":{"street1":"123 Main St","city":"New York","country":"US"}
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventCustomerCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.CustomerData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ID != "cust_1" {
		t.Errorf("ID = %q, want %q", data.ID, "cust_1")
	}
	if data.Name != "Acme Corp" {
		t.Errorf("Name = %q, want %q", data.Name, "Acme Corp")
	}
	if data.Address == nil {
		t.Fatal("expected non-nil Address")
	}
	if data.Address.City != "New York" {
		t.Errorf("Address.City = %q, want %q", data.Address.City, "New York")
	}
}

func TestEventDataAs_KYBStatusData_Updated(t *testing.T) {
	payload := `{
		"customer_id":"1NFHrqBHb3cTfLVkFSGmHZqdDPw",
		"kyb_status":"frozen",
		"reason_code":"pending_proof_of_address"
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventCustomerKYBStatusUpdated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.KYBStatusData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.CustomerID != "1NFHrqBHb3cTfLVkFSGmHZqdDPw" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "1NFHrqBHb3cTfLVkFSGmHZqdDPw")
	}
	if data.Status != "frozen" {
		t.Errorf("Status = %q, want %q", data.Status, "frozen")
	}
	if data.ReasonCode == nil {
		t.Fatal("expected non-nil ReasonCode")
	}
	if *data.ReasonCode != "pending_proof_of_address" {
		t.Errorf("ReasonCode = %q, want %q", *data.ReasonCode, "pending_proof_of_address")
	}
}

// customer.kyb_status.created never carries reason_code, so ReasonCode must
// stay nil rather than decode to an empty string.
func TestEventDataAs_KYBStatusData_Created(t *testing.T) {
	payload := `{
		"customer_id":"1NFHrqBHb3cTfLVkFSGmHZqdDPw",
		"kyb_status":"in_review"
	}`

	event := webhook.Event{
		ID:   "evt_2",
		Type: webhook.EventCustomerKYBStatusCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.KYBStatusData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.CustomerID != "1NFHrqBHb3cTfLVkFSGmHZqdDPw" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "1NFHrqBHb3cTfLVkFSGmHZqdDPw")
	}
	if data.Status != "in_review" {
		t.Errorf("Status = %q, want %q", data.Status, "in_review")
	}
	if data.ReasonCode != nil {
		t.Errorf("ReasonCode = %v, want nil", *data.ReasonCode)
	}
}

func TestEventDataAs_KYBLinkData_Created(t *testing.T) {
	payload := `{
		"customer_id":"zHSlUmFvtibdInTWpolNHbV3lx4",
		"link_type":"hosted",
		"url":"https://example.test/kyb",
		"status":"active",
		"expires_at":1767225600
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventCustomerKYBLinkCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.KYBLinkData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.CustomerID != "zHSlUmFvtibdInTWpolNHbV3lx4" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "zHSlUmFvtibdInTWpolNHbV3lx4")
	}
	if data.LinkType != "hosted" {
		t.Errorf("LinkType = %q, want %q", data.LinkType, "hosted")
	}
	if data.URL != "https://example.test/kyb" {
		t.Errorf("URL = %q, want %q", data.URL, "https://example.test/kyb")
	}
	if data.Status != "active" {
		t.Errorf("Status = %q, want %q", data.Status, "active")
	}
	if data.ExpiresAt == nil {
		t.Fatal("expected non-nil ExpiresAt")
	}
	if *data.ExpiresAt != 1767225600 {
		t.Errorf("ExpiresAt = %d, want %d", *data.ExpiresAt, 1767225600)
	}
}

// TOS links and links from a Persona inquiry with no expired_at omit
// expires_at entirely, so it must decode to nil rather than a silent zero.
func TestEventDataAs_KYBLinkData_Updated(t *testing.T) {
	payload := `{
		"customer_id":"WGZBrOpaYvtu1pZx5QqPmvl4yk4",
		"link_type":"tos",
		"url":"https://example.test/tos",
		"status":"expired"
	}`

	event := webhook.Event{
		ID:   "evt_2",
		Type: webhook.EventCustomerKYBLinkUpdated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.KYBLinkData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.CustomerID != "WGZBrOpaYvtu1pZx5QqPmvl4yk4" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "WGZBrOpaYvtu1pZx5QqPmvl4yk4")
	}
	if data.LinkType != "tos" {
		t.Errorf("LinkType = %q, want %q", data.LinkType, "tos")
	}
	if data.Status != "expired" {
		t.Errorf("Status = %q, want %q", data.Status, "expired")
	}
	if data.ExpiresAt != nil {
		t.Errorf("ExpiresAt = %v, want nil", *data.ExpiresAt)
	}
}

func TestEventDataAs_KYBApplicationSubmittedData(t *testing.T) {
	payload := `{
		"customer_id":"b1h8iYu3xdBGEet3zfHKz2NLGAo",
		"application_id":"3g1JFoSSwionnaiWMNEBjP3qqTV",
		"application_type":"business"
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventCustomerKYBApplicationSubmitted,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.KYBApplicationSubmittedData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.CustomerID != "b1h8iYu3xdBGEet3zfHKz2NLGAo" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "b1h8iYu3xdBGEet3zfHKz2NLGAo")
	}
	if data.ApplicationID != "3g1JFoSSwionnaiWMNEBjP3qqTV" {
		t.Errorf("ApplicationID = %q, want %q", data.ApplicationID, "3g1JFoSSwionnaiWMNEBjP3qqTV")
	}
	if data.ApplicationType != "business" {
		t.Errorf("ApplicationType = %q, want %q", data.ApplicationType, "business")
	}
}

func TestEventDataAs_AutoAccountData(t *testing.T) {
	payload := `{
		"id":"aa_1",
		"customer_id":"cust_1",
		"enabled":true,
		"account_type":"bank",
		"bank_account":{
			"account_holder_name":"Acme Corp",
			"us_details":{
				"account_type":"checking",
				"account_number":"1234567890",
				"routing_number":"021000021"
			}
		}
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventAutoAccountCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.AutoAccountData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ID != "aa_1" {
		t.Errorf("ID = %q, want %q", data.ID, "aa_1")
	}
	if !data.Enabled {
		t.Error("expected Enabled to be true")
	}
	if data.BankAccount == nil {
		t.Fatal("expected non-nil BankAccount")
	}
	if data.BankAccount.USDetails == nil {
		t.Fatal("expected non-nil USDetails")
	}
	if data.BankAccount.USDetails.RoutingNumber != "021000021" {
		t.Errorf(
			"RoutingNumber = %q, want %q",
			data.BankAccount.USDetails.RoutingNumber,
			"021000021",
		)
	}
}

func TestEventDataAs_AutoTransactionData(t *testing.T) {
	payload := `{
		"id":"txn_1",
		"auto_account_id":"aa_1",
		"destination_id":"dest_1",
		"type":"inbound",
		"status":"completed",
		"created_at":1700000000,
		"updated_at":1700001000,
		"receipt":{
			"input_currency":"USD",
			"output_currency":"USDC",
			"initial_amount":"100.00",
			"subtotal_amount":"99.50",
			"converted_amount":"99.50",
			"outgoing_amount":"99.00",
			"external_fee":"0.25",
			"client_fee":"0.25",
			"dakota_fee":"0.00",
			"exchange_rate":"1.0"
		}
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventTransactionAutoUpdated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.AutoTransactionData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ID != "txn_1" {
		t.Errorf("ID = %q, want %q", data.ID, "txn_1")
	}
	if data.Status != "completed" {
		t.Errorf("Status = %q, want %q", data.Status, "completed")
	}
	if data.Receipt == nil {
		t.Fatal("expected non-nil Receipt")
	}
	if data.Receipt.InputCurrency != "USD" {
		t.Errorf(
			"Receipt.InputCurrency = %q, want %q",
			data.Receipt.InputCurrency,
			"USD",
		)
	}
}

func TestEventDataAs_OneOffTransactionData(t *testing.T) {
	payload := `{
		"id":"txn_2",
		"customer_id":"cust_1",
		"destination_id":"dest_1",
		"source_asset":"USDC",
		"source_network_id":"ethereum",
		"destination_amount":"500.00",
		"destination_currency":"USD",
		"status":"pending",
		"created_at":1700000000,
		"updated_at":1700000000,
		"send_amount":{"amount":"501.50","currency":"USDC"}
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventTransactionOneOffCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.OneOffTransactionData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ID != "txn_2" {
		t.Errorf("ID = %q, want %q", data.ID, "txn_2")
	}
	if data.SendAmount == nil {
		t.Fatal("expected non-nil SendAmount")
	}
	if data.SendAmount.Amount != "501.50" {
		t.Errorf(
			"SendAmount.Amount = %q, want %q",
			data.SendAmount.Amount,
			"501.50",
		)
	}
}

func TestEventDataAs_RecipientData(t *testing.T) {
	payload := `{
		"id":"rcpt_1",
		"customer_id":"cust_1",
		"name":"Bob Smith",
		"type":"individual"
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventRecipientCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.RecipientData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ID != "rcpt_1" {
		t.Errorf("ID = %q, want %q", data.ID, "rcpt_1")
	}
	if data.Name != "Bob Smith" {
		t.Errorf("Name = %q, want %q", data.Name, "Bob Smith")
	}
}

func TestEventDataAs_DestinationData(t *testing.T) {
	payload := `{
		"id":"dest_1",
		"customer_id":"cust_1",
		"recipient_id":"rcpt_1",
		"currency":"USD"
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventDestinationCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.DestinationData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ID != "dest_1" {
		t.Errorf("ID = %q, want %q", data.ID, "dest_1")
	}
	if data.Currency != "USD" {
		t.Errorf("Currency = %q, want %q", data.Currency, "USD")
	}
}

func TestEventDataAs_TargetCreatedData(t *testing.T) {
	payload := `{
		"id":"tgt_1",
		"auto_account_id":"aa_1",
		"amount":"1000.00",
		"currency":"USD",
		"frequency":"monthly"
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventTargetCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.TargetCreatedData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ID != "tgt_1" {
		t.Errorf("ID = %q, want %q", data.ID, "tgt_1")
	}
	if data.Frequency != "monthly" {
		t.Errorf("Frequency = %q, want %q", data.Frequency, "monthly")
	}
}

func TestEventDataAs_ExceptionData(t *testing.T) {
	payload := `{
		"id":"exc_1",
		"auto_account_id":"aa_1",
		"type":"balance_too_low",
		"message":"Insufficient balance",
		"created_at":1700000000
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventExceptionCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.ExceptionData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ID != "exc_1" {
		t.Errorf("ID = %q, want %q", data.ID, "exc_1")
	}
	if data.Type != "balance_too_low" {
		t.Errorf("Type = %q, want %q", data.Type, "balance_too_low")
	}
}

func TestEventDataAs_ExceptionClearedData(t *testing.T) {
	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventExceptionCleared,
		Data: webhook.EventData{Object: json.RawMessage(`{"id":"exc_1","auto_account_id":"aa_1","cleared_at":1700002000}`)},
	}

	data, err := webhook.EventDataAs[types.ExceptionClearedData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ClearedAt != 1700002000 {
		t.Errorf("ClearedAt = %d, want %d", data.ClearedAt, 1700002000)
	}
}

// bvnk.onboarding.* has no platform emitter today; this pins the one shape
// the published public spec documents for it.
func TestEventDataAs_BVNKOnboardingData(t *testing.T) {
	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventBVNKOnboardingCreated,
		Data: webhook.EventData{Object: json.RawMessage(`{"customer_id":"mh4981Rh0eiHymFzltxUJjS7aNP"}`)},
	}

	data, err := webhook.EventDataAs[types.BVNKOnboardingData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.CustomerID != "mh4981Rh0eiHymFzltxUJjS7aNP" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "mh4981Rh0eiHymFzltxUJjS7aNP")
	}
}

func TestEventDataAs_WalletEventData(t *testing.T) {
	payload := `{
		"wallet":{
			"id":"w_1",
			"client_id":"cli_1",
			"family":"evm",
			"address":"0xabc123",
			"name":"Treasury",
			"created_at":1700000000,
			"updated_at":1700001000
		},
		"signer_groups":[{
			"id":"sg_1",
			"client_id":"cli_1",
			"name":"Admins",
			"members":[{
				"id":"m_1",
				"name":"Alice",
				"public_key":"0xpub1",
				"key_type":"ecdsa"
			}]
		}],
		"policies":[{
			"id":"pol_1",
			"client_id":"cli_1",
			"signer_group_id":"sg_1",
			"name":"Default Policy",
			"rules":[{
				"id":"rule_1",
				"policy_id":"pol_1",
				"rule_type":"approval_threshold",
				"action":"approve",
				"created_at":1700000000,
				"definition":{
					"approval_threshold":{"threshold":2}
				}
			}]
		}]
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventWalletCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.WalletEventData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.Wallet.ID != "w_1" {
		t.Errorf("Wallet.ID = %q, want %q", data.Wallet.ID, "w_1")
	}
	if data.Wallet.Family != "evm" {
		t.Errorf("Wallet.Family = %q, want %q", data.Wallet.Family, "evm")
	}
	if len(data.SignerGroups) != 1 {
		t.Fatalf("len(SignerGroups) = %d, want 1", len(data.SignerGroups))
	}
	if data.SignerGroups[0].Name != "Admins" {
		t.Errorf(
			"SignerGroups[0].Name = %q, want %q",
			data.SignerGroups[0].Name,
			"Admins",
		)
	}
	if len(data.SignerGroups[0].Members) != 1 {
		t.Fatalf("len(Members) = %d, want 1", len(data.SignerGroups[0].Members))
	}
	if data.SignerGroups[0].Members[0].PublicKey != "0xpub1" {
		t.Errorf(
			"Members[0].PublicKey = %q, want %q",
			data.SignerGroups[0].Members[0].PublicKey,
			"0xpub1",
		)
	}
	if len(data.Policies) != 1 {
		t.Fatalf("len(Policies) = %d, want 1", len(data.Policies))
	}
	if len(data.Policies[0].Rules) != 1 {
		t.Fatalf("len(Rules) = %d, want 1", len(data.Policies[0].Rules))
	}
	rule := data.Policies[0].Rules[0]
	if rule.Definition == nil || rule.Definition.ApprovalThreshold == nil {
		t.Fatal("expected non-nil ApprovalThreshold definition")
	}
	if rule.Definition.ApprovalThreshold.Threshold != 2 {
		t.Errorf(
			"Threshold = %d, want 2",
			rule.Definition.ApprovalThreshold.Threshold,
		)
	}
}

func TestEventDataAs_WalletDepositData(t *testing.T) {
	payload := `{
		"wallet_id":"w_1",
		"transaction_hash":"0xtxhash",
		"sender":"0xsender",
		"recipient":"0xrecipient",
		"amount":"1.5",
		"asset":{
			"id":"asset_1",
			"name":"USDC",
			"network_id":"ethereum",
			"contract_address":"0xcontract",
			"decimals":6,
			"token_standard":"ERC20"
		}
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventWalletDeposit,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.WalletDepositData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.WalletID != "w_1" {
		t.Errorf("WalletID = %q, want %q", data.WalletID, "w_1")
	}
	if data.Amount != "1.5" {
		t.Errorf("Amount = %q, want %q", data.Amount, "1.5")
	}
	if data.Asset == nil {
		t.Fatal("expected non-nil Asset")
	}
	if data.Asset.Decimals != 6 {
		t.Errorf("Asset.Decimals = %d, want 6", data.Asset.Decimals)
	}
	if data.Asset.TokenStandard != "ERC20" {
		t.Errorf(
			"Asset.TokenStandard = %q, want %q",
			data.Asset.TokenStandard,
			"ERC20",
		)
	}
}

func TestEventDataAs_WalletTransactionData(t *testing.T) {
	payload := `{
		"id":"wtxn_1",
		"wallet_id":"w_1",
		"status":"pending",
		"intent":{
			"wallet_id":"w_1",
			"caip2":"eip155:1",
			"sponsor":false,
			"idempotency_key":"idem_1",
			"operation":{
				"kind":"transfer",
				"from":"0xfrom",
				"to":"0xto",
				"amount":"100"
			}
		}
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventWalletTransactionCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.WalletTransactionData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ID != "wtxn_1" {
		t.Errorf("ID = %q, want %q", data.ID, "wtxn_1")
	}
	if data.Intent.Caip2 != "eip155:1" {
		t.Errorf("Intent.Caip2 = %q, want %q", data.Intent.Caip2, "eip155:1")
	}
	if data.Intent.Operation.Kind != "transfer" {
		t.Errorf(
			"Operation.Kind = %q, want %q",
			data.Intent.Operation.Kind,
			"transfer",
		)
	}
}

func TestEventDataAs_CryptoDetails(t *testing.T) {
	txHash := "0xhash123"
	logIdx := 5
	payload := `{
		"id":"txn_1",
		"auto_account_id":"aa_1",
		"destination_id":"dest_1",
		"type":"inbound",
		"status":"completed",
		"created_at":1700000000,
		"updated_at":1700001000,
		"crypto_details":{
			"source_network_id":"ethereum",
			"source_address":"0xsrc",
			"tx_hash":"0xhash123",
			"log_index":5
		}
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventTransactionAutoUpdated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.AutoTransactionData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.CryptoDetails == nil {
		t.Fatal("expected non-nil CryptoDetails")
	}
	if data.CryptoDetails.TxHash == nil || *data.CryptoDetails.TxHash != txHash {
		t.Errorf("TxHash = %v, want %q", data.CryptoDetails.TxHash, txHash)
	}
	if data.CryptoDetails.LogIndex == nil || *data.CryptoDetails.LogIndex != logIdx {
		t.Errorf("LogIndex = %v, want %d", data.CryptoDetails.LogIndex, logIdx)
	}
}

func TestEventDataAs_SenderDetails(t *testing.T) {
	senderName := "John Doe"
	payload := `{
		"id":"txn_1",
		"auto_account_id":"aa_1",
		"destination_id":"dest_1",
		"type":"inbound",
		"status":"completed",
		"created_at":1700000000,
		"updated_at":1700001000,
		"sender_details":{
			"sender_type":"individual",
			"sender_account_holder_name":"John Doe",
			"sender_bank_name":"Chase"
		}
	}`

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventTransactionAutoCreated,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.AutoTransactionData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.SenderDetails == nil {
		t.Fatal("expected non-nil SenderDetails")
	}
	if data.SenderDetails.SenderAccountHolderName == nil || *data.SenderDetails.SenderAccountHolderName != senderName {
		t.Errorf(
			"SenderAccountHolderName = %v, want %q",
			data.SenderDetails.SenderAccountHolderName,
			senderName,
		)
	}
}

func TestEventDataAs_OptionalFieldsOmitted(t *testing.T) {
	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventAutoAccountCreated,
		Data: webhook.EventData{Object: json.RawMessage(`{"id":"aa_1","customer_id":"cust_1","enabled":false,"account_type":"crypto"}`)},
	}

	data, err := webhook.EventDataAs[types.AutoAccountData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.BankAccount != nil {
		t.Error("expected nil BankAccount when not provided")
	}
	if data.Crypto != nil {
		t.Error("expected nil Crypto when not provided")
	}
	if data.OutputAsset != nil {
		t.Error("expected nil OutputAsset when not provided")
	}
}

func TestEventDataAs_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "user data",
			payload: `{"user_id":"usr_1","email":"a@b.com"}`,
		},
		{
			name:    "api key deleted",
			payload: `{"id":"key_1"}`,
		},
		{
			name:    "bvnk onboarding",
			payload: `{"id":"bvnk_1","customer_id":"cust_1","status":"approved"}`,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				var raw json.RawMessage
				if err := json.Unmarshal([]byte(tt.payload), &raw); err != nil {
					t.Fatalf("Unmarshal error: %v", err)
				}

				marshaled, err := json.Marshal(raw)
				if err != nil {
					t.Fatalf("Marshal error: %v", err)
				}

				if string(marshaled) != tt.payload {
					t.Errorf(
						"round-trip mismatch:\n  got:  %s\n  want: %s",
						marshaled,
						tt.payload,
					)
				}
			},
		)
	}
}

func TestEventDataAs_ScheduledPaymentFailedData(t *testing.T) {
	// Full agent-bound payload.
	payload := `{"scheduled_payment_id":"sp_1","signer_id":"sgn_1","wallet_id":"wal_1",` +
		`"address":"0xabc","amount":"100","asset":"USDC","network_id":"base-sepolia",` +
		`"scheduled_at":1705315500,"failure_code":"mandate_denied","failure_reason":"no active mandate",` +
		`"payment_agent_id":"agt_1","recipient_id":"rcp_1","destination_id":"dst_1"}`
	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventScheduledPaymentFailed,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[types.ScheduledPaymentFailedData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.ScheduledPaymentID != "sp_1" || data.FailureCode != "mandate_denied" ||
		data.PaymentAgentID != "agt_1" || data.ScheduledAt != 1705315500 {
		t.Fatalf("unexpected decode: %+v", data)
	}

	// Bare-address schedule: the optional linkage keys are omitted, and address
	// is the only identifier.
	bare := `{"scheduled_payment_id":"sp_2","signer_id":"sgn_1","wallet_id":"wal_1",` +
		`"address":"0xdef","amount":"5","asset":"USDC","network_id":"base-sepolia",` +
		`"scheduled_at":1705315600,"failure_code":"send_error","failure_reason":"rpc timeout"}`
	bareData, err := webhook.EventDataAs[types.ScheduledPaymentFailedData](webhook.Event{
		ID:   "evt_2",
		Type: webhook.EventScheduledPaymentFailed,
		Data: webhook.EventData{Object: json.RawMessage(bare)},
	})
	if err != nil {
		t.Fatalf("EventDataAs (bare) error: %v", err)
	}
	if bareData.PaymentAgentID != "" || bareData.RecipientID != "" || bareData.DestinationID != "" {
		t.Errorf("expected empty linkage keys, got %+v", bareData)
	}
	if bareData.Address != "0xdef" {
		t.Errorf("Address = %q, want 0xdef", bareData.Address)
	}
}
