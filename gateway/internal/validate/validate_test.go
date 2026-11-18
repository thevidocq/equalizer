package validate

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

func f(v float64) *float64 { return &v }
func b(v bool) *bool       { return &v }

func testValidator(t *testing.T) *Validator {
	t.Helper()
	return New(map[string]mapmodel.TypeDef{
		"permit_id":     {Base: "int", Min: f(1), Max: f(99999999)},
		"permit_status": {Base: "enum", Values: []string{"open", "closed"}},
	}, mapmodel.StringDefaults{MaxLength: 20, Charset: `[\p{L}\p{N} .,'-]`})
}

func reason(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Reason
	}
	return ""
}

func TestValues(t *testing.T) {
	v := testValidator(t)
	cases := []struct {
		name   string
		spec   mapmodel.FieldSpec
		raw    string
		want   string
		reason string
	}{
		{"valid int", mapmodel.FieldSpec{Type: "permit_id"}, "42", "42", ""},
		{"int with leading zeros", mapmodel.FieldSpec{Type: "permit_id"}, "0042", "42", ""},
		{"int out of range", mapmodel.FieldSpec{Type: "permit_id"}, "0", "", ReasonRange},
		{"sql injection on int", mapmodel.FieldSpec{Type: "permit_id"}, "1 OR 1=1", "", ReasonType},
		{"path traversal on int", mapmodel.FieldSpec{Type: "permit_id"}, "../etc", "", ReasonType},
		{"valid enum", mapmodel.FieldSpec{Type: "permit_status"}, "open", "open", ""},
		{"enum not allowed", mapmodel.FieldSpec{Type: "permit_status"}, "all", "", ReasonValue},
		{"bool", mapmodel.FieldSpec{Type: "bool"}, "1", "true", ""},
		{"date", mapmodel.FieldSpec{Type: "date"}, "2026-02-30", "", ReasonType},
		{"string with default charset", mapmodel.FieldSpec{Type: "string"}, "O'Brien Mary", "O'Brien Mary", ""},
		{"string with forbidden characters", mapmodel.FieldSpec{Type: "string"}, "<script>", "", ReasonCharset},
		{"string too long", mapmodel.FieldSpec{Type: "string"}, "abcdefghijklmnopqrstuvwxyz", "", ReasonLength},
		{"string with pattern", mapmodel.FieldSpec{Type: "string", Pattern: `^[A-Z0-9]{16}$`}, "RSSMRA80A01H501U", "RSSMRA80A01H501U", ""},
		{"pattern not respected", mapmodel.FieldSpec{Type: "string", Pattern: `^[A-Z0-9]{16}$`}, "abc", "", ReasonPattern},
		{"email", mapmodel.FieldSpec{Type: "email"}, "m.smith@example.org", "m.smith@example.org", ""},
		{"invalid email", mapmodel.FieldSpec{Type: "email"}, "x@y", "", ReasonPattern},
		{"control characters", mapmodel.FieldSpec{Type: "string"}, "ab\x00c", "", ReasonCharset},
	}
	for _, c := range cases {
		got, _, err := v.Value("field", c.spec, c.raw, true)
		if reason(err) != c.reason {
			t.Errorf("%s: reason %q, want %q", c.name, reason(err), c.reason)
			continue
		}
		if c.reason == "" && got != c.want {
			t.Errorf("%s: value %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTextRules(t *testing.T) {
	v := testValidator(t)
	spec := mapmodel.FieldSpec{Type: "text", MaxLength: 200, Charset: `[\p{L}\p{N}\s.,;:!?()'-]`,
		Rules: []string{"no_sql_meta", "no_html", "no_template_meta"}}
	if err := v.Check(map[string]mapmodel.FieldSpec{"message": spec}); err != nil {
		t.Fatal(err)
	}
	ok := []string{"Attached is the receipt from O'Brien.", "Line one\nline two"}
	bad := []string{"' OR '1'='1", "x'; DROP TABLE permits", "test -- comment", "${jndi:ldap://x}", "{{7*7}}"}
	for _, s := range ok {
		if _, _, err := v.Value("message", spec, s, true); err != nil {
			t.Errorf("wrongly rejected %q: %v", s, err)
		}
	}
	for _, s := range bad {
		if _, _, err := v.Value("message", spec, s, true); err == nil {
			t.Errorf("wrongly accepted %q", s)
		}
	}
}

func TestFields(t *testing.T) {
	v := testValidator(t)
	specs := map[string]mapmodel.FieldSpec{
		"status": {Type: "permit_status", Required: b(false)},
		"page":   {Type: "int", Min: f(1), Max: f(500), Default: float64(1)},
	}
	got, err := v.Fields(specs, url.Values{"status": {"open"}, "debug": {"1"}})
	if err != nil {
		t.Fatal(err)
	}
	if got["status"] != "open" || got["page"] != "1" {
		t.Fatalf("unexpected values: %v", got)
	}
	if _, ok := got["debug"]; ok {
		t.Fatal("undeclared parameter not dropped")
	}
	if _, err := v.Fields(specs, url.Values{"status": {"open", "closed"}}); reason(err) != ReasonDuplicate {
		t.Fatalf("repeated parameter not rejected: %v", err)
	}
	if _, err := v.Fields(map[string]mapmodel.FieldSpec{"id": {Type: "permit_id"}}, url.Values{}); reason(err) != ReasonMissing {
		t.Fatalf("missing required parameter not rejected: %v", err)
	}
}

func TestCheckRejectsBadMaps(t *testing.T) {
	v := testValidator(t)
	cases := map[string]mapmodel.FieldSpec{
		"unknown type":    {Type: "mystery_code"},
		"invalid pattern": {Type: "string", Pattern: "(["},
		"unknown rule":    {Type: "text", Rules: []string{"no_everything"}},
	}
	for name, spec := range cases {
		if err := v.Check(map[string]mapmodel.FieldSpec{"x": spec}); err == nil {
			t.Errorf("%s: map wrongly accepted", name)
		}
	}
}

func TestPassword(t *testing.T) {
	v := New(nil, mapmodel.StringDefaults{Charset: `[a-z]`, MaxLength: 5})
	spec := mapmodel.FieldSpec{Type: "password", Rules: []string{"no_template_meta"}}
	// The string defaults do not apply: a password may hold any printable character.
	if got, _, err := v.Value("p", spec, "S3cret! 'é\"#", true); err != nil || got != "S3cret! 'é\"#" {
		t.Fatalf("password rejected: %v", err)
	}
	for _, bad := range []string{"a\x00b", "line\nbreak", "{{7*7}}", strings.Repeat("x", 129)} {
		if _, _, err := v.Value("p", spec, bad, true); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestDeny(t *testing.T) {
	v := New(map[string]mapmodel.TypeDef{"slug": {Base: "string", Pattern: "^[a-z-]+$", Deny: []string{"wp-admin"}}}, mapmodel.StringDefaults{})
	for value, ok := range map[string]bool{"about": true, "wp-admin": false, "WP-ADMIN": false} {
		_, _, err := v.Value("slug", mapmodel.FieldSpec{Type: "slug", Pattern: "^[A-Za-z-]+$"}, value, true)
		if (err == nil) != ok {
			t.Errorf("%s: %v", value, err)
		}
	}
	if _, _, err := v.Value("slug", mapmodel.FieldSpec{Type: "slug", Deny: []string{"about"}}, "about", true); err == nil {
		t.Error("a field's own deny list is ignored")
	}
}
