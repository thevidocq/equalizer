package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

const auditPage = 200

// auditQuery reads the filters of GET /v1/audit: q (words), from and to
// (RFC 3339) and cursor (the next_cursor of the page before).
func auditQuery(r *http.Request) (store.AuditQuery, bool) {
	v := r.URL.Query()
	q := store.AuditQuery{Q: v.Get("q"), Limit: auditPage}
	if len(q.Q) > 200 {
		q.Q = q.Q[:200]
	}
	for _, f := range []struct {
		name string
		dst  **time.Time
	}{{"from", &q.From}, {"to", &q.To}} {
		if s := v.Get(f.name); s != "" {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return q, false
			}
			*f.dst = &t
		}
	}
	if c := v.Get("cursor"); c != "" {
		id, err := strconv.ParseInt(c, 10, 64)
		if err != nil || id <= 0 {
			return q, false
		}
		q.Before = id
	}
	return q, true
}
