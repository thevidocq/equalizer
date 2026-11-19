// Package rebuild re-encodes what a client sends to an area (query and
// body) from a parsed model, so that no client byte reaches the legacy
// application as it was written: urlencoded and multipart forms, JSON, XML
// and plain text. Values are not typed (areas serve the site as it is); what
// cannot be parsed, or cannot be rebuilt, is refused.
package rebuild

import (
	"bytes"
	"errors"
	"mime"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/thevidocq/equalizer/gateway/internal/charset"
)

// Reasons for a refusal (request.blocked events).
const (
	ReasonMalformed   = "malformed_body"
	ReasonUnsupported = "unsupported_body"
	ReasonNUL         = "nul_byte"
	ReasonCharset     = "charset_violation"
	ReasonXMLDoctype  = "xml_doctype"
	ReasonTooMany     = "too_many_fields"
)

// MaxFields bounds the fields of a query or a form, and the parts of a
// multipart form: each one costs memory, and a 16 MB body of "a&a&a…"
// would be millions. PHP's own default (max_input_vars) is 1000.
const MaxFields = 10000

// Error is a refused request part.
type Error struct{ Reason string }

func (e *Error) Error() string { return "request refused: " + e.Reason }

func refuse(reason string) error { return &Error{Reason: reason} }

// Body rebuilds a request body. cs is the legacy charset of form values
// ("" for UTF-8). It returns the new body and its content type.
func Body(contentType string, body []byte, cs string) ([]byte, string, error) {
	if len(body) == 0 {
		return nil, "", nil
	}
	mt, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, "", refuse(ReasonUnsupported)
	}
	switch {
	case mt == "application/x-www-form-urlencoded":
		pairs, err := ParsePairs(string(body))
		if err != nil {
			return nil, "", err
		}
		out, err := EncodePairs(pairs, cs)
		return []byte(out), mt, err
	case mt == "multipart/form-data":
		return multipartBody(body, params["boundary"], cs)
	case mt == "application/json" || strings.HasSuffix(mt, "+json"):
		out, err := JSON(body)
		return out, mt + "; charset=utf-8", err
	case mt == "text/xml" || mt == "application/xml" || strings.HasSuffix(mt, "+xml"):
		out, err := XML(body)
		return out, mt + "; charset=utf-8", err
	case mt == "text/plain":
		if !utf8.Valid(body) {
			return nil, "", refuse(ReasonCharset)
		}
		if bytes.IndexByte(body, 0) >= 0 {
			return nil, "", refuse(ReasonNUL)
		}
		return []byte(string(body)), "text/plain; charset=utf-8", nil
	}
	return nil, "", refuse(ReasonUnsupported)
}

// Pair is one key=value of a query or a urlencoded form, in its order.
type Pair struct{ Key, Value string }

// ParsePairs decodes "a=1&b=2&a=3" keeping order and repetitions (PHP
// arrays like a[]=1&a[]=2 depend on them).
func ParsePairs(raw string) ([]Pair, error) {
	if strings.Count(raw, "&") >= MaxFields {
		return nil, refuse(ReasonTooMany)
	}
	var out []Pair
	for _, kv := range strings.Split(raw, "&") {
		if kv == "" {
			continue
		}
		k, v, _ := strings.Cut(kv, "=")
		key, err1 := url.QueryUnescape(k)
		val, err2 := url.QueryUnescape(v)
		if err1 != nil || err2 != nil {
			return nil, refuse(ReasonMalformed)
		}
		if strings.IndexByte(key, 0) >= 0 || strings.IndexByte(val, 0) >= 0 {
			return nil, refuse(ReasonNUL)
		}
		out = append(out, Pair{key, val})
	}
	return out, nil
}

// EncodePairs encodes pairs again, converting UTF-8 values to the legacy
// charset (pages are served in UTF-8, so browsers send UTF-8).
func EncodePairs(pairs []Pair, cs string) (string, error) {
	var b strings.Builder
	for i, p := range pairs {
		k, err1 := toLegacy(p.Key, cs)
		v, err2 := toLegacy(p.Value, cs)
		if err1 != nil || err2 != nil {
			return "", refuse(ReasonCharset)
		}
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(k))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(v))
	}
	return b.String(), nil
}

func toLegacy(s, cs string) (string, error) {
	if cs == "" || strings.EqualFold(cs, "utf-8") || strings.EqualFold(cs, "utf8") {
		if !utf8.ValidString(s) {
			return "", errors.New("not UTF-8")
		}
		return s, nil
	}
	return charset.FromUTF8(cs, s)
}

// Query rebuilds a raw query string.
func Query(raw, cs string) (string, error) {
	if raw == "" {
		return "", nil
	}
	pairs, err := ParsePairs(raw)
	if err != nil {
		return "", err
	}
	return EncodePairs(pairs, cs)
}
