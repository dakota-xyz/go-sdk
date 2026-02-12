package client

import (
	"time"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

// ParsedCustomer is an SDK-friendly customer model.
type ParsedCustomer struct {
	ID            string
	ApplicationID string
	Name          string
	CustomerType  string
	KYCStatus     string
	Decision      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ParsedApplication is an SDK-friendly application model.
type ParsedApplication struct {
	ID           string
	Type         string
	Status       string
	Decision     string
	BusinessName string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SubmittedAt  *time.Time
	RiskRating   string
}

// ParsedMoney is a normalized amount+asset pair.
type ParsedMoney struct {
	Amount string
	Asset  string
}

// ParsedTransaction is an SDK-friendly transaction model.
type ParsedTransaction struct {
	ID              string
	Type            string
	Status          string
	Description     string
	InputAmount     ParsedMoney
	OutputAmount    ParsedMoney
	ConvertedAmount ParsedMoney
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
}

// ParsedRecipient is an SDK-friendly recipient model.
type ParsedRecipient struct {
	ID     string
	Name   string
	Status string
}

// ParsedEvent is an SDK-friendly event model.
type ParsedEvent struct {
	ID      string
	Type    string
	Content map[string]any
}

// ParseCustomer converts a generated Customer model into ParsedCustomer.
func ParseCustomer(in gen.Customer) ParsedCustomer {
	out := ParsedCustomer{
		ID:           string(in.Id),
		Name:         in.Name,
		CustomerType: string(in.CustomerType),
		KYCStatus:    string(in.KybStatus),
		CreatedAt:    unixSeconds(in.CreatedAt),
		UpdatedAt:    unixSeconds(in.UpdatedAt),
	}
	if in.ApplicationId != nil {
		out.ApplicationID = string(*in.ApplicationId)
	}
	if in.Decision != nil {
		out.Decision = string(*in.Decision)
	}
	return out
}

// ParseCustomers converts a list of generated customers.
func ParseCustomers(in []gen.Customer) []ParsedCustomer {
	out := make([]ParsedCustomer, 0, len(in))
	for _, customer := range in {
		out = append(out, ParseCustomer(customer))
	}
	return out
}

// ParseApplication converts a generated application list item.
func ParseApplication(in gen.ApplicationListItem) ParsedApplication {
	out := ParsedApplication{
		ID:        in.ApplicationId,
		Type:      string(in.ApplicationType),
		Status:    string(in.ApplicationStatus),
		CreatedAt: in.CreatedAt.UTC(),
		UpdatedAt: in.UpdatedAt.UTC(),
	}
	if in.ApplicationDecision != nil {
		out.Decision = string(*in.ApplicationDecision)
	}
	if in.Business != nil {
		out.BusinessName = in.Business.Name
	}
	if in.SubmittedAt != nil {
		t := in.SubmittedAt.UTC()
		out.SubmittedAt = &t
	}
	if in.RiskRating != nil {
		out.RiskRating = string(in.RiskRating.Level)
	}
	return out
}

// ParseApplications converts a list of generated application list items.
func ParseApplications(in []gen.ApplicationListItem) []ParsedApplication {
	out := make([]ParsedApplication, 0, len(in))
	for _, app := range in {
		out = append(out, ParseApplication(app))
	}
	return out
}

// ParseTransaction converts a generated transaction model.
func ParseTransaction(in gen.Transaction) ParsedTransaction {
	out := ParsedTransaction{
		ID:          string(in.Id),
		Type:        string(in.Type),
		Status:      string(in.Status),
		Description: in.Description,
		InputAmount: ParsedMoney{
			Amount: in.InputAmount.Amount,
			Asset:  in.InputAmount.Asset,
		},
		OutputAmount: ParsedMoney{
			Amount: in.OutputAmount.Amount,
			Asset:  in.OutputAmount.Asset,
		},
		ConvertedAmount: ParsedMoney{
			Amount: in.ConvertedAmount.Amount,
			Asset:  in.ConvertedAmount.Asset,
		},
		CreatedAt: unixSeconds(in.CreatedAt),
		UpdatedAt: unixSeconds(in.UpdatedAt),
	}
	if in.CompletedAt != nil {
		t := unixSeconds(*in.CompletedAt)
		out.CompletedAt = &t
	}
	return out
}

// ParseTransactions converts a list of generated transactions.
func ParseTransactions(in []gen.Transaction) []ParsedTransaction {
	out := make([]ParsedTransaction, 0, len(in))
	for _, tx := range in {
		out = append(out, ParseTransaction(tx))
	}
	return out
}

// ParseRecipient converts a generated recipient model.
func ParseRecipient(in gen.RecipientResponse) ParsedRecipient {
	return ParsedRecipient{
		ID:     string(in.Id),
		Name:   in.Name,
		Status: string(in.Status),
	}
}

// ParseRecipients converts a list of generated recipients.
func ParseRecipients(in []gen.RecipientResponse) []ParsedRecipient {
	out := make([]ParsedRecipient, 0, len(in))
	for _, recipient := range in {
		out = append(out, ParseRecipient(recipient))
	}
	return out
}

// ParseEvent converts a generated event model.
func ParseEvent(in gen.Event) ParsedEvent {
	return ParsedEvent{
		ID:      string(in.EventId),
		Type:    in.Type,
		Content: in.Content,
	}
}

// ParseEvents converts a list of generated events.
func ParseEvents(in []gen.Event) []ParsedEvent {
	out := make([]ParsedEvent, 0, len(in))
	for _, event := range in {
		out = append(out, ParseEvent(event))
	}
	return out
}

func unixSeconds(v int) time.Time {
	return time.Unix(int64(v), 0).UTC()
}
