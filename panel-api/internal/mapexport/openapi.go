package mapexport

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OpenAPI writes the version as an OpenAPI 3.0.3 document: every route,
// form and API entry with its parameters, their types and limits. A tool
// that reads it (an API gateway, a WAF, a test suite) learns what the map
// accepts; it does not learn how the gateway rebuilds a request for the
// site, the roles, the checks of ownership or the rate limits.
func OpenAPI(raw []byte, app, domain string, version int) ([]byte, error) {
	var d doc
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("the map could not be read: %w", err)
	}
	paths := map[string]map[string]any{}
	add := func(path, method string, op map[string]any) {
		if paths[path] == nil {
			paths[path] = map[string]any{}
		}
		paths[path][strings.ToLower(method)] = op
	}
	for _, r := range d.list("routes") {
		method := str(r["method"])
		if method == "" {
			method = "GET"
		}
		add(str(r["path"]), method, d.operation(r, "route", nil, ""))
	}
	for _, f := range d.list("forms") {
		ct := "application/x-www-form-urlencoded"
		for _, x := range d.fields(asMap(f["fields"])) {
			if x.base == "file" {
				ct = "multipart/form-data"
			}
		}
		add(str(f["route"]), "POST", d.operation(f, "form", asMap(f["fields"]), ct))
	}
	for _, a := range d.list("api") {
		add(str(a["path"]), str(a["method"]), d.operation(a, "api", asMap(a["body"]), "application/json"))
	}
	for _, a := range d.list("assets") {
		p := strings.TrimSuffix(str(a["prefix"]), "/") + "/{file}"
		add(p, "GET", map[string]any{
			"summary":     "Files of " + str(a["prefix"]),
			"operationId": "asset_" + ident(str(a["prefix"])),
			"parameters": []any{map[string]any{"name": "file", "in": "path", "required": true,
				"schema": map[string]any{"type": "string"}, "description": "Any file under the folder (and its subfolders)"}},
			"responses": map[string]any{"200": map[string]any{"description": "A file of one of these types",
				"content": types(strs(a["types"]))}},
			"x-equalizer-types": strs(a["types"]),
		})
	}
	var areas []any
	for _, a := range d.list("areas") {
		areas = append(areas, map[string]any{"prefix": str(a["prefix"]), "methods": strs(a["methods"]),
			"description": "Served as the site serves it: no typed checks"})
	}
	out := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   app + " (Equalizer map, version " + fmt.Sprint(version) + ")",
			"version": fmt.Sprint(version),
			"description": "What the Equalizer map of " + app + " accepts. Deny by default: a path, a method, a query key or a field " +
				"that is not here is refused. Not in this file: how the gateway rebuilds each request for the site, the roles, " +
				"the ownership checks, the rate limits, the rewriting of the pages, the generic areas (x-equalizer-areas).",
		},
		"paths": paths,
	}
	if domain != "" {
		out["servers"] = []any{map[string]any{"url": "https://" + domain}}
	}
	if len(areas) > 0 {
		out["x-equalizer-areas"] = areas
	}
	return json.MarshalIndent(out, "", "  ")
}

func (d doc) operation(e map[string]any, kind string, body map[string]any, ct string) map[string]any {
	op := map[string]any{"operationId": kind + "_" + str(e["id"]), "responses": map[string]any{
		"200": map[string]any{"description": "The page of the site"},
		"403": map[string]any{"description": "Refused by Equalizer: a value outside its check"}}}
	if roles := strs(e["roles"]); len(roles) > 0 {
		op["x-equalizer-roles"] = roles
	}
	var params []any
	for _, f := range d.fields(asMap(e["params"])) {
		f.required = true // a path parameter is always there
		params = append(params, map[string]any{"name": f.name, "in": "path", "required": true, "schema": schema(f)})
	}
	for _, f := range d.fields(asMap(e["query"])) {
		params = append(params, map[string]any{"name": f.name, "in": "query", "required": f.required, "schema": schema(f)})
	}
	if len(params) > 0 {
		op["parameters"] = params
	}
	if body != nil {
		props, required := map[string]any{}, []string{}
		for _, f := range d.fields(body) {
			props[f.name] = schema(f)
			if f.required {
				required = append(required, f.name)
			}
		}
		s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
		if len(required) > 0 {
			s["required"] = required
		}
		op["requestBody"] = map[string]any{"required": true, "content": map[string]any{ct: map[string]any{"schema": s}}}
	}
	return op
}

// schema is a field's check as a JSON Schema of OpenAPI 3.0.
func schema(f field) map[string]any {
	s := map[string]any{}
	switch f.base {
	case "int":
		s["type"] = "integer"
		if f.min != nil {
			s["minimum"] = *f.min
		}
		if f.max != nil {
			s["maximum"] = *f.max
		}
		return s
	case "enum":
		return map[string]any{"type": "string", "enum": f.values}
	case "bool":
		return map[string]any{"type": "boolean"}
	case "date":
		return map[string]any{"type": "string", "format": "date"}
	case "file":
		s = map[string]any{"type": "string", "format": "binary"}
		if len(f.mime) > 0 {
			s["x-equalizer-types"] = f.mime
		}
		if f.maxSizeKB > 0 {
			s["x-equalizer-max-size-kb"] = f.maxSizeKB
		}
		return s
	}
	s["type"] = "string"
	s["maxLength"] = f.maxLength
	switch {
	case f.base == "email":
		s["format"] = "email"
	case f.base == "password":
		s["format"] = "password"
	}
	switch {
	case f.pattern != "":
		s["pattern"] = f.pattern
	case f.charset != "":
		s["pattern"] = "^(?:" + f.charset + ")*$"
	}
	if len(f.rules) > 0 {
		s["x-equalizer-rules"] = f.rules
	}
	if len(f.deny) > 0 {
		s["not"] = map[string]any{"enum": f.deny}
	}
	return s
}

func types(t []string) map[string]any {
	out := map[string]any{}
	for _, m := range t {
		out[m] = map[string]any{}
	}
	return out
}

// ident is a prefix as an identifier: /wp-content/uploads/ → wp_content_uploads.
func ident(p string) string {
	var b strings.Builder
	for _, c := range strings.Trim(p, "/") {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			b.WriteRune(c)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "root"
	}
	return b.String()
}
