package webhook_test

import (
	"encoding/json"
	"testing"
	"time"

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
		Data: webhook.EventData{Object: json.RawMessage(`{"id":"txn_123","status":"completed"}`)},
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
	payload := `{"id":"evt_1","type":"customer.created","created":1705315500,"data":{"object":{"name":"Acme"}}}`

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
	if event.Created != 1705315500 {
		t.Errorf("got Created %d, want %d", event.Created, 1705315500)
	}
	var obj struct {
		Name string `json:"name"`
	}
	if err := event.DataAs(&obj); err != nil {
		t.Fatalf("DataAs error: %v", err)
	}
	if obj.Name != "Acme" {
		t.Errorf("got Data.Object name %q, want %q", obj.Name, "Acme")
	}
}

func TestEvent_Time(t *testing.T) {
	event := webhook.Event{
		Created: 1705315500,
	}

	got := event.Time()
	want := time.Unix(1705315500, 0)
	if !got.Equal(want) {
		t.Errorf("Time() = %v, want %v", got, want)
	}
}

func TestEventDataAs(t *testing.T) {
	type customerData struct {
		Name string `json:"name"`
	}

	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventCustomerCreated,
		Data: webhook.EventData{Object: json.RawMessage(`{"name":"Acme"}`)},
	}

	data, err := webhook.EventDataAs[customerData](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}
	if data.Name != "Acme" {
		t.Errorf("got Name %q, want %q", data.Name, "Acme")
	}
}

func TestEventDataAs_InvalidJSON(t *testing.T) {
	event := webhook.Event{
		ID:   "evt_1",
		Type: webhook.EventCustomerCreated,
		Data: webhook.EventData{Object: json.RawMessage(`{not json}`)},
	}

	type customerData struct {
		Name string `json:"name"`
	}

	_, err := webhook.EventDataAs[customerData](event)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAllEventTypes(t *testing.T) {
	allTypes := webhook.AllEventTypes

	if len(allTypes) != 39 {
		t.Errorf("expected 39 event types, got %d", len(allTypes))
	}

	seen := make(map[webhook.EventType]struct{}, len(allTypes))
	for _, et := range allTypes {
		if _, ok := seen[et]; ok {
			t.Errorf("duplicate event type %q in AllEventTypes", et)
		}
		seen[et] = struct{}{}

		if !et.IsValid() {
			t.Errorf("expected %q to be valid", et)
		}
	}
}
