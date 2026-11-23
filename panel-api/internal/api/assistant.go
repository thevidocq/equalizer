package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/assistant"
	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/logsearch"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// The assistant: a person asks about one site, the tenant's AI model answers
// with tools that read the site's map, report, logs and history, and may
// propose a change that an administrator turns into a draft.

const (
	assistantMessages = 20
	assistantChars    = 4000
	assistantTimeout  = 100 * time.Second // under the server's 2-minute write timeout
)

type assistantIn struct {
	Messages []struct {
		Role string `json:"role"`
		Text string `json:"text"`
	} `json:"messages"`
}

func (s *Server) askAssistant(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	var in assistantIn
	if !decode(w, r, &in) {
		return
	}
	var turns []assistant.Turn
	for i, m := range in.Messages {
		want := "user"
		if i%2 == 1 {
			want = "assistant"
		}
		if m.Role != want || strings.TrimSpace(m.Text) == "" || len([]rune(m.Text)) > assistantChars {
			fail(w, http.StatusUnprocessableEntity, "invalid", "messages: alternate user and assistant, starting and ending with user, 1 to 4000 characters each")
			return
		}
		turns = append(turns, assistant.Turn{Role: m.Role, Text: m.Text})
	}
	if len(turns) == 0 || len(turns) > assistantMessages || turns[len(turns)-1].Role != "user" {
		fail(w, http.StatusUnprocessableEntity, "invalid", "messages: 1 to 20, the last one the person's")
		return
	}
	model, err := s.assistantModel(r, a.Tenant)
	if err != nil {
		fail(w, http.StatusConflict, "no_ai", err.Error())
		return
	}
	key := "assistant:" + u.Email
	if s.AssistantThrottle != nil {
		if s.AssistantThrottle.Blocked(key) {
			fail(w, http.StatusTooManyRequests, "too_many_questions", "20 questions every 10 minutes: try again in a few minutes")
			return
		}
		s.AssistantThrottle.Fail(key)
	}
	ctx, cancel := context.WithTimeout(r.Context(), assistantTimeout)
	defer cancel()
	src := &siteSource{s: s, a: a, u: u}
	ans, err := assistant.Run(ctx, model, src, assistant.NewMasker(maskKey), a.Name, a.PublicDomain,
		time.Now().UTC().Format("2006-01-02"), turns)
	var pe *assistant.ErrProvider
	switch {
	case errors.As(err, &pe):
		fail(w, http.StatusBadGateway, "ai_failed", pe.Msg)
		return
	case err != nil:
		s.Log.ErrorContext(r.Context(), "assistant failed", "error", err.Error())
		fail(w, http.StatusInternalServerError, "error", "")
		return
	}
	// the activity says a question was asked and what was read, never what
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "assistant.asked", a.ID,
		map[string]any{"tools": uniq(ans.Used), "proposals": len(ans.Proposals), "cards": len(ans.Cards)})
	props := make([]map[string]any, 0, len(ans.Proposals))
	for _, p := range ans.Proposals {
		props = append(props, map[string]any{"id": p.ID, "summary": p.Summary, "reason": p.Reason})
	}
	cards := ans.Cards
	if cards == nil {
		cards = []any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": ans.Text, "proposals": props, "cards": cards, "used": uniq(ans.Used)})
}

// maskKey makes a visitor's code the same across questions while the
// panel runs; it never leaves the process.
var maskKey = func() []byte {
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	return k
}()

// assistantModel is the tenant's model from Settings → AI.
func (s *Server) assistantModel(r *http.Request, tenant string) (assistant.Model, error) {
	l, err := s.Store.TenantLLM(r.Context(), tenant)
	if err != nil {
		return nil, errors.New("no AI model is set for this tenant: an administrator sets one in Settings → AI")
	}
	c := assistant.Config{Provider: l.Provider, Model: l.Model}
	if l.Endpoint != nil {
		c.Endpoint = *l.Endpoint
	}
	if len(l.Key) > 0 {
		k, err := s.MFA.OpenFor(llmLabel(tenant), l.Key)
		if err != nil {
			return nil, errors.New("the AI key cannot be read: save it again in Settings → AI")
		}
		c.Key = string(k)
	}
	if s.AssistantModel != nil {
		return s.AssistantModel(c)
	}
	return assistant.New(c)
}

func uniq(v []string) []string {
	out := []string{}
	for _, x := range v {
		seen := false
		for _, y := range out {
			seen = seen || x == y
		}
		if !seen {
			out = append(out, x)
		}
	}
	return out
}

// siteSource is what the assistant's tools read: one site, as the person
// asking may see it.
type siteSource struct {
	s *Server
	a store.App
	u *auth.User
}

func (src *siteSource) Map(ctx context.Context, version int) (map[string]any, int, string, error) {
	var v store.MapVersion
	var err error
	if version > 0 {
		v, err = src.s.Store.Version(ctx, src.a.ID, version)
	} else {
		v, err = src.s.Store.Published(ctx, src.a.ID)
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, 0, "", errors.New("no such map version (none published yet if no version was given)")
	}
	if err != nil {
		return nil, 0, "", errors.New("the map could not be read")
	}
	var doc map[string]any
	if err := json.Unmarshal(v.Document, &doc); err != nil {
		return nil, 0, "", errors.New("the map could not be read")
	}
	return doc, v.Version, v.Status, nil
}

func (src *siteSource) Report(ctx context.Context) (json.RawMessage, error) {
	ds, err := src.s.Store.Discoveries(ctx, src.a.ID)
	if err != nil {
		return nil, errors.New("the discoveries could not be read")
	}
	for _, d := range ds { // the list carries no reports: read each finished one
		if d.Status != "done" {
			continue
		}
		full, err := src.s.Store.Discovery(ctx, src.a.ID, d.ID)
		if err != nil {
			break
		}
		if len(full.Report) > 0 && string(full.Report) != "null" {
			head, _ := json.Marshal(map[string]any{"discovery": d.ID, "finished_at": d.FinishedAt, "draft_version": d.DraftVersion})
			return json.RawMessage(`{"about":` + string(head) + `,"report":` + string(full.Report) + `}`), nil
		}
	}
	return nil, errors.New("no finished discovery yet")
}

func (src *siteSource) Logs(ctx context.Context, since string, types []string, search string) ([]assistant.Event, error) {
	if src.s.Logs == nil {
		return nil, assistant.ErrNoLogs
	}
	filter := logsearch.Filter{Tenant: src.a.Tenant, App: src.a.ID, Types: types, Search: search, Since: since}
	if err := filter.Check(); err != nil {
		return nil, errors.New("unknown period or event type")
	}
	evs, _, err := src.s.Logs.Events(ctx, filter, 0, 500)
	if err != nil {
		return nil, err
	}
	out := make([]assistant.Event, 0, len(evs))
	for _, e := range evs {
		out = append(out, assistant.Event{TS: e.TS, Type: e.Type, Reason: deref(e.Reason), Route: deref(e.Route),
			Method: deref(e.Method), Path: deref(e.Path), Status: derefInt(e.Status), Field: deref(e.Field), IP: deref(e.SrcIP), RequestID: deref(e.RequestID)})
	}
	return out, nil
}

func (src *siteSource) Activity(ctx context.Context, limit int) ([]assistant.Activity, error) {
	es, err := src.s.Store.Audit(ctx, &src.a.Tenant, store.AuditQuery{Q: src.a.ID, Limit: 200})
	if err != nil {
		return nil, errors.New("the history could not be read")
	}
	var out []assistant.Activity
	for _, e := range es {
		if e.Target != src.a.ID || strings.HasPrefix(e.Action, "assistant.") {
			continue
		}
		out = append(out, assistant.Activity{TS: e.TS, User: deref(e.User), Action: e.Action, Detail: e.Detail})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// Propose checks the change on the map it would go on, then stores it.
func (src *siteSource) Propose(ctx context.Context, c assistant.Change) (string, string, error) {
	base, _, err := src.s.proposalBase(ctx, src.a.ID)
	if err != nil {
		return "", "", err
	}
	var doc map[string]any
	if err := json.Unmarshal(base.Document, &doc); err != nil {
		return "", "", errors.New("the map could not be read")
	}
	summary, err := c.Apply(doc)
	if errors.Is(err, assistant.ErrSame) {
		return "", "", errors.New("version " + strconv.Itoa(base.Version) + " already has it")
	}
	if err != nil {
		return "", "", err
	}
	if _, err := src.s.draftDoc(src.a, src.u, doc); err != nil {
		return "", "", errors.New("the map would not be valid with it: " + err.Error())
	}
	raw, _ := json.Marshal(c)
	id, err := src.s.Store.SaveProposal(ctx, src.a.ID, raw, summary, src.u.ID)
	if err != nil {
		return "", "", errors.New("the proposal could not be saved")
	}
	return id, summary, nil
}
