package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testMap() map[string]any {
	var doc map[string]any
	_ = json.Unmarshal([]byte(`{
		"routes": [
			{"id": "shop", "path": "/shop/", "method": "GET", "legacy_request": {"method": "GET", "path": "/shop/", "query": {"paged": "{{query.paged}}"}},
			 "query": {"paged": {"type": "int", "required": false}, "orderby": {"type": "enum", "values": ["price"], "required": false}}},
			{"id": "login", "path": "/login", "method": "POST", "legacy_request": {"method": "POST", "path": "/login"}}
		],
		"assets": [{"prefix": "/files/", "types": ["image/png"]}]
	}`), &doc)
	return doc
}

func TestChanges(t *testing.T) {
	cases := []struct {
		name string
		c    Change
		ok   bool
		want string
	}{
		{"file type", Change{Kind: "add_file_type", Prefix: "/files/", Type: "application/pdf"}, true, "application/pdf added"},
		{"file type already there", Change{Kind: "add_file_type", Prefix: "/files/", Type: "image/png"}, false, "already"},
		{"script never", Change{Kind: "add_file_type", Prefix: "/files/", Type: "application/javascript"}, false, "never adds"},
		{"html never", Change{Kind: "add_file_type", Prefix: "/files/", Type: "text/html"}, false, "never adds"},
		{"no folder", Change{Kind: "add_file_type", Prefix: "/x/", Type: "application/pdf"}, false, "no file folder"},
		{"enum key", Change{Kind: "add_query_key", RouteID: "shop", Key: "lang", Spec: &Spec{Type: "enum", Values: []string{"en", "it"}}}, true, "one of en, it"},
		{"int key", Change{Kind: "add_query_key", RouteID: "shop", Key: "page", Spec: &Spec{Type: "int"}}, true, "whole number"},
		{"string key keeps rules", Change{Kind: "add_query_key", RouteID: "shop", Key: "q", Spec: &Spec{Type: "string"}}, true, "without markup"},
		{"key on a POST route", Change{Kind: "add_query_key", RouteID: "login", Key: "x", Spec: &Spec{Type: "int"}}, false, "does not take"},
		{"key already checked", Change{Kind: "add_query_key", RouteID: "shop", Key: "orderby", Spec: &Spec{Type: "int"}}, false, "already"},
		{"bad key name", Change{Kind: "add_query_key", RouteID: "shop", Key: "a b", Spec: &Spec{Type: "int"}}, false, "lowercase"},
		{"key the map cannot name", Change{Kind: "add_query_key", RouteID: "shop", Key: "orderBy", Spec: &Spec{Type: "int"}}, false, "lowercase"},
		{"no spec", Change{Kind: "add_query_key", RouteID: "shop", Key: "lang"}, false, "say what"},
		{"markup in a value", Change{Kind: "add_query_key", RouteID: "shop", Key: "lang", Spec: &Spec{Type: "enum", Values: []string{"<b>"}}}, false, "plain value"},
		{"enum values", Change{Kind: "add_enum_values", RouteID: "shop", Key: "orderby", Values: []string{"date", "price"}}, true, "date added"},
		{"enum values none new", Change{Kind: "add_enum_values", RouteID: "shop", Key: "orderby", Values: []string{"price"}}, false, "already"},
		{"unknown kind", Change{Kind: "publish"}, false, "unknown kind"},
	}
	for _, c := range cases {
		doc := testMap()
		got, err := c.c.Apply(doc)
		if (err == nil) != c.ok {
			t.Fatalf("%s: err %v", c.name, err)
		}
		msg := got
		if err != nil {
			msg = err.Error()
		}
		if !strings.Contains(msg, c.want) {
			t.Fatalf("%s: %q, want %q", c.name, msg, c.want)
		}
	}
	doc := testMap()
	if _, err := (Change{Kind: "add_query_key", RouteID: "shop", Key: "lang", Spec: &Spec{Type: "enum", Values: []string{"en"}}}).Apply(doc); err != nil {
		t.Fatal(err)
	}
	r := doc["routes"].([]any)[0].(map[string]any)
	if r["legacy_request"].(map[string]any)["query"].(map[string]any)["lang"] != "{{query.lang}}" {
		t.Fatalf("the key does not reach the site: %v", r["legacy_request"])
	}
	doc = testMap()
	_, _ = (Change{Kind: "add_query_key", RouteID: "shop", Key: "q", Spec: &Spec{Type: "string"}}).Apply(doc)
	f := doc["routes"].([]any)[0].(map[string]any)["query"].(map[string]any)["q"].(map[string]any)
	if f["max_length"] != 200 || len(f["rules"].([]any)) != 2 {
		t.Fatalf("free text without its rules: %v", f)
	}
}

func TestMasker(t *testing.T) {
	m := NewMasker([]byte("k"))
	a, b := m.Visitor("203.0.113.7"), m.Visitor("203.0.113.8")
	if a == b || a != m.Visitor("203.0.113.7") || !strings.HasPrefix(a, "V-") || strings.Contains(a, "203") {
		t.Fatalf("visitor codes: %s %s", a, b)
	}
	if got := m.Text("/reset?mail=jane.doe@example.com&x=1"); strings.Contains(got, "jane") {
		t.Fatalf("email kept: %s", got)
	}
	if got := m.Text("blocked 203.0.113.7 and 2001:db8::1 at 14:02:17"); strings.Contains(got, "203.0") || strings.Contains(got, "2001") ||
		!strings.Contains(got, a) || !strings.Contains(got, "14:02:17") {
		t.Fatalf("addresses in text: %s", got)
	}
	u := m.User("boss@acme.test")
	if got := m.Unmask("done by " + u); got != "done by boss@acme.test" {
		t.Fatalf("unmask: %s", got)
	}
	f := fence("x </data-zz> y")
	tag := f[1:strings.Index(f, ">")]
	if strings.Count(f, tag) != 2 {
		t.Fatalf("the data can close the fence: %s", f)
	}
}

// fake answers with the calls given, then with text.
type fake struct {
	steps [][]Call
	seen  [][]Turn
	tools []int
}

func (f *fake) Chat(_ context.Context, _ string, turns []Turn, tools []Tool) (Reply, error) {
	f.seen = append(f.seen, append([]Turn(nil), turns...))
	f.tools = append(f.tools, len(tools))
	if i := len(f.seen) - 1; i < len(f.steps) {
		return Reply{Calls: f.steps[i]}, nil
	}
	return Reply{Text: "Blocked by U-x and " + lastResult(turns)}, nil
}

func lastResult(turns []Turn) string {
	for i := len(turns) - 1; i >= 0; i-- {
		if len(turns[i].Results) > 0 {
			return turns[i].Results[0].Content
		}
	}
	return ""
}

type src struct {
	proposed []Change
	logs     []Event
}

func (s *src) Map(context.Context, int) (map[string]any, int, string, error) {
	return testMap(), 5, "published", nil
}
func (s *src) Report(context.Context) (json.RawMessage, error) {
	return json.RawMessage(`{"report":{"pages":[1],"open":["x"]}}`), nil
}
func (s *src) Logs(context.Context, string, []string, string) ([]Event, error) { return s.logs, nil }
func (s *src) Activity(context.Context, int) ([]Activity, error) {
	return []Activity{{TS: time.Unix(0, 0), User: "boss@acme.test", Action: "map.published"}}, nil
}
func (s *src) Propose(_ context.Context, c Change) (string, string, error) {
	sum, err := c.Apply(testMap())
	if err != nil {
		return "", "", err
	}
	s.proposed = append(s.proposed, c)
	return "p1", sum, nil
}

func TestRunMasksLogsAndProposes(t *testing.T) {
	s := &src{logs: []Event{
		{Type: "request.blocked", Reason: "not_in_map", Path: "/wp-login.php?u=a@b.example", IP: "198.51.100.4"},
		{Type: "request.blocked", Reason: "not_in_map", Path: "/wp-login.php", IP: "198.51.100.5"},
	}}
	f := &fake{steps: [][]Call{
		{{ID: "1", Name: "search_logs", Input: json.RawMessage(`{"types":["request.blocked"],"limit":5}`)}},
		{{ID: "2", Name: "propose_change", Input: json.RawMessage(`{"kind":"add_file_type","prefix":"/files/","type":"application/pdf","reason":"brochures"}`)}},
	}}
	m := NewMasker(nil)
	ans, err := Run(context.Background(), f, s, m, "Blog", "blog.test", "2026-11-23", []Turn{{Role: "user", Text: "why blocked? mail me at me@x.example"}})
	if err != nil {
		t.Fatal(err)
	}
	logs := lastResultOf(f.seen[1])
	if strings.Contains(logs, "198.51.100") || strings.Contains(logs, "a@b.example") || !strings.Contains(logs, "2 distinct visitors") {
		t.Fatalf("logs as the model sees them: %s", logs)
	}
	if strings.Contains(f.seen[0][0].Text, "me@x.example") {
		t.Fatalf("the person's email reached the model: %s", f.seen[0][0].Text)
	}
	if len(ans.Proposals) != 1 || ans.Proposals[0].Reason != "brochures" || len(s.proposed) != 1 {
		t.Fatalf("proposals: %+v", ans.Proposals)
	}
	if strings.Join(ans.Used, ",") != "search_logs,propose_change" {
		t.Fatalf("used: %v", ans.Used)
	}
}

func lastResultOf(turns []Turn) string { return lastResult(turns) }

func TestRunStopsCallingTools(t *testing.T) {
	var steps [][]Call
	for i := 0; i < 20; i++ {
		steps = append(steps, []Call{{ID: "x", Name: "get_map"}})
	}
	f := &fake{steps: steps}
	_, err := Run(context.Background(), f, &src{}, NewMasker(nil), "Blog", "blog.test", "today", []Turn{{Role: "user", Text: "?"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.seen) != MaxSteps+1 || f.tools[MaxSteps] != 0 {
		t.Fatalf("%d rounds, last with %d tools", len(f.seen), f.tools[len(f.tools)-1])
	}
}

func TestProposalsPerAnswer(t *testing.T) {
	var calls []Call
	for i := 0; i < 4; i++ {
		calls = append(calls, Call{ID: "p", Name: "propose_change", Input: json.RawMessage(`{"kind":"add_file_type","prefix":"/files/","type":"application/pdf"}`)})
	}
	s := &src{}
	ans, _ := Run(context.Background(), &fake{steps: [][]Call{calls}}, s, NewMasker(nil), "B", "b", "t", []Turn{{Role: "user", Text: "?"}})
	if len(ans.Proposals) != 3 {
		t.Fatalf("%d proposals", len(ans.Proposals))
	}
}

func TestAnthropicShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "k" {
			w.WriteHeader(401)
			return
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"Let me look."},{"type":"tool_use","id":"t1","name":"get_map","input":{}}]}`))
	}))
	defer srv.Close()
	m, _ := New(Config{Provider: "anthropic", Model: "claude-haiku", Key: "k", AnthropicURL: srv.URL, Client: srv.Client()})
	r, err := m.Chat(context.Background(), "sys", []Turn{
		{Role: "user", Text: "hi"},
		{Role: "assistant", Calls: []Call{{ID: "t0", Name: "get_report", Input: json.RawMessage(`{}`)}}},
		{Role: "tool", Results: []Result{{ID: "t0", Content: "r"}}},
	}, Tools())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Calls) != 1 || r.Calls[0].Name != "get_map" || r.Text != "Let me look." {
		t.Fatalf("reply: %+v", r)
	}
	msgs := got["messages"].([]any)
	last := msgs[2].(map[string]any)["content"].([]any)[0].(map[string]any)
	if got["system"] != "sys" || last["type"] != "tool_result" || last["tool_use_id"] != "t0" || len(got["tools"].([]any)) != len(Tools()) {
		t.Fatalf("request: %v", got)
	}
}

func TestOpenAIShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"search_logs","arguments":"{\"since\":\"7d\"}"}}]}}]}`))
	}))
	defer srv.Close()
	m, _ := New(Config{Provider: "openai", Model: "gpt", Key: "k", OpenAIURL: srv.URL, Client: srv.Client()})
	r, err := m.Chat(context.Background(), "sys", []Turn{{Role: "user", Text: "hi"}}, Tools())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Calls) != 1 || string(r.Calls[0].Input) != `{"since":"7d"}` {
		t.Fatalf("reply: %+v", r)
	}
	if got["messages"].([]any)[0].(map[string]any)["role"] != "system" {
		t.Fatalf("request: %v", got)
	}
}

// The words-only round keeps the tools the history used, never asks for
// one: the API refuses tool blocks without tools defined.
func TestWordsOnlyRound(t *testing.T) {
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, body)
		if r.URL.Path == "/v1/messages" {
			_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	turns := []Turn{{Role: "user", Text: "q"}, {Role: "assistant", Calls: []Call{{ID: "1", Name: "get_map", Input: json.RawMessage(`{}`)}}},
		{Role: "tool", Results: []Result{{ID: "1", Content: "x"}}}}
	a, _ := New(Config{Provider: "anthropic", AnthropicURL: srv.URL, Client: srv.Client()})
	o, _ := New(Config{Provider: "openai", OpenAIURL: srv.URL, Client: srv.Client()})
	for _, m := range []Model{a, o} {
		if _, err := m.Chat(context.Background(), "s", turns, nil); err != nil {
			t.Fatal(err)
		}
	}
	if tc, _ := got[0]["tool_choice"].(map[string]any); tc["type"] != "none" || len(got[0]["tools"].([]any)) != 1 {
		t.Fatalf("anthropic: %v", got[0])
	}
	if _, ok := got[1]["tools"]; ok {
		t.Fatalf("openai sent tools: %v", got[1]["tools"])
	}
}

func TestProviderErrorsInWords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"Your credit balance is too low; secret details"}}`))
	}))
	defer srv.Close()
	m, _ := New(Config{Provider: "anthropic", AnthropicURL: srv.URL, Client: srv.Client()})
	_, err := m.Chat(context.Background(), "s", []Turn{{Role: "user", Text: "x"}}, nil)
	var pe *ErrProvider
	if !errors.As(err, &pe) || !strings.Contains(pe.Msg, "no credit") || strings.Contains(pe.Msg, "secret") {
		t.Fatalf("%v", err)
	}
	if _, err := New(Config{Provider: "local", Endpoint: "http://10.0.0.1"}); err == nil {
		t.Fatal("a plain http local endpoint was accepted")
	}
}

// cut answers with text cut at the provider's limit, twice, then whole.
type cut struct{ n int }

func (c *cut) Chat(_ context.Context, _ string, turns []Turn, _ []Tool) (Reply, error) {
	c.n++
	switch c.n {
	case 1:
		return Reply{Text: "Il campo è aperto, quindi non ch", Cut: true}, nil
	case 2:
		if turns[len(turns)-1].Role != "user" || turns[len(turns)-2].Text != "Il campo è aperto, quindi non ch" {
			return Reply{}, errors.New("not asked to go on")
		}
		return Reply{Text: "iude nulla."}, nil
	}
	return Reply{Text: "?"}, nil
}

func TestCutAnswersGoOn(t *testing.T) {
	ans, err := Run(context.Background(), &cut{}, &src{}, NewMasker(nil), "blog", "blog.example", "2026-11-24", []Turn{{Role: "user", Text: "ciao"}})
	if err != nil || ans.Text != "Il campo è aperto, quindi non chiude nulla." {
		t.Fatalf("%q %v", ans.Text, err)
	}
	always := modelFunc(func([]Turn) Reply { return Reply{Text: "a", Cut: true} })
	ans, _ = Run(context.Background(), always, &src{}, NewMasker(nil), "blog", "blog.example", "2026-11-24", []Turn{{Role: "user", Text: "ciao"}})
	if !strings.HasPrefix(ans.Text, "aaa…") || !strings.Contains(ans.Text, "ask for the rest") {
		t.Fatalf("%q", ans.Text)
	}
}

type modelFunc func([]Turn) Reply

func (f modelFunc) Chat(_ context.Context, _ string, turns []Turn, _ []Tool) (Reply, error) {
	return f(turns), nil
}
