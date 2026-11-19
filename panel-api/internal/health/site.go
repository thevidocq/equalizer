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
	// CertificateExpires: the end of the site's certificate (reminders).
	CertificateExpires *time.Time `json:"certificate_expires,omitempty"`
}

// Input is what is known of a site, gathered by the caller.
type Input struct {
	App, Name, Domain string
	MapVersion        int             // 0: nothing published
	Map               json.RawMessage // the published document
	Tunnels           map[string]int  // nil: relays not reachable
	Versions          []string        // release of each tunnel of the site's connector
	Platform          string          // release of the platform
	Served, Errors    int             // last 15 minutes
	Traffic           bool            // false: the log store did not answer
	Exposure          string          // closed, exposed, error, pending, …
	TLSStatus         string          // SiteTLS status: ok, missing, incomplete, expiring, expired, not_needed
	CertExpires       *time.Time      // the site's certificate, if any
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
	case in.Tunnels[cn] > 0 && outdated(in.Versions, in.Platform) != "":
		s.Checks = append(s.Checks, Check{"Connector", Degraded, fmt.Sprintf("%s online, release %s older than the platform's %s: update it",
			cn, outdated(in.Versions, in.Platform), in.Platform)})
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
	s.Checks = append(s.Checks, certificateCheck(in))
	s.CertificateExpires = in.CertExpires
	s.State = worst(s.Checks)
	return s
}

func certificateCheck(in Input) Check {
	until := ""
	if in.CertExpires != nil {
		until = in.CertExpires.UTC().Format(time.DateOnly)
	}
	switch in.TLSStatus {
	case "ok":
		return Check{"Certificate", OK, "valid until " + until}
	case "expiring":
		return Check{"Certificate", Degraded, "expires on " + until + ": upload the renewed certificate"}
	case "incomplete":
		return Check{"Certificate", Degraded, "does not cover every domain of the site: those get no HTTPS"}
	case "expired":
		return Check{"Certificate", Down, "expired on " + until + ": browsers refuse the site"}
	case "missing":
		return Check{"Certificate", Down, "no certificate: visitors get no HTTPS (Site → Certificate)"}
	}
	return Check{"Certificate", NotUsed, "the site is served in http"}
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
