package assistant

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type cardSrc struct{ src }

func (c *cardSrc) Map(context.Context, int) (map[string]any, int, string, error) {
	var doc map[string]any
	_ = json.Unmarshal([]byte(`{
		"types": {"slug": {"base": "string", "max_length": 80, "pattern": "^[a-z-]+$"}},
		"routes": [{"id": "shop", "path": "/shop/{cat}/", "method": "GET",
			"params": {"cat": {"type": "slug"}},
			"query": {"orderby": {"type": "enum", "values": ["price", "date"]}, "paged": {"type": "int", "min": 1, "max": 100000}}}],
		"forms": [{"id": "review", "route": "shop", "fields": {"comment": {"type": "text", "max_length": 2000, "rules": ["no_html"], "required": true}}}]
	}`), &doc)
	return doc, 7, "published", nil
}

func TestCards(t *testing.T) {
	ts := time.Date(2026, 11, 23, 14, 2, 17, 0, time.UTC)
	s := &cardSrc{src{logs: []Event{
		{TS: ts, Type: "request.blocked", Reason: "value_not_allowed", Route: "shop", Method: "GET", Path: "/shop/?orderby=1%20AND%20SLEEP(5)", Status: 400, IP: "198.51.100.4", RequestID: "9c41e0b2"},
		{TS: ts, Type: "request.blocked", Reason: "value_not_allowed", IP: "198.51.100.4", RequestID: "aaaaaa01"},
		{TS: ts, Type: "field.rejected", Reason: "no_html", IP: "198.51.100.9", RequestID: "aaaaaa02"},
	}}}
	m := NewMasker([]byte("k"))
	var ans Answer
	show := func(in string) string {
		return Exec(context.Background(), s, m, Call{Name: "show_card", Input: json.RawMessage(in)}, &ans)
	}

	if got := show(`{"kind":"event","request_id":"9c41e0b2"}`); !strings.HasPrefix(got, "shown") || strings.Contains(got, "198.51") {
		t.Fatalf("event: %s", got)
	}
	ev := ans.Cards[0].(EventCard)
	if ev.Path != "/shop/?orderby=1%20AND%20SLEEP(5)" || ev.Visitor != m.Visitor("198.51.100.4") || ev.Status != 400 {
		t.Fatalf("event card: %+v", ev)
	}
	b, _ := json.Marshal(ev)
	if strings.Contains(string(b), "198.51") {
		t.Fatalf("the card carries the address: %s", b)
	}
	if got := show(`{"kind":"event","request_id":"<script>"}`); !strings.HasPrefix(got, "not shown") {
		t.Fatalf("bad id: %s", got)
	}

	show(`{"kind":"refusals","since":"7d"}`)
	rf := ans.Cards[1].(RefusalsCard)
	if rf.Total != 3 || rf.Rows[0].Reason != "value_not_allowed" || rf.Rows[0].Requests != 2 || rf.Rows[0].Visitors != 1 || rf.Partial {
		t.Fatalf("refusals card: %+v", rf)
	}

	show(`{"kind":"route","route_id":"shop"}`)
	rt := ans.Cards[2].(RouteCard)
	rules := map[string]string{}
	for _, c := range rt.Checks {
		rules[c.Where+" "+c.Name] = c.Rule
	}
	want := map[string]string{
		"path cat":            "text, 80 characters at most, with a pattern, type slug",
		"query orderby":       "one of price, date",
		"query paged":         "whole number, 1 to 100000",
		"form review comment": "text, 2000 characters at most, no html, required",
	}
	for k, v := range want {
		if rules[k] != v {
			t.Fatalf("%s: %q, want %q (%+v)", k, rules[k], v, rt)
		}
	}
	if rt.Version != 7 {
		t.Fatalf("version: %d", rt.Version)
	}

	show(`{"kind":"route","route_id":"shop"}`)
	if got := show(`{"kind":"route","route_id":"shop"}`); !strings.Contains(got, "four cards") || len(ans.Cards) != MaxCards {
		t.Fatalf("cards per answer: %d %s", len(ans.Cards), got)
	}
}

// A field's own rules add to its type's, as the gateway applies them.
func TestRuleKeepsTheTypesRules(t *testing.T) {
	named := map[string]map[string]any{"slug": {"base": "string", "rules": []any{"no_lookalikes"}}}
	got := rule(map[string]any{"type": "slug", "rules": []any{"no_html"}}, named)
	if !strings.Contains(got, "no lookalikes") || !strings.Contains(got, "no html") {
		t.Errorf("%q", got)
	}
}
