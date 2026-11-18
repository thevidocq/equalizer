package logsearch

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// Counts for the status page, from the tenant's own events only.

// Recent returns events per site and type in the last window, for the
// types given (e.g. request.served, legacy.error).
func (l *Loki) Recent(ctx context.Context, tenant string, window time.Duration, types ...string) (map[string]map[string]int, error) {
	f := Filter{Tenant: tenant, Types: types}
	q := `sum by (app, type) (count_over_time(` + f.Logs() + ` [` + strconv.Itoa(int(window.Seconds())) + `s]))`
	var res struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  [2]any            `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := l.get(ctx, "/loki/api/v1/query", url.Values{"query": {q}, "time": {strconv.FormatInt(l.now().UnixNano(), 10)}}, &res); err != nil {
		return nil, err
	}
	out := map[string]map[string]int{}
	for _, r := range res.Data.Result {
		s, _ := r.Value[1].(string)
		n, _ := strconv.Atoi(s)
		app := r.Metric["app"]
		if out[app] == nil {
			out[app] = map[string]int{}
		}
		out[app][r.Metric["type"]] += n
	}
	return out, nil
}

// Daily returns, per day (UTC, oldest first, `days` of them), the count of
// one event type grouped by a label (app, or detail_connector_id).
func (l *Loki) Daily(ctx context.Context, tenant, typ, by string, days int) ([]time.Time, map[string][]int, error) {
	f := Filter{Tenant: tenant, Types: []string{typ}}
	q := `sum by (` + by + `) (count_over_time(` + f.Logs() + ` [1d]))`
	day := 24 * time.Hour
	end := l.now().UTC().Truncate(day).Add(day)
	start := end.Add(-time.Duration(days) * day)
	var res struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Values [][2]any          `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	v := url.Values{"query": {q}, "step": {"86400"},
		"start": {strconv.FormatInt(start.Add(day).UnixNano(), 10)}, "end": {strconv.FormatInt(end.UnixNano(), 10)}}
	if err := l.get(ctx, "/loki/api/v1/query_range", v, &res); err != nil {
		return nil, nil, err
	}
	dates := make([]time.Time, days)
	for i := range dates {
		dates[i] = start.Add(time.Duration(i) * day)
	}
	out := map[string][]int{}
	for _, r := range res.Data.Result {
		key := r.Metric[by]
		if out[key] == nil {
			out[key] = make([]int, days)
		}
		for _, val := range r.Values {
			sec, _ := val[0].(float64)
			s, _ := val[1].(string)
			n, _ := strconv.Atoi(s)
			// the sample at the end of a day counts that day
			i := int(time.Unix(int64(sec), 0).UTC().Sub(start)/day) - 1
			if i >= 0 && i < days {
				out[key][i] += n
			}
		}
	}
	return dates, out, nil
}
