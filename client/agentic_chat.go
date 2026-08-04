package client

// Multi-turn agent chat. The platform's proposals endpoint is STATELESS — it
// holds no session and must be sent the whole transcript on every turn.
// AgentConversation hides that bookkeeping so a caller just sends user messages
// and reads the agent's clarifying questions / final proposals.

import (
	"context"
	"fmt"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

// conversationStatusRejectedInput is the boundary screen's verdict for a message
// refused wholesale. Its contract is that the message must NOT be added to the
// conversation history — see ConversationTurn.ConversationStatus.
const conversationStatusRejectedInput = "rejected_input"

// ChatMessage is one turn of an agent conversation. Role is "user" or "assistant".
type ChatMessage struct {
	Role        string       `json:"role"`
	Content     string       `json:"content"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment is an input artifact on a user turn for the agent to read — today a
// document (a PDF or image, e.g. an invoice to draft a payment from). Data is the
// RAW document bytes; the SDK base64-encodes them on the wire.
type Attachment struct {
	MediaType string // e.g. "application/pdf", "image/png", "image/jpeg", "image/webp", "image/gif"
	Data      []byte // raw document bytes (not base64)
	Filename  string // optional, for display/audit
}

// AgentConversation is a stateful, multi-turn proposals chat with one agent.
//
// It keeps the running transcript, appends each user message and the agent's
// reply, and resends the whole transcript on every Send — so callers never deal
// with the stateless endpoint's "send all history each time" contract.
//
// For a stateless backend (one HTTP request per chat message), persist
// Messages() between requests and rebuild with Client.ResumeAgentConversation.
type AgentConversation struct {
	client       *Client
	agentID      string
	history      []ChatMessage
	timezone     string
	clientPolicy *gen.AgenticClientPolicy
}

// ConversationOption configures an AgentConversation.
type ConversationOption func(*AgentConversation)

// WithTimezone sets the customer's IANA timezone (e.g. "America/Los_Angeles").
//
// The agent then resolves every relative date ("tomorrow", "Friday") and clock
// time ("10 am") in THAT zone: a date without a time is drafted for 10:00
// local, and a time without a date means its next local occurrence. Unset,
// times resolve as UTC and the agent says so when a specific clock time
// matters.
//
// The conversation resends it on every turn, since the endpoint is stateless.
// Note the zone's UTC offset is captured at drafting time, so a DST transition
// before a far-future fire date shifts it by the DST delta.
func WithTimezone(tz string) ConversationOption {
	return func(cv *AgentConversation) { cv.timezone = tz }
}

// WithClientPolicy sets a per-turn client_policy override — the vocabulary and
// payout constraints the agent drafts under.
//
// This is a DEVELOPMENT override. It wins for this conversation's turns and the
// server logs that it did. For production, register the policy once with
// PUT /agentic-policy (Raw().UpdateClientAgenticPolicyWithResponse) instead:
// forgetting to pass it here fails SILENTLY — the agent simply narrates in the
// platform's nouns again ("destination", "mandate"), with no error anywhere.
//
// Resolution per request is: a non-empty policy here, else the client's
// registration, else platform defaults.
func WithClientPolicy(p gen.AgenticClientPolicy) ConversationOption {
	return func(cv *AgentConversation) { cv.clientPolicy = &p }
}

// NewAgentConversation starts a fresh conversation with the agent.
//
// Experimental: agentic payments is an alpha surface (x-alpha, flag-gated on
// the platform) and may change without a major-version bump.
func (c *Client) NewAgentConversation(agentID string, opts ...ConversationOption) *AgentConversation {
	cv := &AgentConversation{client: c, agentID: agentID}
	for _, o := range opts {
		o(cv)
	}
	return cv
}

// ResumeAgentConversation rebuilds a conversation from a persisted transcript
// (oldest first) — for backends that store the history between requests.
//
// A stateless backend must pass its options again here: the transcript carries
// the messages, not the timezone or the policy override.
func (c *Client) ResumeAgentConversation(agentID string, history []ChatMessage, opts ...ConversationOption) *AgentConversation {
	h := make([]ChatMessage, len(history))
	copy(h, history)
	cv := &AgentConversation{client: c, agentID: agentID, history: h}
	for _, o := range opts {
		o(cv)
	}
	return cv
}

// ConversationTurn is the agent's response to one user message.
type ConversationTurn struct {
	// Reply is the agent's conversational text — a clarifying question or a
	// confirmation. May be empty when only proposals are returned.
	Reply string
	// Proposals is the reviewable action series, present once the agent reaches
	// high confidence. Accept it via the instructions flow.
	Proposals []gen.AgenticProposal
	// HasProposals reports whether the agent drafted proposals this turn.
	HasProposals bool
	// Blockers are machine-actionable reasons the turn could not complete — for
	// the CLIENT APPLICATION, not the customer. Reply says the same thing in
	// prose, which software cannot branch on: "extend the limit to cover Priya",
	// "I need her bank details" and "that rail is not supported" all arrive as
	// some text.
	//
	// These ACCOMPANY proposals rather than replacing them, and routinely do.
	// The common case is a payee who does not exist yet: the turn proposes
	// creating them AND reports that the limit will not reach them, because the
	// client has to do both, in that order — accept the proposal so the payee
	// has an id, then amend the limit to include it. Never treat proposals and
	// blockers as alternatives.
	//
	// Empty when nothing blocked the turn. Always switch on Code and ignore
	// codes you do not know; new ones are added over time.
	Blockers []gen.AgenticBlocker
	// HasBlockers reports whether the turn returned any blocker.
	HasBlockers bool
	// ConversationStatus is the boundary screen's verdict for this turn:
	//
	//	"ok"             normal payments turn
	//	"warned"         off-topic — the customer was warned but may continue
	//	"blocked"        the chat is terminated; stop serving it and offer a
	//	                 fresh conversation
	//	"rejected_input" this message was refused WHOLESALE (e.g. more payees
	//	                 than one conversation supports). Reply explains what to
	//	                 resend; the conversation continues unaffected, and the
	//	                 SDK has already dropped the refused message from the
	//	                 transcript for you.
	//
	// Empty when the platform sent none. Treat it as an OPEN set — new values
	// may be added.
	ConversationStatus string
}

// Send adds the user's message to the transcript, asks the agent, records the
// agent's reply, and returns the turn. When HasProposals is true the agent has
// drafted a reviewable action series; otherwise Reply holds its next question.
//
// On error the optimistic user turn is rolled back, so the caller may retry the
// same Send without duplicating it.
func (cv *AgentConversation) Send(ctx context.Context, userMessage string) (*ConversationTurn, error) {
	return cv.SendWithAttachments(ctx, userMessage, nil)
}

// SendWithAttachments is Send with documents (e.g. an invoice PDF) attached to the
// user turn for the agent to read and draft payments from. Same turn semantics and
// rollback-on-error behavior as Send. Attachments ride only on the turn they are
// passed; the stateless transcript keeps the agent's text reply, so a follow-up
// turn relies on that summary rather than re-sending the document.
func (cv *AgentConversation) SendWithAttachments(ctx context.Context, userMessage string, attachments []Attachment) (*ConversationTurn, error) {
	cv.history = append(cv.history, ChatMessage{Role: "user", Content: userMessage, Attachments: attachments})

	msgs := make([]gen.AgenticChatMessage, 0, len(cv.history))
	for _, m := range cv.history {
		gm := gen.AgenticChatMessage{
			Role:    gen.AgenticChatMessageRole(m.Role),
			Content: m.Content,
		}
		if len(m.Attachments) > 0 {
			atts := make([]gen.AgenticAttachment, 0, len(m.Attachments))
			for _, a := range m.Attachments {
				doc := gen.AgenticDocumentAttachment{
					MediaType: gen.AgenticDocumentAttachmentMediaType(a.MediaType),
					Data:      a.Data,
				}
				if a.Filename != "" {
					fn := a.Filename
					doc.Filename = &fn
				}
				atts = append(atts, gen.AgenticAttachment{
					Type:     gen.AgenticAttachmentTypeDocument,
					Document: &doc,
				})
			}
			gm.Attachments = &atts
		}
		msgs = append(msgs, gm)
	}

	// Attachments ride only on THIS request: they are serialized into msgs above,
	// then dropped from the stored transcript so they are neither persisted via
	// Messages()/ResumeAgentConversation nor re-sent on later turns.
	if len(attachments) > 0 {
		cv.history[len(cv.history)-1].Attachments = nil
	}

	// Timezone and the policy override are resent on EVERY turn — the endpoint
	// is stateless, so either one given once would be forgotten on the next.
	body := gen.CreateProposalsRequest{Messages: &msgs}
	if cv.timezone != "" {
		tz := cv.timezone
		body.Timezone = &tz
	}
	if cv.clientPolicy != nil {
		body.ClientPolicy = cv.clientPolicy
	}

	resp, err := CheckResponse(cv.client.Raw().CreatePaymentAgentProposalsWithResponse(ctx, cv.agentID, body))
	if err != nil {
		cv.history = cv.history[:len(cv.history)-1] // roll back the optimistic user turn
		return nil, fmt.Errorf("agent conversation: %w", err)
	}
	if resp.JSON200 == nil {
		cv.history = cv.history[:len(cv.history)-1]
		return nil, fmt.Errorf("agent conversation: empty response body")
	}

	turn := &ConversationTurn{}
	if resp.JSON200.Reply != nil {
		turn.Reply = *resp.JSON200.Reply
	}
	if resp.JSON200.ConversationStatus != nil {
		turn.ConversationStatus = string(*resp.JSON200.ConversationStatus)
	}
	if resp.JSON200.Proposals != nil {
		turn.Proposals = *resp.JSON200.Proposals
		turn.HasProposals = len(turn.Proposals) > 0
	}
	if resp.JSON200.Blockers != nil {
		turn.Blockers = *resp.JSON200.Blockers
		turn.HasBlockers = len(turn.Blockers) > 0
	}

	// rejected_input: the server refused this message WHOLESALE and told us not
	// to keep it. Roll the optimistic user turn back and record no assistant
	// turn, so the transcript is byte-identical to before this Send. The
	// conversation itself continues unaffected — the caller still gets the turn,
	// whose Reply explains what to resend.
	//
	// Without this the refused message stays in history and is re-transmitted on
	// every later turn — the exact message the server asked the client to drop —
	// corrupting the conversation from here on. Messages() returns a copy, so a
	// caller could not repair the history even if it noticed.
	if turn.ConversationStatus == conversationStatusRejectedInput {
		cv.history = cv.history[:len(cv.history)-1]
		return turn, nil
	}

	// Record an assistant turn so the transcript keeps alternating — the platform
	// (and the underlying model) reject two consecutive user turns. Fall back to a
	// placeholder when the agent returned only proposals with no reply text.
	assistant := turn.Reply
	if assistant == "" {
		if turn.HasProposals {
			assistant = "(drafted proposals for your review)"
		} else {
			assistant = "(no reply)"
		}
	}
	cv.history = append(cv.history, ChatMessage{Role: "assistant", Content: assistant})

	return turn, nil
}

// Messages returns a copy of the full transcript so far. Persist this between
// requests in a stateless backend and rebuild with ResumeAgentConversation.
func (cv *AgentConversation) Messages() []ChatMessage {
	out := make([]ChatMessage, len(cv.history))
	copy(out, cv.history)
	return out
}
