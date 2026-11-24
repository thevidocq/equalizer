package assistant

import (
	"encoding/json"
	"strings"
	"testing"
)

func narrowMap() map[string]any {
	var doc map[string]any
	_ = json.Unmarshal([]byte(`{
		"routes": [{"id": "product", "path": "/product/{slug}/", "legacy_request": {"method": "GET", "path": "/product/{{params.slug}}/"},
			"query": {"attribute_pa_color": {"type": "string", "max_length": 200, "rules": ["no_html", "no_template_meta"], "required": false},
			          "add-to-cart": {"type": "int", "required": false},
			          "code": {"type": "string", "charset": "[a-z]", "max_length": 50},
			          "name": {"type": "string", "deny": ["admin"]}}}],
		"defaults": {"string": {"max_length": 60, "charset": "[a-z0-9 _-]"}},
		"forms": [{"id": "review", "route": "/review", "fields": {
			"rating": {"type": "string", "max_length": 100, "rules": ["no_html"], "required": true},
			"comment": {"type": "text", "max_length": 5000, "rules": ["no_html", "no_template_meta"], "required": true}}}]
	}`), &doc)
	return doc
}

func TestNarrowField(t *testing.T) {
	cases := []struct {
		name string
		c    Change
		ok   bool
		want string
	}{
		{"slug", Change{Kind: "narrow_field", RouteID: "product", Key: "attribute_pa_color", Spec: &Spec{Type: "slug", MaxLength: 40}}, true, "a slug"},
		{"enum", Change{Kind: "narrow_field", RouteID: "product", Key: "attribute_pa_color", Spec: &Spec{Type: "enum", Values: []string{"red", "blue"}}}, true, "one of red, blue"},
		{"form int", Change{Kind: "narrow_field", FormID: "review", Key: "rating", Spec: &Spec{Type: "int", Min: ptr(1), Max: ptr(5)}}, true, "whole number"},
		{"shorter text", Change{Kind: "narrow_field", FormID: "review", Key: "comment", Spec: &Spec{Type: "text", MaxLength: 2000}}, true, "2000 characters"},
		{"longer text", Change{Kind: "narrow_field", FormID: "review", Key: "comment", Spec: &Spec{Type: "text", MaxLength: 6000}}, false, "not narrower"},
		{"already typed", Change{Kind: "narrow_field", RouteID: "product", Key: "add-to-cart", Spec: &Spec{Type: "digits"}}, false, "only a field that takes free text"},
		{"already a set of characters", Change{Kind: "narrow_field", RouteID: "product", Key: "code", Spec: &Spec{Type: "key"}}, false, "only a field that takes free text"},
		{"to free text", Change{Kind: "narrow_field", RouteID: "product", Key: "attribute_pa_color", Spec: &Spec{Type: "string"}}, false, "type \"string\""},
		{"a pattern of its own", Change{Kind: "narrow_field", RouteID: "product", Key: "attribute_pa_color", Spec: &Spec{Type: ".*"}}, false, "type"},
		{"no field", Change{Kind: "narrow_field", RouteID: "product", Key: "nope", Spec: &Spec{Type: "int"}}, false, "no query key"},
		{"no route nor form", Change{Kind: "narrow_field", Key: "q", Spec: &Spec{Type: "int"}}, false, "name the route"},
	}
	for _, c := range cases {
		got, err := c.c.Apply(narrowMap())
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
	doc := narrowMap()
	if _, err := (Change{Kind: "narrow_field", RouteID: "product", Key: "attribute_pa_color", Spec: &Spec{Type: "slug", MaxLength: 500}}).Apply(doc); err != nil {
		t.Fatal(err)
	}
	f := doc["routes"].([]any)[0].(map[string]any)["query"].(map[string]any)["attribute_pa_color"].(map[string]any)
	if f["pattern"] != shapes["slug"].pattern || f["max_length"] != 200 || f["required"] != false || len(f["rules"].([]any)) != 2 {
		t.Fatalf("slug never longer than it was, required kept, rules kept: %v", f)
	}
	doc = narrowMap()
	_, _ = (Change{Kind: "narrow_field", FormID: "review", Key: "rating", Spec: &Spec{Type: "enum", Values: []string{"1", "2", "3"}}}).Apply(doc)
	r := doc["forms"].([]any)[0].(map[string]any)["fields"].(map[string]any)["rating"].(map[string]any)
	if r["type"] != "enum" || r["required"] != true || r["max_length"] != nil {
		t.Fatalf("form field: %v", r)
	}
}

// Never wider than it was: length, required, refused values, the map's
// default characters.
func TestNarrowNeverWidens(t *testing.T) {
	doc := narrowMap()
	if _, err := (Change{Kind: "narrow_field", RouteID: "product", Key: "name", Spec: &Spec{Type: "email"}}).Apply(doc); err != nil {
		t.Fatal(err)
	}
	f := doc["routes"].([]any)[0].(map[string]any)["query"].(map[string]any)["name"].(map[string]any)
	if f["max_length"] != 60 || f["charset"] != "[a-z0-9 _-]" || f["deny"] == nil || f["required"] != nil {
		t.Fatalf("email from a default string: %v", f)
	}
	doc = narrowMap()
	if _, err := (Change{Kind: "narrow_field", RouteID: "product", Key: "name", Spec: &Spec{Type: "int"}}).Apply(doc); err != nil {
		t.Fatal(err)
	}
	f = doc["routes"].([]any)[0].(map[string]any)["query"].(map[string]any)["name"].(map[string]any)
	if _, set := f["required"]; set {
		t.Fatalf("a required field became optional: %v", f)
	}
	if _, err := (Change{Kind: "narrow_field", RouteID: "product", Key: "attribute_pa_color", Spec: &Spec{Type: "text", MaxLength: 300}}).Apply(narrowMap()); err == nil {
		t.Fatal("text longer than the field takes")
	}
}

func ptr(f float64) *float64 { return &f }
