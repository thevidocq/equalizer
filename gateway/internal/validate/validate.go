// Package validate checks client values against the types declared in the
// map. A value that does not respect its type never reaches the legacy
// application.
package validate

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// Rejection reasons, used as "reason" in field.rejected events.
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

// fallbackMaxLength limits strings when the map says nothing.
const fallbackMaxLength = 1000

var emailRe = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)

// Error describes a rejected value. It never contains the value itself.
type Error struct {
	Field  string
	Reason string
}

func (e *Error) Error() string { return fmt.Sprintf("field %q rejected: %s", e.Field, e.Reason) }

// Validator applies the types and constraints of one map version.
type Validator struct {
	types    map[string]mapmodel.TypeDef
	defaults mapmodel.StringDefaults
	regexps  sync.Map // string → *regexp.Regexp
}

// New creates a validator for a map's types and default values.
func New(types map[string]mapmodel.TypeDef, defaults mapmodel.StringDefaults) *Validator {
	return &Validator{types: types, defaults: defaults}
}

// Check compiles every pattern and charset up front, so a map with invalid
// expressions is rejected at load time.
func (v *Validator) Check(specs map[string]mapmodel.FieldSpec) error {
	for name, t := range v.types { // a type's rules, even if no field uses it yet
		for _, rule := range t.Rules {
			if _, ok := rules[rule]; !ok {
				return fmt.Errorf("type %q: unknown rule %q", name, rule)
			}
		}
	}
	for name, s := range specs {
		r, err := v.resolve(s)
		if err != nil {
			return fmt.Errorf("field %q: %w", name, err)
		}
		for _, expr := range []string{r.pattern, charsetExpr(r.charset)} {
			if expr == "" {
				continue
			}
			if _, err := v.regexp(expr); err != nil {
				return fmt.Errorf("field %q: invalid expression: %w", name, err)
			}
		}
		for _, rule := range r.rules {
			if _, ok := rules[rule]; !ok {
				return fmt.Errorf("field %q: unknown rule %q", name, rule)
			}
		}
	}
	return nil
}

// Fields validates a set of values (query or path parameters). Undeclared
// names are dropped; repeated ones are rejected. It returns the canonical
// values of the fields that are present or have a default.
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

// Value validates a single value. set is false when the field is absent,
// optional and has no default.
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
	for _, d := range r.deny {
		if strings.EqualFold(canon, d) {
			return "", false, &Error{Field: name, Reason: ReasonValue}
		}
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
	case "string", "email", "text", "password":
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

// resolved is a specification with its named type already expanded.
type resolved struct {
	base      string
	required  bool
	def       any
	min, max  *float64
	values    []string
	deny      []string
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
			return resolved{}, fmt.Errorf("unknown type %q", s.Type)
		}
		r = resolved{base: t.Base, min: t.Min, max: t.Max, values: t.Values, deny: t.Deny,
			pattern: t.Pattern, charset: t.Charset, maxLength: t.MaxLength, rules: t.Rules}
	}
	// Field constraints override those of the type.
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
	// A field can only add refused values to those of its type.
	r.deny = append(append([]string{}, r.deny...), s.Deny...)
	if s.Pattern != "" {
		r.pattern = s.Pattern
	}
	if s.Charset != "" {
		r.charset = s.Charset
	}
	if s.MaxLength > 0 {
		r.maxLength = s.MaxLength
	}
	// A field can only add rules to those of its type.
	r.rules = append(append([]string{}, r.rules...), s.Rules...)
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
	if r.base == "password" && r.maxLength == 0 {
		r.maxLength = 128
	}
	if r.maxLength == 0 {
		r.maxLength = fallbackMaxLength
	}
	return r, nil
}

func isBase(t string) bool {
	switch t {
	case "int", "enum", "bool", "date", "string", "email", "text", "password", "file":
		return true
	}
	return false
}

// charsetExpr turns a class of allowed characters into an expression that
// accepts only strings made of those characters.
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

// mustRegexp is used only after Check, which has already compiled everything.
func (v *Validator) mustRegexp(expr string) *regexp.Regexp {
	re, err := v.regexp(expr)
	if err != nil {
		// Never reached after Check; if it happens, the value is rejected.
		return regexp.MustCompile(`\A\z.`)
	}
	return re
}

// Base returns the base type of a field ("" when unknown).
func (v *Validator) Base(spec mapmodel.FieldSpec) string {
	r, err := v.resolve(spec)
	if err != nil {
		return ""
	}
	return r.base
}
