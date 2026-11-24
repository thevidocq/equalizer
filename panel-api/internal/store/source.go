package store

import (
	"context"
	"encoding/json"
	"time"
)

// SourceReader is how a site's source reader reaches its files.
type SourceReader struct {
	App       string
	Method    *string
	Host      *string
	Port      *int
	Username  *string
	Path      *string
	Share     *string
	Container *string // local: the site's Docker container
	Password  []byte  // sealed
	HasToken  bool
	PublicKey *string
	Version   *string
	LastSeen  *time.Time
}

// SourceRead is one read of a site's files.
type SourceRead struct {
	ID         int64           `json:"id"`
	App        string          `json:"-"`
	Status     string          `json:"status"`
	Error      *string         `json:"error"`
	Summary    json.RawMessage `json:"-"`
	Discovery  *int64          `json:"discovery"`
	CreatedAt  time.Time       `json:"created_at"`
	FinishedAt *time.Time      `json:"finished_at"`
}

// SourceReaderOf returns a site's source reader, ErrNotFound without one.
func (s *Store) SourceReaderOf(ctx context.Context, app string) (SourceReader, error) {
	c := SourceReader{App: app}
	err := s.DB.QueryRowContext(ctx, `SELECT method, host, port, username, path, share, container, password, token_hash IS NOT NULL,
		public_key, version, last_seen FROM source_readers WHERE app_id = $1`, app).
		Scan(&c.Method, &c.Host, &c.Port, &c.Username, &c.Path, &c.Share, &c.Container, &c.Password, &c.HasToken, &c.PublicKey,
			&c.Version, &c.LastSeen)
	return c, translate(err)
}

// SetSourceReader saves how the reader reaches the files; a nil password
// keeps the saved one.
func (s *Store) SetSourceReader(ctx context.Context, c SourceReader) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO source_readers (app_id, method, host, port, username, path, share, password, container)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (app_id) DO UPDATE SET method = $2, host = $3, port = $4, username = $5, path = $6, share = $7,
			password = COALESCE($8, source_readers.password), container = $9, updated_at = now()`,
		c.App, c.Method, c.Host, c.Port, c.Username, c.Path, c.Share, c.Password, c.Container)
	return translate(err)
}

// SetSourceToken stores the hash of a new token: the previous one stops.
func (s *Store) SetSourceToken(ctx context.Context, app, hash string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO source_readers (app_id, token_hash) VALUES ($1, $2)
		ON CONFLICT (app_id) DO UPDATE SET token_hash = $2, updated_at = now()`, app, hash)
	if err != nil {
		return translate(err)
	}
	// the reader of the old token can no longer take or finish a read
	_, err = s.DB.ExecContext(ctx, `UPDATE source_reads SET status = 'failed', finished_at = now(),
		error = 'a new token replaced the reader before the read ended' WHERE app_id = $1 AND status IN ('queued', 'running')`, app)
	return err
}

// expireSourceReads fails the site's reads no reader will end: queued for
// 15 minutes (no reader took it) or running for 30 (the reader stopped).
// Without it a dead read would hold the site's one read slot for good.
func (s *Store) expireSourceReads(ctx context.Context, app string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE source_reads SET status = 'failed', finished_at = now(),
		error = CASE status WHEN 'queued' THEN 'no source reader took the read within 15 minutes: is it running?'
			ELSE 'the source reader stopped answering' END
		WHERE app_id = $1 AND (status = 'queued' AND created_at < now() - interval '15 minutes'
			OR status = 'running' AND started_at < now() - interval '30 minutes')`, app)
	return err
}

// DeleteSourceReader forgets the reader, its token and its reads.
func (s *Store) DeleteSourceReader(ctx context.Context, app string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM source_reads WHERE app_id = $1`, app); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM source_readers WHERE app_id = $1`, app); err != nil {
		return err
	}
	return tx.Commit()
}

// SourceApp returns the site of a reader's token hash.
func (s *Store) SourceApp(ctx context.Context, hash string) (string, error) {
	var app string
	err := s.DB.QueryRowContext(ctx, `SELECT app_id FROM source_readers WHERE token_hash = $1`, hash).Scan(&app)
	return app, translate(err)
}

// SourceHello records that the reader runs.
func (s *Store) SourceHello(ctx context.Context, app, version, publicKey string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE source_readers SET version = $2, public_key = NULLIF($3, ''), last_seen = now()
		WHERE app_id = $1`, app, version, publicKey)
	return err
}

// QueueSourceRead queues a read; ErrConflict when one goes on.
func (s *Store) QueueSourceRead(ctx context.Context, app string, discovery *int64, by *int64) (SourceRead, error) {
	if err := s.expireSourceReads(ctx, app); err != nil {
		return SourceRead{}, err
	}
	var id int64
	err := s.DB.QueryRowContext(ctx, `INSERT INTO source_reads (app_id, status, discovery_id, requested_by)
		VALUES ($1, 'queued', $2, $3) RETURNING id`, app, discovery, by).Scan(&id)
	if err != nil {
		return SourceRead{}, translate(err)
	}
	return s.SourceReadByID(ctx, id)
}

const readColumns = `id, app_id, status, error, summary, discovery_id, created_at, finished_at`

func scanRead(row interface{ Scan(...any) error }) (SourceRead, error) {
	var r SourceRead
	var summary []byte
	err := row.Scan(&r.ID, &r.App, &r.Status, &r.Error, &summary, &r.Discovery, &r.CreatedAt, &r.FinishedAt)
	r.Summary = summary
	return r, err
}

// SourceReadByID reads one read.
func (s *Store) SourceReadByID(ctx context.Context, id int64) (SourceRead, error) {
	r, err := scanRead(s.DB.QueryRowContext(ctx, `SELECT `+readColumns+` FROM source_reads WHERE id = $1`, id))
	return r, translate(err)
}

// LatestSourceRead returns a site's newest read, ErrNotFound without one.
func (s *Store) LatestSourceRead(ctx context.Context, app string) (SourceRead, error) {
	if err := s.expireSourceReads(ctx, app); err != nil {
		return SourceRead{}, err
	}
	r, err := scanRead(s.DB.QueryRowContext(ctx, `SELECT `+readColumns+` FROM source_reads WHERE app_id = $1
		ORDER BY id DESC LIMIT 1`, app))
	return r, translate(err)
}

// ClaimSourceRead takes the site's queued read for its reader; reads no
// reader will end fail first.
func (s *Store) ClaimSourceRead(ctx context.Context, app string) (SourceRead, error) {
	if err := s.expireSourceReads(ctx, app); err != nil {
		return SourceRead{}, err
	}
	r, err := scanRead(s.DB.QueryRowContext(ctx, `
		UPDATE source_reads SET status = 'running', started_at = now()
		WHERE id = (SELECT id FROM source_reads WHERE app_id = $1 AND status = 'queued' ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1)
		RETURNING `+readColumns, app))
	return r, translate(err)
}

// FinishSourceRead records a running read's summary or error.
func (s *Store) FinishSourceRead(ctx context.Context, app string, id int64, summary json.RawMessage, failure *string) error {
	status := "done"
	if failure != nil {
		status, summary = "failed", nil
	}
	var doc any
	if summary != nil {
		doc = []byte(summary)
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE source_reads SET status = $3, summary = $4, error = $5, finished_at = now()
		WHERE id = $1 AND app_id = $2 AND status = 'running'`, id, app, status, doc, failure)
	return affected(res, err)
}

// PurgeSourceReads keeps the last 20 reads of each site.
func (s *Store) PurgeSourceReads(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM source_reads r WHERE r.status IN ('done', 'failed') AND r.id NOT IN (
		SELECT id FROM source_reads x WHERE x.app_id = r.app_id ORDER BY id DESC LIMIT 20)`)
	return err
}
