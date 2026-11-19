package health

import (
	"strconv"
	"strings"
)

// Older reports whether release a is older than release b. Only releases
// like 0.1.0 (or v0.1.0) are compared: development builds never count as
// older.
func Older(a, b string) bool {
	x, okA := semver(a)
	y, okB := semver(b)
	if !okA || !okB {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return false
}

func semver(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// outdated returns the first release older than the platform's, or "".
func outdated(versions []string, platform string) string {
	for _, v := range versions {
		if Older(v, platform) {
			return v
		}
	}
	return ""
}
