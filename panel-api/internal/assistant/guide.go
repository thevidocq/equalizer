package assistant

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// The tenant guide, copied from docs/manual/tenant.md by
// panel-ui/scripts/sync-help.mjs (CI checks the copy): the assistant answers
// "how do I…" from it, as the panel's Help shows it.
//
//go:embed guide/tenant.md
var guideText string

type section struct {
	title string // "Chapter › Section"
	body  string
	words map[string]int
}

var guideSections = splitGuide(guideText)

func splitGuide(text string) []section {
	var out []section
	var chapter string
	cur := &section{}
	flush := func() {
		if strings.TrimSpace(cur.body) != "" && cur.title != "" {
			cur.words = wordCount(cur.title + " " + cur.title + " " + cur.body) // the title counts double
			out = append(out, *cur)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			chapter = strings.TrimPrefix(line, "## ")
			cur = &section{title: chapter}
		case strings.HasPrefix(line, "### "):
			flush()
			cur = &section{title: chapter + " › " + strings.TrimPrefix(line, "### ")}
		default:
			if !strings.HasPrefix(line, "![") {
				cur.body += line + "\n"
			}
		}
	}
	flush()
	return out
}

func wordCount(s string) map[string]int {
	m := map[string]int{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(w) > 2 {
			m[w]++
		}
	}
	return m
}

// Guide returns the sections of the guide closest to the query (English
// words work best: the guide is in English), or its table of contents.
func Guide(query string) string {
	q := wordCount(query)
	if len(q) == 0 {
		var b strings.Builder
		b.WriteString("Sections of the guide:\n")
		for _, s := range guideSections {
			b.WriteString("- " + s.title + "\n")
		}
		return b.String()
	}
	type hit struct {
		i     int
		score float64
	}
	var hits []hit
	for i, s := range guideSections {
		score := 0.0
		for w := range q {
			if n := s.words[w]; n > 0 {
				score += 1 + float64(min(n, 5))/5
			}
		}
		if score > 0 {
			hits = append(hits, hit{i, score})
		}
	}
	if len(hits) == 0 {
		return "Nothing in the guide matches; call read_guide with no query for its sections."
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	var b strings.Builder
	for n, h := range hits {
		if n == 3 {
			break
		}
		s := guideSections[h.i]
		fmt.Fprintf(&b, "## %s\n%s\n", s.title, clip(strings.TrimSpace(s.body), 3500))
	}
	return b.String()
}
