package api

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/thevidocq/equalizer/panel-api/internal/review"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// What the assistant reads of the site as a whole (assistant.SiteSource).

func (src *siteSource) Status(ctx context.Context) (any, error) {
	sites, logsOK, err := src.s.SiteStatuses(ctx, src.a.Tenant)
	if err != nil {
		return nil, errors.New("the status could not be read")
	}
	for _, site := range sites {
		if site.App == src.a.ID {
			return map[string]any{"site": site, "logs_available": logsOK}, nil
		}
	}
	return nil, errors.New("the site has no status")
}

func (src *siteSource) Versions(ctx context.Context) (any, error) {
	vs, err := src.s.Store.Versions(ctx, src.a.ID)
	if err != nil {
		return nil, errors.New("the versions could not be read")
	}
	out := make([]map[string]any, 0, len(vs))
	for _, v := range vs {
		row := map[string]any{"version": v.Version, "status": v.Status, "note": src.m.Text(v.Note),
			"author": src.m.User(v.Author), "created_at": v.CreatedAt, "decided_at": v.DecidedAt, "published_at": v.PublishedAt}
		if v.Approver != nil {
			row["approver"] = src.m.User(*v.Approver)
		}
		out = append(out, row)
	}
	return out, nil
}

// version is the given one, or the published one for 0.
func (src *siteSource) version(ctx context.Context, n int) (store.MapVersion, error) {
	if n > 0 {
		v, err := src.s.Store.Version(ctx, src.a.ID, n)
		if err != nil {
			return v, errors.New("no such version")
		}
		return v, nil
	}
	v, err := src.s.Store.Published(ctx, src.a.ID)
	if err != nil {
		return v, errors.New("no published version")
	}
	return v, nil
}

func (src *siteSource) Verification(ctx context.Context, n int) (any, error) {
	v, err := src.version(ctx, n)
	if err != nil {
		return nil, err
	}
	vf, err := src.s.Store.LatestVerification(ctx, src.a.ID, v.Version)
	if errors.Is(err, store.ErrNotFound) {
		return map[string]any{"version": v.Version, "checked": false}, nil
	}
	if err != nil {
		return nil, errors.New("the check could not be read")
	}
	out := map[string]any{"version": v.Version, "checked": true, "status": vf.Status, "finished_at": vf.FinishedAt,
		"score": vf.Score, "broken": vf.Broken, "still_open": vf.Open, "error": vf.Error,
		"corrected_into_version": vf.CorrectedVersion, "corrections": vf.Corrections, "test_accounts": vf.Accounts,
		"requested": vf.RequestedBy != nil}
	return out, nil
}

func (src *siteSource) Compare(ctx context.Context, from, to int) (any, error) {
	a, err := src.version(ctx, from)
	if err != nil {
		return nil, err
	}
	b, err := src.version(ctx, to)
	if err != nil {
		return nil, err
	}
	r := review.Compare(a.Document, b.Document)
	for i := range r.Changes {
		if r.Changes[i].Section == "legacy" {
			r.Changes[i].Detail = "" // where the legacy application lives stays out, as in get_map
		}
	}
	raw, _ := json.Marshal(r)
	return json.RawMessage(raw), nil
}

// Security is the Security tab of the site, its findings cut to the 20 most
// severe of each target, without their evidence: it may hold cookies,
// tokens or file contents, which do not go to the AI provider, and the
// title, address, input and weakness say what the finding is.
func (src *siteSource) Security(ctx context.Context) (any, error) {
	ov, err := src.s.securityOverview(ctx, src.a)
	if err != nil {
		return nil, errors.New("the security scans could not be read")
	}
	for i := range ov.Targets {
		t := &ov.Targets[i]
		if len(t.Findings) > 20 {
			t.Findings = t.Findings[:20]
		}
		for j := range t.Findings {
			t.Findings[j].Evidence = nil
		}
	}
	return ov, nil
}
