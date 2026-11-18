// Package mapschema validates map documents against the map JSON Schema.
//
// gateway-map.schema.json is a copy of schema/map/gateway-map.schema.json
// (Go cannot embed files outside the module); scripts/validate_schemas.py
// fails when the two differ.
package mapschema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed gateway-map.schema.json
var schemaJSON []byte

const schemaURL = "https://equalizer.ai/schema/map/gateway-map.schema.json"

var compiled = func() *jsonschema.Schema {
	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft2020
	if err := c.AddResource(schemaURL, bytes.NewReader(schemaJSON)); err != nil {
		panic(err)
	}
	return c.MustCompile(schemaURL)
}()

// ErrInvalid wraps the validation errors of a document.
type ErrInvalid struct{ Details []string }

func (e *ErrInvalid) Error() string { return "invalid map: " + strings.Join(e.Details, "; ") }

// MaxDetails limits the errors returned for one document.
const MaxDetails = 50

// Validate checks a JSON document against the schema.
func Validate(doc []byte) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return &ErrInvalid{Details: []string{"not valid JSON: " + err.Error()}}
	}
	err := compiled.Validate(v)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return &ErrInvalid{Details: []string{err.Error()}}
	}
	var details []string
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			where := e.InstanceLocation
			if where == "" {
				where = "/"
			}
			details = append(details, fmt.Sprintf("%s: %s", where, e.Message))
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	sort.Strings(details)
	if len(details) > MaxDetails {
		details = append(details[:MaxDetails], fmt.Sprintf("... and %d more", len(details)-MaxDetails))
	}
	return &ErrInvalid{Details: details}
}
