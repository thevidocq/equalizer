package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
)

// Scanner is a tenant's vulnerability scanner connection.
type Scanner struct {
	Endpoint   string
	Key        []byte // sealed
	KeyHint    string
	KeyRefused bool       // at the last real call
	CheckedAt  *time.Time // that call
}

// TenantScanner returns the tenant's scanner, ErrNotFound without one.
func (s *Store) TenantScanner(ctx context.Context, tenant string) (Scanner, error) {
	var c Scanner
	err := s.DB.QueryRowContext(ctx, `SELECT endpoint, api_key, key_hint, key_refused, checked_at FROM tenant_scanner
		WHERE tenant_id = $1`, tenant).Scan(&c.Endpoint, &c.Key, &c.KeyHint, &c.KeyRefused, &c.CheckedAt)
	return c, translate(err)
}

// SetTenantScanner saves the scanner; a nil key keeps the saved one.
func (s *Store) SetTenantScanner(ctx context.Context, tenant string, c Scanner) error {
	if c.Key == nil {
		res, err := s.DB.ExecContext(ctx, `UPDATE tenant_scanner SET endpoint = $2, key_refused = false, checked_at = now(),
			updated_at = now() WHERE tenant_id = $1`,
			tenant, c.Endpoint)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO tenant_scanner (tenant_id, endpoint, api_key, key_hint, key_refused, checked_at) VALUES ($1, $2, $3, $4, false, now())
		ON CONFLICT (tenant_id) DO UPDATE SET endpoint = $2, api_key = $3, key_hint = $4, key_refused = false, checked_at = now(),
			updated_at = now()`,
		tenant, c.Endpoint, c.Key, c.KeyHint)
	return translate(err)
}

// ScannerAnswered records whether the scanner took the tenant's key at a
// real call.
func (s *Store) ScannerAnswered(ctx context.Context, tenant string, refused bool) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE tenant_scanner SET key_refused = $2, checked_at = now() WHERE tenant_id = $1`, tenant, refused)
	return err
}

// DeleteTenantScanner forgets the scanner and its key; scans stay.
func (s *Store) DeleteTenantScanner(ctx context.Context, tenant string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM tenant_scanner WHERE tenant_id = $1`, tenant)
	return err
}

// SecurityScan is one scan of a site.
type SecurityScan struct {
	ID            int64      `json:"id"`
	App           string     `json:"-"`
	Tenant        string     `json:"-"`
	Target        string     `json:"target"`
	Domain        string     `json:"domain"`
	ScanType      string     `json:"scan_type"`
	Status        string     `json:"status"`
	RemoteTarget  string     `json:"-"`
	RemoteScan    string     `json:"-"`
	FindingsCount *int       `json:"findings_count"`
	RiskScore     *int       `json:"risk_score"`
	Error         *string    `json:"error"`
	RequestedBy   *string    `json:"requested_by"`
	RequestedByID *int64     `json:"-"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	DraftVersion  *int       `json:"draft_version"`
	DraftNote     *string    `json:"draft_note"`
}

// SecurityFinding is a finding of a scan, as the scanner gave it.
type SecurityFinding struct {
	RemoteID    string
	Source      string
	Title       string
	Severity    string
	CVSS        *float64
	CVE, CWE    []string
	URL         *string
	HTTPMethod  *string
	Parameter   *string
	Evidence    *string
	FirstSeenAt *time.Time
	LastSeenAt  *time.Time
}

const scanColumns = `sc.id, sc.app_id, a.tenant_id, sc.target, sc.domain, sc.scan_type, sc.status, sc.remote_target,
	sc.remote_scan, sc.findings_count, sc.risk_score, sc.error, u.email, sc.requested_by, sc.created_at, sc.completed_at,
	sc.draft_version, sc.draft_note`

func scanScan(row interface{ Scan(...any) error }) (SecurityScan, error) {
	var c SecurityScan
	err := row.Scan(&c.ID, &c.App, &c.Tenant, &c.Target, &c.Domain, &c.ScanType, &c.Status, &c.RemoteTarget,
		&c.RemoteScan, &c.FindingsCount, &c.RiskScore, &c.Error, &c.RequestedBy, &c.RequestedByID, &c.CreatedAt, &c.CompletedAt,
		&c.DraftVersion, &c.DraftNote)
	return c, err
}

// ReserveSecurityScan records a scan about to be started, before the
// scanner is asked: one scan going on per target of a site (ErrConflict),
// even while a quick scan runs. The runner leaves it alone until
// SetSecurityScanRemote; DropSecurityScan undoes it when the scanner refuses.
func (s *Store) ReserveSecurityScan(ctx context.Context, c SecurityScan, by int64) (int64, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO security_scans (app_id, target, domain, scan_type, status, remote_target, remote_scan, requested_by)
		VALUES ($1, $2, $3, $4, 'pending', '', '', $5) RETURNING id`,
		c.App, c.Target, c.Domain, c.ScanType, by).Scan(&id)
	return id, translate(err)
}

// SetSecurityScanRemote records the scanner's ids of a reserved scan.
func (s *Store) SetSecurityScanRemote(ctx context.Context, id int64, target, scan, status string) (SecurityScan, error) {
	if _, err := s.DB.ExecContext(ctx, `UPDATE security_scans SET remote_target = $2, remote_scan = $3, status = $4 WHERE id = $1`,
		id, target, scan, status); err != nil {
		return SecurityScan{}, err
	}
	return s.securityScan(ctx, id)
}

// DropSecurityScan deletes a reserved scan the scanner did not start.
func (s *Store) DropSecurityScan(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM security_scans WHERE id = $1 AND remote_scan = ''`, id)
	return err
}

// SecurityScanByID reads one scan.
func (s *Store) SecurityScanByID(ctx context.Context, id int64) (SecurityScan, error) {
	return s.securityScan(ctx, id)
}

func (s *Store) securityScan(ctx context.Context, id int64) (SecurityScan, error) {
	c, err := scanScan(s.DB.QueryRowContext(ctx, `SELECT `+scanColumns+`
		FROM security_scans sc JOIN apps a ON a.id = sc.app_id LEFT JOIN users u ON u.id = sc.requested_by
		WHERE sc.id = $1`, id))
	return c, translate(err)
}

// LatestSecurityScans returns the newest scan of each target of a site,
// and the newest completed one when it is older.
func (s *Store) LatestSecurityScans(ctx context.Context, app string) ([]SecurityScan, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT `+scanColumns+` FROM security_scans sc JOIN apps a ON a.id = sc.app_id
		LEFT JOIN users u ON u.id = sc.requested_by
		WHERE sc.id IN (
			SELECT max(id) FROM security_scans WHERE app_id = $1 GROUP BY target
			UNION
			SELECT max(id) FROM security_scans WHERE app_id = $1 AND status = 'completed' GROUP BY target)
		ORDER BY sc.id DESC`, app)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SecurityScan
	for rows.Next() {
		c, err := scanScan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ClaimSecurityScans returns the scans going on that were not polled for a
// minute, marking them polled, so that two panel replicas do not poll the
// same scan at once.
func (s *Store) ClaimSecurityScans(ctx context.Context, limit int) ([]SecurityScan, error) {
	rows, err := s.DB.QueryContext(ctx, `
		WITH due AS (
			SELECT id FROM security_scans
			WHERE status IN ('pending', 'running') AND remote_scan <> ''
			AND (polled_at IS NULL OR polled_at < now() - interval '50 seconds')
			ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED)
		UPDATE security_scans sc SET polled_at = now() FROM due WHERE sc.id = due.id RETURNING sc.id`, limit)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := make([]SecurityScan, 0, len(ids))
	for _, id := range ids {
		c, err := s.securityScan(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// SetSecurityScanStatus records the scanner's status of a scan going on.
func (s *Store) SetSecurityScanStatus(ctx context.Context, id int64, status string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE security_scans SET status = $2 WHERE id = $1 AND status IN ('pending', 'running')`,
		id, status)
	return err
}

// FailSecurityScan ends a scan with an error.
func (s *Store) FailSecurityScan(ctx context.Context, id int64, reason string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE security_scans SET status = 'failed', error = $2, completed_at = now()
		WHERE id = $1 AND status IN ('pending', 'running')`, id, reason)
	return err
}

// CompleteSecurityScan ends a scan with its findings, in one transaction.
// note says what to know about a scan that completed (findings cut). done
// is false when the scan had ended already (another replica, a quick scan
// followed twice): then nothing is written.
func (s *Store) CompleteSecurityScan(ctx context.Context, id int64, risk *int, findings []SecurityFinding, note *string) (done bool, err error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE security_scans SET status = 'completed', findings_count = $2, risk_score = $3,
		error = $4, completed_at = now() WHERE id = $1 AND status IN ('pending', 'running')`, id, len(findings), risk, note)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO security_findings (scan_id, remote_id, source, title, severity, cvss,
		cve, cwe, url, http_method, parameter, evidence, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) ON CONFLICT DO NOTHING`)
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	for _, f := range findings {
		if _, err := stmt.ExecContext(ctx, id, f.RemoteID, f.Source, f.Title, f.Severity, f.CVSS, pq.Array(f.CVE),
			pq.Array(f.CWE), f.URL, f.HTTPMethod, f.Parameter, f.Evidence, f.FirstSeenAt, f.LastSeenAt); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// SecurityFindings returns the findings of a scan, the most severe first.
func (s *Store) SecurityFindings(ctx context.Context, scan int64) ([]SecurityFinding, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT remote_id, source, title, severity, cvss, cve, cwe, url, http_method,
		parameter, evidence, first_seen_at, last_seen_at FROM security_findings WHERE scan_id = $1
		ORDER BY array_position(ARRAY['critical','high','medium','low','info'], severity), cvss DESC NULLS LAST, title`, scan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SecurityFinding{}
	for rows.Next() {
		var f SecurityFinding
		var cvss sql.NullFloat64
		if err := rows.Scan(&f.RemoteID, &f.Source, &f.Title, &f.Severity, &cvss, pq.Array(&f.CVE), pq.Array(&f.CWE),
			&f.URL, &f.HTTPMethod, &f.Parameter, &f.Evidence, &f.FirstSeenAt, &f.LastSeenAt); err != nil {
			return nil, err
		}
		if cvss.Valid {
			v := cvss.Float64
			f.CVSS = &v
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// PurgeSecurityScans drops scans older than a year, as the scanner does.
func (s *Store) PurgeSecurityScans(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM security_scans WHERE created_at < now() - interval '365 days'`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
