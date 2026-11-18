package legacyclient

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
)

// Kinds gives the JSON type of a validated value, by "ns.name": "int" and
// "bool" become JSON numbers and booleans, everything else a string.
type Kinds map[string]string

// renderJSON builds the JSON body from the template. A string that is
// exactly one placeholder takes the value with its JSON type, and is left
// out of its object when the value is absent; other strings are resolved
// as text templates.
func renderJSON(tpl map[string]any, vals placeholder.Values, kinds Kinds) ([]byte, error) {
	v, _, err := renderValue(tpl, vals, kinds)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func renderValue(t any, vals placeholder.Values, kinds Kinds) (any, bool, error) {
	switch x := t.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v, absent, err := renderValue(x[k], vals, kinds)
			if err != nil {
				return nil, false, fmt.Errorf("json %q: %w", k, err)
			}
			if !absent {
				out[k] = v
			}
		}
		return out, false, nil
	case []any:
		out := make([]any, 0, len(x))
		for _, e := range x {
			v, absent, err := renderValue(e, vals, kinds)
			if err != nil {
				return nil, false, err
			}
			if !absent {
				out = append(out, v)
			}
		}
		return out, false, nil
	case string:
		if ns, name, ok := placeholder.Single(x); ok {
			raw, present := vals[ns][name]
			if !present {
				return nil, true, nil
			}
			switch kinds[ns+"."+name] {
			case "int":
				return json.Number(raw), false, nil
			case "bool":
				return raw == "true", false, nil
			}
			return raw, false, nil
		}
		s, absent, err := placeholder.Resolve(x, vals)
		return s, absent, err
	default:
		// Numbers, booleans and null written in the map stay as they are.
		return x, false, nil
	}
}

// CheckJSON verifies the placeholders of a JSON template.
func CheckJSON(t any) error {
	switch x := t.(type) {
	case map[string]any:
		for k, v := range x {
			if err := CheckJSON(v); err != nil {
				return fmt.Errorf("json %q: %w", k, err)
			}
		}
	case []any:
		for _, v := range x {
			if err := CheckJSON(v); err != nil {
				return err
			}
		}
	case string:
		return placeholder.Check(x)
	}
	return nil
}
