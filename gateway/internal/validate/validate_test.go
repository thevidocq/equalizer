package validate

import (
	"errors"
	"net/url"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

func f(v float64) *float64 { return &v }
func b(v bool) *bool       { return &v }

func testValidator(t *testing.T) *Validator {
	t.Helper()
	return New(map[string]mapmodel.TypeDef{
		"id_pratica":    {Base: "int", Min: f(1), Max: f(99999999)},
		"stato_pratica": {Base: "enum", Values: []string{"aperta", "chiusa"}},
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
		{"int valido", mapmodel.FieldSpec{Type: "id_pratica"}, "42", "42", ""},
		{"int con zeri", mapmodel.FieldSpec{Type: "id_pratica"}, "0042", "42", ""},
		{"int fuori intervallo", mapmodel.FieldSpec{Type: "id_pratica"}, "0", "", ReasonRange},
		{"sql injection su int", mapmodel.FieldSpec{Type: "id_pratica"}, "1 OR 1=1", "", ReasonType},
		{"path traversal su int", mapmodel.FieldSpec{Type: "id_pratica"}, "../etc", "", ReasonType},
		{"enum valido", mapmodel.FieldSpec{Type: "stato_pratica"}, "aperta", "aperta", ""},
		{"enum non ammesso", mapmodel.FieldSpec{Type: "stato_pratica"}, "tutte", "", ReasonValue},
		{"bool", mapmodel.FieldSpec{Type: "bool"}, "1", "true", ""},
		{"data", mapmodel.FieldSpec{Type: "date"}, "2026-02-30", "", ReasonType},
		{"stringa con default charset", mapmodel.FieldSpec{Type: "string"}, "D'Angelo Maria", "D'Angelo Maria", ""},
		{"stringa con caratteri vietati", mapmodel.FieldSpec{Type: "string"}, "<script>", "", ReasonCharset},
		{"stringa troppo lunga", mapmodel.FieldSpec{Type: "string"}, "abcdefghijklmnopqrstuvwxyz", "", ReasonLength},
		{"stringa con pattern", mapmodel.FieldSpec{Type: "string", Pattern: `^[A-Z0-9]{16}$`}, "RSSMRA80A01H501U", "RSSMRA80A01H501U", ""},
		{"pattern non rispettato", mapmodel.FieldSpec{Type: "string", Pattern: `^[A-Z0-9]{16}$`}, "abc", "", ReasonPattern},
		{"email", mapmodel.FieldSpec{Type: "email"}, "m.rossi@comune.it", "m.rossi@comune.it", ""},
		{"email non valida", mapmodel.FieldSpec{Type: "email"}, "x@y", "", ReasonPattern},
		{"caratteri di controllo", mapmodel.FieldSpec{Type: "string"}, "ab\x00c", "", ReasonCharset},
	}
	for _, c := range cases {
		got, _, err := v.Value("campo", c.spec, c.raw, true)
		if reason(err) != c.reason {
			t.Errorf("%s: motivo %q, atteso %q", c.name, reason(err), c.reason)
			continue
		}
		if c.reason == "" && got != c.want {
			t.Errorf("%s: valore %q, atteso %q", c.name, got, c.want)
		}
	}
}

func TestTextRules(t *testing.T) {
	v := testValidator(t)
	spec := mapmodel.FieldSpec{Type: "text", MaxLength: 200, Charset: `[\p{L}\p{N}\s.,;:!?()'-]`,
		Rules: []string{"no_sql_meta", "no_html", "no_template_meta"}}
	if err := v.Check(map[string]mapmodel.FieldSpec{"messaggio": spec}); err != nil {
		t.Fatal(err)
	}
	ok := []string{"Allego la ricevuta di D'Angelo.", "Riga uno\nriga due"}
	bad := []string{"' OR '1'='1", "x'; DROP TABLE pratiche", "test -- commento", "${jndi:ldap://x}", "{{7*7}}"}
	for _, s := range ok {
		if _, _, err := v.Value("messaggio", spec, s, true); err != nil {
			t.Errorf("rifiutato a torto %q: %v", s, err)
		}
	}
	for _, s := range bad {
		if _, _, err := v.Value("messaggio", spec, s, true); err == nil {
			t.Errorf("accettato a torto %q", s)
		}
	}
}

func TestFields(t *testing.T) {
	v := testValidator(t)
	specs := map[string]mapmodel.FieldSpec{
		"stato":  {Type: "stato_pratica", Required: b(false)},
		"pagina": {Type: "int", Min: f(1), Max: f(500), Default: float64(1)},
	}
	got, err := v.Fields(specs, url.Values{"stato": {"aperta"}, "debug": {"1"}})
	if err != nil {
		t.Fatal(err)
	}
	if got["stato"] != "aperta" || got["pagina"] != "1" {
		t.Fatalf("valori inattesi: %v", got)
	}
	if _, ok := got["debug"]; ok {
		t.Fatal("parametro non dichiarato non scartato")
	}
	if _, err := v.Fields(specs, url.Values{"stato": {"aperta", "chiusa"}}); reason(err) != ReasonDuplicate {
		t.Fatalf("parametro ripetuto non rifiutato: %v", err)
	}
	if _, err := v.Fields(map[string]mapmodel.FieldSpec{"id": {Type: "id_pratica"}}, url.Values{}); reason(err) != ReasonMissing {
		t.Fatalf("parametro obbligatorio mancante non rifiutato: %v", err)
	}
}

func TestCheckRejectsBadMaps(t *testing.T) {
	v := testValidator(t)
	cases := map[string]mapmodel.FieldSpec{
		"tipo sconosciuto":   {Type: "codice_misterioso"},
		"pattern non valido": {Type: "string", Pattern: "(["},
		"regola sconosciuta": {Type: "text", Rules: []string{"no_tutto"}},
	}
	for name, spec := range cases {
		if err := v.Check(map[string]mapmodel.FieldSpec{"x": spec}); err == nil {
			t.Errorf("%s: mappa accettata a torto", name)
		}
	}
}
