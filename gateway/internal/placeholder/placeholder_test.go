package placeholder

import "testing"

func TestResolve(t *testing.T) {
	vals := Values{"params": {"id": "42"}, "query": {"status": "open"}}
	cases := []struct {
		tpl, want string
		absent    bool
		err       bool
	}{
		{"INT", "INT", false, false},
		{"{{params.id}}", "42", false, false},
		{"{{ query.status }}", "open", false, false},
		{"/permits/{{params.id}}/files", "/permits/42/files", false, false},
		{"{{query.page}}", "", true, false},
		{"p-{{query.page}}", "", false, true},
		{"{{secret.x}}", "", false, true},
		{"{{params.id", "", false, true},
		{"value}}", "", false, true},
	}
	for _, c := range cases {
		got, absent, err := Resolve(c.tpl, vals)
		if (err != nil) != c.err || absent != c.absent || (err == nil && got != c.want) {
			t.Errorf("%q: (%q, %v, %v), want (%q, %v, error=%v)", c.tpl, got, absent, err, c.want, c.absent, c.err)
		}
	}
}

func TestCheck(t *testing.T) {
	if err := Check("/x/{{params.id}}"); err != nil {
		t.Errorf("valid template rejected: %v", err)
	}
	if err := Check("{{env.PATH}}"); err == nil {
		t.Error("disallowed namespace accepted")
	}
}

func TestSingle(t *testing.T) {
	cases := map[string][3]string{
		"{{params.id}}":    {"params", "id", "ok"},
		" {{ hidden.vs }}": {"hidden", "vs", "ok"},
		"p-{{params.id}}":  {"", "", ""},
		"fixed":            {"", "", ""},
		"{{bogus.x}}":      {"", "", ""},
	}
	for tpl, want := range cases {
		ns, name, ok := Single(tpl)
		if ns != want[0] || name != want[1] || ok != (want[2] == "ok") {
			t.Errorf("Single(%q) = %q, %q, %v", tpl, ns, name, ok)
		}
	}
}
