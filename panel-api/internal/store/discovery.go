package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// LLM is a tenant's AI model; Key is sealed.
type LLM struct {
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	Endpoint *string `json:"endpoint"`
	Key      []byte  `json:"-"`
}

// TenantLLM returns the tenant's model; ErrNotFound when it has none.
func (s *Store) TenantLLM(ctx context.Context, tenant string) (LLM, error) {
	var l LLM
	err := s.DB.QueryRowContext(ctx, `SELECT provider, model, endpoint, api_key FROM tenant_llm WHERE tenant_id = $1`, tenant).
		Scan(&l.Provider, &l.Model, &l.Endpoint, &l.Key)
	return l, translate(err)
}

// SetTenantLLM saves the model; a nil key keeps the saved one.
func (s *Store) SetTenantLLM(ctx context.Context, tenant string, l LLM) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO tenant_llm (tenant_id, provider, model, endpoint, api_key) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id) DO UPDATE SET provider = $2, model = $3, endpoint = $4,
			api_key = COALESCE($5, tenant_llm.api_key), updated_at = now()`,
		tenant, l.Provider, l.Model, l.Endpoint, l.Key)
	return translate(err)
}

// DeleteTenantLLM removes the model and its key.
func (s *Store) DeleteTenantLLM(ctx context.Context, tenant string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM tenant_llm WHERE tenant_id = $1`, tenant)
	return err
}

// Discovery is one exploration of a site.
type Discovery struct {
	ID           int64           `json:"id"`
	App          string          `json:"app"`
	Status       string          `json:"status"`
	Trigger      string          `json:"trigger"`
	UseAI        bool            `json:"use_ai"`
	WithAccount  bool            `json:"with_account"`
	WithSource   bool            `json:"with_source"`
	RequestedBy  *string         `json:"requested_by"`
	CreatedAt    time.Time       `json:"created_at"`
	StartedAt    *time.Time      `json:"started_at"`
	FinishedAt   *time.Time      `json:"finished_at"`
	Progress     json.RawMessage `json:"progress"`
	Report       json.RawMessage `json:"report"`
	DraftVersion *int            `json:"draft_version"`
	Error        *string         `json:"error"`
}

// NewDiscovery is what starting a discovery needs.
type NewDiscovery struct {
	App, Trigger  string
	UseAI         bool
	MaxPages      int
	Reach         json.RawMessage
	AccountUser   *string
	AccountSecret []byte
	// AccountsSecret holds every test account, sealed (JSON list).
	AccountsSecret []byte
	RequestedBy    *int64
	// Source has the site's source reader read the files first; the
	// discovery waits for the read (see ClaimDiscovery).
	Source bool
}

// StartDiscovery queues a discovery; ErrConflict while another is active.
func (s *Store) StartDiscovery(ctx context.Context, n NewDiscovery) (Discovery, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Discovery{}, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO discoveries (app_id, trigger, use_ai, max_pages, reach, account_user, account_secret, accounts_secret, requested_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		n.App, n.Trigger, n.UseAI, n.MaxPages, []byte(n.Reach), n.AccountUser, n.AccountSecret, n.AccountsSecret, n.RequestedBy).Scan(&id)
	if err != nil {
		return Discovery{}, translate(err)
	}
	if n.Source {
		// A read already going serves this discovery too.
		if _, err := tx.ExecContext(ctx, `INSERT INTO source_reads (app_id, status, discovery_id, requested_by)
			VALUES ($1, 'queued', $2, $3) ON CONFLICT (app_id) WHERE status IN ('queued', 'running') DO NOTHING`,
			n.App, id, n.RequestedBy); err != nil {
			return Discovery{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE discoveries SET source_read = (SELECT id FROM source_reads
			WHERE app_id = $1 AND status IN ('queued', 'running')) WHERE id = $2`, n.App, id); err != nil {
			return Discovery{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Discovery{}, translate(err)
	}
	return s.Discovery(ctx, n.App, id)
}

const discoveryColumns = `d.id, d.app_id, d.status, d.trigger, d.use_ai, d.account_user IS NOT NULL, d.source_read IS NOT NULL, u.email,
	d.created_at, d.started_at, d.finished_at, d.progress, d.report, d.draft_version, d.error`

func scanDiscovery(row scanner) (Discovery, error) {
	var d Discovery
	var progress, report []byte
	err := row.Scan(&d.ID, &d.App, &d.Status, &d.Trigger, &d.UseAI, &d.WithAccount, &d.WithSource, &d.RequestedBy,
		&d.CreatedAt, &d.StartedAt, &d.FinishedAt, &progress, &report, &d.DraftVersion, &d.Error)
	d.Progress = progress
	if report != nil {
		d.Report = report
	}
	return d, err
}

// Discoveries lists the last discoveries of a site.
func (s *Store) Discoveries(ctx context.Context, app string) ([]Discovery, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+discoveryColumns+`
		FROM discoveries d LEFT JOIN users u ON u.id = d.requested_by
		WHERE d.app_id = $1 ORDER BY d.id DESC LIMIT 50`, app)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Discovery{}
	for rows.Next() {
		d, err := scanDiscovery(rows)
		if err != nil {
			return nil, err
		}
		d.Report = nil // only when reading one
		out = append(out, d)
	}
	return out, rows.Err()
}

// Discovery returns one discovery of a site.
func (s *Store) Discovery(ctx context.Context, app string, id int64) (Discovery, error) {
	d, err := scanDiscovery(s.DB.QueryRowContext(ctx, `SELECT `+discoveryColumns+`
		FROM discoveries d LEFT JOIN users u ON u.id = d.requested_by WHERE d.app_id = $1 AND d.id = $2`, app, id))
	return d, translate(err)
}

// Claim is a discovery taken by the agent.
type Claim struct {
	ID             int64
	App            App
	UseAI          bool
	MaxPages       int
	Reach          json.RawMessage
	AccountUser    *string
	AccountSecret  []byte
	AccountsSecret []byte
	SourceRead     *int64
}

// ClaimDiscovery marks the oldest queued discovery ready to run running
// and returns it; ErrNotFound when none. A run silent for two hours is
// failed first.
func (s *Store) ClaimDiscovery(ctx context.Context) (Claim, error) {
	if _, err := s.DB.ExecContext(ctx, `
		UPDATE discoveries SET status = 'failed', error = 'the agent stopped answering', finished_at = now(),
			account_secret = NULL, accounts_secret = NULL
		WHERE status = 'running' AND started_at < now() - interval '2 hours'`); err != nil {
		return Claim{}, err
	}
	var c Claim
	// A discovery that asked for the source waits for its read to end,
	// 10 minutes at most; it then runs without the summary.
	err := s.DB.QueryRowContext(ctx, `
		UPDATE discoveries SET status = 'running', started_at = now()
		WHERE id = (SELECT d.id FROM discoveries d LEFT JOIN source_reads r ON r.id = d.source_read
			WHERE d.status = 'queued' AND (r.id IS NULL OR r.status IN ('done', 'failed') OR d.created_at < now() - interval '10 minutes')
			ORDER BY d.id FOR UPDATE OF d SKIP LOCKED LIMIT 1)
		RETURNING id, app_id, use_ai, max_pages, reach, account_user, account_secret, accounts_secret, source_read`).
		Scan(&c.ID, &c.App.ID, &c.UseAI, &c.MaxPages, &c.Reach, &c.AccountUser, &c.AccountSecret, &c.AccountsSecret, &c.SourceRead)
	if err != nil {
		return Claim{}, translate(err)
	}
	c.App, err = s.App(ctx, c.App.ID)
	return c, err
}

// DiscoveryProgress stores what the agent has done so far.
func (s *Store) DiscoveryProgress(ctx context.Context, id int64, progress json.RawMessage) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE discoveries SET progress = $2 WHERE id = $1 AND status = 'running'`, id, []byte(progress))
	return affected(res, err)
}

// RunningDiscovery returns the site and requester of a running discovery.
func (s *Store) RunningDiscovery(ctx context.Context, id int64) (app string, requestedBy *int64, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT app_id, requested_by FROM discoveries WHERE id = $1 AND status = 'running'`, id).
		Scan(&app, &requestedBy)
	return app, requestedBy, translate(err)
}

// FinishDiscovery closes a discovery and erases its test account secret.
func (s *Store) FinishDiscovery(ctx context.Context, id int64, report json.RawMessage, draft *int, failure *string) error {
	status := "done"
	if failure != nil {
		status = "failed"
	}
	var rep any
	if report != nil {
		rep = []byte(report)
	}
	res, err := s.DB.ExecContext(ctx, `
		UPDATE discoveries SET status = $2, report = $3, draft_version = $4, error = $5, finished_at = now(),
			account_secret = NULL, accounts_secret = NULL
		WHERE id = $1 AND status = 'running'`, id, status, rep, draft, failure)
	return affected(res, err)
}

// PointDiscovery makes a discovery propose version to, if it still proposes
// from (nil: no draft); ErrNotDraft otherwise (someone moved it meanwhile).
func (s *Store) PointDiscovery(ctx context.Context, app string, id int64, from *int, to int) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE discoveries SET draft_version = $4
		WHERE id = $1 AND app_id = $2 AND draft_version IS NOT DISTINCT FROM $3`, id, app, from, to)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return ErrNotDraft
	}
	return nil
}

// SetDiscoverySchedule turns continuous discovery on or off and sets when
// it runs; the owner is the administrator whose rights the scheduled runs use.
func (s *Store) SetDiscoverySchedule(ctx context.Context, app, schedule string, at DiscoveryTime, owner int64) error {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE apps SET discovery_schedule = $2, discovery_owner = $3, discovery_hour = $4,
			discovery_weekday = $5, discovery_zone = $6, discovery_set_at = now() WHERE id = $1`,
		app, schedule, owner, at.Hour, at.Weekday, at.Zone)
	return affected(res, err)
}

// Due is a site whose continuous discovery should run now.
type Due struct {
	App   string
	Owner int64
}

// DueDiscoveries lists the sites with a published map, none running, and
// whose scheduled hour passed since their latest discovery.
func (s *Store) DueDiscoveries(ctx context.Context) ([]Due, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT a.id, a.discovery_owner, a.discovery_schedule, a.discovery_hour, a.discovery_weekday,
			a.discovery_zone, a.discovery_set_at, (SELECT max(d.created_at) FROM discoveries d WHERE d.app_id = a.id)
		FROM apps a
		WHERE a.discovery_schedule <> 'off' AND a.discovery_owner IS NOT NULL
		AND EXISTS (SELECT 1 FROM map_versions m WHERE m.app_id = a.id AND m.status = 'published')
		AND NOT EXISTS (SELECT 1 FROM discoveries d WHERE d.app_id = a.id AND d.status IN ('queued', 'running'))`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	var out []Due
	for rows.Next() {
		var d Due
		var schedule string
		var t DiscoveryTime
		if err := rows.Scan(&d.App, &d.Owner, &schedule, &t.Hour, &t.Weekday, &t.Zone, &t.setAt, &t.Last); err != nil {
			return nil, err
		}
		if t.due(schedule, now) {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}

// Unmapped is a refused path, counted.
type Unmapped struct {
	App     string `json:"app,omitempty"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Hits    int64  `json:"hits"`
	Clients int64  `json:"clients"`
	// Visitors are the gateway's visitor keys (never addresses); in
	// UnmappedOf, Clients counts them when the gateway sent any.
	Visitors []string `json:"client_keys,omitempty"`
}

// maxVisitors is how many visitor keys a path keeps: enough to tell one
// visitor from several, never a census.
const maxVisitors = 16

// AddUnmapped adds the gateway's counts; unknown sites are skipped.
func (s *Store) AddUnmapped(ctx context.Context, items []Unmapped) error {
	for _, u := range items {
		_, err := s.DB.ExecContext(ctx, `
			INSERT INTO unmapped_hits (app_id, method, path, hits, clients, visitors)
			SELECT $1, $2, $3, $4, $5, coalesce($6::text[], '{}') WHERE EXISTS (SELECT 1 FROM apps WHERE id = $1)
			ON CONFLICT (app_id, method, path) DO UPDATE SET hits = unmapped_hits.hits + $4,
				clients = unmapped_hits.clients + $5, last_seen = now(),
				visitors = (SELECT coalesce(array_agg(v), '{}') FROM (SELECT DISTINCT v FROM
					unnest(unmapped_hits.visitors || coalesce($6::text[], '{}')) v ORDER BY v LIMIT $7) x)`,
			u.App, u.Method, u.Path, u.Hits, u.Clients, pq.Array(u.Visitors), maxVisitors)
		if err != nil {
			return err
		}
	}
	return nil
}

// UnmappedOf returns the most requested refused paths of a site.
func (s *Store) UnmappedOf(ctx context.Context, app string, limit int) ([]Unmapped, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT method, path, hits, c FROM (
			SELECT method, path, hits, CASE WHEN cardinality(visitors) > 0 THEN cardinality(visitors) ELSE clients END c
			FROM unmapped_hits WHERE app_id = $1 AND last_seen > now() - interval '30 days') u
		ORDER BY c DESC, hits DESC, path LIMIT $2`, app, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Unmapped{}
	for rows.Next() {
		var u Unmapped
		if err := rows.Scan(&u.Method, &u.Path, &u.Hits, &u.Clients); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
