package placeholder

import "testing"

func TestResolve(t *testing.T) {
	vals := Values{"params": {"id": "42"}, "query": {"stato": "aperta"}}
	cases := []struct {
		tpl, want string
		absent    bool
		err       bool
	}{
		{"INT", "INT", false, false},
		{"{{params.id}}", "42", false, false},
		{"{{ query.stato }}", "aperta", false, false},
		{"/pratiche/{{params.id}}/allegati", "/pratiche/42/allegati", false, false},
		{"{{query.pagina}}", "", true, false},
		{"p-{{query.pagina}}", "", false, true},
		{"{{segreto.x}}", "", false, true},
		{"{{params.id", "", false, true},
		{"valore}}", "", false, true},
	}
	for _, c := range cases {
		got, absent, err := Resolve(c.tpl, vals)
		if (err != nil) != c.err || absent != c.absent || (err == nil && got != c.want) {
			t.Errorf("%q: (%q, %v, %v), atteso (%q, %v, errore=%v)", c.tpl, got, absent, err, c.want, c.absent, c.err)
		}
	}
}

func TestCheck(t *testing.T) {
	if err := Check("/x/{{params.id}}"); err != nil {
		t.Errorf("template valido rifiutato: %v", err)
	}
	if err := Check("{{env.PATH}}"); err == nil {
		t.Error("spazio non ammesso accettato")
	}
}
