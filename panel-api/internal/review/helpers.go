package review

import (
	"fmt"
	"sort"
	"strings"
)

func opens(before, after []any, anon string) bool {
	if anon == "" {
		return false
	}
	return !contains(before, anon) && contains(after, anon)
}

func limitLooser(b, n obj) bool {
	if len(b) == 0 {
		return false
	}
	for k, bv := range b {
		nv, ok := n[k]
		if !ok || perSecond(str(nv)) > perSecond(str(bv)) {
			return true
		}
	}
	return false
}

func perSecond(v string) float64 {
	var count float64
	var unit string
	if _, err := fmt.Sscanf(strings.Replace(v, "/", " ", 1), "%g %s", &count, &unit); err != nil {
		return 0
	}
	div := map[string]float64{"s": 1, "sec": 1, "min": 60, "hour": 3600, "day": 86400}[unit]
	if div == 0 {
		return 0
	}
	return count / div
}

func where(e obj) string {
	for _, k := range []string{"path", "route"} {
		if s := str(e[k]); s != "" {
			if m := str(e["method"]); m != "" && m != "GET" {
				return m + " " + s
			}
			return s
		}
	}
	return ""
}

func typesOf(doc obj) map[string]obj {
	out := map[string]obj{}
	for k, v := range asObj(doc["types"]) {
		out[k] = asObj(v)
	}
	return out
}

func byID(list any, key string) map[string]obj {
	out := map[string]obj{}
	for _, it := range asList(list) {
		if o := asObj(it); o != nil {
			out[str(o[key])] = o
		}
	}
	return out
}

func keys(m map[string]obj) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func path(o obj, ks ...string) any {
	var cur any = o
	for _, k := range ks {
		m := asObj(cur)
		if m == nil {
			return nil
		}
		cur = m[k]
	}
	return cur
}

func asObj(v any) obj    { m, _ := v.(map[string]any); return m }
func asList(v any) []any { l, _ := v.([]any); return l }
func str(v any) string   { s, _ := v.(string); return s }
func num(v any) float64  { f, _ := v.(float64); return f }
func contains(l []any, s string) bool {
	for _, v := range l {
		if str(v) == s {
			return true
		}
	}
	return false
}
func strs(v any) []string {
	var out []string
	for _, x := range asList(v) {
		out = append(out, str(x))
	}
	return out
}
