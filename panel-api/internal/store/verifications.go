package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Verification is the agent's check of a map version (schema Verification).
type Verification struct {
	ID          int64           `json:"-"`
	App         string          `json:"-"`
	Version     int             `json:"version"`
	Status      string          `json:"status"`
	RequestedBy *string         `json:"requested_by"`
	CreatedAt   time.Time       `json:"created_at"`
	FinishedAt  *time.Time      `json:"finished_at"`
	Score       json.RawMessage `json:"score"`
	Broken      json.RawMessage `json:"broken"`
	Open        json.RawMessage `json:"open"`
	Error       *string         `json:"error"`
}

const verificationColumns = `v.id, v.app_id, v.version, v.status, u.email, v.created_at, v.finished_at,
	v.score, v.broken, v.open, v.error`

func scanVerification(row interface{ Scan(...any) error }) (Verification, error) {
	var v Verification
	var score []byte
	err := row.Scan(&v.ID, &v.App, &v.Version, &v.Status, &v.RequestedBy, &v.CreatedAt, &v.FinishedAt,
		&score, &v.Broken, &v.Open, &v.Error)
	if score != nil {
		v.Score = score
	} else {
		v.Score = json.RawMessage("null")
	}
	return v, translate(err)
}

// QueueVerification asks the agent to verify a version, unless one is
// already waiting or running for it (that one is returned).
func (s *Store) QueueVerification(ctx context.Context, app string, version int, by *int64) (Verification, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT id FROM map_verifications WHERE app_id = $1 AND version = $2 AND status IN ('queued', 'running')
		ORDER BY id DESC LIMIT 1`, app, version).Scan(&id)
	if err == sql.ErrNoRows {
		err = s.DB.QueryRowContext(ctx, `
			INSERT INTO map_verifications (app_id, version, requested_by) VALUES ($1, $2, $3) RETURNING id`,
			app, version, by).Scan(&id)
	}
	if err != nil {
		return Verification{}, translate(err)
	}
	return s.verification(ctx, id)
}

func (s *Store) verification(ctx context.Context, id int64) (Verification, error) {
	return scanVerification(s.DB.QueryRowContext(ctx, `SELECT `+verificationColumns+`
		FROM map_verifications v LEFT JOIN users u ON u.id = v.requested_by WHERE v.id = $1`, id))
}

// LatestVerification is the newest verification of a version.
func (s *Store) LatestVerification(ctx context.Context, app string, version int) (Verification, error) {
	return scanVerification(s.DB.QueryRowContext(ctx, `SELECT `+verificationColumns+`
		FROM map_verifications v LEFT JOIN users u ON u.id = v.requested_by
		WHERE v.app_id = $1 AND v.version = $2 ORDER BY v.id DESC LIMIT 1`, app, version))
}

// ClaimVerification hands the oldest queued verification to the agent;
// one running for more than an hour is given up.
func (s *Store) ClaimVerification(ctx context.Context) (Verification, error) {
	if _, err := s.DB.ExecContext(ctx, `
		UPDATE map_verifications SET status = 'failed', error = 'the agent stopped answering', finished_at = now()
		WHERE status = 'running' AND started_at < now() - interval '1 hour'`); err != nil {
		return Verification{}, err
	}
	var id int64
	err := s.DB.QueryRowContext(ctx, `
		UPDATE map_verifications SET status = 'running', started_at = now()
		WHERE id = (SELECT id FROM map_verifications WHERE status = 'queued' ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1)
		RETURNING id`).Scan(&id)
	if err != nil {
		return Verification{}, translate(err)
	}
	return s.verification(ctx, id)
}

// FinishVerification stores the agent's result of a running verification.
func (s *Store) FinishVerification(ctx context.Context, id int64, score, broken, open json.RawMessage, failure *string) error {
	status := "done"
	if failure != nil {
		status = "failed"
	}
	res, err := s.DB.ExecContext(ctx, `
		UPDATE map_verifications SET status = $2, score = $3, broken = $4, open = $5, error = $6, finished_at = now()
		WHERE id = $1 AND status = 'running'`, id, status, nullJSON(score), orEmpty(broken), orEmpty(open), failure)
	return affected(res, err)
}

// StudiedPages are the pages the latest finished discovery of a site
// studied anonymously (its report's pages).
func (s *Store) StudiedPages(ctx context.Context, app string) ([]string, error) {
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `
		SELECT report->'pages' FROM discoveries WHERE app_id = $1 AND status = 'done' AND report ? 'pages'
		ORDER BY id DESC LIMIT 1`, app).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pages []string
	_ = json.Unmarshal(raw, &pages)
	return pages, nil
}

func nullJSON(j json.RawMessage) any {
	if len(j) == 0 || string(j) == "null" {
		return nil
	}
	return []byte(j)
}

func orEmpty(j json.RawMessage) []byte {
	if len(j) == 0 || string(j) == "null" {
		return []byte("[]")
	}
	return []byte(j)
}
