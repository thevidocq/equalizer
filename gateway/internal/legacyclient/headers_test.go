package legacyclient

import "testing"

func TestAppHeader(t *testing.T) {
	for k, want := range map[string]bool{
		"X-WP-Total": true, "X-Pagination-Count": true, "X-CSRF-Token": true,
		"X-Powered-By": false, "X-Accel-Redirect": false, "X-Sendfile": false, "X-Debug-Token": false,
		"X-Varnish": false, "X-Frame-Options": false, "X-Eq-Request": false, "Content-Type": false,
	} {
		if appHeader(k) != want {
			t.Errorf("%s: want %v", k, want)
		}
	}
}
