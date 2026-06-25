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

// ChatMessage is one turn of an agent conversation. Role is "user" or "assistant".
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
	client  *Client
	agentID string
	history []ChatMessage
}

// NewAgentConversation starts a fresh conversation with the agent.
func (c *Client) NewAgentConversation(agentID string) *AgentConversation {
	return &AgentConversation{client: c, agentID: agentID}
}

// ResumeAgentConversation rebuilds a conversation from a persisted transcript
// (oldest first) — for backends that store the history between requests.
func (c *Client) ResumeAgentConversation(agentID string, history []ChatMessage) *AgentConversation {
	h := make([]ChatMessage, len(history))
	copy(h, history)
	return &AgentConversation{client: c, agentID: agentID, history: h}
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
}

// Send adds the user's message to the transcript, asks the agent, records the
// agent's reply, and returns the turn. When HasProposals is true the agent has
// drafted a reviewable action series; otherwise Reply holds its next question.
//
// On error the optimistic user turn is rolled back, so the caller may retry the
// same Send without duplicating it.
func (cv *AgentConversation) Send(ctx context.Context, userMessage string) (*ConversationTurn, error) {
	cv.history = append(cv.history, ChatMessage{Role: "user", Content: userMessage})

	msgs := make([]gen.AgenticChatMessage, 0, len(cv.history))
	for _, m := range cv.history {
		msgs = append(msgs, gen.AgenticChatMessage{
			Role:    gen.AgenticChatMessageRole(m.Role),
			Content: m.Content,
		})
	}

	resp, err := CheckResponse(cv.client.Raw().CreateProposalsWithResponse(ctx, cv.agentID, gen.CreateProposalsRequest{
		Messages: &msgs,
	}))
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
	if resp.JSON200.Proposals != nil {
		turn.Proposals = *resp.JSON200.Proposals
		turn.HasProposals = len(turn.Proposals) > 0
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
