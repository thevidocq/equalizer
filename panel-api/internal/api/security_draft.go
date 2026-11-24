package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/mapcheck"
	"github.com/thevidocq/equalizer/panel-api/internal/securityscan"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// After a scan, the panel builds one map draft with what closes the
// findings the map serves as free text, on the latest draft or the
// published map, as the assistant's drafts: one draft to review, in the Map
// tab, checked by the agent like any other.

// HardenAfterScan builds the draft of an ended scan and records it on the
// scan (its version and what it closes, or why there is none).
func (s *Server) HardenAfterScan(ctx context.Context, id int64) {
	sc, err := s.Store.SecurityScanByID(ctx, id)
	if err != nil || sc.Status != "completed" {
		return
	}
	version, note, fixes := s.hardenScan(ctx, sc)
	if err := s.Store.SetSecurityDraft(ctx, sc.ID, version, note, fixes); err != nil {
		s.Log.Error("security draft not recorded", slog.Int64("scan", sc.ID), slog.String("error", err.Error()))
	}
}

func (s *Server) hardenScan(ctx context.Context, sc store.SecurityScan) (*int, string, map[string]string) {
	findings, err := s.Store.SecurityFindings(ctx, sc.ID)
	if err != nil {
		return nil, "the findings could not be read", nil
	}
	author := s.scanAuthor(ctx, sc)
	if author == nil {
		return nil, "no draft: the person who started the scan is no longer an administrator of the site", nil
	}
	a, err := s.Store.App(ctx, sc.App)
	if err != nil {
		return nil, "no draft: the site could not be read", nil
	}
	var version *int
	var note string
	var fixes map[string]string
	err = s.Store.WithAppLock(ctx, a.ID, func() error {
		version, note, fixes = s.buildSecurityDraft(ctx, a, author, sc, findings)
		return nil
	})
	if err != nil {
		return nil, "no draft: the site's map could not be locked", nil
	}
	return version, note, fixes
}

// scanAuthor is the person who started the scan, while still an
// administrator of the site's tenant: the draft is theirs.
func (s *Server) scanAuthor(ctx context.Context, sc store.SecurityScan) *auth.User {
	if sc.RequestedByID == nil {
		return nil
	}
	users, err := s.Store.Users(ctx, sc.Tenant)
	if err != nil {
		return nil
	}
	for i := range users {
		if users[i].ID == *sc.RequestedByID && users[i].TenantRole != nil && *users[i].TenantRole == "admin" {
			return &users[i]
		}
	}
	return nil
}

func (s *Server) buildSecurityDraft(ctx context.Context, a store.App, u *auth.User, sc store.SecurityScan,
	findings []store.SecurityFinding) (*int, string, map[string]string) {
	base, fromDraft, err := s.proposalBase(ctx, a.ID)
	if err != nil {
		return nil, "no draft: " + err.Error(), nil
	}
	var doc map[string]any
	if err := json.Unmarshal(base.Document, &doc); err != nil {
		return nil, "no draft: the map could not be read", nil
	}
	m, err := securityscan.ParseMap(base.Document)
	if err != nil {
		return nil, "no draft: the map could not be read", nil
	}
	found := securityscan.Harden(doc, m, findings)
	if len(found) == 0 {
		return nil, "nothing the map can close by itself: the open findings need a precise entry (a discovery) or a fix on the server", nil
	}
	out, err := s.draftDoc(a, u, doc)
	if err != nil {
		return nil, "no draft: the map would not be valid: " + err.Error(), nil
	}
	if err := s.mapChecked(ctx, out, false); err != nil {
		var inc *mapcheck.Inconsistent
		if errors.As(err, &inc) {
			return nil, "no draft: the gateway would refuse it: " + inc.Reason, nil
		}
		return nil, "no draft: no gateway could check the map now; scan again later", nil
	}
	changes := make([]string, len(found))
	for i, f := range found {
		changes[i] = f.Change
	}
	what := fmt.Sprintf("closes %d finding(s): %s", len(found), strings.Join(changes, "; "))
	if r := []rune(what); len(r) > 900 {
		what = string(r[:900]) + "…"
	}
	note := fmt.Sprintf("From version %d, after the security scan of %s (%s): %s", base.Version,
		sc.Domain, sc.CreatedAt.UTC().Format("2 Jan 2006"), what)
	version, err := s.Store.CreateDraft(ctx, a.ID, out, note, u.ID)
	if err != nil {
		return nil, "no draft: it could not be saved", nil
	}
	if fromDraft {
		if err := s.Store.Supersede(ctx, a.ID, base.Version, version); err != nil {
			_ = s.Store.RetireDraft(ctx, a.ID, version)
			return nil, fmt.Sprintf("no draft: version %d was taken on meanwhile; scan again", base.Version), nil
		}
	}
	_, _ = s.Store.QueueVerification(ctx, a.ID, version, &u.ID, store.VerifyOptions{Author: &u.ID})
	tenant := a.Tenant
	_ = s.Store.Record(ctx, nil, &tenant, "security.draft", a.ID,
		map[string]any{"version": version, "from": base.Version, "closes": len(found), "target": sc.Target})
	fixes := map[string]string{}
	for _, f := range found {
		fixes[f.Finding] = f.Change
	}
	return &version, fmt.Sprintf("Draft version %d %s", version, what), fixes
}
