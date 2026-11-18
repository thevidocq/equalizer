package store

import (
	"context"
	"encoding/json"
)

// AIReview is the AI part of a version's review.
type AIReview struct {
	ID        int64    `json:"-"`
	Status    string   `json:"status"`
	Summary   *string  `json:"summary"`
	Questions []string `json:"questions"`
	Error     *string  `json:"error"`
}

// QueueReview asks for an AI review of a version; ErrConflict while one is
// queued or running.
func (s *Store) QueueReview(ctx context.Context, app string, version int) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO map_reviews (app_id, version) VALUES ($1, $2)`, app, version)
	return translate(err)
}

// LatestReview returns the last AI review of a version; ErrNotFound when none.
func (s *Store) LatestReview(ctx context.Context, app string, version int) (AIReview, error) {
	var r AIReview
	var q []byte
	err := s.DB.QueryRowContext(ctx, `SELECT id, status, summary, questions, error FROM map_reviews
		WHERE app_id = $1 AND version = $2 ORDER BY id DESC LIMIT 1`, app, version).Scan(&r.ID, &r.Status, &r.Summary, &q, &r.Error)
	if err != nil {
		return r, translate(err)
	}
	_ = json.Unmarshal(q, &r.Questions)
	if r.Questions == nil {
		r.Questions = []string{}
	}
	return r, nil
}

// ReviewClaim is a review taken by the agent.
type ReviewClaim struct {
	ID      int64
	App     string
	Version int
}

// ClaimReview marks the oldest queued review running; ErrNotFound when none.
// A review silent for 30 minutes is failed first.
func (s *Store) ClaimReview(ctx context.Context) (ReviewClaim, error) {
	if _, err := s.DB.ExecContext(ctx, `UPDATE map_reviews SET status = 'failed', error = 'the agent stopped answering',
		finished_at = now() WHERE status = 'running' AND started_at < now() - interval '30 minutes'`); err != nil {
		return ReviewClaim{}, err
	}
	var c ReviewClaim
	err := s.DB.QueryRowContext(ctx, `
		UPDATE map_reviews SET status = 'running', started_at = now()
		WHERE id = (SELECT id FROM map_reviews WHERE status = 'queued' ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1)
		RETURNING id, app_id, version`).Scan(&c.ID, &c.App, &c.Version)
	return c, translate(err)
}

// FinishReview stores the AI's summary and questions, or why it failed.
func (s *Store) FinishReview(ctx context.Context, id int64, summary string, questions []string, failure *string) error {
	status := "done"
	if failure != nil {
		status = "failed"
	}
	q, _ := json.Marshal(questions)
	res, err := s.DB.ExecContext(ctx, `UPDATE map_reviews SET status = $2, summary = NULLIF($3, ''), questions = $4,
		error = $5, finished_at = now() WHERE id = $1 AND status = 'running'`, id, status, summary, q, failure)
	return affected(res, err)
}
