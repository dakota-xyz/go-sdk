package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dakota-xyz/go-sdk/client"
	"github.com/dakota-xyz/go-sdk/client/gen"
)

// A receipt read through the client exposes the developer fee under both
// names: developer_fee, and the deprecated client_fee that existing
// integrations read (ENG-3905 B5/F3). The platform sends both, always equal.
func TestGetTransaction_ReceiptExposesDeveloperFeeAndDeprecatedClientFee(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transactions/2mGCBGNkLcS2a3y6sPGnHXKzRNt" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"resource_type":"one_off",
			"id":"2mGCBGNkLcS2a3y6sPGnHXKzRNt",
			"customer_id":"2mGCBGNkLcS2a3y6sPGnHXKzRNu",
			"status":"completed",
			"source_asset":"USDC",
			"destination_id":"1NFHrqBHb3cTfLVkFSGmHZqdDPi",
			"destination_asset":"USD",
			"developer_fee_bps":50,
			"created_at":1700000000,
			"updated_at":1700001000,
			"receipt":{
				"input":{"amount":"100.00","asset":"USDC"},
				"output":{"amount":"99.50","asset":"USD"},
				"exchange_rate":"1.0",
				"payment_reference":null,
				"client_fee":{"amount":"0.50","asset":"USDC"},
				"developer_fee":{"amount":"0.50","asset":"USDC"}
			}
		}`))
	}))
	defer ts.Close()

	c, err := client.New(client.WithBaseURL(ts.URL), client.WithAPIKey("test_key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, err := client.CheckResponse(
		c.Raw().GetTransactionWithResponse(context.Background(), "2mGCBGNkLcS2a3y6sPGnHXKzRNt"),
	)
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatal("expected JSON200 payload")
	}
	txn, err := resp.JSON200.AsOneOffTransaction()
	if err != nil {
		t.Fatalf("AsOneOffTransaction: %v", err)
	}
	if txn.DeveloperFeeBps != 50 {
		t.Errorf("DeveloperFeeBps = %d, want 50", txn.DeveloperFeeBps)
	}
	if txn.Receipt == nil {
		t.Fatal("expected non-nil Receipt")
	}
	if fee := txn.Receipt.DeveloperFee; fee == nil || fee.Amount != "0.50" || fee.Asset != "USDC" {
		t.Errorf("Receipt.DeveloperFee = %+v, want 0.50 USDC", fee)
	}
	// Existing code that reads the deprecated field must keep compiling and
	// keep getting the same value.
	//nolint:staticcheck // SA1019: deliberately exercising the deprecated field.
	if fee := txn.Receipt.ClientFee; fee == nil || fee.Amount != "0.50" || fee.Asset != "USDC" {
		t.Errorf("Receipt.ClientFee = %+v, want 0.50 USDC", fee)
	}
}

// feeServer answers every request with status and body, and records the
// decoded JSON body of the last request it saw.
func feeServer(t *testing.T, status int, body string, got *map[string]any) *client.Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(got); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	c, err := client.New(client.WithBaseURL(ts.URL), client.WithAPIKey("test_key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return c
}

// A fixed developer fee goes on the wire as a decimal string, not a number,
// and without developer_fee_bps (ENG-3918 B5). The response reports the fee
// and the minimum deposit it implies.
func TestCreateAccount_DeveloperFeeFixedSerializesAsString(t *testing.T) {
	var got map[string]any
	c := feeServer(t, http.StatusCreated, `{
		"id":"2mGCBGNkLcS2a3y6sPGnHXKzRNt",
		"account_type":"swap",
		"developer_fee_bps":0,
		"developer_fee_fixed":"10.00",
		"minimum_deposit":"10.01"
	}`, &got)

	resp, err := client.CheckResponse(c.Raw().CreateAccountWithResponse(context.Background(), nil, gen.AccountCreateRequest{
		AccountType:       gen.AccountTypeSwap,
		DeveloperFeeFixed: ptr("10.00"),
	}))
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if v, ok := got["developer_fee_fixed"].(string); !ok || v != "10.00" {
		t.Errorf(`request developer_fee_fixed = %#v, want the string "10.00"`, got["developer_fee_fixed"])
	}
	if _, ok := got["developer_fee_bps"]; ok {
		t.Errorf("request carries developer_fee_bps = %v, want it omitted", got["developer_fee_bps"])
	}
	acct := resp.JSON201
	if acct == nil {
		t.Fatal("expected JSON201 payload")
	}
	if acct.DeveloperFeeFixed == nil || *acct.DeveloperFeeFixed != "10.00" {
		t.Errorf("DeveloperFeeFixed = %v, want 10.00", acct.DeveloperFeeFixed)
	}
	if acct.MinimumDeposit == nil || *acct.MinimumDeposit != "10.01" {
		t.Errorf("MinimumDeposit = %v, want 10.01", acct.MinimumDeposit)
	}
}

// Bps-only usage is unchanged (ENG-3918 F3): no developer_fee_fixed is sent,
// and a bps account's response leaves the fixed-fee fields nil.
func TestCreateAccount_BpsOnlyUnchanged(t *testing.T) {
	var got map[string]any
	c := feeServer(t, http.StatusCreated, `{
		"id":"2mGCBGNkLcS2a3y6sPGnHXKzRNt",
		"account_type":"swap",
		"developer_fee_bps":50
	}`, &got)

	resp, err := client.CheckResponse(c.Raw().CreateAccountWithResponse(context.Background(), nil, gen.AccountCreateRequest{
		AccountType:     gen.AccountTypeSwap,
		DeveloperFeeBps: ptr(int32(50)),
	}))
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if v, ok := got["developer_fee_bps"].(float64); !ok || v != 50 {
		t.Errorf("request developer_fee_bps = %#v, want 50", got["developer_fee_bps"])
	}
	if _, ok := got["developer_fee_fixed"]; ok {
		t.Errorf("request carries developer_fee_fixed = %v, want it omitted", got["developer_fee_fixed"])
	}
	acct := resp.JSON201
	if acct == nil {
		t.Fatal("expected JSON201 payload")
	}
	if acct.DeveloperFeeBps != 50 {
		t.Errorf("DeveloperFeeBps = %d, want 50", acct.DeveloperFeeBps)
	}
	if acct.DeveloperFeeFixed != nil || acct.MinimumDeposit != nil {
		t.Errorf("DeveloperFeeFixed = %v, MinimumDeposit = %v, want both nil", acct.DeveloperFeeFixed, acct.MinimumDeposit)
	}
}

func TestUpdateAccount_DeveloperFeeFixedSerializesAsString(t *testing.T) {
	var got map[string]any
	c := feeServer(t, http.StatusOK, `{
		"id":"2mGCBGNkLcS2a3y6sPGnHXKzRNt",
		"account_type":"onramp",
		"developer_fee_bps":0,
		"developer_fee_fixed":"2.50",
		"minimum_deposit":"2.51"
	}`, &got)

	resp, err := client.CheckResponse(c.Raw().UpdateAccountWithResponse(context.Background(), "2mGCBGNkLcS2a3y6sPGnHXKzRNt", nil, gen.AccountUpdateRequest{
		AccountType:       gen.AccountTypeOnramp,
		DeveloperFeeFixed: ptr("2.50"),
	}))
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if v, ok := got["developer_fee_fixed"].(string); !ok || v != "2.50" {
		t.Errorf(`request developer_fee_fixed = %#v, want the string "2.50"`, got["developer_fee_fixed"])
	}
	if acct := resp.JSON200; acct == nil || acct.MinimumDeposit == nil || *acct.MinimumDeposit != "2.51" {
		t.Errorf("JSON200 = %+v, want MinimumDeposit 2.51", acct)
	}
}

func TestCreateOneOff_DeveloperFeeFixedSerializesAsString(t *testing.T) {
	var got map[string]any
	c := feeServer(t, http.StatusCreated, `{
		"id":"2mGCBGNkLcS2a3y6sPGnHXKzRNt",
		"customer_id":"2mGCBGNkLcS2a3y6sPGnHXKzRNu",
		"status":"pending",
		"developer_fee_bps":0,
		"developer_fee_fixed":"10.00"
	}`, &got)

	resp, err := client.CheckResponse(c.Raw().CreateTransactionWithResponse(context.Background(), nil, gen.OneOffTransactionRequest{
		CustomerId:        "2mGCBGNkLcS2a3y6sPGnHXKzRNu",
		DestinationId:     "1NFHrqBHb3cTfLVkFSGmHZqdDPi",
		DestinationAsset:  "USDC",
		SourceAsset:       "USDC",
		SourceNetworkId:   "ethereum-mainnet",
		DeveloperFeeFixed: ptr("10.00"),
	}))
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if v, ok := got["developer_fee_fixed"].(string); !ok || v != "10.00" {
		t.Errorf(`request developer_fee_fixed = %#v, want the string "10.00"`, got["developer_fee_fixed"])
	}
	if _, ok := got["developer_fee_bps"]; ok {
		t.Errorf("request carries developer_fee_bps = %v, want it omitted", got["developer_fee_bps"])
	}
	if txn := resp.JSON201; txn == nil || txn.DeveloperFeeFixed == nil || *txn.DeveloperFeeFixed != "10.00" {
		t.Errorf("JSON201 = %+v, want DeveloperFeeFixed 10.00", txn)
	}
}

// Agentic defaults take a fixed fee per payout type (ENG-3923): it goes on the
// wire as a decimal string with no developer_fee_bps beside it, and a bps
// payout type next to it is unchanged. The drafted create_auto_account action
// reports its own fixed fee back.
func TestCreateProposals_DeveloperFeeDefaultsFixed(t *testing.T) {
	var got map[string]any
	c := feeServer(t, http.StatusOK, `{
		"proposals":[{"actions":[{
			"type":"create_auto_account",
			"create_auto_account":{
				"source_asset":"USDC",
				"source_network_id":"base-mainnet",
				"output_asset":"USDC",
				"output_network_id":"solana-mainnet",
				"developer_fee_fixed":"10.00"
			}
		}]}]
	}`, &got)

	resp, err := client.CheckResponse(c.Raw().CreatePaymentAgentProposalsWithResponse(context.Background(), "2mGCBGNkLcS2a3y6sPGnHXKzRNt", gen.CreateProposalsRequest{
		Prompt: ptr("pay Bruno 100 USDC on Solana"),
		DeveloperFeeDefaults: &gen.DeveloperFeeDefaults{
			Swap:    &gen.DeveloperFeeRate{DeveloperFeeFixed: ptr("10.00")},
			Offramp: &gen.DeveloperFeeRate{DeveloperFeeBps: ptr(int32(25))},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	defaults, _ := got["developer_fee_defaults"].(map[string]any)
	swap, _ := defaults["swap"].(map[string]any)
	if v, ok := swap["developer_fee_fixed"].(string); !ok || v != "10.00" {
		t.Errorf(`request swap.developer_fee_fixed = %#v, want the string "10.00"`, swap["developer_fee_fixed"])
	}
	if _, ok := swap["developer_fee_bps"]; ok {
		t.Errorf("request swap carries developer_fee_bps = %v, want it omitted", swap["developer_fee_bps"])
	}
	offramp, _ := defaults["offramp"].(map[string]any)
	if v, ok := offramp["developer_fee_bps"].(float64); !ok || v != 25 {
		t.Errorf("request offramp.developer_fee_bps = %#v, want 25", offramp["developer_fee_bps"])
	}
	if _, ok := offramp["developer_fee_fixed"]; ok {
		t.Errorf("request offramp carries developer_fee_fixed = %v, want it omitted", offramp["developer_fee_fixed"])
	}

	if resp.JSON200 == nil || resp.JSON200.Proposals == nil || len(*resp.JSON200.Proposals) != 1 {
		t.Fatalf("JSON200 = %+v, want one proposal", resp.JSON200)
	}
	actions := (*resp.JSON200.Proposals)[0].Actions
	if len(actions) != 1 {
		t.Fatalf("proposal actions = %+v, want one", actions)
	}
	action := actions[0].CreateAutoAccount
	if action == nil || action.DeveloperFeeFixed == nil || *action.DeveloperFeeFixed != "10.00" {
		t.Errorf("CreateAutoAccount = %+v, want DeveloperFeeFixed 10.00", action)
	}
}

// An action-level fixed fee on create_auto_account is posted back on accept as
// a decimal string, without developer_fee_bps (ENG-3923).
func TestCreateInstructions_AutoAccountDeveloperFeeFixedSerializesAsString(t *testing.T) {
	var got map[string]any
	c := feeServer(t, http.StatusCreated, `{"instruction_ids":["2mGCBGNkLcS2a3y6sPGnHXKzRNv"]}`, &got)

	_, err := client.CheckResponse(c.Raw().CreateInstructionsWithResponse(context.Background(), gen.CreateInstructionsRequest{
		PaymentAgentId: "2mGCBGNkLcS2a3y6sPGnHXKzRNt",
		Proposals: []gen.AgenticProposal{{Actions: []gen.AgenticAction{{
			Type: gen.AgenticActionTypeCreateAutoAccount,
			CreateAutoAccount: &gen.CreateAutoAccountAction{
				SourceAsset:       "USDC",
				SourceNetworkId:   "base-mainnet",
				OutputAsset:       "USDC",
				DeveloperFeeFixed: ptr("10.00"),
			},
		}}}},
	}))
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	proposals, _ := got["proposals"].([]any)
	if len(proposals) != 1 {
		t.Fatalf("request proposals = %#v, want one", got["proposals"])
	}
	actions, _ := proposals[0].(map[string]any)["actions"].([]any)
	if len(actions) != 1 {
		t.Fatalf("request actions = %#v, want one", proposals[0])
	}
	auto, _ := actions[0].(map[string]any)["create_auto_account"].(map[string]any)
	if v, ok := auto["developer_fee_fixed"].(string); !ok || v != "10.00" {
		t.Errorf(`request create_auto_account.developer_fee_fixed = %#v, want the string "10.00"`, auto["developer_fee_fixed"])
	}
	if _, ok := auto["developer_fee_bps"]; ok {
		t.Errorf("request create_auto_account carries developer_fee_bps = %v, want it omitted", auto["developer_fee_bps"])
	}
}
