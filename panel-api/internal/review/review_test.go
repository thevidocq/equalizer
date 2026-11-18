package review

import (
	"strings"
	"testing"
)

const base = `{"legacy":{"base_url":"https://a.example","reach":{"mode":"connector","connector_id":"cn-1","allow":["10.0.0.1:443"]}},
"roles":{"anonymous":"guest","list":{"guest":{},"citizen":{}}},
"defaults":{"rate_limit":{"per_ip":"60/min"}},
"types":{"permit_id":{"base":"int","min":1,"max":1000},"slug":{"base":"string","pattern":"^[a-z-]+$","deny":["admin","wp-admin"]}},
"routes":[
 {"id":"home","path":"/","method":"GET","roles":["guest"]},
 {"id":"permit","path":"/permits/{id}","method":"GET","roles":["citizen"],"params":{"id":{"type":"permit_id"}},"authz":{"citizen":{"check":"owned_by_user","target":"params.id"}}},
 {"id":"old","path":"/old","method":"GET","roles":["guest"]}],
"forms":[{"id":"search","route":"/search","roles":["guest"],"fields":{"q":{"type":"string","max_length":50,"pattern":"^[a-z ]+$"}}}],
"assets":[{"prefix":"/css/","types":["text/css"]}]}`

func TestCompare(t *testing.T) {
	next := strings.NewReplacer(
		`"per_ip":"60/min"`, `"per_ip":"600/min"`,
		`"roles":["citizen"],"params":{"id":{"type":"permit_id"}},"authz":{"citizen":{"check":"owned_by_user","target":"params.id"}}`,
		`"roles":["citizen","guest"],"params":{"id":{"type":"string"}}`,
		`"max_length":50,"pattern":"^[a-z ]+$"`, `"max_length":500`,
		`"deny":["admin","wp-admin"]`, `"deny":["admin"]`,
		`,
 {"id":"old","path":"/old","method":"GET","roles":["guest"]}]`, `,
 {"id":"backup","path":"/backup/{file}","method":"GET","roles":["guest"],"params":{"file":{"type":"slug"}}},
 {"id":"account","path":"/account/{id}","method":"GET","roles":["guest"],"params":{"id":{"type":"permit_id"}}}]`,
		`"types":["text/css"]}]`, `"types":["text/css"]},{"prefix":"/uploads/","types":["text/html"]}]`,
	).Replace(base)
	r := Compare([]byte(base), []byte(next))
	rules := map[string]string{}
	for _, f := range r.Findings {
		rules[f.Rule+" "+f.Entry] = f.Severity
	}
	for want, sev := range map[string]string{
		"type_widened routes.permit.params.id":  "high",
		"authz_removed routes.permit":           "high",
		"opened_to_anonymous routes.permit":     "medium",
		"rate_limit_raised defaults.rate_limit": "medium",
		"pattern_removed forms.search.fields.q": "medium",
		"longer forms.search.fields.q":          "medium",
		"deny_removed types.slug":               "high",
		"sensitive_path routes.backup":          "high",
		"personal_without_authz routes.account": "medium",
		"asset_type assets./uploads/":           "high",
		"removed routes.old":                    "low",
	} {
		if rules[want] != sev {
			t.Errorf("%s: got %q, want %q", want, rules[want], sev)
		}
	}
	if r.Findings[0].Severity != "high" {
		t.Error("findings must start with the highest severity")
	}
	if len(r.Changes) < 6 {
		t.Errorf("changes: %+v", r.Changes)
	}
	// No published map: everything is added, the rules still apply.
	first := Compare(nil, []byte(next))
	if len(first.Changes) == 0 || first.Changes[0].Change != "added" {
		t.Errorf("first version: %+v", first.Changes)
	}
	same := Compare([]byte(base), []byte(base))
	if len(same.Changes) != 0 || len(same.Findings) != 0 {
		t.Errorf("identical maps: %+v", same)
	}
}

func TestIssuesAdded(t *testing.T) {
	old := `{"api":[{"id":"list","path":"/api/p","method":"GET","roles":["g"],"response":{"fields":{"id":{"type":"int"}}}}]}`
	next := strings.Replace(old, `"fields":{"id":{"type":"int"}}`, `"fields":{"id":{"type":"int"}},"issues":{"id":"get.params.id"}`, 1)
	for _, f := range Compare([]byte(old), []byte(next)).Findings {
		if f.Rule == "issues_added" && f.Entry == "api.list" && f.Severity == "medium" {
			return
		}
	}
	t.Error("issues_added not found")
}
