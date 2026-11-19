package mapschema

import (
	"errors"
	"os"
	"strings"
	"testing"
)

const valid = `{"meta":{"schema":"gateway-map/1.0","tenant":"t","app":"a","version":1,"status":"draft"},
 "legacy":{"base_url":"http://10.0.0.1:8080","reach":{"mode":"direct","allow":["10.0.0.1:8080"]}},
 "roles":{"anonymous":"guest","list":{"guest":{}}},
 "routes":[{"id":"home","path":"/","method":"GET","roles":["guest"],"legacy_request":{"method":"GET","path":"/"},
   "render":"faithful","origin":"human","risk":"low"}]}`

func TestValidate(t *testing.T) {
	if err := Validate([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(valid, `"method":"GET","roles"`, `"method":"POST","roles"`, 1)
	err := Validate([]byte(bad))
	var ie *ErrInvalid
	if !errors.As(err, &ie) || len(ie.Details) == 0 || !strings.Contains(strings.Join(ie.Details, " "), "/routes/0/method") {
		t.Fatalf("route with POST: %v", err)
	}
	if err := Validate([]byte(`{`)); err == nil {
		t.Fatal("broken JSON accepted")
	}
}

// The embedded copy must equal the contract.
func TestCopyMatchesContract(t *testing.T) {
	want, err := os.ReadFile("../../../schema/map/gateway-map.schema.json")
	if err != nil {
		t.Skip("contract not found (module built outside the repository)")
	}
	if string(want) != string(schemaJSON) {
		t.Fatal("internal/mapschema/gateway-map.schema.json differs from schema/map: copy it again")
	}
}

func TestOIDCAuth(t *testing.T) {
	on := strings.Replace(valid, `"list":{"guest":{}}`, `"list":{"guest":{}},"auth":{"provider":"oidc","issuer":"https://login.example/v2.0","client_id":"abc","roles":[{"group":"*","role":"guest"}]}`, 1)
	if err := Validate([]byte(on)); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]string{
		"http issuer": strings.Replace(on, "https://login", "http://login", 1),
		"no roles":    strings.Replace(on, `,"roles":[{"group":"*","role":"guest"}]`, "", 1),
	} {
		if Validate([]byte(bad)) == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
