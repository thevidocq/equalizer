package mapexport

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func sample(t *testing.T) []byte {
	b, err := os.ReadFile("testdata/map.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestOpenAPI(t *testing.T) {
	b, err := OpenAPI(sample(t), "permits-portal", "permits.example.gov", 14)
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		OpenAPI string                               `json:"openapi"`
		Servers []map[string]string                  `json:"servers"`
		Paths   map[string]map[string]map[string]any `json:"paths"`
		Areas   []map[string]any                     `json:"x-equalizer-areas"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	if d.OpenAPI != "3.0.3" || d.Servers[0]["url"] != "https://permits.example.gov" || len(d.Areas) != 1 {
		t.Fatalf("%s", b)
	}
	get := func(path, method string) map[string]any {
		op := d.Paths[path][method]
		if op == nil {
			t.Fatalf("no %s %s in %v", method, path, d.Paths)
		}
		return op
	}
	params := get("/permits", "get")["parameters"].([]any)
	page := params[0].(map[string]any)
	if page["name"] != "page" || page["schema"].(map[string]any)["maximum"] != 500.0 {
		t.Errorf("an int with its range: %v", page)
	}
	status := params[1].(map[string]any)["schema"].(map[string]any)
	if strings.Join(toStrings(status["enum"]), ",") != "open,in_progress,closed" {
		t.Errorf("a named type: %v", status)
	}
	id := get("/permits/{id}", "get")["parameters"].([]any)[0].(map[string]any)
	if id["in"] != "path" || id["schema"].(map[string]any)["minimum"] != 1.0 {
		t.Errorf("path parameter: %v", id)
	}
	form := get("/permits/{id}/supplement", "post")["requestBody"].(map[string]any)["content"].(map[string]any)
	if _, ok := form["multipart/form-data"]; !ok {
		t.Errorf("a form with a file is multipart: %v", form)
	}
	login := get("/login", "post")["requestBody"].(map[string]any)["content"].(map[string]any)["application/x-www-form-urlencoded"].(map[string]any)["schema"].(map[string]any)
	user := login["properties"].(map[string]any)["username"].(map[string]any)
	if user["pattern"] != "^(?:[A-Za-z0-9._@-])*$" || user["maxLength"] != 64.0 || login["additionalProperties"] != false {
		t.Errorf("charset as a pattern, no other field: %v", login)
	}
	api := get("/api/permits/{id}/status", "put")["requestBody"].(map[string]any)["content"].(map[string]any)
	if _, ok := api["application/json"]; !ok {
		t.Errorf("api body: %v", api)
	}
	if _, ok := d.Paths["/css/{file}"]["get"]; !ok {
		t.Errorf("assets")
	}
	q := get("/search", "get")["parameters"].([]any)[1].(map[string]any)["schema"].(map[string]any)
	if q["maxLength"] != 80.0 || len(toStrings(q["x-equalizer-rules"])) != 2 {
		t.Errorf("text with its rules: %v", q)
	}
}

func toStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func TestModSecurity(t *testing.T) {
	b, err := ModSecurity(sample(t), "permits-portal", 14)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, want := range []string{
		`SecRule REQUEST_FILENAME "@rx ^/permits/-?[0-9]{1,18}$" "t:none,setvar:'tx.eq_entry=r_permit_detail'"`,
		`SecRule REQUEST_FILENAME "@rx ^/login$" "t:none,setvar:'tx.eq_entry=f_login'"`,
		`SecRule REQUEST_FILENAME "@rx ^/admin/" "t:none,setvar:'tx.eq_entry=area_admin'"`,
		`SecRule &TX:EQ_ENTRY "@eq 0" "id:`,
		`SecRule ARGS_GET_NAMES "!@rx ^(?:page|status)$"`,
		`SecRule ARGS_GET:status "!@rx ^(?:open|in_progress|closed)$"`,
		`SecRule ARGS_POST:username "!@rx ^(?:[A-Za-z0-9._@-])*$"`,
		`SecRule ARGS_POST:message "@gt 1000" "t:none,t:length"`,
		`SecRule FILES_SIZES:attachment "@gt 5242880"`,
		`SecRule ARGS_POST_NAMES "!@rx ^(?:json\.note|json\.status)$"`,
		`ctl:requestBodyProcessor=JSON`,
		`SecRule RESPONSE_HEADERS:Content-Type "!@rx ^(?:text/css)\s*(?:;|$)"`,
		`SecRule ARGS_GET:q "@rx [<>]|&#"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	// every id once, in the local range
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, `"id:`); i >= 0 {
			id := strings.SplitN(line[i+4:], ",", 2)[0]
			if seen[id] {
				t.Fatalf("id %s twice", id)
			}
			seen[id] = true
		}
	}
	if !seen["10000"] || len(seen) < 30 {
		t.Fatalf("%d rules", len(seen))
	}
	if strings.Contains(out, "%!") || strings.Contains(out, "%d") {
		t.Fatal("a format left in the rules")
	}
}

func TestNothingEscapesItsArgument(t *testing.T) {
	m := `{"routes":[{"id":"x","path":"/a","method":"GET","query":{"k":{"type":"string","pattern":"^a\\\"b$","deny":["a\" \"t:none\"\nSecAction \"id:1,phase:1,ctl:ruleEngine=Off\"\n#"]},
		"e":{"type":"enum","values":["x\"y","z\\"]}}}],"areas":[{"id":"a","prefix":"/q\"/"}]}`
	b, err := ModSecurity([]byte(m), "it's", 1)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, line := range strings.Split(out, "\n") {
		l := strings.TrimSpace(line)
		if l != "" && !strings.HasPrefix(l, "#") && !strings.HasPrefix(l, "SecRule ") {
			t.Fatalf("a line that is not a rule: %q", line)
		}
		// each rule line has its four quotes: operator and actions
		if strings.HasPrefix(l, "SecRule ") && strings.Count(strings.ReplaceAll(l, `\"`, ""), `"`) != 4 {
			t.Fatalf("an argument left open: %q", line)
		}
	}
	if strings.Contains(out, "it's") || !strings.Contains(out, `x\x22y`) {
		t.Fatalf("%s", out)
	}
	if _, err := ModSecurity([]byte(`{"routes":[{"id":"x","path":"/a","query":{"a b":{"type":"int"}}}]}`), "x", 1); err == nil {
		t.Fatal("a field name that is not an identifier")
	}
}
