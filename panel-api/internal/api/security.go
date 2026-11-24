package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/cveradar"
	"github.com/thevidocq/equalizer/panel-api/internal/exposure"
	"github.com/thevidocq/equalizer/panel-api/internal/securityscan"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// Security scans: the tenant connects its vulnerability scanner (CVE Radar)
// with its own address and key; a site is scanned through Equalizer (its
// public domain) or, with a confirmation, as the original site. Findings
// are read against the published map.

const scannerTimeout = 60 * time.Second // finding or creating a target, then starting the scan

// securityTargets are the two domains of a site a scan may aim at.
type securityTargets struct {
	m        *securityscan.Map
	domains  map[string]string // target → domain
	reasons  map[string]string // target → why it cannot be scanned
	original string
}

func (s *Server) securityTargets(ctx context.Context, a store.App) (securityTargets, error) {
	t := securityTargets{domains: map[string]string{}, reasons: map[string]string{}}
	if a.Published == nil {
		t.reasons["protected"] = "the site has no published map: Equalizer serves nothing yet"
		t.reasons["original"] = "the site has no published map: its original address is not known yet"
		return t, nil
	}
	v, err := s.Store.Version(ctx, a.ID, *a.Published)
	if err != nil {
		return t, err
	}
	if t.m, err = securityscan.ParseMap(v.Document); err != nil {
		return t, err
	}
	t.domains["protected"] = a.PublicDomain
	var doc struct {
		Legacy struct {
			BaseURL string `json:"base_url"`
			Reach   struct {
				Mode string `json:"mode"`
			} `json:"reach"`
		} `json:"legacy"`
	}
	_ = json.Unmarshal(v.Document, &doc)
	host := ""
	if u, err := url.Parse(doc.Legacy.BaseURL); err == nil {
		host = strings.ToLower(u.Hostname())
	}
	switch {
	case doc.Legacy.Reach.Mode != "direct":
		t.reasons["original"] = "the original site is reached through the connector: it has no public address to scan"
	case host == "" || net.ParseIP(host) != nil || !strings.Contains(host, "."):
		t.reasons["original"] = "the original site has no public name of its own (it is reached by address)"
	case host == strings.ToLower(a.PublicDomain):
		t.reasons["original"] = "the original site has the same name as the protected one: its domain now leads to Equalizer"
	default:
		t.domains["original"] = host
	}
	return t, nil
}

type findingView struct {
	ID          string               `json:"id"`
	Source      string               `json:"source"`
	Title       string               `json:"title"`
	Severity    string               `json:"severity"`
	CVSS        *float64             `json:"cvss"`
	CVE         []string             `json:"cve"`
	CWE         []string             `json:"cwe"`
	URL         *string              `json:"url"`
	HTTPMethod  *string              `json:"http_method"`
	Parameter   *string              `json:"parameter"`
	Evidence    *string              `json:"evidence"`
	FirstSeenAt *time.Time           `json:"first_seen_at"`
	LastSeenAt  *time.Time           `json:"last_seen_at"`
	Map         securityscan.Verdict `json:"map"`
	Fix         *store.SecurityFix   `json:"fix"`
}

type targetView struct {
	Target    string              `json:"target"`
	Domain    *string             `json:"domain"`
	Available bool                `json:"available"`
	Reason    *string             `json:"reason"`
	Scan      *store.SecurityScan `json:"scan"`
	Findings  []findingView       `json:"findings"`
}

func (s *Server) getSecurity(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	ov, err := s.securityOverview(r.Context(), a)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

type securityOverview struct {
	Configured bool         `json:"configured"`
	Targets    []targetView `json:"targets"`
}

// securityOverview is the Security tab: each target, its latest scan, and
// the findings of its latest completed scan read against the published map.
func (s *Server) securityOverview(ctx context.Context, a store.App) (securityOverview, error) {
	ov := securityOverview{Targets: []targetView{}}
	_, err := s.Store.TenantScanner(ctx, a.Tenant)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return ov, err
	}
	ov.Configured = err == nil
	targets, err := s.securityTargets(ctx, a)
	if err != nil {
		return ov, err
	}
	scans, err := s.Store.LatestSecurityScans(ctx, a.ID)
	if err != nil {
		return ov, err
	}
	// A draft replaced or set aside since is no longer the one to open.
	gone := map[int]bool{}
	if vs, err := s.Store.Versions(ctx, a.ID); err == nil {
		for _, v := range vs {
			gone[v.Version] = v.Status == "retired" || v.Status == "rejected"
		}
	}
	for i := range scans {
		if d := scans[i].DraftVersion; d != nil && gone[*d] {
			note := fmt.Sprintf("Version %d, built after this scan, was replaced or set aside: see the Map tab", *d)
			scans[i].DraftVersion, scans[i].DraftNote = nil, &note
		}
	}
	for _, name := range []string{"protected", "original"} {
		v := targetView{Target: name, Findings: []findingView{}}
		if d, ok := targets.domains[name]; ok {
			v.Domain, v.Available = &d, true
		}
		if why, ok := targets.reasons[name]; ok {
			v.Reason = &why
		}
		var done *store.SecurityScan
		for i := range scans {
			sc := &scans[i]
			if sc.Target != name {
				continue
			}
			if v.Scan == nil {
				v.Scan = sc
			}
			if sc.Status == "completed" && done == nil {
				done = sc
			}
		}
		if done != nil {
			findings, err := s.Store.SecurityFindings(ctx, done.ID)
			if err != nil {
				return ov, err
			}
			fixes, err := s.Store.SecurityFixes(ctx, done.ID)
			if err != nil {
				return ov, err
			}
			for _, f := range findings {
				v.Findings = append(v.Findings, findingView{ID: f.RemoteID, Source: f.Source, Title: f.Title,
					Severity: f.Severity, CVSS: f.CVSS, CVE: nonNil(f.CVE), CWE: nonNil(f.CWE), URL: f.URL,
					HTTPMethod: f.HTTPMethod, Parameter: f.Parameter, Evidence: f.Evidence, FirstSeenAt: f.FirstSeenAt,
					LastSeenAt: f.LastSeenAt, Map: targets.m.Judge(deref(f.URL), deref(f.HTTPMethod), deref(f.Parameter)),
					Fix: fixOf(fixes, f.RemoteID, gone)})
			}
		}
		ov.Targets = append(ov.Targets, v)
	}
	return ov, nil
}

func (s *Server) startSecurityScan(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	var in struct {
		Target   string `json:"target"`
		ScanType string `json:"scan_type"`
		Confirm  bool   `json:"confirm"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.ScanType == "" {
		in.ScanType = "standard"
	}
	if in.Target != "protected" && in.Target != "original" || in.ScanType != "quick" && in.ScanType != "standard" && in.ScanType != "deep" {
		fail(w, http.StatusUnprocessableEntity, "invalid", "target: protected or original; scan_type: quick, standard or deep")
		return
	}
	if in.Target == "original" && !in.Confirm {
		fail(w, http.StatusUnprocessableEntity, "confirm_required",
			"the original site is scanned without Equalizer in front: confirm that it is yours or that its owner allowed it in writing")
		return
	}
	client, err := s.ScannerFor(r.Context(), a.Tenant)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusConflict, "no_scanner", "connect a scanner first: Settings → Security scanner")
		return
	}
	if err != nil {
		fail(w, http.StatusConflict, "no_scanner", "the saved key cannot be read any more: enter it again in Settings → Security scanner")
		return
	}
	targets, err := s.securityTargets(r.Context(), a)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	domain, ok := targets.domains[in.Target]
	if !ok {
		fail(w, http.StatusConflict, "no_target", targets.reasons[in.Target])
		return
	}
	if in.Target == "original" && !s.publicName(r.Context(), domain) {
		// An internal name would be handed to a third party, and scanned
		// wherever it resolves on the Internet.
		fail(w, http.StatusConflict, "no_target", "the original site's name does not lead to public addresses only")
		return
	}
	// Reserved before the scanner is asked: one scan per target, even while
	// a quick scan runs (5-20 s) or two people click at once.
	reserved, err := s.Store.ReserveSecurityScan(r.Context(), store.SecurityScan{App: a.ID, Target: in.Target,
		Domain: domain, ScanType: in.ScanType}, u.ID)
	if errors.Is(err, store.ErrConflict) {
		fail(w, http.StatusConflict, "scan_in_progress", "a scan of this target is going on: wait for it to end")
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), scannerTimeout)
	defer cancel()
	target, err := client.EnsureTarget(ctx, domain)
	var scan cveradar.Scan
	if err == nil {
		scan, err = client.StartScan(ctx, target.ID, in.ScanType)
	}
	var se *cveradar.Error
	if refused := errors.As(err, &se) && (se.Status == http.StatusUnauthorized || se.Status == http.StatusForbidden); refused || err == nil {
		_ = s.Store.ScannerAnswered(context.WithoutCancel(r.Context()), a.Tenant, refused)
	}
	if err != nil {
		if derr := s.Store.DropSecurityScan(context.WithoutCancel(r.Context()), reserved); derr != nil {
			s.Log.Error("a scan reservation was not dropped: it expires in 5 minutes", slog.Int64("scan", reserved), slog.String("error", derr.Error()))
		}
		scannerFailure(w, err)
		return
	}
	// A quick scan answers when it is over: it is recorded as going on and
	// followed at once, so its findings are read like any scan's.
	status := scan.Status
	if status != "running" {
		status = "pending"
	}
	sc, err := s.Store.SetSecurityScanRemote(context.WithoutCancel(r.Context()), reserved, target.ID, scan.ID, status)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "security.scan", a.ID, map[string]any{"target": in.Target,
		"domain": domain, "scan_type": in.ScanType, "confirmed_original": in.Target == "original" && in.Confirm})
	if scan.Status == "completed" || scan.Status == "failed" {
		if s.FollowScan != nil {
			go s.FollowScan(context.WithoutCancel(r.Context()), sc)
		}
	}
	writeJSON(w, http.StatusAccepted, sc)
}

// publicName reports whether a name resolves, and only to public addresses.
func (s *Server) publicName(ctx context.Context, name string) bool {
	lookup := s.LookupHost
	if lookup == nil {
		lookup = net.DefaultResolver.LookupHost
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addrs, err := lookup(ctx, name)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, a := range addrs {
		if _, ok := exposure.PublicIP(a); !ok {
			return false
		}
	}
	return true
}

// scannerFailure tells the user what the scanner answered.
func scannerFailure(w http.ResponseWriter, err error) {
	var se *cveradar.Error
	if errors.As(err, &se) {
		switch {
		case se.Status == http.StatusConflict:
			fail(w, http.StatusConflict, "scan_in_progress", "the scanner is already scanning this domain: wait for it to end")
			return
		case se.Status == http.StatusTooManyRequests:
			if se.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(se.RetryAfter.Seconds())))
			}
			fail(w, http.StatusTooManyRequests, "scanner_busy", "the scanner's limits are reached (scans per day or requests per minute): try again later")
			return
		case se.Status == http.StatusUnauthorized || se.Status == http.StatusForbidden:
			fail(w, http.StatusFailedDependency, "scanner_failed", "the scanner refused the key (expired, or without the read, scan and write permissions): "+se.Error())
			return
		}
	}
	fail(w, http.StatusFailedDependency, "scanner_failed", err.Error())
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func fixOf(fixes map[string]store.SecurityFix, id string, gone map[int]bool) *store.SecurityFix {
	if f, ok := fixes[id]; ok && !gone[f.Version] {
		return &f
	}
	return nil
}
