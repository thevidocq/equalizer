package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Only a closed list of literals the code accepts becomes an enum.
func TestInArray(t *testing.T) {
	for src, want := range map[string]string{
		`<?php if ( ! in_array( $_GET['o'], array( 'asc', 'desc' ), true ) ) { return; }`: "enum",
		`<?php $o = in_array( $_GET['o'], [ 'asc', 'desc' ] ) ? $_GET['o'] : 'asc';`:      "enum",
		`<?php if ( in_array( $_GET['o'], [ 'admin', 'root' ] ) ) { die; }`:               "unknown",
		"<?php in_array($_GET['o'], array('name','date',\n 'price'));":                    "unknown",
		`<?php in_array($_GET['o'], array('a', $b));`:                                     "unknown",
	} {
		s := &Summary{}
		s.phpFile("a.php", src, "")
		got := ""
		for _, in := range s.Inputs {
			if in.Name == "o" {
				got = in.Type
			}
		}
		if got != want {
			t.Errorf("%q: %s, want %s", src, got, want)
		}
	}
}

// A plugin with a long version line does not fail the read.
func TestFit(t *testing.T) {
	long := "1.0.0 - a longer description of this release, sixty-four chars"
	s := &Summary{Components: []Component{{Name: "plugin:a", Kind: "plugin", Version: &long}},
		Routes: []Route{{Path: "/" + string(make([]byte, 400))}}}
	s.fit()
	if len(s.Components) != 1 || s.Components[0].Version != nil || len(s.Routes) != 0 {
		t.Fatalf("%+v", s)
	}
}

// A key compared with a token written in the code: the summary never
// carries the token.
func TestSecretComparedValues(t *testing.T) {
	for src, want := range map[string]string{
		`<?php if ( ! in_array( $_GET['token'], array( 'adm1n-backd00r' ) ) ) { return; }`:                "unknown",
		`<?php if ( ! in_array( $_GET['mode'], array( 'a8F3kd92LmQ7zx41' ) ) ) { return; }`:               "unknown",
		`<?php if ( ! in_array( $_GET['country_code'], array( 'IT', 'FR' ) ) ) { return; }`:               "enum",
		`<?php if ( ! in_array( $_GET['orderby'], array( 'date', 'price', 'popularity' ) ) ) { return; }`: "enum",
	} {
		s := &Summary{Inputs: []Input{}}
		s.phpFile("x.php", src, "/x.php")
		if len(s.Inputs) == 0 || s.Inputs[0].Type != want || want == "unknown" && s.Inputs[0].Values != nil {
			t.Errorf("%s: %+v", src, s.Inputs)
		}
	}
}

// A request value kept in a variable, then used in a risky call: the call
// counts, with the key; a value made a number or escaped does not.
func TestTaint(t *testing.T) {
	src := `<?php
$id = $_GET['id'];
$q = "SELECT * FROM t WHERE id = $id";
$wpdb->query( $q );
$page = absint( $_GET['page'] );
$wpdb->query( "SELECT * FROM t LIMIT $page" );
$name = $request->get_param( 'name' );
file_put_contents( '/tmp/' . $name, 'x' );
$safe = esc_sql( $_POST['s'] );
$wpdb->get_results( "SELECT * FROM t WHERE s = '$safe'" );
// $wpdb->query( $id );
/* x */ $wpdb->query( "SELECT " . $_GET['a'] );
`
	s := &Summary{Inputs: []Input{}, Risks: []Risk{}}
	s.phpFile("x.php", src, "/x.php")
	got := map[string]int{}
	for _, r := range s.Risks {
		got[r.Kind+":"+r.Input] = r.Where.Line
	}
	if len(got) != 3 || got["sql:id"] != 4 || got["file_write:name"] != 8 || got["sql:a"] != 12 {
		t.Fatalf("%v", got)
	}
}

func TestFilterInput(t *testing.T) {
	s := &Summary{Inputs: []Input{}, Risks: []Risk{}}
	s.phpFile("x.php", "<?php $p = filter_input( INPUT_GET, 'page', FILTER_VALIDATE_INT );\n$m = filter_input(INPUT_POST, 'mail', FILTER_SANITIZE_EMAIL);\n$x = filter_input(INPUT_GET, 'raw');\n$t = filter_input(INPUT_POST, 'tel', FILTER_SANITIZE_NUMBER_INT);", "/x.php")
	got := map[string]string{}
	for _, in := range s.Inputs {
		got[in.Name] = in.Source + " " + in.Type
	}
	if got["page"] != "query int" || got["mail"] != "body unknown" || got["raw"] != "query unknown" || got["tel"] != "body unknown" {
		t.Fatalf("%v", got)
	}
}

// A value of the code that may be a secret never becomes part of a route
// or an action name.
func TestResolvedSecrets(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"wp-includes/version.php": "<?php $wp_version = '6.7.1';",
		"wp-content/plugins/p/p.php": "<?php\n$pw = 'Sup3rS3cretPassw0rd';\n$ns = 'shop/v1';\nadd_action( 'wp_ajax_' . $pw, 'f' );\n" +
			"register_rest_route( $ns, '/x/' . $pw, array() );\nclass A { const API_KEY = 'abcdefgh12'; function r() { register_rest_route( 'a/v1', '/k/' . self::API_KEY ); } }",
	} {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		_ = os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
	}
	s, _ := Read(dir, time.Now())
	b, _ := json.Marshal(s)
	if strings.Contains(string(b), "Sup3r") || strings.Contains(string(b), "abcdefgh12") {
		t.Fatalf("%s", b)
	}
	if !strings.Contains(string(b), "/wp-json/shop/v1/x/{pw}") || !strings.Contains(string(b), `"action":"*"`) {
		t.Fatalf("%s", b)
	}
}
