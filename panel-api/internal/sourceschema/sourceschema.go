// Package sourceschema validates a source reader's summary against
// schema/source/summary.schema.json, embedded here as a copy (Go cannot
// embed files outside the module); scripts/validate_schemas.py checks the
// copy is identical.
package sourceschema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed summary.schema.json
var schemaJSON []byte

const schemaURL = "https://equalizer.ai/schema/source/summary.schema.json"

var compiled = func() *jsonschema.Schema {
	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft2020
	if err := c.AddResource(schemaURL, bytes.NewReader(schemaJSON)); err != nil {
		panic(err)
	}
	return c.MustCompile(schemaURL)
}()

// Validate checks a summary; the error names the first places it fails.
func Validate(doc []byte) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("not valid JSON: %w", err)
	}
	err := compiled.Validate(v)
	var ve *jsonschema.ValidationError
	if err == nil || !errors.As(err, &ve) {
		return err
	}
	for len(ve.Causes) > 0 {
		ve = ve.Causes[0]
	}
	return fmt.Errorf("%s: %s", ve.InstanceLocation, ve.Message)
}
