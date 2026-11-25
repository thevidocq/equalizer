package extract

import (
	"regexp"
	"strings"
)

// The request readers of each application, besides $_GET and the like
// (php.go), Joomla's input (cms_joomla.go) and Moodle's optional_param
// (cms_moodle.go): what each makes of a key.

type reader struct {
	stacks []string       // the applications that read the request this way
	re     *regexp.Regexp // [method] [source] name … (named groups)
	typeOf func(m map[string]string) string
	source func(m map[string]string) string
}

// castBefore: (int) or intval( before the reader's object chain
// ((int) $this->getRequest()->getParam('id')) makes its value an int; a
// call with arguments in between ((int) strlen(…)) does not.
var castBefore = regexp.MustCompile(`(?:\(\s*int(?:eger)?\s*\)|\bintval\s*\(|\babsint\s*\()\s*(?:\$\w+|[\w\\]+::\w+\(\s*\))?(?:\s*->\s*\w+(?:\s*\(\s*\))?)*\s*$`)

func named(re *regexp.Regexp, m []string) map[string]string {
	out := map[string]string{}
	for i, n := range re.SubexpNames() {
		if n != "" && i < len(m) {
			out[n] = m[i]
		}
	}
	return out
}

func always(t string) func(map[string]string) string {
	return func(map[string]string) string { return t }
}

var readers = []reader{
	{ // Symfony's Request (Drupal, phpBB's controllers, Magento's admin is apart)
		stacks: []string{"drupal", "phpbb", "prestashop"},
		re: regexp.MustCompile(`->\s*(?P<src>query|request)\s*->\s*(?P<method>getInt|getDigits|getBoolean|getAlpha|getAlnum|get)` +
			`\s*\(\s*['"](?P<name>[A-Za-z0-9_\-]{1,100})['"]`),
		typeOf: func(m map[string]string) string {
			return map[string]string{"getInt": "int", "getBoolean": "bool", "getAlpha": "key",
				"getAlnum": "key"}[m["method"]]
		},
		source: func(m map[string]string) string {
			switch {
			case m["src"] == "query":
				return "query"
			case m["method"] == "get":
				return "request" // Request::get() reads the query too
			}
			return "body"
		},
	},
	{ // MediaWiki's WebRequest
		stacks: []string{"mediawiki"},
		re: regexp.MustCompile(`->\s*(?P<method>getVal|getRawVal|getText|getInt|getIntOrNull|getIntArray|getBool|getFuzzyBool|getCheck|getArray)` +
			`\s*\(\s*['"](?P<name>[A-Za-z0-9_\-]{1,100})['"]`),
		typeOf: func(m map[string]string) string {
			return map[string]string{"getInt": "int", "getIntOrNull": "int", "getIntArray": "int", "getBool": "bool",
				"getFuzzyBool": "bool", "getCheck": "bool", "getText": "text"}[m["method"]]
		},
	},
	{ // phpBB's request: the default value's type is the value's
		stacks: []string{"phpbb"},
		re:     regexp.MustCompile(`->\s*variable\s*\(\s*['"](?P<name>[A-Za-z0-9_\-]{1,100})['"]\s*,\s*(?P<default>-?[0-9]+(?:\.[0-9]*)?|true|false|''|""|array|\[)`),
		typeOf: func(m map[string]string) string {
			switch d := m["default"]; {
			case d == "true" || d == "false":
				return "bool"
			case d == "''" || d == `""`:
				return "text"
			case d == "array" || d == "[" || strings.Contains(d, "."):
				return "" // a list, or a float (any number, not an int)
			default:
				return "int"
			}
		},
	},
	{ // TYPO3: Extbase arguments, the PSR-7 request, the old GeneralUtility
		stacks: []string{"typo3"},
		re: regexp.MustCompile(`(?:->\s*getArgument\s*\(|GeneralUtility::_(?:GP|GET|POST)\s*\(|->\s*get(?:QueryParams|ParsedBody)\s*\(\s*\)\s*\[)` +
			`\s*['"](?P<name>[A-Za-z0-9_\-]{1,100})['"]`),
	},
	{ // OpenCart: $this->request->get['id'], ->post['email']
		stacks: []string{"opencart"},
		re:     regexp.MustCompile(`->\s*request\s*->\s*(?P<src>get|post|request)\s*\[\s*['"](?P<name>[A-Za-z0-9_\-]{1,100})['"]`),
		source: func(m map[string]string) string {
			return map[string]string{"get": "query", "post": "body"}[m["src"]]
		},
	},
	{ // PrestaShop: Tools::getValue('id_product')
		stacks: []string{"prestashop"},
		re:     regexp.MustCompile(`Tools::get(?:Value|IsSet)\s*\(\s*['"](?P<name>[A-Za-z0-9_\-]{1,100})['"]`),
	},
	{ // Magento: $this->getRequest()->getParam('id'), ->getPost('email')
		stacks: []string{"magento"},
		re:     regexp.MustCompile(`->\s*(?P<method>getParam|getPost|getPostValue|getQuery)\s*\(\s*['"](?P<name>[A-Za-z0-9_\-]{1,100})['"]`),
		source: func(m map[string]string) string {
			return map[string]string{"getQuery": "query", "getPost": "body", "getPostValue": "body"}[m["method"]]
		},
	},
}

// readerKeys: the keys a line reads through its application's request
// reader and keeps as given (a reader that makes a number or a yes or no
// leaves nothing harmful): the taint of php.go.
func readerKeys(stack, line string) []string {
	var keys []string
	for _, r := range readers {
		if !hasStack(r.stacks, stack) {
			continue
		}
		for _, m := range r.re.FindAllStringSubmatch(line, -1) {
			g := named(r.re, m)
			if r.typeOf != nil && (r.typeOf(g) == "int" || r.typeOf(g) == "bool") {
				continue
			}
			keys = append(keys, g["name"])
		}
	}
	return keys
}

func hasStack(stacks []string, s string) bool {
	for _, x := range stacks {
		if x == s {
			return true
		}
	}
	return false
}

// frameworkInputs: the keys a file reads through its application's request
// reader, with the type the reader (or an (int) right before it) gives.
func (s *Summary) frameworkInputs(src, route string) {
	seen := map[string]bool{}
	for i, line := range strings.Split(src, "\n") {
		if comment(line) {
			continue
		}
		for _, r := range readers {
			if !hasStack(r.stacks, s.Stack) {
				continue
			}
			for _, ix := range r.re.FindAllStringSubmatchIndex(line, -1) {
				m := make([]string, len(ix)/2)
				for g := range m {
					if ix[2*g] >= 0 {
						m[g] = line[ix[2*g]:ix[2*g+1]]
					}
				}
				g := named(r.re, m)
				typ := ""
				if r.typeOf != nil {
					typ = r.typeOf(g)
				}
				if castBefore.MatchString(line[:ix[0]]) {
					typ = "int"
				}
				if typ == "" {
					typ = "unknown"
				}
				src := "request"
				if r.source != nil && r.source(g) != "" {
					src = r.source(g)
				}
				s.addInput(seen, Input{Name: g["name"], Source: src, Type: typ, Route: route, Where: s.at(i + 1)})
			}
		}
	}
}
