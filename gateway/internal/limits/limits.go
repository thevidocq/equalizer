package limits

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// AllEntries is the entry id of the map's default limits.
const AllEntries = "*"

// Subject is who a limit counts: an IP, a session, a logged-in user. Values
// are hashed before they become store keys.
type Subject struct {
	IP            string
	Session       string // session hash; "" without a session
	Authenticated bool
}

// id returns the subject for a dimension. Without a session (or without a
// login, for "user") the IP is used, so a limit is never skipped.
func (s Subject) id(per string) string {
	switch per {
	case "session":
		if s.Session != "" {
			return "s:" + s.Session
		}
	case "user":
		if s.Authenticated && s.Session != "" {
			return "u:" + s.Session
		}
	}
	return "ip:" + s.IP
}

// Decision is the outcome of a check: Status 0 means allowed.
type Decision struct {
	Status     int           // 0, 429 or 403
	RetryAfter time.Duration // for 429 and 403
	Reason     string        // per_ip, per_session, per_user, behavior_block, behavior_throttle
	Rule       string        // entry or behaviour rule that decided
}

// Allowed reports whether the request may go on.
func (d Decision) Allowed() bool { return d.Status == 0 }

// Alert is a behaviour rule that crossed its threshold.
type Alert struct {
	Rule   string
	Signal string
	Count  int64
}

// Observation is what a served request tells the behaviour rules.
type Observation struct {
	Params      map[string]string // validated path parameters and query values
	FailedLogin bool
}

type rate struct {
	limit  int64
	window time.Duration
	per    string
	reason string
}

type rule struct {
	mapmodel.BehaviorRule
	window time.Duration
	per    string
	block  bool
	slow   bool // throttle or challenge
	alert  bool
}

// Set holds the compiled limits of one map version. It is immutable.
type Set struct {
	store     Store
	prefix    string
	defaults  []rate
	entries   map[string][]rate
	rules     []rule
	legacyRPS int64
	now       func() time.Time
}

var rateRe = regexp.MustCompile(`^([0-9]+)/(s|sec|min|hour|day)$`)

// ParseRate reads "N/unit".
func ParseRate(v string) (int64, time.Duration, error) {
	m := rateRe.FindStringSubmatch(v)
	if m == nil {
		return 0, 0, fmt.Errorf("invalid rate %q: use N/s, N/min, N/hour or N/day", v)
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	if n < 1 {
		return 0, 0, fmt.Errorf("invalid rate %q: at least 1", v)
	}
	unit := map[string]time.Duration{"s": time.Second, "sec": time.Second, "min": time.Minute, "hour": time.Hour, "day": 24 * time.Hour}[m[2]]
	return n, unit, nil
}

func rates(rl mapmodel.RateLimit) ([]rate, error) {
	var out []rate
	for _, r := range []struct{ v, per string }{{rl.PerIP, "ip"}, {rl.PerSession, "session"}, {rl.PerUser, "user"}} {
		if r.v == "" {
			continue
		}
		n, w, err := ParseRate(r.v)
		if err != nil {
			return nil, err
		}
		out = append(out, rate{limit: n, window: w, per: r.per, reason: "per_" + r.per})
	}
	return out, nil
}

// Compile builds the limits of a map. notes lists the rules the gateway
// accepts but does not apply yet.
func Compile(m *mapmodel.Model, store Store) (*Set, []string, error) {
	s := &Set{store: store, prefix: "eq:" + m.Meta.Tenant + ":" + m.Meta.App, entries: map[string][]rate{}, now: time.Now}
	var err error
	if s.defaults, err = rates(m.Defaults.RateLimit); err != nil {
		return nil, nil, fmt.Errorf("defaults.rate_limit: %w", err)
	}
	for _, r := range m.Routes {
		if s.entries[r.ID], err = rates(r.RateLimit); err != nil {
			return nil, nil, fmt.Errorf("route %q, rate_limit: %w", r.ID, err)
		}
	}
	for _, a := range m.API {
		if s.entries[a.ID], err = rates(a.RateLimit); err != nil {
			return nil, nil, fmt.Errorf("api %q: %w", a.ID, err)
		}
	}
	for _, f := range m.Forms {
		if s.entries[f.ID], err = rates(f.RateLimit); err != nil {
			return nil, nil, fmt.Errorf("form %q, rate_limit: %w", f.ID, err)
		}
	}
	if rps := m.Defaults.LegacyBudget.MaxRPS; rps > 0 {
		s.legacyRPS = int64(math.Ceil(rps))
	}
	var notes []string
	for _, b := range m.Behavior {
		r, note, err := compileRule(b)
		if err != nil {
			return nil, nil, fmt.Errorf("behavior %q: %w", b.ID, err)
		}
		if note != "" {
			notes = append(notes, note)
			continue
		}
		s.rules = append(s.rules, r)
	}
	return s, notes, nil
}

func compileRule(b mapmodel.BehaviorRule) (rule, string, error) {
	r := rule{BehaviorRule: b, per: b.Threshold.Per}
	if r.per == "" {
		r.per = "ip"
	}
	if b.Threshold.Count < 1 {
		return rule{}, "", fmt.Errorf("threshold.count is required")
	}
	w, err := time.ParseDuration(b.Threshold.Window)
	if err != nil || w <= 0 {
		return rule{}, "", fmt.Errorf("invalid threshold.window %q", b.Threshold.Window)
	}
	r.window = w
	switch b.Signal {
	case "distinct_param_values":
		if b.Param == "" {
			return rule{}, "", fmt.Errorf("param is required for distinct_param_values")
		}
	case "request_rate", "failed_logins":
	case "missing_predecessor":
		return rule{}, fmt.Sprintf("behavior %q: signal missing_predecessor not applied yet", b.ID), nil
	default:
		return rule{}, "", fmt.Errorf("unknown signal %q", b.Signal)
	}
	r.block = slices.Contains(b.Action, "block")
	r.slow = slices.Contains(b.Action, "throttle") || slices.Contains(b.Action, "challenge")
	r.alert = slices.Contains(b.Action, "alert")
	return r, "", nil
}

func (r rule) matches(entry string) bool { return r.Route == "" || r.Route == entry }

func hash(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:8])
}

func (s *Set) blockKey(r rule, subj Subject) string {
	return s.prefix + ":bl:" + r.ID + ":" + hash(subj.id(r.per))
}

func (s *Set) slowKey(r rule, subj Subject) string {
	return s.prefix + ":th:" + r.ID + ":" + hash(subj.id(r.per))
}

// Check applies, before serving, the marks left by behaviour rules and the
// rate limits of the entry (and the map defaults when withDefaults).
func (s *Set) Check(ctx context.Context, subj Subject, entry string, withDefaults bool) (Decision, error) {
	var blocks, slows []string
	var blockRules, slowRules []string
	for _, r := range s.rules {
		if r.block {
			blocks, blockRules = append(blocks, s.blockKey(r, subj)), append(blockRules, r.ID)
		}
		if r.slow && r.matches(entry) {
			slows, slowRules = append(slows, s.slowKey(r, subj)), append(slowRules, r.ID)
		}
	}
	if d, err := s.store.Marked(ctx, blocks...); err != nil || d > 0 {
		return Decision{Status: http.StatusForbidden, RetryAfter: d, Reason: "behavior_block", Rule: first(blockRules)}, err
	}
	if d, err := s.store.Marked(ctx, slows...); err != nil || d > 0 {
		return Decision{Status: http.StatusTooManyRequests, RetryAfter: d, Reason: "behavior_throttle", Rule: first(slowRules)}, err
	}

	type scoped struct {
		entry string
		r     rate
	}
	var all []scoped
	if withDefaults {
		for _, r := range s.defaults {
			all = append(all, scoped{AllEntries, r})
		}
	}
	for _, r := range s.entries[entry] {
		all = append(all, scoped{entry, r})
	}
	now := s.now()
	worst := Decision{}
	for _, sc := range all {
		idx := now.UnixNano() / int64(sc.r.window)
		key := fmt.Sprintf("%s:rl:%s:%s:%s:%d", s.prefix, sc.entry, sc.r.per, hash(subj.id(sc.r.per)), idx)
		n, err := s.store.Incr(ctx, key, sc.r.window)
		if err != nil {
			return Decision{}, err
		}
		if n > sc.r.limit {
			retry := time.Duration((idx+1)*int64(sc.r.window) - now.UnixNano())
			if retry > worst.RetryAfter {
				worst = Decision{Status: http.StatusTooManyRequests, RetryAfter: retry, Reason: sc.r.reason, Rule: sc.entry}
			}
		}
	}
	return worst, nil
}

// Observe feeds a served request to the behaviour rules. It returns the
// alerts crossed now and the decision for this very request (a rule that
// blocks applies from the request that crossed the threshold).
func (s *Set) Observe(ctx context.Context, subj Subject, entry string, obs Observation) ([]Alert, Decision, error) {
	var alerts []Alert
	var dec Decision
	now := s.now()
	for _, r := range s.rules {
		if !r.matches(entry) {
			continue
		}
		idx := now.UnixNano() / int64(r.window)
		key := fmt.Sprintf("%s:bh:%s:%s:%d", s.prefix, r.ID, hash(subj.id(r.per)), idx)
		var n int64
		var err error
		switch r.Signal {
		case "distinct_param_values":
			v := obs.Params[r.Param]
			if v == "" {
				continue
			}
			n, err = s.store.AddDistinct(ctx, key, hash(v), r.window)
		case "request_rate":
			n, err = s.store.Incr(ctx, key, r.window)
		case "failed_logins":
			if !obs.FailedLogin {
				continue
			}
			n, err = s.store.Incr(ctx, key, r.window)
		}
		if err != nil {
			return alerts, dec, err
		}
		if n < int64(r.Threshold.Count) {
			continue
		}
		if r.alert && n == int64(r.Threshold.Count) {
			alerts = append(alerts, Alert{Rule: r.ID, Signal: r.Signal, Count: n})
		}
		if r.block {
			if err := s.store.Mark(ctx, s.blockKey(r, subj), r.window); err != nil {
				return alerts, dec, err
			}
			dec = Decision{Status: http.StatusForbidden, RetryAfter: r.window, Reason: "behavior_block", Rule: r.ID}
		} else if r.slow {
			if err := s.store.Mark(ctx, s.slowKey(r, subj), r.window); err != nil {
				return alerts, dec, err
			}
			if dec.Allowed() {
				dec = Decision{Status: http.StatusTooManyRequests, RetryAfter: r.window, Reason: "behavior_throttle", Rule: r.ID}
			}
		}
	}
	return alerts, dec, nil
}

// LegacyAllowed applies defaults.legacy_budget.max_rps across replicas.
func (s *Set) LegacyAllowed(ctx context.Context) (bool, error) {
	if s.legacyRPS == 0 {
		return true, nil
	}
	sec := s.now().Unix()
	n, err := s.store.Incr(ctx, fmt.Sprintf("%s:rps:%d", s.prefix, sec), 2*time.Second)
	if err != nil {
		return true, err
	}
	return n <= s.legacyRPS, nil
}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
