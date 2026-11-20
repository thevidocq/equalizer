package store

import (
	"strings"
	"testing"
)

func TestSetMetaDropsTheRetiredRisk(t *testing.T) {
	in := `{"routes":[{"id":"home","risk":"low"}],"forms":[{"id":"f","risk":"high",` +
		`"fields":{"risk":{"type":"enum","values":["low","high"]},"q":{"type":"int","risk":"medium"}}}],` +
		`"areas":[{"id":"site","prefix":"/","risk":"medium"}]}`
	out, err := SetMeta([]byte(in), map[string]any{"version": 2})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, `"risk":"`) {
		t.Errorf("risk left: %s", s)
	}
	if !strings.Contains(s, `"risk":{"type":"enum"`) { // a form field named risk stays
		t.Errorf("the field named risk was removed: %s", s)
	}
}
