package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dakota-xyz/go-sdk/client"
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
