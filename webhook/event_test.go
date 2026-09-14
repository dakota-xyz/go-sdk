package webhook_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

	// The count itself is pinned by TestAllEventTypes_MatchesSpec, which
	// derives it from the vendored spec rather than a literal to bump.
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

// TestAllEventTypes_MatchesSpec pins AllEventTypes to the EventType enum in
// the vendored spec, in both directions.
//
// The enum is the platform's contract for what a target can subscribe to. A
// spec sync that adds a value must add a constant here, or a consumer has only
// a string literal to register on and IsValid says the platform's own event is
// unknown; one that removes a value must remove the constant, or the SDK keeps
// advertising an event nothing emits. Reading the enum out of the YAML by hand
// keeps this package free of a YAML dependency it has no other use for.
func TestAllEventTypes_MatchesSpec(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "client", "gen", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	var enumerated []string
	inSchema, inEnum := false, false
	for _, line := range strings.Split(string(raw), "\n") {
		// A CRLF checkout (core.autocrlf on Windows) would otherwise match
		// nothing and fail on the count below — loudly, but for no reason.
		line = strings.TrimRight(line, "\r")
		switch {
		case line == "    EventType:":
			inSchema = true
		case inSchema && !inEnum && strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     "):
			// The next sibling schema: the block is over.
			inSchema = false
		case inSchema && line == "      enum:":
			inEnum = true
		case inEnum && strings.HasPrefix(line, "        - "):
			enumerated = append(enumerated, strings.TrimPrefix(line, "        - "))
		case inEnum:
			inEnum, inSchema = false, false
		}
	}
	if len(enumerated) < 40 {
		t.Fatalf("found only %d EventType enum values in the spec; the scan is broken", len(enumerated))
	}

	known := make(map[string]struct{}, len(webhook.AllEventTypes))
	for _, et := range webhook.AllEventTypes {
		known[string(et)] = struct{}{}
	}
	specSet := make(map[string]struct{}, len(enumerated))
	for _, v := range enumerated {
		specSet[v] = struct{}{}
		if _, ok := known[v]; !ok {
			t.Errorf("spec enumerates %q but AllEventTypes has no constant for it", v)
		}
	}
	for _, et := range webhook.AllEventTypes {
		if _, ok := specSet[string(et)]; !ok {
			t.Errorf("AllEventTypes carries %q, which the spec's EventType enum does not list", et)
		}
	}
}
