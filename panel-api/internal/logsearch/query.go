// Package logsearch reads the security events of one tenant from Loki. The
// tenant always comes from the signed-in user and every value from the
// client is checked and quoted before it enters a LogQL query.
package logsearch

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Types are the event types of schema/events/event.schema.json.
var Types = []string{
	"request.served", "request.blocked", "field.rejected", "ratelimit.hit", "behavior.alert",
	"parse.failed", "legacy.error", "connector.down", "exposure.alert",
}

// Periods the panel offers, with the interval of the chart.
var periods = map[string]struct{ span, step time.Duration }{
	"1h":  {time.Hour, 5 * time.Minute},
	"24h": {24 * time.Hour, time.Hour},
	"7d":  {7 * 24 * time.Hour, 6 * time.Hour},
	"30d": {30 * 24 * time.Hour, 24 * time.Hour},
}

var appRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ErrFilter is a filter the client should not have sent.
var ErrFilter = errors.New("invalid filter")

// Filter selects events. Tenant is set by the server, never by the client.
type Filter struct {
	Tenant string
	App    string
	Types  []string
	Search string
	Since  string // 1h, 24h, 7d, 30d
	// From and To narrow the period to a window (a bar of the chart);
	// zero: the whole period.
	From, To time.Time
}

// Check validates the client's part of the filter.
func (f *Filter) Check() error {
	if f.Since == "" {
		f.Since = "24h"
	}
	if _, ok := periods[f.Since]; !ok {
		return ErrFilter
	}
	if !appRe.MatchString(f.Tenant) || (f.App != "" && !appRe.MatchString(f.App)) {
		return ErrFilter
	}
	for _, t := range f.Types {
		if !slices.Contains(Types, t) {
			return ErrFilter
		}
	}
	if !f.From.IsZero() && !f.To.IsZero() && !f.To.After(f.From) {
		return ErrFilter
	}
	if len(f.Search) > 200 || strings.ContainsAny(f.Search, "\x00\n\r") {
		return ErrFilter
	}
	return nil
}

// Span returns the period and the chart interval.
func (f Filter) Span() (time.Duration, time.Duration) {
	p := periods[f.Since]
	return p.span, p.step
}

// selector is the log query of the filter, without the type filter.
func (f Filter) selector() string {
	var b strings.Builder
	b.WriteString(`{stream="security"}`)
	if f.Search != "" {
		b.WriteString(` |= ` + quote(f.Search))
	}
	b.WriteString(` | json | tenant=` + quote(f.Tenant))
	if f.App != "" {
		b.WriteString(` | app=` + quote(f.App))
	}
	return b.String()
}

// Logs is the query of the event list.
func (f Filter) Logs() string {
	q := f.selector()
	if len(f.Types) > 0 {
		parts := make([]string, len(f.Types))
		for i, t := range f.Types {
			parts[i] = regexp.QuoteMeta(t)
		}
		q += ` | type=~` + quote("^("+strings.Join(parts, "|")+")$")
	}
	return q
}

// Counts is the instant query of events per type over the period.
func (f Filter) Counts() string {
	return `sum by (type) (count_over_time(` + f.selector() + ` [` + f.Since + `]))`
}

// PerType is the range query of events per type per interval; the chart's
// refused and served series are sums of it.
func (f Filter) PerType() string {
	_, step := f.Span()
	return `sum by (type) (count_over_time(` + f.selector() + ` [` + strconv.Itoa(int(step.Seconds())) + `s]))`
}

// quote writes a LogQL double-quoted string.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
