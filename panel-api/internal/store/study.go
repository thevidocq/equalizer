package store

import (
	"context"
	"encoding/json"

	"github.com/lib/pq"
)

// The security study of a discovery: written by the agent after the check
// of the discovery's draft, on the map as the check left it; at once when
// the discovery proposed no draft (on the published map).

// StudyClaim is a study the agent takes.
type StudyClaim struct {
	Discovery int64
	App       string
	Report    json.RawMessage
	Version   *int // the discovery's draft, nil: the published map
}

// StudyAfter is the study status a discovery ends with: waiting for the
// check of its draft, or queued when there is no draft to check.
func StudyAfter(draft *int) *string {
	st := "queued"
	if draft != nil {
		st = "waiting"
	}
	return &st
}

// ReleaseStudy queues the study of the discovery whose draft was version
// (or the corrected version that replaced it), once its check is over.
func (s *Store) ReleaseStudy(ctx context.Context, app string, versions ...int) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE discoveries SET study_status = 'queued'
		WHERE app_id = $1 AND study_status = 'waiting' AND draft_version = ANY($2)`, app, pq.Array(versions))
	return err
}

// ClaimStudy takes the next study. A study whose check never ended (the
// check failed before the agent could answer) is taken after 3 hours; one
// running for an hour has failed.
func (s *Store) ClaimStudy(ctx context.Context) (StudyClaim, error) {
	if _, err := s.DB.ExecContext(ctx, `UPDATE discoveries SET study_status = 'failed',
		report = jsonb_set(COALESCE(report, '{}'), '{security}', '{"used": false, "error": "the agent stopped answering"}')
		WHERE study_status = 'running' AND study_started_at < now() - interval '1 hour'`); err != nil {
		return StudyClaim{}, err
	}
	var c StudyClaim
	err := s.DB.QueryRowContext(ctx, `
		UPDATE discoveries SET study_status = 'running', study_started_at = now()
		WHERE id = (SELECT id FROM discoveries
			WHERE study_status = 'queued' OR study_status = 'waiting' AND finished_at < now() - interval '3 hours'
			ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1)
		RETURNING id, app_id, report, draft_version`).Scan(&c.Discovery, &c.App, &c.Report, &c.Version)
	return c, translate(err)
}

// FinishStudy writes the study into the discovery's report: done when it
// was written or skipped on purpose (no AI model), failed otherwise.
func (s *Store) FinishStudy(ctx context.Context, discovery int64, security json.RawMessage, ok bool) error {
	status := "done"
	if !ok {
		status = "failed"
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE discoveries SET study_status = $3,
		report = jsonb_set(COALESCE(report, '{}'), '{security}', $2::jsonb)
		WHERE id = $1 AND study_status = 'running'`, discovery, []byte(security), status)
	return affected(res, err)
}
