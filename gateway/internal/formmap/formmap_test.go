package formmap

import (
	"net/url"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

func testMap(t *testing.T) *Map {
	t.Helper()
	base, _ := url.Parse("http://10.20.0.15:8080/app/")
	return New(base, &mapmodel.Model{Forms: []mapmodel.Form{
		{ID: "supplement", Route: "/permits/{id}/supplement",
			Params: map[string]mapmodel.FieldSpec{"id": {Type: "int"}},
			LegacyRequest: mapmodel.LegacyRequest{Method: "POST", Path: "/permits/supplement.do",
				Form: map[string]string{"permitId": "{{params.id}}", "text": "{{fields.message}}", "vs": "{{hidden.vs}}", "kind": "SUP"}}},
		{ID: "contact", Route: "/contact",
			LegacyRequest: mapmodel.LegacyRequest{Method: "POST", Path: "/contact.do", Query: map[string]string{"a": "send"},
				Form: map[string]string{"email": "{{fields.email}}"}}},
		{ID: "login", Route: "/ucp.php",
			LegacyRequest: mapmodel.LegacyRequest{Method: "POST", Path: "/ucp.php",
				Query: map[string]string{"mode": "login", "sid": "{{hidden.sid}}"},
				Form:  map[string]string{"username": "{{fields.username}}"}}},
	}})
}

func TestMatch(t *testing.T) {
	m := testMap(t)
	u, _ := url.Parse("http://10.20.0.15:8080/app/permits/supplement.do")
	got, ok := m.Match(u, map[string]string{"permitId": "42", "vs": "abc", "debug": "1"})
	if !ok {
		t.Fatal("form not recognised")
	}
	if got.FormID != "supplement" || got.Action != "/permits/42/supplement" || got.Params["id"] != "42" {
		t.Fatalf("wrong match: %+v", got)
	}
	if got.Hidden["vs"] != "abc" || len(got.Hidden) != 1 {
		t.Errorf("only hidden fields referenced by the map are kept: %v", got.Hidden)
	}
	if got.Rename["text"] != "message" || len(got.Rename) != 1 {
		t.Errorf("rename: %v", got.Rename)
	}

	u, _ = url.Parse("http://10.20.0.15:8080/app/contact.do?a=send")
	if got, ok := m.Match(u, nil); !ok || got.Action != "/contact" {
		t.Errorf("contact: %+v %v", got, ok)
	}
}

// A value the page put in the action (phpBB's session id) is kept like a
// hidden input: sealed in the token, sent back to the legacy site as it was.
func TestQueryKeptServerSide(t *testing.T) {
	m := testMap(t)
	u, _ := url.Parse("http://10.20.0.15:8080/app/ucp.php?mode=login&sid=0123abcd")
	got, ok := m.Match(u, nil)
	if !ok || got.FormID != "login" || got.Action != "/ucp.php" || got.Hidden["sid"] != "0123abcd" || len(got.Params) != 0 {
		t.Fatalf("login: %+v %v", got, ok)
	}
	u, _ = url.Parse("http://10.20.0.15:8080/app/ucp.php?mode=register&sid=0123abcd")
	if _, ok := m.Match(u, nil); ok {
		t.Error("another fixed value must not match")
	}
}

func TestNoMatch(t *testing.T) {
	m := testMap(t)
	for _, c := range []struct {
		action string
		hidden map[string]string
	}{
		{"http://10.20.0.15:8080/app/permits/supplement.do", nil},                                  // id missing
		{"http://10.20.0.15:8080/app/contact.do", nil},                                             // fixed query missing
		{"http://10.20.0.15:8080/app/contact.do?a=send&x=1", nil},                                  // extra query
		{"http://10.20.0.15:8080/app/contact.do?a=delete", nil},                                    // other fixed value
		{"http://evil.example/app/contact.do?a=send", nil},                                         // other host
		{"http://10.20.0.15:8080/other/contact.do?a=send", nil},                                    // outside base path
		{"http://10.20.0.15:8080/app/permits/x/supplement.do", map[string]string{"permitId": "1"}}, // other path
	} {
		u, _ := url.Parse(c.action)
		if got, ok := m.Match(u, c.hidden); ok {
			t.Errorf("%s: unexpected match %+v", c.action, got)
		}
	}
}
