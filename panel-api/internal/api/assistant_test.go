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
	ask := map[string]any{"messages": []map[string]string{{"role": "user", "text": "Why are PDFs refused?"}}}

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
	boss.must(422, "POST", "/v1/apps/blog/assistant", map[string]any{"messages": []map[string]string{{"role": "assistant", "text": "x"}}}, nil)
}
