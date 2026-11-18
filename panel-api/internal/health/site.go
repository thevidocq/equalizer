package health

import (
	"encoding/json"
	"fmt"
	"time"
)

// Check is one thing a site needs, with its state in plain words.
type Check struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

// Day counts the problems of one day.
type Day struct {
	Date           string `json:"date"`
	LegacyErrors   int    `json:"legacy_errors"`
	ConnectorDrops int    `json:"connector_drops"`
}

// Site is one row of the status page.
type Site struct {
	App          string  `json:"app"`
	Name         string  `json:"name"`
	PublicDomain string  `json:"public_domain"`
	State        string  `json:"state"`
	Checks       []Check `json:"checks"`
	Days         []Day   `json:"days"`
}

// Input is what is known of a site, gathered by the caller.
type Input struct {
	App, Name, Domain string
	MapVersion        int             // 0: nothing published
	Map               json.RawMessage // the published document
	Tunnels           map[string]int  // nil: relays not reachable
	Served, Errors    int             // last 15 minutes
	Traffic           bool            // false: the log store did not answer
	Exposure          string          // closed, exposed, error, pending, …
}

// Connector returns the connector id of a map, "" in direct mode.
func Connector(doc json.RawMessage) string {
	var m struct {
		Legacy struct {
			Reach struct {
				Mode        string `json:"mode"`
				ConnectorID string `json:"connector_id"`
			} `json:"reach"`
		} `json:"legacy"`
	}
	_ = json.Unmarshal(doc, &m)
	if m.Legacy.Reach.Mode == "connector" {
		return m.Legacy.Reach.ConnectorID
	}
	return ""
}

// Evaluate turns what is known into the checks of a site.
func Evaluate(in Input) Site {
	s := Site{App: in.App, Name: in.Name, PublicDomain: in.Domain}
	if in.MapVersion == 0 {
		s.Checks = append(s.Checks, Check{"Map", Down, "no published map: the gateway does not serve the site"})
	} else {
		s.Checks = append(s.Checks, Check{"Map", OK, fmt.Sprintf("version %d published", in.MapVersion)})
	}

	switch cn := Connector(in.Map); {
	case in.MapVersion == 0:
		s.Checks = append(s.Checks, Check{"Connector", Unknown, "known once a map is published"})
	case cn == "":
		s.Checks = append(s.Checks, Check{"Connector", NotUsed, "the site is reached directly"})
	case in.Tunnels == nil:
		s.Checks = append(s.Checks, Check{"Connector", Unknown, "the relay does not answer"})
	case in.Tunnels[cn] > 0:
		s.Checks = append(s.Checks, Check{"Connector", OK, fmt.Sprintf("%s online", cn)})
	default:
		s.Checks = append(s.Checks, Check{"Connector", Down, fmt.Sprintf("%s offline: check the server it runs on", cn)})
	}

	switch {
	case !in.Traffic:
		s.Checks = append(s.Checks, Check{"Legacy site", Unknown, "the log store does not answer"})
	case in.Served == 0 && in.Errors == 0:
		s.Checks = append(s.Checks, Check{"Legacy site", Unknown, "no visits in the last 15 minutes"})
	case in.Errors == 0:
		s.Checks = append(s.Checks, Check{"Legacy site", OK, fmt.Sprintf("%d requests served in the last 15 minutes", in.Served)})
	case in.Served == 0:
		s.Checks = append(s.Checks, Check{"Legacy site", Down, fmt.Sprintf("%d errors and nothing served in the last 15 minutes", in.Errors)})
	default:
		s.Checks = append(s.Checks, Check{"Legacy site", Degraded, fmt.Sprintf("%d errors, %d served in the last 15 minutes", in.Errors, in.Served)})
	}

	switch in.Exposure {
	case "closed":
		s.Checks = append(s.Checks, Check{"Isolation", OK, "not reachable on its old IP"})
	case "exposed":
		s.Checks = append(s.Checks, Check{"Isolation", Down, "still answers on its old IP: it can be reached without Equalizer"})
	case "error":
		s.Checks = append(s.Checks, Check{"Isolation", Unknown, "the last check failed"})
	case "pending":
		s.Checks = append(s.Checks, Check{"Isolation", Unknown, "first check pending"})
	default:
		s.Checks = append(s.Checks, Check{"Isolation", Unknown, "old IP not set (Site → Isolation)"})
	}
	s.State = worst(s.Checks)
	return s
}

func worst(cs []Check) string {
	rank := map[string]int{OK: 1, Degraded: 2, Down: 3}
	out, best := Unknown, 0
	for _, c := range cs {
		if r := rank[c.State]; r > best {
			out, best = c.State, r
		}
	}
	return out
}

// Days builds the history rows from the daily counts.
func Days(dates []time.Time, errors, drops []int) []Day {
	out := make([]Day, len(dates))
	for i, d := range dates {
		out[i] = Day{Date: d.Format("2006-01-02")}
		if i < len(errors) {
			out[i].LegacyErrors = errors[i]
		}
		if i < len(drops) {
			out[i].ConnectorDrops = drops[i]
		}
	}
	return out
}
