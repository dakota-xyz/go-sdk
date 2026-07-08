package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAgentConversationMultiTurn proves the conversation hides the stateless
// multi-turn plumbing: it resends the full alternating transcript each turn,
// records the agent's reply, surfaces proposals when they arrive, and can be
// resumed from a persisted transcript.
func TestAgentConversationMultiTurn(t *testing.T) {
	t.Parallel()

	var captured [][]ChatMessage // the messages array sent on each turn
	turn := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/payment-agents/agt_1/proposals", r.URL.Path)
		var body struct {
			Messages []ChatMessage `json:"messages"`
			Prompt   *string       `json:"prompt"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Nil(t, body.Prompt, "conversation sends messages, never a single-shot prompt")
		captured = append(captured, body.Messages)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch turn {
		case 0:
			_, _ = w.Write([]byte(`{"reply":"Which network and how much?"}`))
		default:
			_, _ = w.Write([]byte(`{"reply":"Drafted — review below.","proposals":[{"summary":"Pay Alice 10 USDC","actions":[]}]}`))
		}
		turn++
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)
	conv := c.NewAgentConversation("agt_1")

	// Turn 1: clarifying question, no proposals.
	t1, err := conv.Send(context.Background(), "pay alice")
	require.NoError(t, err)
	require.Equal(t, "Which network and how much?", t1.Reply)
	require.False(t, t1.HasProposals)
	require.Equal(t, []ChatMessage{{Role: "user", Content: "pay alice"}}, captured[0])

	// Turn 2: the SDK resends the FULL alternating transcript; proposals arrive.
	t2, err := conv.Send(context.Background(), "base-sepolia, 10 USDC")
	require.NoError(t, err)
	require.True(t, t2.HasProposals)
	require.Len(t, t2.Proposals, 1)
	require.Equal(t, []ChatMessage{
		{Role: "user", Content: "pay alice"},
		{Role: "assistant", Content: "Which network and how much?"},
		{Role: "user", Content: "base-sepolia, 10 USDC"},
	}, captured[1])

	// The caller can persist the transcript (user, assistant, user, assistant).
	require.Len(t, conv.Messages(), 4)

	// Resume from a stored transcript and continue.
	resumed := c.ResumeAgentConversation("agt_1", conv.Messages())
	_, err = resumed.Send(context.Background(), "thanks")
	require.NoError(t, err)
	require.Len(t, captured[2], 5, "resumed conversation resends the prior 4 turns + the new user turn")
}

// TestAgentConversationRollsBackOnError: a failed Send must not leave a dangling
// user turn, so a retry doesn't duplicate it.
func TestAgentConversationRollsBackOnError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"detail":"upstream down"}`))
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)
	conv := c.NewAgentConversation("agt_1")

	_, err = conv.Send(context.Background(), "pay alice")
	require.Error(t, err)
	require.Empty(t, conv.Messages(), "a failed turn must roll back the optimistic user message")
}
