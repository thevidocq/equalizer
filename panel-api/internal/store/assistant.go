package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

// AssistantProposal is a change the assistant proposed on a site's map.
type AssistantProposal struct {
	ID           string
	App          string
	Change       json.RawMessage
	Summary      string
	CreatedAt    time.Time
	DraftVersion *int
}

// SaveProposal stores a checked change and returns its id. Proposals older
// than 30 days are dropped on the way.
func (s *Store) SaveProposal(ctx context.Context, app string, change json.RawMessage, summary string, by int64) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	_, _ = s.DB.ExecContext(ctx, `DELETE FROM assistant_proposals WHERE created_at < now() - interval '30 days'`)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO assistant_proposals (id, app_id, change, summary, asked_by) VALUES ($1, $2, $3, $4, $5)`,
		id, app, []byte(change), summary, by)
	return id, translate(err)
}

// Proposal is one proposal of a site.
func (s *Store) Proposal(ctx context.Context, app, id string) (AssistantProposal, error) {
	p := AssistantProposal{ID: id, App: app}
	var change []byte
	err := s.DB.QueryRowContext(ctx, `SELECT change, summary, created_at, draft_version FROM assistant_proposals
		WHERE id = $1 AND app_id = $2 AND created_at >= now() - interval '30 days'`, id, app).
		Scan(&change, &p.Summary, &p.CreatedAt, &p.DraftVersion)
	p.Change = change
	return p, translate(err)
}

// ProposalDrafted records the draft a proposal became; ErrConflict when it
// already became one.
func (s *Store) ProposalDrafted(ctx context.Context, app, id string, version int) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE assistant_proposals SET draft_version = $3
		WHERE id = $1 AND app_id = $2 AND draft_version IS NULL`, id, app, version)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrConflict
	}
	return nil
}

// WithAppLock runs fn while no other caller holds the site's lock (one
// draft built at a time from the assistant's proposals, across replicas).
func (s *Store) WithAppLock(ctx context.Context, app string, fn func() error) error {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext('assistant:' || $1))`, app); err != nil {
		return err
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(hashtext('assistant:' || $1))`, app) //nolint:errcheck
	return fn()
}
