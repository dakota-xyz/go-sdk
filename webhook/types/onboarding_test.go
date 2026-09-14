package types_test

import (
	"testing"

	"github.com/dakota-xyz/go-sdk/webhook"
	"github.com/dakota-xyz/go-sdk/webhook/types"
)

// Fixtures below are shaped from the platform's emitters
// (pkg/core/events/event_customer_rfi.go,
// event_customer_capability_status_updated.go), not from the SDK's own types,
// so decodeEvent's strictness checks the struct against what is actually sent.

func TestEventDataAs_CustomerCapabilityStatusUpdatedData(t *testing.T) {
	payload := `{
		"customer_id":"b1h8iYu3xdBGEet3zfHKz2NLGAo",
		"capability":"international_wire",
		"status":"action_required",
		"requirements":[
			{"type":"terms_acceptance","key":"terms_v2","title":"Accept terms","severity":"required","version":"2"},
			{"type":"document","key":"proof_of_address","title":"Proof of address","severity":"requested","url":"https://example.test/upload"}
		]
	}`

	data := decodeEvent[types.CustomerCapabilityStatusUpdatedData](
		t, "evt_capability_status", webhook.EventCustomerCapabilityStatusUpdated, payload,
	)
	if data.Capability != "international_wire" || data.Status != "action_required" {
		t.Fatalf("unexpected decode: %+v", data)
	}
	if len(data.Requirements) != 2 {
		t.Fatalf("Requirements len = %d, want 2", len(data.Requirements))
	}
	if data.Requirements[0].Version != "2" || data.Requirements[0].URL != "" {
		t.Errorf("first requirement = %+v", data.Requirements[0])
	}
	if data.Requirements[1].URL != "https://example.test/upload" || data.Requirements[1].Version != "" {
		t.Errorf("second requirement = %+v", data.Requirements[1])
	}

	// Nothing outstanding: the platform sends an empty array, not a missing key.
	clear := `{"customer_id":"b1h8iYu3xdBGEet3zfHKz2NLGAo","capability":"international_wire","status":"available","requirements":[]}`
	clearData := decodeEvent[types.CustomerCapabilityStatusUpdatedData](
		t, "evt_capability_status_clear", webhook.EventCustomerCapabilityStatusUpdated, clear,
	)
	if clearData.Requirements == nil || len(clearData.Requirements) != 0 {
		t.Errorf("Requirements = %#v, want empty non-nil", clearData.Requirements)
	}
}

func TestEventDataAs_CustomerRFIRequestedData(t *testing.T) {
	// Every requirement vocabulary at once, as the platform's own example
	// shows them: a document by type, a document by purpose that is already
	// on file for a named individual, a field, and a question that needs an
	// upload with its answer.
	payload := `{
		"customer_id":"1NFHrqBHb3cTfLVkFSGmHZqdDPw",
		"application_id":"1NFHrqBHb3cTfLVkFSGmHZqdApp",
		"message_id":"rfimsg_1NFHrqBHb3cTfLVkFSGmHZqd",
		"requested_at":1735689700,
		"requirements":[
			{"type":"document","document_type":"bank_statement","entity":{"kind":"business"},"status":"missing"},
			{"type":"document","purpose":"individual_proof_of_address","entity":{"kind":"individual","id":"1NFHrqBHb3cTfLVkFSGmHZqdInd"},"status":"on_file"},
			{"type":"field","path":"business.business_description"},
			{"type":"question","key":"q1","prompt":"Explain the source of the June deposits.","require_document":true}
		]
	}`

	data := decodeEvent[types.CustomerRFIRequestedData](
		t, "evt_rfi_requested", webhook.EventCustomerRFIRequested, payload,
	)
	if data.MessageID != "rfimsg_1NFHrqBHb3cTfLVkFSGmHZqd" || data.RequestedAt != 1735689700 {
		t.Fatalf("unexpected decode: %+v", data)
	}
	if len(data.Requirements) != 4 {
		t.Fatalf("Requirements len = %d, want 4", len(data.Requirements))
	}

	doc := data.Requirements[0]
	if doc.Type != "document" || doc.DocumentType != "bank_statement" || doc.Purpose != "" || doc.Status != "missing" {
		t.Errorf("document-by-type = %+v", doc)
	}
	if doc.Entity == nil || doc.Entity.Kind != "business" || doc.Entity.ID != "" {
		t.Errorf("business entity = %+v", doc.Entity)
	}

	byPurpose := data.Requirements[1]
	if byPurpose.Purpose != "individual_proof_of_address" || byPurpose.DocumentType != "" || byPurpose.Status != "on_file" {
		t.Errorf("document-by-purpose = %+v", byPurpose)
	}
	if byPurpose.Entity == nil || byPurpose.Entity.ID != "1NFHrqBHb3cTfLVkFSGmHZqdInd" {
		t.Errorf("individual entity = %+v", byPurpose.Entity)
	}

	if f := data.Requirements[2]; f.Type != "field" || f.Path != "business.business_description" || f.Entity != nil {
		t.Errorf("field = %+v", f)
	}
	if q := data.Requirements[3]; q.Type != "question" || q.Key != "q1" || !q.RequireDocument {
		t.Errorf("question = %+v", q)
	}

	// A request whose thread message was not recorded omits message_id; a
	// document whose party is unknown omits entity rather than guessing one.
	bare := `{
		"customer_id":"1NFHrqBHb3cTfLVkFSGmHZqdDPw",
		"application_id":"1NFHrqBHb3cTfLVkFSGmHZqdApp",
		"requested_at":1735689700,
		"requirements":[{"type":"document","document_type":"utility_bill","status":"missing"}]
	}`
	bareData := decodeEvent[types.CustomerRFIRequestedData](
		t, "evt_rfi_requested_bare", webhook.EventCustomerRFIRequested, bare,
	)
	if bareData.MessageID != "" || bareData.Requirements[0].Entity != nil {
		t.Errorf("unexpected decode: %+v", bareData)
	}
}

func TestEventDataAs_CustomerRFIRespondedData(t *testing.T) {
	payload := `{
		"customer_id":"1NFHrqBHb3cTfLVkFSGmHZqdDPw",
		"application_id":"1NFHrqBHb3cTfLVkFSGmHZqdApp",
		"responded_at":1735776100
	}`

	data := decodeEvent[types.CustomerRFIRespondedData](
		t, "evt_rfi_responded", webhook.EventCustomerRFIResponded, payload,
	)
	if data.CustomerID != "1NFHrqBHb3cTfLVkFSGmHZqdDPw" || data.RespondedAt != 1735776100 {
		t.Fatalf("unexpected decode: %+v", data)
	}
}

func TestEventDataAs_CustomerApplicationWithdrawnData(t *testing.T) {
	payload := `{
		"customer_id":"1NFHrqBHb3cTfLVkFSGmHZqdDPw",
		"application_id":"1NFHrqBHb3cTfLVkFSGmHZqdApp",
		"reason":"Customer opted not to proceed"
	}`

	data := decodeEvent[types.CustomerApplicationWithdrawnData](
		t, "evt_application_withdrawn", webhook.EventCustomerApplicationWithdrawn, payload,
	)
	if data.Reason != "Customer opted not to proceed" {
		t.Fatalf("unexpected decode: %+v", data)
	}

	// The platform omits reason rather than sending an empty string.
	bare := `{"customer_id":"1NFHrqBHb3cTfLVkFSGmHZqdDPw","application_id":"1NFHrqBHb3cTfLVkFSGmHZqdApp"}`
	bareData := decodeEvent[types.CustomerApplicationWithdrawnData](
		t, "evt_application_withdrawn_bare", webhook.EventCustomerApplicationWithdrawn, bare,
	)
	if bareData.Reason != "" {
		t.Errorf("Reason = %q, want empty", bareData.Reason)
	}
}
