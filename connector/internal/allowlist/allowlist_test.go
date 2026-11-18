package allowlist

import "testing"

func TestAllowlist(t *testing.T) {
	l, err := Parse(" 10.20.0.15:8080, Legacy.Local:443,10.20.0.15:8080")
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Items()) != 2 {
		t.Fatalf("items: %v", l.Items())
	}
	for d, want := range map[string]bool{
		"10.20.0.15:8080": true, "legacy.local:443": true, "LEGACY.LOCAL:443": true,
		"10.20.0.15:22": false, "10.20.0.15": false, "169.254.169.254:80": false,
	} {
		if l.Contains(d) != want {
			t.Errorf("Contains(%q) = %v", d, !want)
		}
	}
	for _, bad := range []string{"", " , ", "10.0.0.1", ":80", "host:"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) accepted", bad)
		}
	}
}
