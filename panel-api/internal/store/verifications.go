package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/lib/pq"
)

// Verification is the agent's check of a map version (schema Verification).
type Verification struct {
	ID               int64           `json:"-"`
	App              string          `json:"-"`
	Version          int             `json:"version"`
	Status           string          `json:"status"`
	RequestedBy      *string         `json:"requested_by"`
	CreatedAt        time.Time       `json:"created_at"`
	FinishedAt       *time.Time      `json:"finished_at"`
	Score            json.RawMessage `json:"score"`
	Broken           json.RawMessage `json:"broken"`
	Open             json.RawMessage `json:"open"`
	Error            *string         `json:"error"`
	CorrectedVersion *int            `json:"corrected_version"`
	Corrections      json.RawMessage `json:"corrections"`
	Before           json.RawMessage `json:"before"`
	Accounts         []string        `json:"accounts"`

	AccountsSecret []byte `json:"-"` // only when claimed
	Correct        bool   `json:"-"`
	AuthorID       *int64 `json:"-"` // who authors a corrected draft
}

// VerifyOptions are what a verification is asked to do besides measuring.
type VerifyOptions struct {
	AccountsSecret []byte   // sealed test accounts, erased when it ends
	Accounts       []string // their labels
	Correct        bool     // correct the version into a new draft
	Author         *int64   // the corrected draft's author
}

const verificationColumns = `v.id, v.app_id, v.version, v.status, u.email, v.created_at, v.finished_at,
	v.score, v.broken, v.open, v.error, v.corrected_version, v.corrections, v.before, v.accounts, v.correct, v.author_id`

func scanVerification(row interface{ Scan(...any) error }) (Verification, error) {
	var v Verification
	var score, before []byte
	err := row.Scan(&v.ID, &v.App, &v.Version, &v.Status, &v.RequestedBy, &v.CreatedAt, &v.FinishedAt,
		&score, &v.Broken, &v.Open, &v.Error, &v.CorrectedVersion, &v.Corrections, &before, pq.Array(&v.Accounts),
		&v.Correct, &v.AuthorID)
	v.Score, v.Before = orNull(score), orNull(before)
	if v.Accounts == nil {
		v.Accounts = []string{}
	}
	return v, translate(err)
}

func orNull(b []byte) json.RawMessage {
	if b == nil {
		return json.RawMessage("null")
	}
	return b
}

// QueueVerification asks the agent to verify a version, unless one is
// already waiting or running for it (that one is returned).
func (s *Store) QueueVerification(ctx context.Context, app string, version int, by *int64, o VerifyOptions) (Verification, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT id FROM map_verifications WHERE app_id = $1 AND version = $2 AND status IN ('queued', 'running')
		ORDER BY id DESC LIMIT 1`, app, version).Scan(&id)
	if err == sql.ErrNoRows {
		accounts := o.Accounts
		if accounts == nil {
			accounts = []string{}
		}
		err = s.DB.QueryRowContext(ctx, `
			INSERT INTO map_verifications (app_id, version, requested_by, accounts_secret, accounts, correct, author_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			app, version, by, o.AccountsSecret, pq.Array(accounts), o.Correct, o.Author).Scan(&id)
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

// RunningVerification returns a running verification.
func (s *Store) RunningVerification(ctx context.Context, id int64) (Verification, error) {
	v, err := s.verification(ctx, id)
	if err == nil && v.Status != "running" {
		return Verification{}, ErrNotFound
	}
	return v, err
}

// ClaimVerification hands the oldest queued verification to the agent,
// with its sealed accounts; one running for more than an hour is given up.
func (s *Store) ClaimVerification(ctx context.Context) (Verification, error) {
	if _, err := s.DB.ExecContext(ctx, `
		UPDATE map_verifications SET status = 'failed', error = 'the agent stopped answering', finished_at = now(),
			accounts_secret = NULL
		WHERE status = 'running' AND started_at < now() - interval '1 hour'`); err != nil {
		return Verification{}, err
	}
	var id int64
	var secret []byte
	err := s.DB.QueryRowContext(ctx, `
		UPDATE map_verifications SET status = 'running', started_at = now()
		WHERE id = (SELECT id FROM map_verifications WHERE status = 'queued' ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1)
		RETURNING id, accounts_secret`).Scan(&id, &secret)
	if err != nil {
		return Verification{}, translate(err)
	}
	v, err := s.verification(ctx, id)
	v.AccountsSecret = secret
	return v, err
}

// VerificationResult is what the agent reports.
type VerificationResult struct {
	Score, Broken, Open json.RawMessage
	Corrections, Before json.RawMessage
	CorrectedVersion    *int
	Failure             *string
}

// FinishVerification stores the agent's result and erases the accounts.
func (s *Store) FinishVerification(ctx context.Context, id int64, r VerificationResult) error {
	status := "done"
	if r.Failure != nil {
		status = "failed"
	}
	res, err := s.DB.ExecContext(ctx, `
		UPDATE map_verifications SET status = $2, score = $3, broken = $4, open = $5, error = $6, finished_at = now(),
			corrected_version = $7, corrections = $8, before = $9, accounts_secret = NULL
		WHERE id = $1 AND status = 'running'`, id, status, nullJSON(r.Score), orEmpty(r.Broken), orEmpty(r.Open),
		r.Failure, r.CorrectedVersion, orEmpty(r.Corrections), nullJSON(r.Before))
	return affected(res, err)
}

// RecordVerification stores a verification already done (a corrected
// draft arrives with the agent's last measure of it).
func (s *Store) RecordVerification(ctx context.Context, app string, version int, accounts []string, r VerificationResult) error {
	if accounts == nil {
		accounts = []string{}
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO map_verifications (app_id, version, status, started_at, finished_at, score, broken, open, accounts,
			corrections, before)
		VALUES ($1, $2, 'done', now(), now(), $3, $4, $5, $6, $7, $8)`,
		app, version, nullJSON(r.Score), orEmpty(r.Broken), orEmpty(r.Open), pq.Array(accounts),
		orEmpty(r.Corrections), nullJSON(r.Before))
	return err
}

// RetireDraft retires a draft nobody will review (a correction withdrawn).
func (s *Store) RetireDraft(ctx context.Context, app string, version int) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE map_versions SET status = 'retired'
		WHERE app_id = $1 AND version = $2 AND status = 'draft'`, app, version)
	return err
}

// ErrNotDraft: the version is no longer a draft.
var ErrNotDraft = errors.New("not a draft")

// Supersede replaces a draft by the version the agent corrected it into:
// the draft is retired (unsigned, so it can never be rolled back to) and
// the discovery that proposed it proposes the corrected one. The person
// reviews one draft, the agent's last.
func (s *Store) Supersede(ctx context.Context, app string, from, to int) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM map_versions WHERE app_id = $1 AND version = $2 FOR UPDATE`,
		app, from).Scan(&status); err != nil {
		return err
	}
	if status != "draft" {
		return ErrNotDraft // a person took it on meanwhile: it stays, and so does the discovery's link
	}
	if _, err := tx.ExecContext(ctx, `UPDATE map_versions SET status = 'retired'
		WHERE app_id = $1 AND version = $2`, app, from); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE discoveries SET draft_version = $3
		WHERE app_id = $1 AND draft_version = $2`, app, from, to); err != nil {
		return err
	}
	return tx.Commit()
}

// StudiedPages are the pages the latest finished discovery of a site
// studied anonymously, and those it studied with each test account.
func (s *Store) StudiedPages(ctx context.Context, app string) ([]string, map[string][]string, error) {
	var raw, byAccount []byte
	err := s.DB.QueryRowContext(ctx, `
		SELECT report->'pages', report->'pages_by_account' FROM discoveries
		WHERE app_id = $1 AND status = 'done' AND report ? 'pages'
		ORDER BY id DESC LIMIT 1`, app).Scan(&raw, &byAccount)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var pages []string
	_ = json.Unmarshal(raw, &pages)
	accounts := map[string][]string{}
	if byAccount != nil {
		_ = json.Unmarshal(byAccount, &accounts)
	}
	return pages, accounts, nil
}

// DiscoverySecrets reads a running discovery's sealed test accounts, so a
// verification can keep them after the discovery erases its own.
func (s *Store) DiscoverySecrets(ctx context.Context, id int64) (Claim, error) {
	var c Claim
	err := s.DB.QueryRowContext(ctx, `
		SELECT app_id, account_user, account_secret, accounts_secret FROM discoveries WHERE id = $1`, id).
		Scan(&c.App.ID, &c.AccountUser, &c.AccountSecret, &c.AccountsSecret)
	return c, translate(err)
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
