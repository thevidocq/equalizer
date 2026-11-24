package validate

import (
	"strings"
	"unicode"
)

// rules are specific checks, not generic signatures: each one accepts or
// rejects a precise class of characters or sequences.
var rules = map[string]func(string) bool{
	// no_html: no HTML tags or numeric entities.
	"no_html": func(s string) bool {
		return !strings.ContainsAny(s, "<>") && !strings.Contains(s, "&#")
	},
	// no_sql_meta: no SQL metacharacters. The apostrophe is allowed only
	// between two letters (e.g. "O'Brien"), never as a delimiter.
	"no_sql_meta": func(s string) bool {
		for _, seq := range []string{"--", "/*", "*/", ";", "\\", "\"", "\x00", "`"} {
			if strings.Contains(s, seq) {
				return false
			}
		}
		runes := []rune(s)
		for i, c := range runes {
			if c != '\'' {
				continue
			}
			if i == 0 || i == len(runes)-1 || !unicode.IsLetter(runes[i-1]) || !unicode.IsLetter(runes[i+1]) {
				return false
			}
		}
		return true
	},
	// no_lookalikes: no characters that look like ASCII or fold to it on a
	// legacy stack (NFKC, iconv //TRANSLIT, best-fit code pages, MySQL
	// collations): fullwidth ａｄｍｉｎ, 𝐚𝐝𝐦𝐢𝐧, ﬁ, ſ, the modifier apostrophe ʼ,
	// ⒈, invisible fillers. Combining accents (admiń) are letters and stay:
	// a *_unicode_ci collation may still match them to admin. The
	// agent's estimate holds the same ranges (agent/equalizer_agent/slugs.py).
	"no_lookalikes": func(s string) bool {
		for _, c := range s {
			for _, r := range lookalikes {
				if c >= r[0] && c <= r[1] {
					return false
				}
			}
		}
		return true
	},
	// no_template_meta: no template or server-side code sequences.
	"no_template_meta": func(s string) bool {
		for _, seq := range []string{"<?", "?>", "<%", "%>", "${", "#{", "{{", "}}"} {
			if strings.Contains(s, seq) {
				return false
			}
		}
		return true
	},
}

// lookalikes are the code points no_lookalikes refuses.
var lookalikes = [][2]rune{
	{0x00AA, 0x00AA},   // feminine ordinal ª
	{0x00BA, 0x00BA},   // masculine ordinal º
	{0x0132, 0x0133},   // ligature Ĳ ĳ
	{0x013F, 0x0140},   // Ŀ ŀ
	{0x017F, 0x017F},   // long s ſ
	{0x01C4, 0x01CC},   // digraphs Ǆ…ǌ
	{0x01F1, 0x01F3},   // digraphs Ǳ ǲ ǳ
	{0x02B0, 0x02FF},   // spacing modifier letters (ʼ ʹ ˮ)
	{0x0337, 0x0338},   // combining solidus overlays
	{0x034F, 0x034F},   // combining grapheme joiner
	{0x115F, 0x1160},   // Hangul fillers (invisible)
	{0x1D2C, 0x1D6A},   // phonetic modifier letters (ᴬ ᵃ)
	{0x1D78, 0x1D78},   // modifier letter ᵸ
	{0x1D9B, 0x1DBF},   // modifier letters (ᶜ ᶻ)
	{0x1E9A, 0x1E9A},   // ẚ
	{0x2070, 0x209F},   // superscripts and subscripts (ⁱ ₐ)
	{0x20D0, 0x20FF},   // combining marks for symbols (enclosing)
	{0x2100, 0x214F},   // letterlike symbols (ℌ ℓ K)
	{0x2460, 0x24FF},   // enclosed alphanumerics (⒈ ⑴)
	{0x3164, 0x3164},   // Hangul filler (invisible)
	{0x3200, 0x33FF},   // enclosed CJK letters, CJK compatibility
	{0xFB00, 0xFB06},   // Latin ligatures (ﬁ)
	{0xFE00, 0xFE0F},   // variation selectors
	{0xFF00, 0xFFEF},   // halfwidth and fullwidth forms (ａｄｍｉｎ)
	{0x1D400, 0x1D7FF}, // mathematical alphanumerics (𝐚𝐝𝐦𝐢𝐧 𝟏)
	{0x1F100, 0x1F1FF}, // enclosed alphanumeric supplement
	{0x1FBF0, 0x1FBF9}, // segmented digits
	{0xE0100, 0xE01EF}, // variation selectors supplement
}
