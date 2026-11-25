package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Event is what the assistant sees of a security event: no field values.
type Event struct {
	TS        time.Time
	Type      string
	Reason    string
	Route     string
	Method    string
	Path      string
	Status    int
	Field     string
	IP        string
	RequestID string
}

// Activity is one action in the site's history.
type Activity struct {
	TS     time.Time
	User   string // email; masked before the model sees it
	Action string
	Detail map[string]any
}

// Source is what the tools read, for one site, and where a proposal goes.
type Source interface {
	Map(ctx context.Context, version int) (doc map[string]any, v int, status string, err error)
	Report(ctx context.Context) (json.RawMessage, error)
	Logs(ctx context.Context, since string, types []string, search string) ([]Event, error)
	Activity(ctx context.Context, limit int) ([]Activity, error)
	Propose(ctx context.Context, c Change) (id, summary string, err error)
}

// ErrNoLogs: the platform has no log store.
var ErrNoLogs = errors.New("no log store")

var logTypes = []string{"request.served", "request.blocked", "field.rejected", "ratelimit.hit", "behavior.alert",
	"parse.failed", "legacy.error", "connector.down", "exposure.alert"}

// Tools are the assistant's tools: those that read (map, report, logs,
// history, status, versions, checks, comparisons, guide), one that shows a
// card, one that proposes.
func Tools() []Tool {
	str := func(d string) map[string]any { return map[string]any{"type": "string", "description": d} }
	return append([]Tool{
		{Name: "get_map", Description: "The site's map: its routes with the checks on each value, forms, file folders and areas. Without version: the published map.",
			Schema: obj(map[string]any{"version": map[string]any{"type": "integer", "description": "a map version; omit for the published one"}})},
		{Name: "get_report", Description: "The latest discovery's report: pages found, coverage, what is still open and why, what was kept out, what was added from visitors' requests.",
			Schema: obj(map[string]any{})},
		{Name: "search_logs", Description: "The site's security events (requests served and refused, rejected fields, rate limits, legacy errors): totals by type, reason, entry and path, distinct visitors, and the latest matching events. Visitors appear as codes (V-…).",
			Schema: obj(map[string]any{
				"since":  map[string]any{"type": "string", "enum": []string{"1h", "24h", "7d", "30d"}, "description": "period, default 24h"},
				"types":  map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": logTypes}},
				"search": str("words a matching event contains: a path, a request id, a reason"),
				"limit":  map[string]any{"type": "integer", "description": "latest events to list, 0 to 30, default 10"},
			})},
		{Name: "get_activity", Description: "The site's history: discoveries, drafts, publications, alerts, who did what and when.",
			Schema: obj(map[string]any{"limit": map[string]any{"type": "integer", "description": "1 to 30, default 15"}})},
		cardTool(),
		{Name: "propose_change", Description: "Proposes ONE change to the map, which an administrator may turn into a draft with a click; the agent then checks the draft on the site. It changes nothing by itself, and these four kinds are ALL it can do: " +
			"add_file_type (prefix, type: the media type the site sends); " +
			"add_query_key (route_id, key, spec: {type: int|date|enum|string, values for enum, min/max for int}; prefer enum with the values the site offers); " +
			"add_enum_values (route_id, key, values); " +
			"narrow_field (route_id or form_id, key, spec: {type: " + strings.Join(NarrowKinds, "|") + ", values for enum, min/max for int, max_length for text, digits, slug, key}): a field that takes free text gets a narrower check; take the type from the values the site uses (get_report, get_map) and the code when the report has it.",
			Schema: obj(map[string]any{
				"kind":   map[string]any{"type": "string", "enum": []string{"add_file_type", "add_query_key", "add_enum_values", "narrow_field"}},
				"prefix": str("a file folder of the map"), "type": str("a media type, e.g. application/pdf"),
				"route_id": str("a route's id"), "form_id": str("a form's id (narrow_field)"), "key": str("a query key, or a form's field"),
				"spec": map[string]any{"type": "object", "properties": map[string]any{
					"type": map[string]any{"type": "string", "enum": append([]string{"string"}, NarrowKinds...)},
					"min":  map[string]any{"type": "number"}, "max": map[string]any{"type": "number"},
					"values":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"max_length": map[string]any{"type": "integer"}}},
				"values": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"reason": str("why, in one sentence, for the person"),
			}, "kind", "reason")},
	}, siteTools()...)
}

func obj(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// Proposal is a change the model proposed and the panel stored.
type Proposal struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Reason  string `json:"reason"`
}

// Exec runs one tool call for the model; its answer is text, an error too.
func Exec(ctx context.Context, src Source, m *Masker, c Call, ans *Answer) string {
	proposals := &ans.Proposals
	var in map[string]any
	_ = json.Unmarshal(c.Input, &in)
	if out, ok := execSite(ctx, src, m, c, in); ok {
		return out
	}
	switch c.Name {
	case "get_map":
		doc, v, status, err := src.Map(ctx, intOf(in["version"], 0))
		if err != nil {
			return "error: " + err.Error()
		}
		return fmt.Sprintf("version %d (%s)\n", v, status) + fence(compactMap(doc))
	case "get_report":
		raw, err := src.Report(ctx)
		if err != nil {
			return "error: " + err.Error()
		}
		return fence(m.Text(compactReport(raw)))
	case "search_logs":
		since, _ := in["since"].(string)
		if since == "" {
			since = "24h"
		}
		search, _ := in["search"].(string)
		var types []string
		for _, t := range list(in["types"]) {
			if s, ok := t.(string); ok {
				types = append(types, s)
			}
		}
		evs, err := src.Logs(ctx, since, types, search)
		if errors.Is(err, ErrNoLogs) {
			return "error: this platform keeps no logs the assistant can read (the Logs page needs EQ_LOKI_URL)"
		}
		if err != nil {
			return "error: the log store does not answer; try again in a moment"
		}
		return fence(summarizeLogs(evs, m, min(max(intOf(in["limit"], 10), 0), 30), since))
	case "get_activity":
		acts, err := src.Activity(ctx, min(max(intOf(in["limit"], 15), 1), 30))
		if err != nil {
			return "error: " + err.Error()
		}
		var b strings.Builder
		for _, a := range acts {
			d, _ := json.Marshal(a.Detail)
			fmt.Fprintf(&b, "%s %s by %s %s\n", a.TS.UTC().Format(time.RFC3339), a.Action, or(m.User(a.User), "the platform"), m.Text(string(d)))
		}
		return fence(b.String())
	case "show_card":
		if len(ans.Cards) >= MaxCards {
			return "error: four cards already in this answer"
		}
		cd, what, err := card(ctx, src, m, in)
		if err != nil {
			return "not shown: " + err.Error()
		}
		ans.Cards = append(ans.Cards, cd)
		return "shown to the person: " + what + ". Do not repeat its details; say what it means."
	case "propose_change":
		var ch Change
		_ = json.Unmarshal(c.Input, &ch)
		if len(*proposals) >= 3 {
			return "error: three proposals already in this answer; let the person decide on those first"
		}
		id, summary, err := src.Propose(ctx, ch)
		if err != nil {
			return "not proposed: " + err.Error()
		}
		*proposals = append(*proposals, Proposal{ID: id, Summary: summary, Reason: ch.Reason})
		return "proposed (" + summary + "); the person sees a button to make it a draft. Mention it in one sentence."
	}
	return "error: unknown tool " + c.Name
}

func intOf(v any, fallback int) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return fallback
}

// compactMap keeps what a question about checks needs, in a few lines.
func compactMap(doc map[string]any) string {
	out := map[string]any{}
	for _, k := range []string{"routes", "forms", "api", "assets", "areas", "types"} {
		if v, ok := doc[k]; ok {
			out[k] = v
		}
	}
	for _, x := range list(out["routes"]) {
		if r, ok := x.(map[string]any); ok {
			delete(r, "legacy_request")
			delete(r, "response")
		}
	}
	b, _ := json.Marshal(out)
	return clip(string(b), 24000)
}

// compactReport is the report as lines: its plain facts first (the CMS and
// its version, counts, coverage), then each list capped, so a long list can
// never push a fact out of what the model reads.
func compactReport(raw json.RawMessage) string {
	var top map[string]any
	if json.Unmarshal(raw, &top) != nil {
		return "no report"
	}
	rep, _ := top["report"].(map[string]any)
	if rep == nil {
		rep = top
	}
	for _, k := range []string{"pages", "pages_by_account", "samples"} {
		delete(rep, k)
	}
	var facts, objects, lists []string
	keys := make([]string, 0, len(rep))
	for k := range rep {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch v := rep[k].(type) {
		case []any:
			shown := v
			more := ""
			limit := 40
			if k == "components" { // "is plugin X there?" needs them all (the agent sends 200 at most)
				limit = 200
			}
			if len(v) > limit {
				shown, more = v[:limit], fmt.Sprintf(" …and %d more", len(v)-limit)
			}
			b, _ := json.Marshal(shown)
			lists = append(lists, fmt.Sprintf("%s (%d): %s%s", k, len(v), clip(string(b), 3000), more))
		case map[string]any:
			b, _ := json.Marshal(v)
			objects = append(objects, k+": "+clip(string(b), 2000))
		default:
			b, _ := json.Marshal(v)
			facts = append(facts, k+": "+string(b))
		}
	}
	var b strings.Builder
	if about, ok := top["about"]; ok {
		a, _ := json.Marshal(about)
		b.WriteString("discovery: " + string(a) + "\n")
	}
	for _, part := range [][]string{facts, objects, lists} {
		for _, l := range part {
			b.WriteString(l + "\n")
		}
	}
	return clip(b.String(), 20000)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + " …(cut)"
}

// summarizeLogs counts the events and lists the latest, visitors as codes.
func summarizeLogs(evs []Event, m *Masker, limit int, since string) string {
	byType, byReason, byEntry, byPath := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	visitors := map[string]bool{}
	for _, e := range evs {
		byType[e.Type]++
		if e.Reason != "" {
			byReason[e.Type+": "+e.Reason]++
		}
		if e.Route != "" {
			byEntry[e.Route]++
		}
		if e.Path != "" {
			byPath[m.Text(e.Path)]++
		}
		if e.IP != "" {
			visitors[m.Visitor(e.IP)] = true
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d events in the last %s (at most 500 read), %d distinct visitors\n", len(evs), since, len(visitors))
	for _, t := range []struct {
		name string
		c    map[string]int
	}{{"by type", byType}, {"by reason", byReason}, {"by entry", byEntry}, {"top paths", byPath}} {
		fmt.Fprintf(&b, "%s: %s\n", t.name, top(t.c, 12))
	}
	if limit > 0 {
		b.WriteString("latest:\n")
	}
	for i, e := range evs {
		if i == limit {
			break
		}
		fmt.Fprintf(&b, "%s %s %s %s %d visitor=%s entry=%s reason=%s field=%s id=%s\n", e.TS.UTC().Format(time.RFC3339),
			e.Type, e.Method, m.Text(e.Path), e.Status, m.Visitor(e.IP), e.Route, e.Reason, e.Field, e.RequestID)
	}
	return b.String()
}

func top(c map[string]int, n int) string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return c[keys[i]] > c[keys[j]] || c[keys[i]] == c[keys[j]] && keys[i] < keys[j] })
	var parts []string
	for i, k := range keys {
		if i == n {
			parts = append(parts, fmt.Sprintf("… %d more", len(keys)-n))
			break
		}
		parts = append(parts, fmt.Sprintf("%s=%d", k, c[k]))
	}
	return strings.Join(parts, ", ")
}
