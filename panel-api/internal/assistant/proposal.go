package assistant

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Change is a change to a map the assistant may propose. A person turns it
// into a draft; nothing else of the model's words ever reaches a map, and
// only these kinds, checked here, can.
type Change struct {
	Kind    string   `json:"kind"` // add_file_type, add_query_key, add_enum_values, narrow_field
	Prefix  string   `json:"prefix,omitempty"`
	Type    string   `json:"type,omitempty"`
	RouteID string   `json:"route_id,omitempty"`
	FormID  string   `json:"form_id,omitempty"`
	Key     string   `json:"key,omitempty"`
	Spec    *Spec    `json:"spec,omitempty"`
	Values  []string `json:"values,omitempty"`
	Reason  string   `json:"reason,omitempty"`
}

// Spec is the check of a new query key (a number, a date, one of a few
// values, or text without markup) or of a narrowed field (NarrowKinds).
type Spec struct {
	Type      string   `json:"type"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Values    []string `json:"values,omitempty"`
	MaxLength int      `json:"max_length,omitempty"`
}

var (
	mimeRe  = regexp.MustCompile(`^[a-z]+/[a-z0-9.+-]{1,80}$`)
	keyRe   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`) // the map's identifiers
	valueRe = regexp.MustCompile(`^[A-Za-z0-9_.,:-]{1,50}$`)
	// ErrSame: the map already has it.
	ErrSame = errors.New("the map already has it")
)

// risky types run in the browser or on the server, or say nothing of what
// they are: never added by the assistant, whoever asks.
var risky = []string{"javascript", "ecmascript", "html", "php", "x-sh", "octet-stream", "x-msdownload", "java-archive"}

// Apply makes the change on doc (a map document) and says what it did.
// An error says why it cannot, in words for the model and the person.
func (c Change) Apply(doc map[string]any) (string, error) {
	switch c.Kind {
	case "add_file_type":
		return c.fileType(doc)
	case "add_query_key":
		return c.queryKey(doc)
	case "add_enum_values":
		return c.enumValues(doc)
	case "narrow_field":
		return c.narrow(doc)
	}
	return "", fmt.Errorf("unknown kind %q: add_file_type, add_query_key, add_enum_values or narrow_field", c.Kind)
}

func (c Change) fileType(doc map[string]any) (string, error) {
	t := strings.ToLower(strings.TrimSpace(c.Type))
	if !mimeRe.MatchString(t) {
		return "", fmt.Errorf("%q is not a media type such as application/pdf", c.Type)
	}
	for _, r := range risky {
		if strings.Contains(t, r) {
			return "", fmt.Errorf("%s files can run code or hide what they are: the assistant never adds them", t)
		}
	}
	for _, x := range list(doc["assets"]) {
		a, _ := x.(map[string]any)
		if a == nil || a["prefix"] != c.Prefix {
			continue
		}
		types := list(a["types"])
		if slices.Contains(types, any(t)) {
			return "", ErrSame
		}
		a["types"] = append(types, t)
		return fmt.Sprintf("%s added to the folder %s", t, c.Prefix), nil
	}
	return "", fmt.Errorf("the map has no file folder %q", c.Prefix)
}

func (c Change) route(doc map[string]any) (map[string]any, map[string]any, error) {
	for _, x := range list(doc["routes"]) {
		r, _ := x.(map[string]any)
		if r == nil || r["id"] != c.RouteID {
			continue
		}
		lr, _ := r["legacy_request"].(map[string]any)
		if lr == nil || (r["method"] != nil && r["method"] != "GET") || (lr["method"] != nil && lr["method"] != "GET") {
			return nil, nil, fmt.Errorf("route %s does not take query keys", c.RouteID)
		}
		return r, lr, nil
	}
	return nil, nil, fmt.Errorf("the map has no route %q", c.RouteID)
}

func (c Change) queryKey(doc map[string]any) (string, error) {
	if !keyRe.MatchString(c.Key) {
		return "", fmt.Errorf("%q: the map takes lowercase keys (a-z, 0-9, _) only", c.Key)
	}
	r, lr, err := c.route(doc)
	if err != nil {
		return "", err
	}
	q, _ := r["query"].(map[string]any)
	params, _ := r["params"].(map[string]any)
	lq, _ := lr["query"].(map[string]any)
	if _, ok := q[c.Key]; ok {
		return "", ErrSame
	}
	if _, ok := params[c.Key]; ok {
		return "", fmt.Errorf("%s is a path parameter of route %s", c.Key, c.RouteID)
	}
	if _, ok := lq[c.Key]; ok {
		return "", fmt.Errorf("the site already gets a %s key on route %s", c.Key, c.RouteID)
	}
	spec, what, err := c.Spec.field()
	if err != nil {
		return "", err
	}
	if q == nil {
		q = map[string]any{}
	}
	if lq == nil {
		lq = map[string]any{}
	}
	q[c.Key] = spec
	lq[c.Key] = "{{query." + c.Key + "}}"
	r["query"], lr["query"] = q, lq
	return fmt.Sprintf("?%s= added to route %s, as %s", c.Key, c.RouteID, what), nil
}

// field is the map's field spec: never free text with markup or templates.
func (s *Spec) field() (map[string]any, string, error) {
	if s == nil {
		return nil, "", errors.New("say what the key takes: int, date, enum (with its values) or string")
	}
	switch s.Type {
	case "int":
		f := map[string]any{"type": "int", "required": false}
		if s.Min != nil {
			f["min"] = *s.Min
		}
		if s.Max != nil {
			f["max"] = *s.Max
		}
		return f, "a whole number", nil
	case "date":
		return map[string]any{"type": "date", "required": false}, "a date", nil
	case "enum":
		vals, err := tokens(s.Values, nil)
		if err != nil {
			return nil, "", err
		}
		return map[string]any{"type": "enum", "values": anys(vals), "required": false}, "one of " + strings.Join(vals, ", "), nil
	case "string":
		return map[string]any{"type": "string", "max_length": 200, "required": false,
			"rules": []any{"no_html", "no_template_meta"}}, "text without markup (200 characters at most)", nil
	}
	return nil, "", fmt.Errorf("type %q: int, date, enum or string", s.Type)
}

func (c Change) enumValues(doc map[string]any) (string, error) {
	r, _, err := c.route(doc)
	if err != nil {
		return "", err
	}
	q, _ := r["query"].(map[string]any)
	f, _ := q[c.Key].(map[string]any)
	if f == nil || f["type"] != "enum" {
		return "", fmt.Errorf("route %s has no list of values for %q", c.RouteID, c.Key)
	}
	have := list(f["values"])
	add, err := tokens(c.Values, have)
	if err != nil {
		return "", err
	}
	if len(add) == 0 {
		return "", ErrSame
	}
	if len(have)+len(add) > 50 {
		return "", errors.New("a list holds 50 values at most")
	}
	f["values"] = append(have, anys(add)...)
	return fmt.Sprintf("%s added to the values of ?%s= on route %s", strings.Join(add, ", "), c.Key, c.RouteID), nil
}

// tokens are plain values (price, date-desc, en_US), new ones only.
func tokens(in []string, have []any) ([]string, error) {
	var out []string
	for _, v := range in {
		if !valueRe.MatchString(v) {
			return nil, fmt.Errorf("%q is not a plain value (letters, digits, _ . , : -)", v)
		}
		if !slices.Contains(have, any(v)) && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	if len(out) > 50 {
		return nil, errors.New("50 values at most")
	}
	if len(out) == 0 && have == nil {
		return nil, errors.New("give the values the site offers")
	}
	return out, nil
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func anys(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

// Clone is a deep copy of a map document, to try a change on.
func Clone(doc map[string]any) map[string]any {
	b, _ := json.Marshal(doc)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}
