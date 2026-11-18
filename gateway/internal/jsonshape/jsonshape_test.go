package jsonshape

import (
	"errors"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

var v = validate.New(nil, mapmodel.StringDefaults{})

func TestBody(t *testing.T) {
	specs := map[string]mapmodel.FieldSpec{
		"id": {Type: "int", Required: ptr(true)}, "active": {Type: "bool", Required: ptr(false)}, "name": {Type: "string", MaxLength: 10, Required: ptr(false)},
	}
	out, err := Body(v, specs, strings.NewReader(`{"id": 42, "active": true, "name": "Anna"}`))
	if err != nil || out["id"] != "42" || out["active"] != "true" || out["name"] != "Anna" {
		t.Fatalf("valid body: %v %v", out, err)
	}
	for body, reason := range map[string]string{
		`{"id": 1, "admin": true}`:          ReasonUnknownField,
		`{"id": "x"}`:                       validate.ReasonType,
		`{"id": {"$gt": 0}}`:                validate.ReasonType,
		`[1,2]`:                             ReasonNotObject,
		`{"id":`:                            ReasonNotJSON,
		`{"active": true}`:                  validate.ReasonMissing,
		`{"id": 1, "name": "toolongname!"}`: validate.ReasonLength,
	} {
		_, err := Body(v, specs, strings.NewReader(body))
		var ve *validate.Error
		if !errors.As(err, &ve) || ve.Reason != reason {
			t.Errorf("%s: %v, want %s", body, err, reason)
		}
	}
	big := `{"name":"` + strings.Repeat("a", MaxBody) + `"}`
	if _, err := Body(v, specs, strings.NewReader(big)); err == nil {
		t.Error("oversized body accepted")
	}
}

func TestResponse(t *testing.T) {
	resp := mapmodel.APIResponse{List: true, Path: "data.items", Fields: map[string]mapmodel.FieldSpec{
		"id": {Type: "int"}, "status": {Type: "enum", Values: []string{"open", "closed"}}, "title": {Type: "string"},
	}}
	legacy := `{"data":{"items":[
		{"id": 1, "status": "open", "title": "Permit", "owner_tax_code": "RSSMRA80A01H501U", "debug": {"sql": "SELECT"}},
		{"id": "2", "status": "weird", "title": "x"},
		"not an object"
	]}}`
	out, err := Response(v, resp, []byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `[{"id":1,"status":"open","title":"Permit"},{"title":"x"}]` {
		t.Fatalf("filtered response: %s", out)
	}
	for _, bad := range []string{`{"data":[]}`, `<html>error</html>`, `{"data":{"items":{}}}`} {
		if _, err := Response(v, resp, []byte(bad)); !errors.Is(err, ErrShape) {
			t.Errorf("%s: %v", bad, err)
		}
	}
	one := mapmodel.APIResponse{Fields: map[string]mapmodel.FieldSpec{"count": {Type: "int"}}}
	if out, _ := Response(v, one, []byte(`{"count": 7, "secret": "x"}`)); string(out) != `{"count":7}` {
		t.Fatalf("object: %s", out)
	}
}

func ptr[T any](v T) *T { return &v }
