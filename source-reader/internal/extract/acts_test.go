package extract

import "testing"

func TestActs(t *testing.T) {
	for src, want := range map[string]bool{
		`<?php if ( isset( $_POST['go'] ) ) { do_it( $_POST['go'] ); }`:                                      true,
		`<?php require 'wp-load.php'; echo esc_html( $_GET['q'] );`:                                          true,
		`<?php class A { function b() { return $_GET['x']; } }`:                                              false,
		`<?php function helper( $a ) { if ( $a ) { echo $_REQUEST['z']; } return 1; }`:                       false,
		`<div><?php echo $title; ?></div><?php // $_GET['ignored'] in a comment`:                             false,
		`<?php $sql = "SELECT * FROM t WHERE a = '$_GET[a]'"; $x = 1;`:                                       false, // the string is blanked: no request read outside it
		`<?php namespace A; use B; /** $_POST['doc'] */ final class C { public function d() { exec($x); } }`: false,
		`<?php $r = $wpdb->get_results( "SELECT 1" ); exec( 'ls' );`:                                         true,
	} {
		if got := acts(src); got != want {
			t.Errorf("%q: %v, want %v", src, got, want)
		}
	}
}

func TestLibrary(t *testing.T) {
	for rel, want := range map[string]bool{
		"wp-includes/class-wpdb.php":                        true,
		"wp-admin/admin-ajax.php":                           true,
		"wp-login.php":                                      true,
		"wp-content/plugins/a/vendor/guzzle/src/Client.php": true,
		"wp-content/plugins/a/includes/ajax.php":            false,
		"wp-content/themes/t/functions.php":                 false,
	} {
		if got := library("wordpress", rel); got != want {
			t.Errorf("%s: %v", rel, got)
		}
	}
}

func TestBacktick(t *testing.T) {
	s := &Summary{}
	s.phpFile("a.php", "<?php $wpdb->query( \"ALTER TABLE `$t` ADD `x`\" . $_GET['k'] );", "")
	for _, r := range s.Risks {
		if r.Kind == "exec" {
			t.Fatalf("a backtick in SQL counted as a command: %+v", r)
		}
	}
	s = &Summary{}
	s.phpFile("b.php", "<?php $o = `ls $_GET[d]`;", "")
	if len(s.Risks) == 0 || s.Risks[0].Kind != "exec" {
		t.Fatalf("a command in backticks missed: %+v", s.Risks)
	}
}
