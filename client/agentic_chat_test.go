package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
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

// TestAgentConversationAttachmentsAreOneShot proves an attachment rides only on
// the turn it is passed: it reaches that request, but is never persisted in the
// transcript nor re-sent on later turns (guards against sensitive-document
// retention / re-transmission).
func TestAgentConversationAttachmentsAreOneShot(t *testing.T) {
	t.Parallel()

	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"reply":"ok"}`))
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)
	conv := c.NewAgentConversation("agt_1")

	// base64 of the raw bytes is what actually crosses the wire.
	want := base64.StdEncoding.EncodeToString([]byte("HelloInvoice"))

	// Turn 1 carries a document attachment.
	_, err = conv.SendWithAttachments(context.Background(), "here is the invoice", []Attachment{{
		MediaType: "application/pdf",
		Data:      []byte("HelloInvoice"),
		Filename:  "invoice.pdf",
	}})
	require.NoError(t, err)
	require.Contains(t, bodies[0], want, "the attachment must ride on its own turn's request")

	// It must NOT be persisted in the transcript (Messages()/ResumeAgentConversation).
	for _, m := range conv.Messages() {
		require.Empty(t, m.Attachments, "attachments must not be persisted in the transcript")
	}

	// Turn 2 carries no attachment and must NOT re-send turn 1's document.
	_, err = conv.Send(context.Background(), "any update?")
	require.NoError(t, err)
	require.NotContains(t, bodies[1], want, "attachments must not be re-sent on later turns")
}

// TestAgentConversationBlockersAccompanyProposals: blockers are NOT an
// alternative to proposals. The common case is a payee who does not exist yet —
// the turn proposes creating them AND reports that the limit will not reach
// them, because the client has to do both, in that order.
func TestAgentConversationBlockersAccompanyProposals(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"reply":"I can add Priya, but your limit will not reach her.",
			"proposals":[{"summary":"Create payee Priya","actions":[]}],
			"blockers":[{"code":"mandate_does_not_cover_payee","mandate_id":"mandate_1","payee_name":"Priya","detail":"the limit does not target this payee"}]
		}`))
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)

	turn, err := c.NewAgentConversation("agt_1").Send(context.Background(), "pay priya 10 usdc")
	require.NoError(t, err)

	require.True(t, turn.HasProposals, "a blocked turn can still carry work to accept first")
	require.Len(t, turn.Proposals, 1)
	require.True(t, turn.HasBlockers)
	require.Len(t, turn.Blockers, 1)
	require.Equal(t, "mandate_does_not_cover_payee", string(turn.Blockers[0].Code))
	require.NotNil(t, turn.Blockers[0].PayeeName)
	require.Equal(t, "Priya", *turn.Blockers[0].PayeeName)
}

// TestAgentConversationNoBlockers: a clean turn reports none, and must not
// invent any.
func TestAgentConversationNoBlockers(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"reply":"ok"}`))
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)

	turn, err := c.NewAgentConversation("agt_1").Send(context.Background(), "hi")
	require.NoError(t, err)
	require.False(t, turn.HasBlockers)
	require.Empty(t, turn.Blockers)
}

// TestAgentConversationResendsTimezone: the endpoint is stateless, so a value
// given once would be forgotten on the next turn. Timezone must ride on EVERY
// request, and must be absent (not empty) when unset.
//
// The conversation deliberately sends NO client_policy. A policy is a property
// of the client, not of a request: it is registered once via
// PUT /agentic-policy and resolved server-side for every drafting turn and
// every accept. Carrying one per request let a two-call conversation disagree
// with itself — a draft judged legal under one policy could be refused at the
// customer's approval click — so the request body no longer accepts the field.
func TestAgentConversationResendsTimezone(t *testing.T) {
	t.Parallel()

	type capturedBody struct {
		Timezone     *string         `json:"timezone"`
		ClientPolicy json.RawMessage `json:"client_policy"`
	}
	var captured []capturedBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b capturedBody
		require.NoError(t, json.NewDecoder(r.Body).Decode(&b))
		captured = append(captured, b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"reply":"ok"}`))
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)

	conv := c.NewAgentConversation("agt_1", WithTimezone("America/Los_Angeles"))
	_, err = conv.Send(context.Background(), "pay alice tomorrow")
	require.NoError(t, err)
	_, err = conv.Send(context.Background(), "make it friday")
	require.NoError(t, err)

	require.Len(t, captured, 2)
	for i, b := range captured {
		require.NotNil(t, b.Timezone, "turn %d lost the timezone", i)
		require.Equal(t, "America/Los_Angeles", *b.Timezone)
		require.Empty(t, b.ClientPolicy, "turn %d sent a client_policy the endpoint no longer accepts", i)
	}

	// Unset: the key is absent rather than empty.
	plain := c.NewAgentConversation("agt_1")
	_, err = plain.Send(context.Background(), "pay alice")
	require.NoError(t, err)
	require.Nil(t, captured[2].Timezone)
}

// TestAgentConversationRejectedInputLeavesTranscriptClean: a rejected_input turn
// was refused WHOLESALE and must not enter the transcript — the spec is explicit
// that it "should NOT be added to the conversation history".
//
// Left in, the refused message is re-transmitted on every later turn — the exact
// message the server asked the client to drop — corrupting the conversation from
// that point on. Messages() returns a copy, so a caller could not repair it.
func TestAgentConversationRejectedInputLeavesTranscriptClean(t *testing.T) {
	t.Parallel()

	var sent [][]ChatMessage
	turn := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []ChatMessage `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		sent = append(sent, body.Messages)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // NOTE: 200, not an error — that is the trap
		switch turn {
		case 0:
			_, _ = w.Write([]byte(`{"reply":"Got it.","conversation_status":"ok"}`))
		case 1:
			_, _ = w.Write([]byte(`{"reply":"Too many payees — resend with one.","conversation_status":"rejected_input"}`))
		default:
			_, _ = w.Write([]byte(`{"reply":"Sure.","conversation_status":"ok"}`))
		}
		turn++
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
	require.NoError(t, err)
	conv := c.NewAgentConversation("agt_1")

	_, err = conv.Send(context.Background(), "pay alice")
	require.NoError(t, err)
	before := conv.Messages()
	require.Len(t, before, 2) // user + assistant

	// The refused turn: the caller still gets the reply and the status...
	rejected, err := conv.Send(context.Background(), "pay these 400 payees")
	require.NoError(t, err)
	require.Equal(t, "rejected_input", rejected.ConversationStatus)
	require.Equal(t, "Too many payees — resend with one.", rejected.Reply)

	// ...but the transcript is byte-identical to before it.
	require.Equal(t, before, conv.Messages(),
		"a rejected_input message must leave no trace — neither the user turn nor a synthetic assistant turn")

	// And the next turn must not re-transmit the refused message.
	_, err = conv.Send(context.Background(), "pay bob")
	require.NoError(t, err)
	require.Len(t, sent, 3)
	for _, m := range sent[2] {
		require.NotEqual(t, "pay these 400 payees", m.Content,
			"the refused message was re-sent — the server explicitly asked the client to drop it")
	}
	// The third request carries: alice, assistant, bob. Not the refused one.
	require.Equal(t, []ChatMessage{
		{Role: "user", Content: "pay alice"},
		{Role: "assistant", Content: "Got it."},
		{Role: "user", Content: "pay bob"},
	}, sent[2])
}

// TestAgentConversationWarnedAndBlockedStayInTranscript: only rejected_input is
// dropped. warned and blocked are ordinary turns that happened, and removing
// them would break the alternating transcript the platform requires.
func TestAgentConversationWarnedAndBlockedStayInTranscript(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"ok", "warned", "blocked"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"reply":"noted","conversation_status":"` + status + `"}`))
			}))
			defer srv.Close()

			c, err := New(WithBaseURL(srv.URL), WithAPIKey("test"))
			require.NoError(t, err)
			conv := c.NewAgentConversation("agt_1")

			_, err = conv.Send(context.Background(), "hello")
			require.NoError(t, err)
			require.Equal(t, []ChatMessage{
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "noted"},
			}, conv.Messages())
		})
	}
}
