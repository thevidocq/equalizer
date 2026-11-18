// Package validate controlla i valori in ingresso dal client contro i tipi
// dichiarati nella mappa. Un valore che non rispetta il tipo non arriva mai
// al legacy.
package validate

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// Motivi di rifiuto, usati come "reason" negli eventi field.rejected.
const (
	ReasonMissing     = "missing_required"
	ReasonType        = "type_mismatch"
	ReasonRange       = "out_of_range"
	ReasonValue       = "value_not_allowed"
	ReasonLength      = "too_long"
	ReasonCharset     = "charset_violation"
	ReasonPattern     = "pattern_mismatch"
	ReasonRule        = "rule_violation"
	ReasonDuplicate   = "duplicate_param"
	ReasonUnknown     = "unknown_type"
	ReasonUnsupported = "unsupported_type"
)

// fallbackMaxLength limita le stringhe quando la mappa non dice nulla.
const fallbackMaxLength = 1000

var emailRe = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)

// Error descrive un valore rifiutato. Non contiene mai il valore stesso.
type Error struct {
	Field  string
	Reason string
}

func (e *Error) Error() string { return fmt.Sprintf("campo %q rifiutato: %s", e.Field, e.Reason) }

// Validator applica tipi e vincoli di una versione della mappa.
type Validator struct {
	types    map[string]mapmodel.TypeDef
	defaults mapmodel.StringDefaults
	regexps  sync.Map // string → *regexp.Regexp
}

// New crea un validatore per i tipi e i valori predefiniti di una mappa.
func New(types map[string]mapmodel.TypeDef, defaults mapmodel.StringDefaults) *Validator {
	return &Validator{types: types, defaults: defaults}
}

// Check compila in anticipo pattern e charset di tutte le specifiche,
// così una mappa con espressioni non valide viene rifiutata al caricamento.
func (v *Validator) Check(specs map[string]mapmodel.FieldSpec) error {
	for name, s := range specs {
		r, err := v.resolve(s)
		if err != nil {
			return fmt.Errorf("campo %q: %w", name, err)
		}
		for _, expr := range []string{r.pattern, charsetExpr(r.charset)} {
			if expr == "" {
				continue
			}
			if _, err := v.regexp(expr); err != nil {
				return fmt.Errorf("campo %q: espressione non valida: %w", name, err)
			}
		}
		for _, rule := range r.rules {
			if _, ok := rules[rule]; !ok {
				return fmt.Errorf("campo %q: regola sconosciuta %q", name, rule)
			}
		}
	}
	return nil
}

// Fields valida un insieme di valori (query o parametri di percorso).
// I nomi non dichiarati vengono scartati; quelli ripetuti rifiutati.
// Restituisce i valori canonici dei soli campi presenti o con default.
func (v *Validator) Fields(specs map[string]mapmodel.FieldSpec, values url.Values) (map[string]string, error) {
	out := make(map[string]string, len(specs))
	for name, spec := range specs {
		raw, present := values[name]
		if len(raw) > 1 {
			return nil, &Error{Field: name, Reason: ReasonDuplicate}
		}
		value := ""
		if present {
			value = raw[0]
		}
		canon, set, err := v.Value(name, spec, value, present)
		if err != nil {
			return nil, err
		}
		if set {
			out[name] = canon
		}
	}
	return out, nil
}

// Value valida un singolo valore. set è false se il campo è assente e
// facoltativo senza default.
func (v *Validator) Value(name string, spec mapmodel.FieldSpec, raw string, present bool) (string, bool, error) {
	r, err := v.resolve(spec)
	if err != nil {
		return "", false, &Error{Field: name, Reason: ReasonUnknown}
	}
	if !present || raw == "" {
		if r.def != nil {
			return fmt.Sprint(r.def), true, nil
		}
		if r.required {
			return "", false, &Error{Field: name, Reason: ReasonMissing}
		}
		return "", false, nil
	}
	canon, reason := v.check(r, raw)
	if reason != "" {
		return "", false, &Error{Field: name, Reason: reason}
	}
	return canon, true, nil
}

func (v *Validator) check(r resolved, raw string) (string, string) {
	switch r.base {
	case "int":
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return "", ReasonType
		}
		if (r.min != nil && float64(n) < *r.min) || (r.max != nil && float64(n) > *r.max) {
			return "", ReasonRange
		}
		return strconv.FormatInt(n, 10), ""
	case "enum":
		for _, allowed := range r.values {
			if raw == allowed {
				return raw, ""
			}
		}
		return "", ReasonValue
	case "bool":
		switch raw {
		case "true", "1":
			return "true", ""
		case "false", "0":
			return "false", ""
		}
		return "", ReasonType
	case "date":
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			return "", ReasonType
		}
		return raw, ""
	case "string", "email", "text":
		return v.checkText(r, raw)
	default:
		return "", ReasonUnsupported
	}
}

func (v *Validator) checkText(r resolved, raw string) (string, string) {
	if !utf8.ValidString(raw) {
		return "", ReasonType
	}
	for _, c := range raw {
		if unicode.IsControl(c) && !(r.base == "text" && (c == '\n' || c == '\r' || c == '\t')) {
			return "", ReasonCharset
		}
	}
	if utf8.RuneCountInString(raw) > r.maxLength {
		return "", ReasonLength
	}
	if r.base == "email" && !emailRe.MatchString(raw) {
		return "", ReasonPattern
	}
	if r.charset != "" && !v.mustRegexp(charsetExpr(r.charset)).MatchString(raw) {
		return "", ReasonCharset
	}
	if r.pattern != "" && !v.mustRegexp(r.pattern).MatchString(raw) {
		return "", ReasonPattern
	}
	for _, name := range r.rules {
		if rule, ok := rules[name]; !ok || !rule(raw) {
			return "", ReasonRule
		}
	}
	return raw, ""
}

// resolved è una specifica con il tipo nominale già espanso.
type resolved struct {
	base      string
	required  bool
	def       any
	min, max  *float64
	values    []string
	pattern   string
	charset   string
	maxLength int
	rules     []string
}

func (v *Validator) resolve(s mapmodel.FieldSpec) (resolved, error) {
	r := resolved{base: s.Type}
	if !isBase(s.Type) {
		t, ok := v.types[s.Type]
		if !ok || !isBase(t.Base) {
			return resolved{}, fmt.Errorf("tipo sconosciuto %q", s.Type)
		}
		r = resolved{base: t.Base, min: t.Min, max: t.Max, values: t.Values,
			pattern: t.Pattern, charset: t.Charset, maxLength: t.MaxLength}
	}
	// I vincoli del campo prevalgono su quelli del tipo.
	r.required = s.Required == nil || *s.Required
	r.def = s.Default
	if s.Min != nil {
		r.min = s.Min
	}
	if s.Max != nil {
		r.max = s.Max
	}
	if len(s.Values) > 0 {
		r.values = s.Values
	}
	if s.Pattern != "" {
		r.pattern = s.Pattern
	}
	if s.Charset != "" {
		r.charset = s.Charset
	}
	if s.MaxLength > 0 {
		r.maxLength = s.MaxLength
	}
	r.rules = s.Rules
	if r.base == "string" {
		if r.charset == "" && r.pattern == "" {
			r.charset = v.defaults.Charset
		}
		if r.maxLength == 0 {
			r.maxLength = v.defaults.MaxLength
		}
	}
	if r.base == "email" && r.maxLength == 0 {
		r.maxLength = 254
	}
	if r.maxLength == 0 {
		r.maxLength = fallbackMaxLength
	}
	return r, nil
}

func isBase(t string) bool {
	switch t {
	case "int", "enum", "bool", "date", "string", "email", "text", "file":
		return true
	}
	return false
}

// charsetExpr trasforma una classe di caratteri ammessi in un'espressione
// che accetta solo stringhe composte da quei caratteri.
func charsetExpr(charset string) string {
	if charset == "" {
		return ""
	}
	return `^(?:` + charset + `)*$`
}

func (v *Validator) regexp(expr string) (*regexp.Regexp, error) {
	if re, ok := v.regexps.Load(expr); ok {
		return re.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	v.regexps.Store(expr, re)
	return re, nil
}

// mustRegexp si usa solo dopo Check, che ha già compilato tutto.
func (v *Validator) mustRegexp(expr string) *regexp.Regexp {
	re, err := v.regexp(expr)
	if err != nil {
		// Mai raggiunto dopo Check; se accade, il valore viene rifiutato.
		return regexp.MustCompile(`\A\z.`)
	}
	return re
}
