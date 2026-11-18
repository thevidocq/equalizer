package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// MapVersion is one version of a site's map.
type MapVersion struct {
	App         string          `json:"app"`
	Version     int             `json:"version"`
	Status      string          `json:"status"`
	Note        string          `json:"note"`
	AuthorID    int64           `json:"author_id"`
	Author      string          `json:"author"`
	ApproverID  *int64          `json:"approver_id"`
	Approver    *string         `json:"approver"`
	CreatedAt   time.Time       `json:"created_at"`
	DecidedAt   *time.Time      `json:"decided_at"`
	PublishedAt *time.Time      `json:"published_at"`
	Document    json.RawMessage `json:"document,omitempty"`
	Payload     *string         `json:"-"`
	Signature   *string         `json:"-"`
	KeyID       *string         `json:"-"`
}

// CreateDraft stores a new draft with the next version number.
func (s *Store) CreateDraft(ctx context.Context, app string, doc json.RawMessage, note string, author int64) (int, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	// Lock the app row: two drafts at once still get different versions.
	var id string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM apps WHERE id = $1 FOR UPDATE`, app).Scan(&id); err != nil {
		return 0, translate(err)
	}
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(version), 0) + 1 FROM map_versions WHERE app_id = $1`, app).Scan(&version); err != nil {
		return 0, err
	}
	doc, err = setMeta(doc, map[string]any{"version": version, "status": "draft"})
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO map_versions (app_id, version, status, document, note, author_id) VALUES ($1, $2, 'draft', $3, $4, $5)`,
		app, version, []byte(doc), note, author); err != nil {
		return 0, translate(err)
	}
	return version, tx.Commit()
}

const versionColumns = `m.app_id, m.version, m.status, m.note, m.author_id, au.email, m.approver_id, ap.email,
	m.created_at, m.decided_at, m.published_at`

func scanVersion(row interface{ Scan(...any) error }) (MapVersion, error) {
	var v MapVersion
	err := row.Scan(&v.App, &v.Version, &v.Status, &v.Note, &v.AuthorID, &v.Author, &v.ApproverID, &v.Approver,
		&v.CreatedAt, &v.DecidedAt, &v.PublishedAt)
	return v, err
}

// Versions lists the versions of a site, newest first, without documents.
func (s *Store) Versions(ctx context.Context, app string) ([]MapVersion, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+versionColumns+`
		FROM map_versions m JOIN users au ON au.id = m.author_id LEFT JOIN users ap ON ap.id = m.approver_id
		WHERE m.app_id = $1 ORDER BY m.version DESC`, app)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MapVersion{}
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Version returns one version with its document.
func (s *Store) Version(ctx context.Context, app string, version int) (MapVersion, error) {
	var v MapVersion
	var doc []byte
	err := s.DB.QueryRowContext(ctx, `SELECT `+versionColumns+`, m.document, m.payload, m.signature, m.key_id
		FROM map_versions m JOIN users au ON au.id = m.author_id LEFT JOIN users ap ON ap.id = m.approver_id
		WHERE m.app_id = $1 AND m.version = $2`, app, version).
		Scan(&v.App, &v.Version, &v.Status, &v.Note, &v.AuthorID, &v.Author, &v.ApproverID, &v.Approver,
			&v.CreatedAt, &v.DecidedAt, &v.PublishedAt, &doc, &v.Payload, &v.Signature, &v.KeyID)
	if err != nil {
		return MapVersion{}, translate(err)
	}
	v.Document = doc
	return v, nil
}

// Transition moves a version from one of the from states to to. approver is
// recorded for approve and reject.
func (s *Store) Transition(ctx context.Context, app string, version int, from []string, to string, approver *int64) error {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE map_versions SET status = $4,
			approver_id = coalesce($5, approver_id),
			decided_at = CASE WHEN $5::bigint IS NULL THEN decided_at ELSE now() END
		WHERE app_id = $1 AND version = $2 AND status = ANY($3)`,
		app, version, pqArray(from), to, approver)
	if err := affected(res, err); err != nil {
		if err == ErrNotFound {
			if _, gerr := s.Version(ctx, app, version); gerr == nil {
				return ErrState
			}
		}
		return err
	}
	return nil
}

// Publish signs off an approved version: the published one is retired and
// this one becomes published with its signed payload, in one transaction.
func (s *Store) Publish(ctx context.Context, app string, version int, doc json.RawMessage, payload, signature, keyID string) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM map_versions WHERE app_id = $1 AND version = $2 FOR UPDATE`, app, version).Scan(&status); err != nil {
		return translate(err)
	}
	if status != "approved" {
		return ErrState
	}
	if _, err := tx.ExecContext(ctx, `UPDATE map_versions SET status = 'retired' WHERE app_id = $1 AND status = 'published'`, app); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE map_versions SET status = 'published', document = $3, payload = $4, signature = $5, key_id = $6, published_at = now()
		WHERE app_id = $1 AND version = $2`, app, version, []byte(doc), payload, signature, keyID); err != nil {
		return err
	}
	return tx.Commit()
}

// Published returns the published version of a site.
func (s *Store) Published(ctx context.Context, app string) (MapVersion, error) {
	var version int
	if err := s.DB.QueryRowContext(ctx, `SELECT version FROM map_versions WHERE app_id = $1 AND status = 'published'`, app).Scan(&version); err != nil {
		return MapVersion{}, translate(err)
	}
	return s.Version(ctx, app, version)
}

// setMeta overwrites fields of the document's meta object.
func setMeta(doc json.RawMessage, fields map[string]any) (json.RawMessage, error) {
	var m map[string]any
	if err := json.Unmarshal(doc, &m); err != nil {
		return nil, err
	}
	meta, _ := m["meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
	}
	for k, v := range fields {
		meta[k] = v
	}
	m["meta"] = meta
	return json.Marshal(m)
}

// SetMeta is setMeta for the callers that prepare a document.
func SetMeta(doc json.RawMessage, fields map[string]any) (json.RawMessage, error) {
	return setMeta(doc, fields)
}

// SignedVersion is a published version as the gateway needs it.
type SignedVersion struct {
	App       string
	Version   int
	Payload   string
	Signature string
	KeyID     string
}

// AllPublished returns every published, signed version, ordered by site.
func (s *Store) AllPublished(ctx context.Context) ([]SignedVersion, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT app_id, version, payload, signature, key_id FROM map_versions
		WHERE status = 'published' AND payload IS NOT NULL AND signature IS NOT NULL AND key_id IS NOT NULL
		ORDER BY app_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SignedVersion{}
	for rows.Next() {
		var v SignedVersion
		if err := rows.Scan(&v.App, &v.Version, &v.Payload, &v.Signature, &v.KeyID); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Rollback publishes again a version that was published before (retired,
// still signed) and retires the current one. Its approval still holds: the
// signed document is the one approved.
func (s *Store) Rollback(ctx context.Context, app string, version int) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var status string
	var signed bool
	if err := tx.QueryRowContext(ctx, `SELECT status, signature IS NOT NULL FROM map_versions WHERE app_id = $1 AND version = $2 FOR UPDATE`,
		app, version).Scan(&status, &signed); err != nil {
		return translate(err)
	}
	if status != "retired" || !signed {
		return ErrState
	}
	if _, err := tx.ExecContext(ctx, `UPDATE map_versions SET status = 'retired' WHERE app_id = $1 AND status = 'published'`, app); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE map_versions SET status = 'published' WHERE app_id = $1 AND version = $2`, app, version); err != nil {
		return err
	}
	return tx.Commit()
}
