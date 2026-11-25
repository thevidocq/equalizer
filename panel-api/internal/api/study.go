package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// The security study of a discovery (SecurityStudy): the agent takes it
// once the check of the discovery's draft is over, studies the map as the
// check left it, and hands it back into the discovery's report.

type studyJob struct {
	ID       int64           `json:"id"`
	Tenant   string          `json:"tenant"`
	App      string          `json:"app"`
	Version  int             `json:"version"`
	Report   json.RawMessage `json:"report"`
	Document json.RawMessage `json:"document"`
	LLM      map[string]any  `json:"llm"`
}

func (s *Server) claimStudy(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.ClaimStudy(r.Context())
	if errors.Is(err, store.ErrNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	end := func(reason string) {
		b, _ := json.Marshal(map[string]any{"used": false, "reason": reason})
		_ = s.Store.FinishStudy(r.Context(), c.Discovery, b, true) // skipped on purpose, not failed
		w.WriteHeader(http.StatusNoContent)
	}
	a, err := s.Store.App(r.Context(), c.App)
	if err != nil {
		end("the site is gone")
		return
	}
	// the draft as the check left it: the discovery points at the
	// corrected version when there is one; else the published map
	var v store.MapVersion
	if c.Version != nil {
		v, err = s.Store.Version(r.Context(), a.ID, *c.Version)
	} else {
		v, err = s.Store.Published(r.Context(), a.ID)
	}
	if err != nil {
		end("no map to study")
		return
	}
	llm := s.tenantModel(r, a.Tenant)
	if llm == nil {
		end("no AI model configured for this tenant")
		return
	}
	writeJSON(w, http.StatusOK, studyJob{ID: c.Discovery, Tenant: a.Tenant, App: a.ID, Version: v.Version,
		Report: c.Report, Document: v.Document, LLM: llm})
}

func (s *Server) studyResult(w http.ResponseWriter, r *http.Request) {
	n, ok := pathInt(r, "id")
	id := int64(n)
	if !ok {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	var in struct {
		Security json.RawMessage `json:"security"`
	}
	if !decode(w, r, &in) {
		return
	}
	var sec struct {
		Used  *bool  `json:"used"`
		Error string `json:"error"`
	}
	if len(in.Security) > 512<<10 || json.Unmarshal(in.Security, &sec) != nil || sec.Used == nil {
		fail(w, http.StatusUnprocessableEntity, "invalid", "security: a SecurityStudy of 512 KB at most")
		return
	}
	if err := s.Store.FinishStudy(r.Context(), id, in.Security, *sec.Used || sec.Error == ""); err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// studyPending: the agent left the report's study to be written after the
// check of the draft.
func studyPending(report json.RawMessage) bool {
	var r struct {
		Security struct {
			Pending bool `json:"pending"`
		} `json:"security"`
	}
	return json.Unmarshal(report, &r) == nil && r.Security.Pending
}

// withSecurity is report with its security section replaced.
func withSecurity(report json.RawMessage, sec map[string]any) json.RawMessage {
	var r map[string]any
	if json.Unmarshal(report, &r) != nil || r == nil {
		return report
	}
	r["security"] = sec
	b, err := json.Marshal(r)
	if err != nil {
		return report
	}
	return b
}

// studyAtFinish is the report a discovery ends with and the status of its
// security study: written after the check of the draft, or at once when
// there is none; not written when the discovery failed (no map to study).
func studyAtFinish(report json.RawMessage, draft *int, failed bool) (json.RawMessage, *string) {
	if !studyPending(report) {
		return report, nil
	}
	if failed {
		return withSecurity(report, map[string]any{"used": false, "reason": "the discovery did not end with a map"}), nil
	}
	return report, store.StudyAfter(draft)
}
