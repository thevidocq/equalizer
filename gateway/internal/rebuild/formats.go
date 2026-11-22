package rebuild

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path"
	"strings"
	"unicode"

	"github.com/thevidocq/equalizer/gateway/internal/jsontext"
)

const maxDepth = 128

// JSON re-encodes a JSON document token by token: same order, same numbers,
// nothing but the parsed values.
func JSON(body []byte) ([]byte, error) {
	if !jsontext.Valid(body) {
		return nil, refuse(ReasonCharset)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var out bytes.Buffer
	if err := jsonValue(dec, &out, 0); err != nil {
		return nil, refuse(ReasonMalformed)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, refuse(ReasonMalformed) // trailing data
	}
	return out.Bytes(), nil
}

func jsonValue(dec *json.Decoder, out *bytes.Buffer, depth int) error {
	if depth > maxDepth {
		return errors.New("too deep")
	}
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch t := tok.(type) {
	case json.Delim:
		closing := map[json.Delim]json.Delim{'{': '}', '[': ']'}[t]
		if closing == 0 {
			return errors.New("unexpected delimiter")
		}
		out.WriteRune(rune(t))
		seen := map[string]bool{} // a repeated key is read as the first by some parsers, the last by others
		for i := 0; dec.More(); i++ {
			if i > 0 {
				out.WriteByte(',')
			}
			if t == '{' {
				key, err := dec.Token()
				if err != nil {
					return err
				}
				k, ok := key.(string)
				if !ok || seen[k] {
					return errors.New("object key")
				}
				seen[k] = true
				b, _ := json.Marshal(k)
				out.Write(b)
				out.WriteByte(':')
			}
			if err := jsonValue(dec, out, depth+1); err != nil {
				return err
			}
		}
		if end, err := dec.Token(); err != nil || end != closing {
			return errors.New("unbalanced")
		}
		out.WriteRune(rune(closing))
	case string:
		b, _ := json.Marshal(t)
		out.Write(b)
	case json.Number:
		out.WriteString(t.String())
	case bool:
		out.WriteString(map[bool]string{true: "true", false: "false"}[t])
	case nil:
		out.WriteString("null")
	}
	return nil
}

// XML re-encodes an XML document from its tokens, prefixes kept as written.
// A DOCTYPE (where external entities live) is refused.
func XML(body []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = true
	var out bytes.Buffer
	depth := 0
	for {
		tok, err := dec.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, refuse(ReasonMalformed)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > maxDepth {
				return nil, refuse(ReasonMalformed)
			}
			out.WriteString("<" + qname(t.Name))
			for _, a := range t.Attr {
				out.WriteString(" " + qname(a.Name) + `="`)
				_ = xml.EscapeText(&out, []byte(a.Value))
				out.WriteByte('"')
			}
			out.WriteByte('>')
		case xml.EndElement:
			depth--
			out.WriteString("</" + qname(t.Name) + ">")
		case xml.CharData:
			_ = xml.EscapeText(&out, t)
		case xml.ProcInst:
			// Only the declaration is kept: other instructions (<?php …?>,
			// <?xml-stylesheet …?>) are not data, and some servers act on them.
			if t.Target == "xml" {
				out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
			}
		case xml.Directive:
			return nil, refuse(ReasonXMLDoctype)
		case xml.Comment:
			// dropped
		}
	}
	if depth != 0 {
		return nil, refuse(ReasonMalformed)
	}
	return out.Bytes(), nil
}

func qname(n xml.Name) string {
	if n.Space != "" {
		return n.Space + ":" + n.Local
	}
	return n.Local
}

// MaxFieldBytes bounds one non-file field of a multipart form.
const MaxFieldBytes = 8 << 20

// multipartBody rebuilds a multipart form held in memory.
func multipartBody(body []byte, boundary, cs string) ([]byte, string, error) {
	var out bytes.Buffer
	ctype, err := Multipart(&out, bytes.NewReader(body), boundary, cs)
	if err != nil {
		return nil, "", err
	}
	return out.Bytes(), ctype, nil
}

// Multipart rebuilds a multipart form from src into dst with a new
// boundary, part by part: fields are re-encoded like urlencoded values,
// files keep their content under a cleaned name, unless the server could
// run them (fileCheck). Files are streamed, so large uploads never sit in
// memory.
func Multipart(dst io.Writer, src io.Reader, boundary, cs string) (string, error) {
	if boundary == "" {
		return "", refuse(ReasonMalformed)
	}
	mr := multipart.NewReader(src, boundary)
	mw := multipart.NewWriter(dst)
	for parts := 0; ; parts++ {
		if parts >= MaxFields {
			return "", refuse(ReasonTooMany)
		}
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", malformed(err)
		}
		name := part.FormName()
		// A backslash in a name is read as an escape by some servers and
		// not by others: the rebuilt name could reach the legacy
		// application as another field. Browsers never send one.
		if name == "" || strings.Contains(name, `\`) {
			return "", refuse(ReasonMalformed)
		}
		key, err := toLegacy(name, cs)
		if err != nil {
			return "", refuse(ReasonCharset)
		}
		if part.FileName() == "" && !isFilePart(part) {
			data, err := io.ReadAll(io.LimitReader(part, MaxFieldBytes+1))
			if err != nil {
				return "", malformed(err)
			}
			if len(data) > MaxFieldBytes {
				return "", refuse(ReasonMalformed)
			}
			if bytes.IndexByte(data, 0) >= 0 {
				return "", refuse(ReasonNUL)
			}
			v, err := toLegacy(string(data), cs)
			if err != nil {
				return "", refuse(ReasonCharset)
			}
			h := textproto.MIMEHeader{}
			h.Set("Content-Disposition", `form-data; name="`+quoteEscaper.Replace(key)+`"`)
			fw, err := mw.CreatePart(h)
			if err != nil {
				return "", err
			}
			if _, err := io.WriteString(fw, v); err != nil {
				return "", err
			}
			continue
		}
		file := cleanName(part.FileName())
		if runnableName(file) {
			return "", refuse(ReasonFileRefused)
		}
		h := textproto.MIMEHeader{}
		// as browsers write it: name first, both quoted
		h.Set("Content-Disposition", `form-data; name="`+quoteEscaper.Replace(key)+`"; filename="`+quoteEscaper.Replace(file)+`"`)
		ct := "application/octet-stream"
		if mt, _, err := mime.ParseMediaType(part.Header.Get("Content-Type")); err == nil {
			ct = mt
		}
		h.Set("Content-Type", ct)
		w, err := mw.CreatePart(h)
		if err != nil {
			return "", err
		}
		check := &fileCheck{declared: ct}
		if _, err := io.Copy(io.MultiWriter(check, w), part); err != nil {
			var re *Error
			if errors.As(err, &re) {
				return "", err
			}
			return "", malformed(err)
		}
		if err := check.done(); err != nil {
			return "", err
		}
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	return mw.FormDataContentType(), nil
}

// malformed keeps reading errors that are not about the form (a body over
// its limit, a dropped connection) and turns the others into a refusal.
func malformed(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return err
	}
	return refuse(ReasonMalformed)
}

// quoteEscaper writes names as browsers do (HTML standard): a quote or a
// line break becomes %22, %0D, %0A. Backslash escapes are read by some
// servers and not by others, so they are never written.
var quoteEscaper = strings.NewReplacer(`"`, "%22", "\r", "%0D", "\n", "%0A")

func isFilePart(p *multipart.Part) bool {
	_, params, err := mime.ParseMediaType(p.Header.Get("Content-Disposition"))
	_, has := params["filename"]
	return err == nil && has
}

// CleanName keeps the base name of an uploaded file, without paths or
// control characters.
func CleanName(name string) string { return cleanName(name) }

func cleanName(name string) string {
	if name == "" {
		return "" // no file chosen: the legacy application must see it so
	}
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	if name == "." || name == "/" || name == ".." {
		return "file"
	}
	if len(name) > 200 {
		name = name[len(name)-200:]
	}
	return name
}
