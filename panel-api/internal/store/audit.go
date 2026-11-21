package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/lib/pq"
)

// AuditEntry is one action recorded in the audit log.
type AuditEntry struct {
	ID     int64          `json:"id"`
	TS     time.Time      `json:"ts"`
	UserID *int64         `json:"user_id"`
	User   *string        `json:"user"`
	Tenant *string        `json:"tenant"`
	Action string         `json:"action"`
	Target string         `json:"target"`
	Detail map[string]any `json:"detail"`
}

// Record writes an audit entry. Detail must never contain secrets.
func (s *Store) Record(ctx context.Context, userID *int64, tenant *string, action, target string, detail map[string]any) error {
	if detail == nil {
		detail = map[string]any{}
	}
	b, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO audit (user_id, tenant_id, action, target, detail) VALUES ($1, $2, $3, $4, $5)`,
		userID, tenant, action, target, b)
	return err
}

// AuditQuery narrows the action log: words to look for, a time range and
// a page (entries older than Before, an id).
type AuditQuery struct {
	Q        string
	From, To *time.Time
	Before   int64
	Limit    int
}

// Audit lists the latest entries of a tenant, or of the platform itself
// (entries without tenant) when tenant is nil, newest first.
func (s *Store) Audit(ctx context.Context, tenant *string, q AuditQuery) ([]AuditEntry, error) {
	if q.Limit <= 0 || q.Limit > 500 {
		q.Limit = 200
	}
	var like *string
	if w := strings.TrimSpace(q.Q); w != "" {
		v := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(w) + "%"
		like = &v
	}
	var before *int64
	if q.Before > 0 {
		before = &q.Before
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT a.id, a.ts, a.user_id, u.email, a.tenant_id, a.action, a.target, a.detail
		FROM audit a LEFT JOIN users u ON u.id = a.user_id
		WHERE a.tenant_id IS NOT DISTINCT FROM $1::text
		  AND ($2::text IS NULL OR a.action ILIKE $2 OR a.target ILIKE $2 OR u.email ILIKE $2 OR a.detail::text ILIKE $2)
		  AND ($3::timestamptz IS NULL OR a.ts >= $3)
		  AND ($4::timestamptz IS NULL OR a.ts < $4)
		  AND ($5::bigint IS NULL OR a.id < $5)
		ORDER BY a.id DESC LIMIT $6`, tenant, like, q.From, q.To, before, q.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		var detail []byte
		if err := rows.Scan(&e.ID, &e.TS, &e.UserID, &e.User, &e.Tenant, &e.Action, &e.Target, &detail); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(detail, &e.Detail)
		out = append(out, e)
	}
	return out, rows.Err()
}

func pqArray(v []string) any { return pq.Array(v) }
