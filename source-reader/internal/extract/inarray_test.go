package extract

import "testing"

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
