// Package jsonshape handles the JSON of API routes: it reads client bodies
// field by field against the map, and rebuilds legacy responses keeping
// only the declared fields with their declared types. Nothing else of the
// legacy document reaches the client.
package jsonshape

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

// MaxBody bounds a client JSON body.
const MaxBody = 1 << 20

// Reasons of a rejected body (field.rejected events).
const (
	ReasonNotJSON      = "not_json"
	ReasonNotObject    = "not_an_object"
	ReasonUnknownField = "unknown_field"
	ReasonTooLarge     = "body_too_large"
)

// ErrShape reports a legacy document without the declared shape.
var ErrShape = errors.New("legacy JSON does not have the declared shape")

// Body validates a client JSON object: every key must be declared, every
// value must have its type. It returns canonical strings, as the other
// namespaces, and the JSON kind of each field for the legacy request.
func Body(v *validate.Validator, specs map[string]mapmodel.FieldSpec, r io.Reader) (map[string]string, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxBody+1))
	if err != nil || len(raw) > MaxBody {
		return nil, &validate.Error{Field: "(body)", Reason: ReasonTooLarge}
	}
	obj := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(raw)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(raw))
		tok, err := dec.Token()
		if err != nil {
			return nil, &validate.Error{Field: "(body)", Reason: ReasonNotJSON}
		}
		if d, ok := tok.(json.Delim); !ok || d != '{' {
			return nil, &validate.Error{Field: "(body)", Reason: ReasonNotObject}
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, &validate.Error{Field: "(body)", Reason: ReasonNotJSON}
		}
	}
	for k := range obj {
		if _, ok := specs[k]; !ok {
			return nil, &validate.Error{Field: k, Reason: ReasonUnknownField}
		}
	}
	out := make(map[string]string, len(specs))
	for name, spec := range specs {
		s, present, err := scalar(obj[name])
		if err != nil {
			return nil, &validate.Error{Field: name, Reason: validate.ReasonType}
		}
		canon, set, err := v.Value(name, spec, s, present)
		if err != nil {
			return nil, err
		}
		if set {
			out[name] = canon
		}
	}
	return out, nil
}

// scalar turns a JSON value into the string the validator checks. Objects
// and arrays are refused: fields are flat.
func scalar(raw json.RawMessage) (string, bool, error) {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || string(t) == "null" {
		return "", false, nil
	}
	switch t[0] {
	case '"':
		var s string
		err := json.Unmarshal(t, &s)
		return s, true, err
	case 't', 'f':
		var b bool
		err := json.Unmarshal(t, &b)
		return fmt.Sprint(b), true, err
	case '{', '[':
		return "", false, errors.New("not a scalar")
	default:
		var n json.Number
		err := json.Unmarshal(t, &n)
		return n.String(), true, err
	}
}

// Kinds returns the JSON kind of each body field ("int", "bool" or "").
func Kinds(v *validate.Validator, ns string, specs map[string]mapmodel.FieldSpec) map[string]string {
	out := map[string]string{}
	for name, spec := range specs {
		if b := v.Base(spec); b == "int" || b == "bool" {
			out[ns+"."+name] = b
		}
	}
	return out
}

// Response rebuilds the legacy JSON: the object (or list of objects) at
// resp.Path, with only the declared fields. A field whose value does not
// have its type is left out.
func Response(v *validate.Validator, resp mapmodel.APIResponse, body []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: not JSON", ErrShape)
	}
	if resp.Path != "" {
		for _, key := range strings.Split(resp.Path, ".") {
			obj, ok := doc.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrShape, resp.Path)
			}
			doc = obj[key]
		}
	}
	if !resp.List {
		obj, ok := doc.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: object expected", ErrShape)
		}
		return json.Marshal(pick(v, resp.Fields, obj))
	}
	items, ok := doc.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: list expected", ErrShape)
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if obj, ok := it.(map[string]any); ok {
			out = append(out, pick(v, resp.Fields, obj))
		}
	}
	return json.Marshal(out)
}

func pick(v *validate.Validator, fields map[string]mapmodel.FieldSpec, obj map[string]any) map[string]any {
	out := make(map[string]any, len(fields))
	for name, spec := range fields {
		if val, ok := typed(v, name, spec, obj[name]); ok {
			out[name] = val
		}
	}
	return out
}

// typed returns the value with its declared JSON type, or ok=false.
func typed(v *validate.Validator, name string, spec mapmodel.FieldSpec, val any) (any, bool) {
	if val == nil {
		return nil, false
	}
	base := v.Base(spec)
	var s string
	switch x := val.(type) {
	case json.Number:
		if base != "int" {
			return nil, false
		}
		s = x.String()
	case bool:
		if base != "bool" {
			return nil, false
		}
		return x, true
	case string:
		if base == "int" || base == "bool" {
			return nil, false
		}
		s = x
	default:
		return nil, false
	}
	canon, set, err := v.Value(name, spec, s, true)
	if err != nil || !set {
		return nil, false
	}
	if base == "int" {
		return json.Number(canon), true
	}
	return canon, true
}
