package types_test

import (
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
				data := decodeEvent[types.UserData](
					t, "evt_user_created", webhook.EventUserCreated, tt.payload,
				)
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
	data := decodeEvent[types.UserDeletedData](
		t, "evt_user_deleted_data", webhook.EventUserDeleted, `{"user_id":"usr_1"}`,
	)
	if data.UserID != "usr_1" {
		t.Errorf("UserID = %q, want %q", data.UserID, "usr_1")
	}
}

// api_key.created carries the key id and its last 6 characters. The emitter
// also sends `hash`; the SDK deliberately does not surface it.
func TestEventDataAs_APIKeyData(t *testing.T) {
	data := decodeEventIgnoring[types.APIKeyData](
		t, "evt_api_key_data", webhook.EventAPIKeyCreated,
		`{"id":"key_1","last_6":"a1b2c3","hash":"e3b0c44298fc1c14"}`,
		"hash",
	)
	if data.ID != "key_1" {
		t.Errorf("ID = %q, want %q", data.ID, "key_1")
	}
	if data.Last6 != "a1b2c3" {
		t.Errorf("Last6 = %q, want %q", data.Last6, "a1b2c3")
	}
}

// customer.created carries the customer id under `customer_id`, plus name and
// type; external_id and import_reference appear only when set.
func TestEventDataAs_CustomerData(t *testing.T) {
	payload := `{
		"customer_id":"cust_1",
		"name":"Acme Corp",
		"type":"business",
		"external_id":"acme-erp-4471",
		"import_reference":{"source":"persona","reference":"cnst_9xKq2"}
	}`

	data := decodeEvent[types.CustomerData](
		t, "evt_customer_data", webhook.EventCustomerCreated, payload,
	)
	if data.CustomerID != "cust_1" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "cust_1")
	}
	if data.Name != "Acme Corp" {
		t.Errorf("Name = %q, want %q", data.Name, "Acme Corp")
	}
	if data.Type != "business" {
		t.Errorf("Type = %q, want %q", data.Type, "business")
	}
	if data.ExternalID == nil || *data.ExternalID != "acme-erp-4471" {
		t.Errorf("ExternalID = %v, want %q", data.ExternalID, "acme-erp-4471")
	}
	if data.ImportReference == nil {
		t.Fatal("expected non-nil ImportReference")
	}
	if data.ImportReference.Source != "persona" {
		t.Errorf(
			"ImportReference.Source = %q, want %q",
			data.ImportReference.Source,
			"persona",
		)
	}
	if data.ImportReference.Reference != "cnst_9xKq2" {
		t.Errorf(
			"ImportReference.Reference = %q, want %q",
			data.ImportReference.Reference,
			"cnst_9xKq2",
		)
	}
}

// customer.updated never carries import_reference, and omits external_id when
// the customer has none — both must decode to nil, not to a zero value.
func TestEventDataAs_CustomerData_Updated(t *testing.T) {
	payload := `{
		"customer_id":"cust_2",
		"name":"Acme Corp",
		"type":"business"
	}`

	data := decodeEvent[types.CustomerData](
		t, "evt_customer_data_updated", webhook.EventCustomerUpdated, payload,
	)
	if data.CustomerID != "cust_2" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "cust_2")
	}
	if data.ExternalID != nil {
		t.Errorf("ExternalID = %v, want nil", *data.ExternalID)
	}
	if data.ImportReference != nil {
		t.Errorf("ImportReference = %+v, want nil", *data.ImportReference)
	}
}

func TestEventDataAs_KYBStatusData_Updated(t *testing.T) {
	payload := `{
		"customer_id":"1NFHrqBHb3cTfLVkFSGmHZqdDPw",
		"kyb_status":"frozen",
		"reason_code":"pending_proof_of_address"
	}`

	data := decodeEvent[types.KYBStatusData](
		t, "evt_kyb_status_data_updated", webhook.EventCustomerKYBStatusUpdated, payload,
	)
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

	data := decodeEvent[types.KYBStatusData](
		t, "evt_kyb_status_data_created", webhook.EventCustomerKYBStatusCreated, payload,
	)
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
		"link_type":"persona",
		"url":"https://example.test/kyb",
		"status":"active",
		"expires_at":1767225600
	}`

	data := decodeEvent[types.KYBLinkData](
		t, "evt_kyb_link_data_created", webhook.EventCustomerKYBLinkCreated, payload,
	)
	if data.CustomerID != "zHSlUmFvtibdInTWpolNHbV3lx4" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "zHSlUmFvtibdInTWpolNHbV3lx4")
	}
	if data.LinkType != "persona" {
		t.Errorf("LinkType = %q, want %q", data.LinkType, "persona")
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

	data := decodeEvent[types.KYBLinkData](
		t, "evt_kyb_link_data_updated", webhook.EventCustomerKYBLinkUpdated, payload,
	)
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

	data := decodeEvent[types.KYBApplicationSubmittedData](
		t, "evt_kyb_application_submitted_data", webhook.EventCustomerKYBApplicationSubmitted, payload,
	)
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
		"developer_fee_bps":25,
		"bank_account":{
			"account_holder_name":"Acme Corp",
			"us_details":{
				"account_type":"checking",
				"account_number":"1234567890",
				"routing_number":"021000021"
			}
		}
	}`

	data := decodeEventIgnoring[types.AutoAccountData](
		t, "evt_auto_account_data", webhook.EventAutoAccountCreated, payload,
		"developer_fee_bps",
	)
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

	data := decodeEvent[types.AutoTransactionData](
		t, "evt_auto_transaction_data", webhook.EventTransactionAutoUpdated, payload,
	)
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
		"send_amount":{"amount":"501.50","currency":"USDC"},
		"sender_details":{
			"sender_wallet_address":"0xsender",
			"sender_network":"ethereum"
		}
	}`

	data := decodeEvent[types.OneOffTransactionData](
		t, "evt_one_off_transaction_data", webhook.EventTransactionOneOffCreated, payload,
	)
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
	// One-off transactions carry only the crypto subset of sender_details;
	// the bank fields are never stored for them.
	if data.SenderDetails == nil {
		t.Fatal("expected non-nil SenderDetails")
	}
	if data.SenderDetails.SenderWalletAddress == nil ||
		*data.SenderDetails.SenderWalletAddress != "0xsender" {
		t.Errorf(
			"SenderWalletAddress = %v, want %q",
			data.SenderDetails.SenderWalletAddress,
			"0xsender",
		)
	}
	if data.SenderDetails.SenderNetwork == nil ||
		*data.SenderDetails.SenderNetwork != "ethereum" {
		t.Errorf(
			"SenderNetwork = %v, want %q",
			data.SenderDetails.SenderNetwork,
			"ethereum",
		)
	}
}

func TestEventDataAs_RecipientData(t *testing.T) {
	payload := `{
		"id":"rcpt_1",
		"customer_id":"cust_1",
		"name":"Bob Smith",
		"status":"active",
		"address":{"street1":"123 Main St","city":"New York","country":"US"}
	}`

	data := decodeEvent[types.RecipientData](
		t, "evt_recipient_data", webhook.EventRecipientCreated, payload,
	)
	if data.ID != "rcpt_1" {
		t.Errorf("ID = %q, want %q", data.ID, "rcpt_1")
	}
	if data.CustomerID != "cust_1" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "cust_1")
	}
	if data.Name != "Bob Smith" {
		t.Errorf("Name = %q, want %q", data.Name, "Bob Smith")
	}
	if data.Status != "active" {
		t.Errorf("Status = %q, want %q", data.Status, "active")
	}
	if data.Address == nil {
		t.Fatal("expected non-nil Address")
	}
	if data.Address.City != "New York" {
		t.Errorf("Address.City = %q, want %q", data.Address.City, "New York")
	}
}

// recipient.updated carries no customer_id at all — the emitter simply does not
// send it. Modeling one here would hand every consumer a silent "".
func TestEventDataAs_RecipientUpdatedData(t *testing.T) {
	payload := `{
		"id":"rcpt_1",
		"name":"Bob Smith",
		"status":"active"
	}`

	data := decodeEvent[types.RecipientUpdatedData](
		t, "evt_recipient_updated_data", webhook.EventRecipientUpdated, payload,
	)
	if data.ID != "rcpt_1" {
		t.Errorf("ID = %q, want %q", data.ID, "rcpt_1")
	}
	if data.Status != "active" {
		t.Errorf("Status = %q, want %q", data.Status, "active")
	}
	if data.Address != nil {
		t.Errorf("Address = %+v, want nil", *data.Address)
	}
}

func TestEventDataAs_RecipientDeletedData(t *testing.T) {
	data := decodeEvent[types.RecipientDeletedData](
		t, "evt_recipient_deleted_data", webhook.EventRecipientDeleted,
		`{"id":"rcpt_1","customer_id":"cust_1"}`,
	)
	if data.ID != "rcpt_1" {
		t.Errorf("ID = %q, want %q", data.ID, "rcpt_1")
	}
	if data.CustomerID != "cust_1" {
		t.Errorf("CustomerID = %q, want %q", data.CustomerID, "cust_1")
	}
}

func TestEventDataAs_DestinationData(t *testing.T) {
	payload := `{
		"id":"dest_1",
		"recipient_id":"rcpt_1",
		"name":"Bob Smith Checking",
		"type":"bank_account",
		"bank_account":{
			"account_holder_name":"Bob Smith",
			"us_details":{
				"account_type":"checking",
				"account_number":"1234567890",
				"routing_number":"021000021"
			}
		}
	}`

	data := decodeEvent[types.DestinationData](
		t, "evt_destination_data", webhook.EventDestinationCreated, payload,
	)
	if data.ID != "dest_1" {
		t.Errorf("ID = %q, want %q", data.ID, "dest_1")
	}
	if data.RecipientID != "rcpt_1" {
		t.Errorf("RecipientID = %q, want %q", data.RecipientID, "rcpt_1")
	}
	if data.Name != "Bob Smith Checking" {
		t.Errorf("Name = %q, want %q", data.Name, "Bob Smith Checking")
	}
	if data.Type != "bank_account" {
		t.Errorf("Type = %q, want %q", data.Type, "bank_account")
	}
	if data.BankAccount == nil {
		t.Fatal("expected non-nil BankAccount")
	}
	if data.Crypto != nil {
		t.Errorf("Crypto = %+v, want nil", *data.Crypto)
	}
}

// A crypto destination carries `crypto` in place of `bank_account`.
func TestEventDataAs_DestinationData_Crypto(t *testing.T) {
	payload := `{
		"id":"dest_2",
		"recipient_id":"rcpt_1",
		"name":"Bob Smith USDC",
		"type":"crypto",
		"crypto":{"network_id":"ethereum","address":"0xdest"}
	}`

	data := decodeEvent[types.DestinationData](
		t, "evt_destination_data_crypto", webhook.EventDestinationCreated, payload,
	)
	if data.Crypto == nil {
		t.Fatal("expected non-nil Crypto")
	}
	if data.Crypto.NetworkID != "ethereum" {
		t.Errorf("Crypto.NetworkID = %q, want %q", data.Crypto.NetworkID, "ethereum")
	}
	if data.BankAccount != nil {
		t.Errorf("BankAccount = %+v, want nil", *data.BankAccount)
	}
}

func TestEventDataAs_DestinationDeletedData(t *testing.T) {
	data := decodeEvent[types.DestinationDeletedData](
		t, "evt_destination_deleted_data", webhook.EventDestinationDeleted,
		`{"id":"dest_1","recipient_id":"rcpt_1"}`,
	)
	if data.ID != "dest_1" {
		t.Errorf("ID = %q, want %q", data.ID, "dest_1")
	}
	if data.RecipientID != "rcpt_1" {
		t.Errorf("RecipientID = %q, want %q", data.RecipientID, "rcpt_1")
	}
}

// A "target" is a webhook endpoint registration, not a savings or payout
// target. target.created names the endpoint URL `url`; the updated and deleted
// events both name it `target_url`.
func TestEventDataAs_TargetCreatedData(t *testing.T) {
	payload := `{
		"target_id":"tgt_1",
		"url":"https://example.test/hooks/dakota",
		"global":false,
		"event_types":["customer.created","transaction.one_off.updated"]
	}`

	data := decodeEvent[types.TargetCreatedData](
		t, "evt_target_created_data", webhook.EventTargetCreated, payload,
	)
	if data.TargetID != "tgt_1" {
		t.Errorf("TargetID = %q, want %q", data.TargetID, "tgt_1")
	}
	if data.URL != "https://example.test/hooks/dakota" {
		t.Errorf("URL = %q, want %q", data.URL, "https://example.test/hooks/dakota")
	}
	if data.Global {
		t.Error("expected Global to be false")
	}
	if len(data.EventTypes) != 2 {
		t.Fatalf("len(EventTypes) = %d, want 2", len(data.EventTypes))
	}
	if data.EventTypes[0] != "customer.created" {
		t.Errorf("EventTypes[0] = %q, want %q", data.EventTypes[0], "customer.created")
	}
}

// A global target subscribes to everything, so the emitter omits event_types
// entirely rather than sending the full enum.
func TestEventDataAs_TargetUpdatedData(t *testing.T) {
	payload := `{
		"target_id":"tgt_1",
		"target_url":"https://example.test/hooks/dakota",
		"global":true
	}`

	data := decodeEvent[types.TargetUpdatedData](
		t, "evt_target_updated_data", webhook.EventTargetUpdated, payload,
	)
	if data.TargetID != "tgt_1" {
		t.Errorf("TargetID = %q, want %q", data.TargetID, "tgt_1")
	}
	if data.TargetURL != "https://example.test/hooks/dakota" {
		t.Errorf(
			"TargetURL = %q, want %q",
			data.TargetURL,
			"https://example.test/hooks/dakota",
		)
	}
	if !data.Global {
		t.Error("expected Global to be true")
	}
	if data.EventTypes != nil {
		t.Errorf("EventTypes = %v, want nil", data.EventTypes)
	}
}

func TestEventDataAs_TargetDeletedData(t *testing.T) {
	data := decodeEvent[types.TargetDeletedData](
		t, "evt_target_deleted_data", webhook.EventTargetDeleted,
		`{"target_id":"tgt_1","target_url":"https://example.test/hooks/dakota"}`,
	)
	if data.TargetID != "tgt_1" {
		t.Errorf("TargetID = %q, want %q", data.TargetID, "tgt_1")
	}
	if data.TargetURL != "https://example.test/hooks/dakota" {
		t.Errorf(
			"TargetURL = %q, want %q",
			data.TargetURL,
			"https://example.test/hooks/dakota",
		)
	}
}

// exception.created identifies the exception as `exception_id` and carries an
// open-ended `exception_content` blob. customer_id is present only for
// exceptions that belong to a customer.
func TestEventDataAs_ExceptionData(t *testing.T) {
	payload := `{
		"exception_id":"exc_1",
		"type":"balance_too_low",
		"customer_id":"cust_1",
		"exception_content":{"available":"12.00","required":"250.00"}
	}`

	data := decodeEvent[types.ExceptionData](
		t, "evt_exception_data", webhook.EventExceptionCreated, payload,
	)
	if data.ExceptionID != "exc_1" {
		t.Errorf("ExceptionID = %q, want %q", data.ExceptionID, "exc_1")
	}
	if data.Type != "balance_too_low" {
		t.Errorf("Type = %q, want %q", data.Type, "balance_too_low")
	}
	if data.CustomerID == nil || *data.CustomerID != "cust_1" {
		t.Errorf("CustomerID = %v, want %q", data.CustomerID, "cust_1")
	}
	if data.Content["required"] != "250.00" {
		t.Errorf("Content[required] = %v, want %q", data.Content["required"], "250.00")
	}
}

// A client-level exception carries neither customer_id nor exception_content.
func TestEventDataAs_ExceptionData_Minimal(t *testing.T) {
	data := decodeEvent[types.ExceptionData](
		t, "evt_exception_data_minimal", webhook.EventExceptionCreated,
		`{"exception_id":"exc_2","type":"balance_too_low"}`,
	)
	if data.CustomerID != nil {
		t.Errorf("CustomerID = %v, want nil", *data.CustomerID)
	}
	if data.Content != nil {
		t.Errorf("Content = %v, want nil", data.Content)
	}
}

// exception.cleared repeats the exception id and type, and carries no
// timestamp of its own — the envelope's `created` is the clearing time.
func TestEventDataAs_ExceptionClearedData(t *testing.T) {
	data := decodeEvent[types.ExceptionClearedData](
		t, "evt_exception_cleared_data", webhook.EventExceptionCleared,
		`{"exception_id":"exc_1","type":"balance_too_low","customer_id":"cust_1"}`,
	)
	if data.ExceptionID != "exc_1" {
		t.Errorf("ExceptionID = %q, want %q", data.ExceptionID, "exc_1")
	}
	if data.Type != "balance_too_low" {
		t.Errorf("Type = %q, want %q", data.Type, "balance_too_low")
	}
	if data.CustomerID == nil || *data.CustomerID != "cust_1" {
		t.Errorf("CustomerID = %v, want %q", data.CustomerID, "cust_1")
	}
}

// bvnk.onboarding.* has no platform emitter today; this pins the one shape
// the published public spec documents for it.
func TestEventDataAs_BVNKOnboardingData(t *testing.T) {
	data := decodeEvent[types.BVNKOnboardingData](
		t, "evt_bvnk_onboarding_data", webhook.EventBVNKOnboardingCreated, `{"customer_id":"mh4981Rh0eiHymFzltxUJjS7aNP"}`,
	)
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

	data := decodeEvent[types.WalletEventData](
		t, "evt_wallet_event_data", webhook.EventWalletCreated, payload,
	)
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

// A policy rule definition is a tagged union: the platform emits exactly one of
// approval_threshold, amount_threshold, or address_list per rule, selected by
// rule_type. Each arm needs its own fixture, because an arm no fixture reaches
// is an arm whose field types nothing checks.
func TestEventDataAs_WalletEventDataAmountThresholdRule(t *testing.T) {
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
		"signer_groups":null,
		"policies":[{
			"id":"pol_1",
			"client_id":"cli_1",
			"signer_group_id":"sg_1",
			"name":"High Value Policy",
			"rules":[{
				"id":"rule_2",
				"policy_id":"pol_1",
				"rule_type":"amount_threshold",
				"action":"deny",
				"created_at":1700000000,
				"definition":{
					"amount_threshold":{
						"min_amount":1000000,
						"threshold":2,
						"asset":{"id":"USDC","name":"USD Coin"}
					}
				}
			}]
		}]
	}`

	data := decodeEvent[types.WalletEventData](
		t, "evt_wallet_amount_threshold", webhook.EventWalletCreated, payload,
	)
	if len(data.Policies) != 1 {
		t.Fatalf("len(Policies) = %d, want 1", len(data.Policies))
	}
	if len(data.Policies[0].Rules) != 1 {
		t.Fatalf("len(Rules) = %d, want 1", len(data.Policies[0].Rules))
	}
	rule := data.Policies[0].Rules[0]
	if rule.RuleType != "amount_threshold" {
		t.Errorf("RuleType = %q, want %q", rule.RuleType, "amount_threshold")
	}
	if rule.Definition == nil || rule.Definition.AmountThreshold == nil {
		t.Fatal("expected non-nil AmountThreshold definition")
	}
	amount := rule.Definition.AmountThreshold
	// min_amount is a JSON number of minor units, not a decimal string: a
	// string field here fails the whole event, not just this one key.
	if amount.MinAmount != 1000000 {
		t.Errorf("MinAmount = %d, want 1000000", amount.MinAmount)
	}
	if amount.Threshold != 2 {
		t.Errorf("Threshold = %d, want 2", amount.Threshold)
	}
	if amount.Asset.ID != "USDC" {
		t.Errorf("Asset.ID = %q, want %q", amount.Asset.ID, "USDC")
	}
	if amount.Asset.Name != "USD Coin" {
		t.Errorf("Asset.Name = %q, want %q", amount.Asset.Name, "USD Coin")
	}
	if rule.Definition.ApprovalThreshold != nil {
		t.Error("expected nil ApprovalThreshold on an amount_threshold rule")
	}
	if rule.Definition.AddressList != nil {
		t.Error("expected nil AddressList on an amount_threshold rule")
	}
}

func TestEventDataAs_WalletEventDataAddressListRule(t *testing.T) {
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
		"signer_groups":null,
		"policies":[{
			"id":"pol_1",
			"client_id":"cli_1",
			"signer_group_id":"sg_1",
			"name":"Allowlist Policy",
			"rules":[{
				"id":"rule_3",
				"policy_id":"pol_1",
				"rule_type":"address_list",
				"action":"approve",
				"created_at":1700000000,
				"definition":{
					"address_list":{
						"addresses":["0xaaa","0xbbb"]
					}
				}
			}]
		}]
	}`

	data := decodeEvent[types.WalletEventData](
		t, "evt_wallet_address_list", webhook.EventWalletCreated, payload,
	)
	if len(data.Policies) != 1 {
		t.Fatalf("len(Policies) = %d, want 1", len(data.Policies))
	}
	if len(data.Policies[0].Rules) != 1 {
		t.Fatalf("len(Rules) = %d, want 1", len(data.Policies[0].Rules))
	}
	rule := data.Policies[0].Rules[0]
	if rule.RuleType != "address_list" {
		t.Errorf("RuleType = %q, want %q", rule.RuleType, "address_list")
	}
	if rule.Definition == nil || rule.Definition.AddressList == nil {
		t.Fatal("expected non-nil AddressList definition")
	}
	got := rule.Definition.AddressList.Addresses
	if len(got) != 2 || got[0] != "0xaaa" || got[1] != "0xbbb" {
		t.Errorf("Addresses = %v, want [0xaaa 0xbbb]", got)
	}
	if rule.Definition.ApprovalThreshold != nil {
		t.Error("expected nil ApprovalThreshold on an address_list rule")
	}
	if rule.Definition.AmountThreshold != nil {
		t.Error("expected nil AmountThreshold on an address_list rule")
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

	data := decodeEvent[types.WalletDepositData](
		t, "evt_wallet_deposit_data", webhook.EventWalletDeposit, payload,
	)
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
			"idempotency_key":"idem_1",
			"operation":{
				"kind":"transfer",
				"from":"0xfrom",
				"to":"0xto",
				"amount":"100",
				"asset_id":"usdc"
			}
		}
	}`

	data := decodeEvent[types.WalletTransactionData](
		t, "evt_wallet_transaction_data", webhook.EventWalletTransactionCreated, payload,
	)
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
	if data.Intent.Operation.AssetID != "usdc" {
		t.Errorf(
			"Operation.AssetID = %q, want %q",
			data.Intent.Operation.AssetID,
			"usdc",
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

	data := decodeEvent[types.AutoTransactionData](
		t, "evt_crypto_details", webhook.EventTransactionAutoUpdated, payload,
	)
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

	data := decodeEvent[types.AutoTransactionData](
		t, "evt_sender_details", webhook.EventTransactionAutoCreated, payload,
	)
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
	data := decodeEvent[types.AutoAccountData](
		t, "evt_optional_fields_omitted", webhook.EventAutoAccountCreated, `{"id":"aa_1","customer_id":"cust_1","enabled":false,"account_type":"crypto"}`,
	)
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

func TestEventDataAs_APIKeyDeletedData(t *testing.T) {
	data := decodeEvent[types.APIKeyDeletedData](
		t, "evt_api_key_deleted_data", webhook.EventAPIKeyDeleted,
		`{"id":"key_1"}`,
	)
	if data.ID != "key_1" {
		t.Errorf("ID = %q, want %q", data.ID, "key_1")
	}
}

func TestEventDataAs_ScheduledPaymentFailedData(t *testing.T) {
	// Full agent-bound payload.
	payload := `{"scheduled_payment_id":"sp_1","signer_id":"sgn_1","wallet_id":"wal_1",` +
		`"address":"0xabc","amount":"100","asset":"USDC","network_id":"base-sepolia",` +
		`"scheduled_at":1705315500,"failure_code":"mandate_denied","failure_reason":"no active mandate",` +
		`"payment_agent_id":"agt_1","recipient_id":"rcp_1","destination_id":"dst_1"}`
	data := decodeEvent[types.ScheduledPaymentFailedData](
		t, "evt_scheduled_payment_failed_data", webhook.EventScheduledPaymentFailed, payload,
	)
	if data.ScheduledPaymentID != "sp_1" || data.FailureCode != "mandate_denied" ||
		data.PaymentAgentID != "agt_1" || data.ScheduledAt != 1705315500 {
		t.Fatalf("unexpected decode: %+v", data)
	}

	// Bare-address schedule: the optional linkage keys are omitted, and address
	// is the only identifier.
	bare := `{"scheduled_payment_id":"sp_2","signer_id":"sgn_1","wallet_id":"wal_1",` +
		`"address":"0xdef","amount":"5","asset":"USDC","network_id":"base-sepolia",` +
		`"scheduled_at":1705315600,"failure_code":"send_error","failure_reason":"rpc timeout"}`
	bareData := decodeEvent[types.ScheduledPaymentFailedData](
		t, "evt_scheduled_payment_failed_bare",
		webhook.EventScheduledPaymentFailed, bare,
	)
	if bareData.PaymentAgentID != "" || bareData.RecipientID != "" || bareData.DestinationID != "" {
		t.Errorf("expected empty linkage keys, got %+v", bareData)
	}
	if bareData.Address != "0xdef" {
		t.Errorf("Address = %q, want 0xdef", bareData.Address)
	}
}
