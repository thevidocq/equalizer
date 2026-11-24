package assistant

import (
	"errors"
	"fmt"
	"strings"
)

// narrow_field gives a free-text field of the map (a route's query key or
// a form's field) a narrower check: a number, a date, an e-mail, one of a
// few values, or text of a known shape. The shapes are Equalizer's own
// patterns, anchored: the model never writes a pattern. Only a field that
// takes free text can be narrowed, and only to something narrower; the
// agent then checks the draft on the site, as every other draft.

// shapes are the text shapes a field can be narrowed to.
var shapes = map[string]struct {
	pattern, what string
	max           int
}{
	"digits": {`^[0-9]+$`, "digits only", 40},
	"slug":   {`^[a-z0-9]+(?:[-_][a-z0-9]+)*$`, "a slug (lowercase letters, digits, - and _)", 200},
	"key":    {`^[A-Za-z0-9_.-]+$`, "a key (letters, digits, _ . -)", 200},
}

// NarrowKinds are the types narrow_field accepts.
var NarrowKinds = []string{"int", "date", "email", "enum", "bool", "digits", "slug", "key", "text"}

func (c Change) narrow(doc map[string]any) (string, error) {
	field, where, err := c.field(doc)
	if err != nil {
		return "", err
	}
	if !freeText(field) {
		return "", fmt.Errorf("%s is already checked as %v: only a field that takes free text is narrowed", where, field["type"])
	}
	if c.Spec == nil {
		return "", errors.New("say what the field takes: " + strings.Join(NarrowKinds, ", "))
	}
	required := field["required"]
	next, what, err := c.Spec.narrowed(field)
	if err != nil {
		return "", err
	}
	if required != nil {
		next["required"] = required
	}
	for k := range field {
		delete(field, k)
	}
	for k, v := range next {
		field[k] = v
	}
	return fmt.Sprintf("%s narrowed to %s", where, what), nil
}

// field finds the field the change names: route_id and key, or form_id and
// key.
func (c Change) field(doc map[string]any) (map[string]any, string, error) {
	if c.Key == "" {
		return nil, "", errors.New("name the field: key, with route_id or form_id")
	}
	switch {
	case c.RouteID != "":
		for _, x := range list(doc["routes"]) {
			r, _ := x.(map[string]any)
			if r == nil || r["id"] != c.RouteID {
				continue
			}
			q, _ := r["query"].(map[string]any)
			if f, ok := q[c.Key].(map[string]any); ok {
				return f, fmt.Sprintf("?%s= of route %s", c.Key, c.RouteID), nil
			}
			return nil, "", fmt.Errorf("route %s has no query key %q", c.RouteID, c.Key)
		}
		return nil, "", fmt.Errorf("the map has no route %q", c.RouteID)
	case c.FormID != "":
		for _, x := range list(doc["forms"]) {
			f, _ := x.(map[string]any)
			if f == nil || f["id"] != c.FormID {
				continue
			}
			fields, _ := f["fields"].(map[string]any)
			if spec, ok := fields[c.Key].(map[string]any); ok {
				return spec, fmt.Sprintf("field %s of form %s", c.Key, c.FormID), nil
			}
			return nil, "", fmt.Errorf("form %s has no field %q", c.FormID, c.Key)
		}
		return nil, "", fmt.Errorf("the map has no form %q", c.FormID)
	}
	return nil, "", errors.New("name the route (route_id) or the form (form_id) of the field")
}

// freeText: a field that takes any text (string or text, no pattern and no
// list of values).
func freeText(f map[string]any) bool {
	t, _ := f["type"].(string)
	_, pattern := f["pattern"]
	_, values := f["values"]
	return (t == "string" || t == "text") && !pattern && !values
}

// narrowed is the new check; what says it in words.
func (s *Spec) narrowed(old map[string]any) (map[string]any, string, error) {
	n := s.MaxLength
	if have, ok := old["max_length"].(float64); ok && (n == 0 || float64(n) > have) {
		n = int(have) // never longer than it was
	}
	switch s.Type {
	case "int", "date", "enum":
		return s.field()
	case "email":
		return map[string]any{"type": "email", "max_length": 254}, "an e-mail address", nil
	case "bool":
		return map[string]any{"type": "bool"}, "yes or no (true, false, 1, 0)", nil
	case "text":
		if n <= 0 || n > 5000 {
			return nil, "", errors.New("text: give max_length, the longest value the site takes (5000 at most)")
		}
		if have, ok := old["max_length"].(float64); ok && float64(n) >= have {
			return nil, "", fmt.Errorf("text of %d characters is not narrower than the %d it takes now", n, int(have))
		}
		f := map[string]any{"type": old["type"], "max_length": n, "rules": rulesOf(old)}
		return f, fmt.Sprintf("text of %d characters at most", n), nil
	}
	shape, ok := shapes[s.Type]
	if !ok {
		return nil, "", fmt.Errorf("type %q: %s", s.Type, strings.Join(NarrowKinds, ", "))
	}
	if n <= 0 || n > shape.max {
		n = shape.max
	}
	f := map[string]any{"type": "string", "pattern": shape.pattern, "max_length": n, "rules": rulesOf(old)}
	return f, fmt.Sprintf("%s, %d characters at most", shape.what, n), nil
}

// rulesOf keeps the field's rules, and the two every text field has.
func rulesOf(old map[string]any) []any {
	out := []any{}
	for _, r := range list(old["rules"]) {
		out = append(out, r)
	}
	for _, r := range []string{"no_html", "no_template_meta"} {
		if !containsAny(out, r) {
			out = append(out, r)
		}
	}
	return out
}

func containsAny(l []any, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}
