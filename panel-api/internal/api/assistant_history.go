package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/thevidocq/equalizer/panel-api/internal/assistant"
	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// A person's conversations with the assistant: theirs only, kept as long as
// the tenant says (Settings → AI).

// pastTurns are the last answered messages, starting with a question.
func pastTurns(msgs []store.ConversationMessage) []assistant.Turn {
	start := max(0, len(msgs)-(assistantMessages-1))
	if start < len(msgs) && msgs[start].Role == "assistant" {
		start++
	}
	var turns []assistant.Turn
	for _, m := range msgs[start:] {
		if n := len(turns); n > 0 && turns[n-1].Role == m.Role {
			continue // the API wants user and assistant in turn
		}
		text := []rune(m.Text)
		if len(text) > assistantChars {
			text = append(text[:assistantChars-1], '…')
		}
		turns = append(turns, assistant.Turn{Role: m.Role, Text: string(text)})
	}
	if n := len(turns); n > 0 && turns[n-1].Role == "user" {
		turns = turns[:n-1] // the new question follows
	}
	return turns
}

// keepExchange records the question and its answer in the person's
// conversation, opening it on the first question; its id.
func (s *Server) keepExchange(ctx context.Context, u *auth.User, a store.App, in assistantIn, answer string, cards []any, props []map[string]any) (string, error) {
	id := in.Conversation
	if id == "" {
		title := []rune(in.Text)
		if len(title) > 80 {
			title = append(title[:79], '…')
		}
		var err error
		if id, err = s.Store.StartConversation(ctx, u.ID, a.ID, string(title)); err != nil {
			return "", err
		}
	}
	c, _ := json.Marshal(cards)
	p, _ := json.Marshal(props)
	if err := s.Store.AddExchange(ctx, id, in.Text, answer, c, p); err != nil {
		return "", err
	}
	_ = s.Store.PurgeConversations(ctx, a.Tenant)
	return id, nil
}

func (s *Server) listConversations(w http.ResponseWriter, r *http.Request, u *auth.User) {
	cs, err := s.Store.Conversations(r.Context(), u.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	out := []store.Conversation{}
	for _, c := range cs {
		if u.CanRead(c.Tenant) { // a site of a tenant the person left: not shown
			out = append(out, c)
		}
	}
	writeJSON(w, http.StatusOK, list(out))
}

func (s *Server) getConversation(w http.ResponseWriter, r *http.Request, u *auth.User) {
	c, msgs, err := s.Store.ConversationOf(r.Context(), u.ID, r.PathValue("conversation"))
	if err != nil || !u.CanRead(c.Tenant) {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	// a proposal shows the draft it became, whoever made it
	var ids []string
	for _, m := range msgs {
		var ps []struct{ ID string }
		_ = json.Unmarshal(m.Proposals, &ps)
		for _, p := range ps {
			ids = append(ids, p.ID)
		}
	}
	drafts, err := s.Store.ProposalDrafts(r.Context(), c.App, ids)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	for i, m := range msgs {
		var ps []map[string]any
		_ = json.Unmarshal(m.Proposals, &ps)
		for _, p := range ps {
			id, _ := p["id"].(string)
			if v, ok := drafts[id]; ok {
				p["draft_version"] = v
			} else {
				p["draft_version"] = nil
			}
		}
		if ps == nil {
			ps = []map[string]any{}
		}
		msgs[i].Proposals, _ = json.Marshal(ps)
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": c, "messages": msgs})
}

func (s *Server) deleteConversation(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if err := s.Store.DeleteConversation(r.Context(), u.ID, r.PathValue("conversation")); err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type assistantSettings struct {
	KeepDays int `json:"keep_days"`
}

func (s *Server) getAssistantSettings(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.HasTenantRole(tenant, "admin", "viewer") {
		forbidden(w)
		return
	}
	d, err := s.Store.AssistantKeepDays(r.Context(), tenant)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, assistantSettings{KeepDays: d})
}

func (s *Server) putAssistantSettings(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
		return
	}
	var in assistantSettings
	if !decode(w, r, &in) {
		return
	}
	if in.KeepDays < 1 || in.KeepDays > 365 {
		fail(w, http.StatusUnprocessableEntity, "invalid", "keep_days: 1 to 365")
		return
	}
	if err := s.Store.SetAssistantKeepDays(r.Context(), tenant, in.KeepDays); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "assistant.settings", tenant, map[string]any{"keep_days": in.KeepDays})
	writeJSON(w, http.StatusOK, in)
}
