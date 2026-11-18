package api

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/logsearch"
)

// The Logs page: security events of the tenant's own sites, read from Loki.
// The tenant comes from the user, never from the query string.

const (
	logPage   = 50
	logExport = 10000
)

func (s *Server) logFilter(w http.ResponseWriter, r *http.Request, u *auth.User) (logsearch.Filter, bool) {
	tenant := r.PathValue("tenant")
	if !u.HasTenantRole(tenant, "admin", "approver", "auditor") {
		forbidden(w)
		return logsearch.Filter{}, false
	}
	if s.Logs == nil {
		fail(w, http.StatusServiceUnavailable, "logs_unavailable", "The log store is not configured (EQ_LOKI_URL).")
		return logsearch.Filter{}, false
	}
	q := r.URL.Query()
	f := logsearch.Filter{Tenant: tenant, App: q.Get("app"), Search: q.Get("q"), Since: q.Get("since")}
	if t := q.Get("types"); t != "" {
		f.Types = strings.Split(t, ",")
	}
	if err := f.Check(); err != nil {
		fail(w, http.StatusUnprocessableEntity, "invalid", "Unknown filter value.")
		return logsearch.Filter{}, false
	}
	return f, true
}

func (s *Server) logError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, logsearch.ErrUnavailable) {
		s.Log.WarnContext(r.Context(), "log store unavailable", "error", err.Error())
		fail(w, http.StatusServiceUnavailable, "logs_unavailable", "The log store does not answer. Try again in a moment.")
		return
	}
	s.Log.ErrorContext(r.Context(), "log query failed", "error", err.Error())
	fail(w, http.StatusInternalServerError, "error", "")
}

func (s *Server) listLogs(w http.ResponseWriter, r *http.Request, u *auth.User) {
	f, ok := s.logFilter(w, r, u)
	if !ok {
		return
	}
	var before int64
	if b := r.URL.Query().Get("before"); b != "" {
		n, err := strconv.ParseInt(b, 10, 64)
		if err != nil || n <= 0 {
			fail(w, http.StatusUnprocessableEntity, "invalid", "before: a cursor from next_cursor")
			return
		}
		before = n
	}
	items, next, err := s.Logs.Events(r.Context(), f, before, logPage)
	if err != nil {
		s.logError(w, r, err)
		return
	}
	var cursor *string
	if next != "" {
		cursor = &next
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": cursor})
}

func (s *Server) logStats(w http.ResponseWriter, r *http.Request, u *auth.User) {
	f, ok := s.logFilter(w, r, u)
	if !ok {
		return
	}
	f.Types = nil
	counts, refused, err := s.Logs.Stats(r.Context(), f)
	if err != nil {
		s.logError(w, r, err)
		return
	}
	_, step := f.Span()
	writeJSON(w, http.StatusOK, map[string]any{"counts": counts, "refused": refused, "step_seconds": int(step.Seconds())})
}

func (s *Server) exportLogs(w http.ResponseWriter, r *http.Request, u *auth.User) {
	f, ok := s.logFilter(w, r, u)
	if !ok {
		return
	}
	var rows []logsearch.Event
	var before int64
	for len(rows) < logExport {
		items, next, err := s.Logs.Events(r.Context(), f, before, 500)
		if err != nil {
			s.logError(w, r, err)
			return
		}
		rows = append(rows, items...)
		if next == "" {
			break
		}
		before, _ = strconv.ParseInt(next, 10, 64)
	}
	rows = rows[:min(len(rows), logExport)]
	_ = s.Store.Record(r.Context(), &u.ID, &f.Tenant, "logs.export", f.App, nil)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="equalizer-logs.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"time", "type", "reason", "site", "map_entry", "map_version", "method", "path", "status", "src_ip", "request_id", "field"})
	for _, e := range rows {
		_ = cw.Write([]string{e.TS.UTC().Format("2006-01-02T15:04:05.000Z"), e.Type, cell(e.Reason), cell(e.App), cell(e.Route),
			num(e.MapVersion), cell(e.Method), cell(e.Path), num(e.Status), cell(e.SrcIP), cell(e.RequestID), cell(e.Field)})
	}
	cw.Flush()
}

// cell neutralises spreadsheet formulas: a path can start with "=" or "+".
func cell(p *string) string {
	if p == nil {
		return ""
	}
	if v := *p; v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return *p
}

func num(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}
