package store

import "context"

// The map draft built from a scan, and which finding each change hardens.

// ClaimSecurityDraft marks a completed scan's draft as being built, unless
// it is built already or another replica is on it (for 5 minutes).
func (s *Store) ClaimSecurityDraft(ctx context.Context, scan int64) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `UPDATE security_scans SET draft_claimed_at = now()
		WHERE id = $1 AND status = 'completed' AND draft_note IS NULL
		AND (draft_claimed_at IS NULL OR draft_claimed_at < now() - interval '5 minutes')`, scan)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// PendingSecurityDrafts are completed scans of the last day whose draft is
// not built yet (the map was busy, or a replica stopped): tried again.
func (s *Store) PendingSecurityDrafts(ctx context.Context, limit int) ([]int64, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM security_scans
		WHERE status = 'completed' AND draft_note IS NULL AND completed_at > now() - interval '1 day'
		AND (draft_claimed_at IS NULL OR draft_claimed_at < now() - interval '5 minutes')
		ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SecurityFix is the draft change that closes a finding.
type SecurityFix struct {
	Version int    `json:"version"`
	Change  string `json:"change"`
}

// SetSecurityDraft records the draft built from a scan (nil: none) and why.
func (s *Store) SetSecurityDraft(ctx context.Context, scan int64, version *int, note string, fixes map[string]string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE security_scans SET draft_version = $2, draft_note = $3 WHERE id = $1`,
		scan, version, note); err != nil {
		return err
	}
	for id, change := range fixes {
		if version == nil {
			break
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO security_fixes (scan_id, remote_id, version, change) VALUES ($1, $2, $3, $4)
			ON CONFLICT (scan_id, remote_id) DO UPDATE SET version = $3, change = $4`, scan, id, *version, change); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SecurityFixes returns the draft changes of a scan, by finding.
func (s *Store) SecurityFixes(ctx context.Context, scan int64) (map[string]SecurityFix, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT remote_id, version, change FROM security_fixes WHERE scan_id = $1`, scan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SecurityFix{}
	for rows.Next() {
		var id string
		var f SecurityFix
		if err := rows.Scan(&id, &f.Version, &f.Change); err != nil {
			return nil, err
		}
		out[id] = f
	}
	return out, rows.Err()
}
