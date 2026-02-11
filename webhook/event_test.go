package webhook_test

import (
	"encoding/json"
	"testing"

	"github.com/dakota-xyz/go-sdk/webhook"
)

func TestEventType_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		eventType webhook.EventType
		want      bool
	}{
		{"user.created", webhook.EventUserCreated, true},
		{"wallet.deposit", webhook.EventWalletDeposit, true},
		{"transaction.auto.updated", webhook.EventTransactionAutoUpdated, true},
		{"unknown type", webhook.EventType("unknown"), false},
		{"empty string", webhook.EventType(""), false},
		{"made up", webhook.EventType("foo.bar.baz"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.eventType.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEventType_String(t *testing.T) {
	et := webhook.EventCustomerCreated
	if got := et.String(); got != "customer.created" {
		t.Errorf("String() = %q, want %q", got, "customer.created")
	}
}

func TestEvent_DataAs(t *testing.T) {
	type txnData struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventTransactionAutoUpdated,
		Data: json.RawMessage(`{"id":"txn_123","status":"completed"}`),
	}

	var data txnData
	if err := event.DataAs(&data); err != nil {
		t.Fatalf("DataAs error: %v", err)
	}

	if data.ID != "txn_123" {
		t.Errorf("got ID %q, want %q", data.ID, "txn_123")
	}
	if data.Status != "completed" {
		t.Errorf("got Status %q, want %q", data.Status, "completed")
	}
}

func TestEvent_JSONRoundTrip(t *testing.T) {
	payload := `{"id":"evt_1","event":"customer.created","data":{"name":"Acme"},"created_at":"2024-01-15T10:45:00Z"}`

	var event webhook.Event
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if event.ID != "evt_1" {
		t.Errorf("got ID %q, want %q", event.ID, "evt_1")
	}
	if event.Type != webhook.EventCustomerCreated {
		t.Errorf("got Type %q, want %q", event.Type, webhook.EventCustomerCreated)
	}
	if event.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestAllEventTypes(t *testing.T) {
	allTypes := []webhook.EventType{
		webhook.EventUserCreated,
		webhook.EventUserUpdated,
		webhook.EventUserDeleted,
		webhook.EventAPIKeyCreated,
		webhook.EventAPIKeyDeleted,
		webhook.EventCustomerCreated,
		webhook.EventCustomerUpdated,
		webhook.EventCustomerKYBLinkCreated,
		webhook.EventCustomerKYBLinkUpdated,
		webhook.EventCustomerKYBStatusCreated,
		webhook.EventCustomerKYBStatusUpdated,
		webhook.EventAutoAccountCreated,
		webhook.EventAutoAccountUpdated,
		webhook.EventAutoAccountDeleted,
		webhook.EventTransactionAutoCreated,
		webhook.EventTransactionAutoUpdated,
		webhook.EventTransactionOneOffCreated,
		webhook.EventTransactionOneOffUpdated,
		webhook.EventRecipientCreated,
		webhook.EventRecipientUpdated,
		webhook.EventRecipientDeleted,
		webhook.EventDestinationCreated,
		webhook.EventDestinationDeleted,
		webhook.EventTargetCreated,
		webhook.EventTargetUpdated,
		webhook.EventTargetDeleted,
		webhook.EventExceptionCreated,
		webhook.EventExceptionCleared,
		webhook.EventBVNKOnboardingCreated,
		webhook.EventBVNKOnboardingUpdated,
		webhook.EventWalletCreated,
		webhook.EventWalletUpdated,
		webhook.EventWalletSignerGroupCreated,
		webhook.EventWalletSignerGroupUpdated,
		webhook.EventWalletPolicyCreated,
		webhook.EventWalletPolicyUpdated,
		webhook.EventWalletTransactionCreated,
		webhook.EventWalletTransactionUpdated,
		webhook.EventWalletDeposit,
	}

	if len(allTypes) != 39 {
		t.Errorf("expected 39 event types, got %d", len(allTypes))
	}

	for _, et := range allTypes {
		if !et.IsValid() {
			t.Errorf("expected %q to be valid", et)
		}
	}
}
