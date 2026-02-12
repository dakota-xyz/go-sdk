package client_test

import (
	"testing"
	"time"

	"github.com/dakota-xyz/go-sdk/client"
	"github.com/dakota-xyz/go-sdk/client/gen"
)

func TestParseCustomer(t *testing.T) {
	decision := gen.CustomerDecision("approved")
	applicationID := gen.KSUID("2B5J8KZ9N7M1K3P6Q8R4T7V9ABC")
	in := gen.Customer{
		Id:            gen.KSUID("2B5J8KZ9N7M1K3P6Q8R4T7V9ABD"),
		ApplicationId: &applicationID,
		Name:          "Acme",
		CustomerType:  gen.CustomerCustomerType("business"),
		KybStatus:     gen.KybStatus("pending"),
		Decision:      &decision,
		CreatedAt:     1700000000,
		UpdatedAt:     1700000600,
	}

	out := client.ParseCustomer(in)
	if out.ID != string(in.Id) {
		t.Fatalf("ID = %q, want %q", out.ID, string(in.Id))
	}
	if out.ApplicationID != string(applicationID) {
		t.Fatalf("ApplicationID = %q, want %q", out.ApplicationID, string(applicationID))
	}
	if out.Decision != "approved" {
		t.Fatalf("Decision = %q, want %q", out.Decision, "approved")
	}
	if out.CreatedAt != time.Unix(1700000000, 0).UTC() {
		t.Fatalf("CreatedAt = %v, unexpected", out.CreatedAt)
	}
}

func TestParseCustomers_Batch(t *testing.T) {
	in := []gen.Customer{
		{Id: gen.KSUID("id_1"), Name: "A", CustomerType: gen.CustomerCustomerType("business"), KybStatus: gen.KybStatus("pending"), CreatedAt: 1, UpdatedAt: 2},
		{Id: gen.KSUID("id_2"), Name: "B", CustomerType: gen.CustomerCustomerType("individual"), KybStatus: gen.KybStatus("active"), CreatedAt: 3, UpdatedAt: 4},
	}
	out := client.ParseCustomers(in)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[0].ID != "id_1" || out[1].ID != "id_2" {
		t.Fatalf("unexpected parsed ids: %#v", out)
	}
}

func TestParseApplication(t *testing.T) {
	decision := gen.ApplicationListItemApplicationDecision("approved")
	submittedAt := time.Unix(1700000100, 0).UTC()
	in := gen.ApplicationListItem{
		ApplicationId:       "app_123",
		ApplicationType:     gen.ApplicationListItemApplicationType("business"),
		ApplicationStatus:   gen.ApplicationStatus("submitted"),
		ApplicationDecision: &decision,
		Business: &struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		}{Id: "biz_1", Name: "Acme LLC"},
		SubmittedAt: &submittedAt,
		CreatedAt:   time.Unix(1700000000, 0).UTC(),
		UpdatedAt:   time.Unix(1700000300, 0).UTC(),
		RiskRating: &gen.RiskRating{
			Level: gen.RiskRatingLevel("high"),
		},
	}

	out := client.ParseApplication(in)
	if out.ID != "app_123" {
		t.Fatalf("ID = %q, want %q", out.ID, "app_123")
	}
	if out.BusinessName != "Acme LLC" {
		t.Fatalf("BusinessName = %q, want %q", out.BusinessName, "Acme LLC")
	}
	if out.RiskRating != "high" {
		t.Fatalf("RiskRating = %q, want %q", out.RiskRating, "high")
	}
	if out.SubmittedAt == nil || !out.SubmittedAt.Equal(submittedAt) {
		t.Fatalf("SubmittedAt = %v, want %v", out.SubmittedAt, submittedAt)
	}
}

func TestParseApplications_Batch(t *testing.T) {
	in := []gen.ApplicationListItem{
		{ApplicationId: "app_1", ApplicationType: gen.ApplicationListItemApplicationType("business"), ApplicationStatus: gen.ApplicationStatus("created"), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ApplicationId: "app_2", ApplicationType: gen.ApplicationListItemApplicationType("business"), ApplicationStatus: gen.ApplicationStatus("submitted"), CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	out := client.ParseApplications(in)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[1].ID != "app_2" {
		t.Fatalf("unexpected parsed application: %#v", out[1])
	}
}

func TestParseTransaction(t *testing.T) {
	completedAt := 1700001200
	in := gen.Transaction{
		Id:              gen.KSUID("2B5J8KZ9N7M1K3P6Q8R4T7V9ABE"),
		Type:            gen.TransactionType("onramp"),
		Status:          gen.TransactionStatus("completed"),
		Description:     "test transaction",
		InputAmount:     gen.TransactionAmount{Amount: "100", Asset: "USD"},
		OutputAmount:    gen.TransactionAmount{Amount: "99", Asset: "USDC"},
		ConvertedAmount: gen.TransactionAmount{Amount: "99", Asset: "USDC"},
		CreatedAt:       1700000000,
		UpdatedAt:       1700001000,
		CompletedAt:     &completedAt,
	}

	out := client.ParseTransaction(in)
	if out.ID != string(in.Id) {
		t.Fatalf("ID = %q, want %q", out.ID, string(in.Id))
	}
	if out.InputAmount.Amount != "100" {
		t.Fatalf("InputAmount.Amount = %q, want %q", out.InputAmount.Amount, "100")
	}
	if out.CompletedAt == nil || !out.CompletedAt.Equal(time.Unix(int64(completedAt), 0).UTC()) {
		t.Fatalf("CompletedAt = %v, unexpected", out.CompletedAt)
	}
}

func TestParseTransactions_Batch(t *testing.T) {
	in := []gen.Transaction{
		{Id: gen.KSUID("tx_1"), Type: gen.TransactionType("onramp"), Status: gen.TransactionStatus("pending"), InputAmount: gen.TransactionAmount{Amount: "1", Asset: "USD"}, OutputAmount: gen.TransactionAmount{Amount: "1", Asset: "USDC"}, ConvertedAmount: gen.TransactionAmount{Amount: "1", Asset: "USDC"}, CreatedAt: 1, UpdatedAt: 2},
		{Id: gen.KSUID("tx_2"), Type: gen.TransactionType("swap"), Status: gen.TransactionStatus("completed"), InputAmount: gen.TransactionAmount{Amount: "2", Asset: "ETH"}, OutputAmount: gen.TransactionAmount{Amount: "3", Asset: "USDC"}, ConvertedAmount: gen.TransactionAmount{Amount: "3", Asset: "USDC"}, CreatedAt: 3, UpdatedAt: 4},
	}
	out := client.ParseTransactions(in)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[0].ID != "tx_1" || out[1].ID != "tx_2" {
		t.Fatalf("unexpected parsed transactions: %#v", out)
	}
}

func TestParseRecipientAndEvents(t *testing.T) {
	recipient := gen.RecipientResponse{Id: gen.KSUID("rec_1"), Name: "Recipient", Status: gen.RecipientResponseStatus("active")}
	parsedRecipient := client.ParseRecipient(recipient)
	if parsedRecipient.ID != "rec_1" {
		t.Fatalf("recipient ID = %q, want %q", parsedRecipient.ID, "rec_1")
	}

	parsedRecipients := client.ParseRecipients([]gen.RecipientResponse{recipient})
	if len(parsedRecipients) != 1 || parsedRecipients[0].ID != "rec_1" {
		t.Fatalf("unexpected parsed recipients: %#v", parsedRecipients)
	}

	event := gen.Event{EventId: gen.KSUID("evt_1"), Type: "created", Content: map[string]any{"k": "v"}}
	parsedEvent := client.ParseEvent(event)
	if parsedEvent.ID != "evt_1" || parsedEvent.Type != "created" {
		t.Fatalf("unexpected parsed event: %#v", parsedEvent)
	}

	parsedEvents := client.ParseEvents([]gen.Event{event})
	if len(parsedEvents) != 1 || parsedEvents[0].ID != "evt_1" {
		t.Fatalf("unexpected parsed events: %#v", parsedEvents)
	}
}
