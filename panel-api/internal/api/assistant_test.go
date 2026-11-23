package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/panel-api/internal/assistant"
)

// scripted asks for the calls given, one round each, then answers with the
// last tool result, so the test sees what the tools returned.
type scripted struct{ rounds [][]assistant.Call }

func (s *scripted) Chat(_ context.Context, _ string, turns []assistant.Turn, _ []assistant.Tool) (assistant.Reply, error) {
	done := 0
	for _, t := range turns {
		if t.Role == "tool" {
			done++
		}
	}
	if done < len(s.rounds) {
		return assistant.Reply{Calls: s.rounds[done]}, nil
	}
	last := ""
	if t := turns[len(turns)-1]; t.Role == "tool" {
		last = t.Results[0].Content
	}
	return assistant.Reply{Text: "answer: " + last}, nil
}

// The assistant reads one site, proposes a checked change, and only an
// administrator turns it into a draft, which the agent then checks.
func TestAssistant(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "viewer"}, nil)
	viewer := e.join("ap@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-web01"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, nil)
	ask := map[string]any{"text": "Why are PDFs refused?"}

	// no model in Settings → AI
	boss.must(409, "POST", "/v1/apps/blog/assistant", ask, nil)
	boss.must(200, "PUT", "/v1/tenants/acme/llm", map[string]any{"provider": "anthropic", "model": "claude-haiku-5-5", "api_key": "sk-test"}, nil)

	model := &scripted{}
	var gotKey string
	e.server.AssistantModel = func(c assistant.Config) (assistant.Model, error) { gotKey = c.Key; return model, nil }

	// no map yet: the tool says so, the answer still comes
	model.rounds = [][]assistant.Call{{{ID: "1", Name: "get_map", Input: json.RawMessage(`{}`)}}}
	var out struct {
		Text      string
		Proposals []struct{ ID, Summary, Reason string }
		Used      []string
	}
	viewer.must(200, "POST", "/v1/apps/blog/assistant", ask, &out)
	if gotKey != "sk-test" || !strings.Contains(out.Text, "none published") || out.Used[0] != "get_map" {
		t.Fatalf("answer: %+v (key %q)", out, gotKey)
	}

	// a discovery's draft, checked by the agent
	var d struct{ ID int64 }
	boss.must(201, "POST", "/v1/apps/blog/discoveries", map[string]any{"connector_id": "cn-web01", "address": "127.0.0.1:8443"}, &d)
	var job struct{ Template map[string]any }
	internalPost(t, e, "/v1/internal/discoveries/claim", nil, &job)
	internalPost(t, e, "/v1/internal/discoveries/"+itoa(d.ID)+"/result", map[string]any{"report": map[string]any{"routes_added": []any{}}, "document": job.Template}, nil)
	var v struct{ ID int64 }
	internalPost(t, e, "/v1/internal/verifications/claim", nil, &v)
	score := map[string]any{"functions_total": 1, "functions_identical": 1, "inputs_total": 0, "inputs_protected": 0, "area_percent": 0}
	internalPost(t, e, "/v1/internal/verifications/"+itoa(v.ID)+"/result", map[string]any{"score": score}, nil)

	// the report of the last discovery reaches the model
	model.rounds = [][]assistant.Call{{{ID: "r", Name: "get_report", Input: json.RawMessage(`{}`)}}}
	viewer.must(200, "POST", "/v1/apps/blog/assistant", ask, &out)
	if !strings.Contains(out.Text, "routes_added") || !strings.Contains(out.Text, `"discovery":`) {
		t.Fatalf("report: %s", out.Text)
	}

	// a card of an entry, built from the map; none from logs the platform does not keep
	model.rounds = [][]assistant.Call{{
		{ID: "c1", Name: "show_card", Input: json.RawMessage(`{"kind":"route","route_id":"home","version":1}`)},
		{ID: "c2", Name: "show_card", Input: json.RawMessage(`{"kind":"refusals"}`)},
	}}
	var withCards struct {
		Cards []struct {
			Kind   string
			ID     string
			Checks []struct{ Name, Where, Rule string }
		}
	}
	viewer.must(200, "POST", "/v1/apps/blog/assistant", ask, &withCards)
	if len(withCards.Cards) != 1 || withCards.Cards[0].Kind != "route" || len(withCards.Cards[0].Checks) == 0 {
		t.Fatalf("cards: %+v", withCards)
	}

	// the site's versions and its status, from the same source the panel uses
	model.rounds = [][]assistant.Call{{{ID: "v", Name: "list_versions", Input: json.RawMessage(`{}`)}}}
	viewer.must(200, "POST", "/v1/apps/blog/assistant", ask, &out)
	if !strings.Contains(out.Text, `"status":"draft"`) || strings.Contains(out.Text, "@") && !strings.Contains(out.Text, "boss@acme.test") {
		t.Fatalf("versions: %s", out.Text)
	}
	model.rounds = [][]assistant.Call{{{ID: "s", Name: "get_site", Input: json.RawMessage(`{}`)}}}
	viewer.must(200, "POST", "/v1/apps/blog/assistant", ask, &out)
	if !strings.Contains(out.Text, `"app":"blog"`) {
		t.Fatalf("status: %s", out.Text)
	}

	// a script type is refused, whoever asks; a PDF type is proposed
	model.rounds = [][]assistant.Call{
		{{ID: "1", Name: "propose_change", Input: json.RawMessage(`{"kind":"add_file_type","prefix":"/wp-content/uploads/","type":"application/x-php","reason":"x"}`)}},
		{{ID: "2", Name: "propose_change", Input: json.RawMessage(`{"kind":"add_file_type","prefix":"/wp-content/uploads/","type":"text/csv","reason":"price lists"}`)}},
	}
	viewer.must(200, "POST", "/v1/apps/blog/assistant", ask, &out)
	if len(out.Proposals) != 1 || !strings.Contains(out.Proposals[0].Summary, "text/csv") {
		t.Fatalf("proposals: %+v", out)
	}
	id := out.Proposals[0].ID

	// the viewer reads, the administrator decides
	viewer.must(403, "POST", "/v1/apps/blog/assistant/proposals/"+id+"/draft", nil, nil)
	boss.must(404, "POST", "/v1/apps/blog/assistant/proposals/nope/draft", nil, nil)
	var made struct{ Version int }
	boss.must(201, "POST", "/v1/apps/blog/assistant/proposals/"+id+"/draft", nil, &made)
	boss.must(409, "POST", "/v1/apps/blog/assistant/proposals/"+id+"/draft", nil, nil)
	// the viewer's conversation shows the draft the proposal became
	var convs struct{ Items []struct{ ID string } }
	viewer.must(200, "GET", "/v1/assistant/conversations", nil, &convs)
	shown := 0
	for _, c := range convs.Items {
		var conv struct {
			Messages []struct {
				Proposals []struct {
					ID           string
					DraftVersion *int `json:"draft_version"`
				}
			}
		}
		viewer.must(200, "GET", "/v1/assistant/conversations/"+c.ID, nil, &conv)
		for _, m := range conv.Messages {
			for _, p := range m.Proposals {
				if p.ID == id && p.DraftVersion != nil && *p.DraftVersion == made.Version {
					shown++
				}
			}
		}
	}
	if shown != 1 {
		t.Fatalf("the proposal does not show its draft (%d)", shown)
	}
	var mapList struct {
		Items []struct {
			Version int
			Status  string
			Note    string
		}
	}
	boss.must(200, "GET", "/v1/apps/blog/maps", nil, &mapList)
	maps := mapList.Items
	if maps[0].Version != made.Version || maps[0].Status != "draft" || !strings.Contains(maps[0].Note, "proposed by the assistant") || maps[1].Status != "retired" {
		t.Fatalf("versions: %+v", maps)
	}
	var doc struct{ Document map[string]any }
	boss.must(200, "GET", "/v1/apps/blog/maps/"+itoa(int64(made.Version)), nil, &doc)
	raw, _ := json.Marshal(doc.Document["assets"])
	if !strings.Contains(string(raw), "text/csv") {
		t.Fatalf("the draft lacks the change: %s", raw)
	}
	var ver struct{ Status string }
	boss.must(200, "GET", "/v1/apps/blog/maps/"+itoa(int64(made.Version))+"/verify", nil, &ver)
	if ver.Status != "queued" {
		t.Fatalf("the agent does not check the draft: %+v", ver)
	}
	// while the agent checks it, nothing goes on it
	model.rounds = [][]assistant.Call{{{ID: "3", Name: "propose_change", Input: json.RawMessage(`{"kind":"add_file_type","prefix":"/wp-content/uploads/","type":"image/avif","reason":"x"}`)}}}
	viewer.must(200, "POST", "/v1/apps/blog/assistant", ask, &out)
	if len(out.Proposals) != 0 || !strings.Contains(out.Text, "being checked") {
		t.Fatalf("proposed on a draft being checked: %+v", out)
	}

	// a version under review stops new drafts: one built beside it would undo it
	internalPost(t, e, "/v1/internal/verifications/claim", nil, &v)
	internalPost(t, e, "/v1/internal/verifications/"+itoa(v.ID)+"/result", map[string]any{"score": score}, nil)
	boss.must(200, "POST", "/v1/apps/blog/maps/"+itoa(int64(made.Version))+"/submit", nil, nil)
	model.rounds = [][]assistant.Call{{{ID: "4", Name: "propose_change", Input: json.RawMessage(`{"kind":"add_file_type","prefix":"/wp-content/uploads/","type":"image/avif","reason":"x"}`)}}}
	viewer.must(200, "POST", "/v1/apps/blog/assistant", ask, &out)
	if len(out.Proposals) != 0 || !strings.Contains(out.Text, "under review") {
		t.Fatalf("proposed beside a version under review: %+v", out)
	}

	// the history says a question was asked, never what
	var audit struct {
		Items []struct {
			Action string
			Detail map[string]any
		}
	}
	boss.must(200, "GET", "/v1/audit", nil, &audit)
	found := false
	for _, a := range audit.Items {
		b, _ := json.Marshal(a.Detail)
		if strings.Contains(string(b), "Why are PDFs") {
			t.Fatalf("the question is in the history: %s", b)
		}
		found = found || a.Action == "assistant.draft"
	}
	if !found {
		t.Fatalf("no assistant.draft in the history: %+v", audit)
	}

	// another tenant's site does not exist
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "other", "name": "Other", "admin_email": "eve@other.test", "admin_name": "Eve"}, nil)
	eve := e.join("eve@other.test")
	eve.must(404, "POST", "/v1/apps/blog/assistant", ask, nil)
	eve.must(404, "POST", "/v1/apps/blog/assistant/proposals/"+id+"/draft", nil, nil)
	boss.must(422, "POST", "/v1/apps/blog/assistant", map[string]any{"text": " "}, nil)
}

// remembers keeps what the model was given on each question.
type remembers struct{ seen [][]assistant.Turn }

func (m *remembers) Chat(_ context.Context, _ string, turns []assistant.Turn, _ []assistant.Tool) (assistant.Reply, error) {
	m.seen = append(m.seen, turns)
	return assistant.Reply{Text: "answer " + turns[len(turns)-1].Text}, nil
}

// A conversation is its author's only, goes on where it stopped, and lasts
// as long as the tenant keeps it.
func TestAssistantConversations(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ed@acme.test", "name": "Ed", "role": "admin"}, nil)
	ed := e.join("ed@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "shop", "name": "Shop", "public_domain": "shop.acme.test", "kind": "wordpress"}, nil)
	boss.must(200, "PUT", "/v1/tenants/acme/llm", map[string]any{"provider": "anthropic", "model": "claude-haiku-5-5", "api_key": "sk-test"}, nil)
	model := &remembers{}
	e.server.AssistantModel = func(assistant.Config) (assistant.Model, error) { return model, nil }

	var first struct{ Conversation, Text string }
	boss.must(200, "POST", "/v1/apps/blog/assistant", map[string]any{"text": "one"}, &first)
	var second struct{ Conversation string }
	boss.must(200, "POST", "/v1/apps/blog/assistant", map[string]any{"text": "two", "conversation": first.Conversation}, &second)
	if second.Conversation != first.Conversation || len(model.seen[1]) != 3 || model.seen[1][1].Text != "answer one" {
		t.Fatalf("the conversation did not go on: %+v %+v", second, model.seen)
	}
	// not someone else's, not on another site
	ed.must(404, "POST", "/v1/apps/blog/assistant", map[string]any{"text": "x", "conversation": first.Conversation}, nil)
	ed.must(404, "GET", "/v1/assistant/conversations/"+first.Conversation, nil, nil)
	ed.must(404, "DELETE", "/v1/assistant/conversations/"+first.Conversation, nil, nil)
	boss.must(404, "POST", "/v1/apps/shop/assistant", map[string]any{"text": "x", "conversation": first.Conversation}, nil)
	var edList struct{ Items []any }
	ed.must(200, "GET", "/v1/assistant/conversations", nil, &edList)
	if len(edList.Items) != 0 {
		t.Fatalf("another person's conversations: %+v", edList)
	}

	boss.must(200, "POST", "/v1/apps/shop/assistant", map[string]any{"text": "about the shop"}, nil)
	var mine struct {
		Items []struct{ ID, App, AppName, Title string }
	}
	boss.must(200, "GET", "/v1/assistant/conversations", nil, &mine)
	if len(mine.Items) != 2 || mine.Items[0].App != "shop" || mine.Items[1].Title != "one" {
		t.Fatalf("list: %+v", mine)
	}
	var conv struct {
		Conversation struct{ App string }
		Messages     []struct{ Role, Text string }
	}
	boss.must(200, "GET", "/v1/assistant/conversations/"+first.Conversation, nil, &conv)
	if conv.Conversation.App != "blog" || len(conv.Messages) != 4 || conv.Messages[3].Text != "answer two" {
		t.Fatalf("conversation: %+v", conv)
	}

	// the retention, from the tenant's settings
	var set struct {
		KeepDays int `json:"keep_days"`
	}
	ed.must(200, "GET", "/v1/tenants/acme/assistant", nil, &set)
	if set.KeepDays != 30 {
		t.Fatalf("default retention: %d", set.KeepDays)
	}
	boss.must(422, "PUT", "/v1/tenants/acme/assistant", map[string]int{"keep_days": 0}, nil)
	boss.must(200, "PUT", "/v1/tenants/acme/assistant", map[string]int{"keep_days": 7}, nil)
	if _, err := e.st.DB.Exec(`UPDATE assistant_conversations SET updated_at = now() - interval '8 days' WHERE id = $1`, first.Conversation); err != nil {
		t.Fatal(err)
	}
	boss.must(404, "GET", "/v1/assistant/conversations/"+first.Conversation, nil, nil)
	boss.must(404, "POST", "/v1/apps/blog/assistant", map[string]any{"text": "x", "conversation": first.Conversation}, nil)
	// and the hourly purge deletes it, even if nobody asks anything
	if err := e.st.PurgeAssistant(context.Background()); err != nil {
		t.Fatal(err)
	}
	var left int
	_ = e.st.DB.QueryRow(`SELECT count(*) FROM assistant_messages WHERE conversation_id = $1`, first.Conversation).Scan(&left)
	if left != 0 {
		t.Fatalf("%d messages past retention", left)
	}

	boss.must(204, "DELETE", "/v1/assistant/conversations/"+mine.Items[0].ID, nil, nil)
	boss.must(200, "GET", "/v1/assistant/conversations", nil, &mine)
	if len(mine.Items) != 0 {
		t.Fatalf("left: %+v", mine)
	}
}

type refuses struct{}

func (refuses) Chat(context.Context, string, []assistant.Turn, []assistant.Tool) (assistant.Reply, error) {
	return assistant.Reply{}, &assistant.ErrProvider{Msg: "the AI provider refused the API key: check it in Settings → AI"}
}

// A refusal of the provider is a 424 with words to act on: never a 502,
// whose body a proxy may replace.
func TestAssistantProviderRefusal(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, nil)
	boss.must(200, "PUT", "/v1/tenants/acme/llm", map[string]any{"provider": "anthropic", "model": "claude-haiku-5-5", "api_key": "sk-bad"}, nil)
	e.server.AssistantModel = func(assistant.Config) (assistant.Model, error) { return refuses{}, nil }
	var out struct{ Error, Message string }
	if code := boss.do("POST", "/v1/apps/blog/assistant", map[string]any{"text": "hi"}, &out); code != 424 {
		t.Fatalf("status %d", code)
	}
	if out.Error != "ai_failed" || !strings.Contains(out.Message, "Settings → AI") {
		t.Fatalf("body: %+v", out)
	}
}
