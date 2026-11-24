package logsearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// ErrUnavailable means Loki could not answer.
var ErrUnavailable = errors.New("log store unavailable")

// Event is what the panel shows of a security event: never field values.
type Event struct {
	TS         time.Time `json:"ts"`
	Type       string    `json:"type"`
	Reason     *string   `json:"reason"`
	App        *string   `json:"app"`
	Route      *string   `json:"route"`
	MapVersion *int      `json:"map_version"`
	RequestID  *string   `json:"request_id"`
	SrcIP      *string   `json:"src_ip"`
	Method     *string   `json:"method"`
	Path       *string   `json:"path"`
	Status     *int      `json:"status"`
	Field      *string   `json:"field"`
	Connector  *string   `json:"connector"`
}

// Bucket is one interval of the refused chart.
type Bucket struct {
	TS    time.Time `json:"ts"`
	Count int       `json:"count"`
}

// Loki queries a Loki server over its HTTP API.
type Loki struct {
	URL    string // e.g. http://loki:3100
	Client *http.Client
	Now    func() time.Time
}

func (l *Loki) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

func (l *Loki) get(ctx context.Context, path string, q url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.URL+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	c := l.Client
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	return json.Unmarshal(body, out)
}

// Events returns at most limit events older than before (Unix ns; 0: now),
// newest first, and the cursor of the next page ("" at the end).
func (l *Loki) Events(ctx context.Context, f Filter, before int64, limit int) ([]Event, string, error) {
	span, _ := f.Span()
	end := l.now()
	if before > 0 {
		end = time.Unix(0, before)
	}
	start := l.now().Add(-span)
	if f.From.After(start) {
		start = f.From
	}
	if !f.To.IsZero() && f.To.Before(end) {
		end = f.To
	}
	if !end.After(start) {
		return []Event{}, "", nil
	}
	q := url.Values{
		"query": {f.Logs()}, "direction": {"backward"}, "limit": {strconv.Itoa(limit)},
		"start": {strconv.FormatInt(start.UnixNano(), 10)}, "end": {strconv.FormatInt(end.UnixNano(), 10)},
	}
	var res struct {
		Data struct {
			Result []struct {
				Values [][2]string `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := l.get(ctx, "/loki/api/v1/query_range", q, &res); err != nil {
		return nil, "", err
	}
	type line struct {
		ns  int64
		raw string
	}
	var lines []line
	for _, s := range res.Data.Result {
		for _, v := range s.Values {
			ns, err := strconv.ParseInt(v[0], 10, 64)
			// the cursor's own event was on the previous page, whether or
			// not Loki treats end as inclusive
			if err == nil && (before == 0 || ns < before) {
				lines = append(lines, line{ns, v[1]})
			}
		}
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].ns > lines[j].ns })
	if len(lines) > limit {
		lines = lines[:limit]
	}
	out := make([]Event, 0, len(lines))
	for _, ln := range lines {
		if ev, ok := parse(ln.raw, f.Tenant); ok {
			out = append(out, ev)
		}
	}
	next := ""
	if len(lines) == limit {
		next = strconv.FormatInt(lines[len(lines)-1].ns, 10)
	}
	return out, next, nil
}

// parse keeps the panel's fields of one event line, and only the tenant's.
func parse(raw, tenant string) (Event, bool) {
	var e struct {
		Event
		Tenant string         `json:"tenant"`
		Detail map[string]any `json:"detail"`
	}
	if json.Unmarshal([]byte(raw), &e) != nil || e.Tenant != tenant || e.Type == "" {
		return Event{}, false
	}
	ev := e.Event
	if f, ok := e.Detail["field"].(string); ok && f != "" {
		ev.Field = &f
	}
	if c, ok := e.Detail["connector_id"].(string); ok && c != "" {
		ev.Connector = &c
	}
	return ev, true
}

// Stats returns the events per type and the refused chart of the period.
func (l *Loki) Stats(ctx context.Context, f Filter) (map[string]int, []Bucket, error) {
	now := l.now()
	var inst struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  [2]any            `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	q := url.Values{"query": {f.Counts()}, "time": {strconv.FormatInt(now.UnixNano(), 10)}}
	if err := l.get(ctx, "/loki/api/v1/query", q, &inst); err != nil {
		return nil, nil, err
	}
	counts := map[string]int{}
	for _, r := range inst.Data.Result {
		if s, ok := r.Value[1].(string); ok && r.Metric["type"] != "" {
			n, _ := strconv.Atoi(s)
			counts[r.Metric["type"]] = n
		}
	}
	span, step := f.Span()
	end := now.Truncate(step).Add(step)
	start := end.Add(-span)
	var rng struct {
		Data struct {
			Result []struct {
				Values [][2]any `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	q = url.Values{"query": {f.Refused()}, "step": {strconv.Itoa(int(step.Seconds()))},
		"start": {strconv.FormatInt(start.UnixNano(), 10)}, "end": {strconv.FormatInt(end.UnixNano(), 10)}}
	if err := l.get(ctx, "/loki/api/v1/query_range", q, &rng); err != nil {
		return nil, nil, err
	}
	got := map[int64]int{}
	for _, r := range rng.Data.Result {
		for _, v := range r.Values {
			sec, _ := v[0].(float64)
			s, _ := v[1].(string)
			n, _ := strconv.Atoi(s)
			got[int64(sec)] = n
		}
	}
	var buckets []Bucket
	for t := start.Add(step); !t.After(end); t = t.Add(step) {
		buckets = append(buckets, Bucket{TS: t.Add(-step).UTC(), Count: got[t.Unix()]}) // start of the interval
	}
	return counts, buckets, nil
}
