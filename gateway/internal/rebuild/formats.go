package rebuild

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"path"
	"strings"
	"unicode"
)

const maxDepth = 128

// JSON re-encodes a JSON document token by token: same order, same numbers,
// nothing but the parsed values.
func JSON(body []byte) ([]byte, error) {
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
				if !ok {
					return errors.New("object key")
				}
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
			if t.Target == "xml" {
				out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
			} else {
				out.WriteString("<?" + t.Target + " " + string(bytes.ReplaceAll(t.Inst, []byte("?>"), nil)) + "?>")
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

// multipartBody rebuilds a multipart form with a new boundary: fields are
// re-encoded like urlencoded values, files keep their content under a
// cleaned name.
func multipartBody(body []byte, boundary, cs string) ([]byte, string, error) {
	if boundary == "" {
		return nil, "", refuse(ReasonMalformed)
	}
	mr := multipart.NewReader(bytes.NewReader(body), boundary)
	var out bytes.Buffer
	mw := multipart.NewWriter(&out)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", refuse(ReasonMalformed)
		}
		name := part.FormName()
		data, err := io.ReadAll(part)
		if err != nil || name == "" {
			return nil, "", refuse(ReasonMalformed)
		}
		key, err := toLegacy(name, cs)
		if err != nil {
			return nil, "", refuse(ReasonCharset)
		}
		if part.FileName() == "" && !isFilePart(part) {
			if bytes.IndexByte(data, 0) >= 0 {
				return nil, "", refuse(ReasonNUL)
			}
			v, err := toLegacy(string(data), cs)
			if err != nil {
				return nil, "", refuse(ReasonCharset)
			}
			if err := mw.WriteField(key, v); err != nil {
				return nil, "", err
			}
			continue
		}
		h := textproto.MIMEHeader{}
		// as browsers write it: name first, both quoted
		h.Set("Content-Disposition", `form-data; name="`+quoteEscaper.Replace(key)+`"; filename="`+quoteEscaper.Replace(cleanName(part.FileName()))+`"`)
		ct := "application/octet-stream"
		if mt, _, err := mime.ParseMediaType(part.Header.Get("Content-Type")); err == nil {
			ct = mt
		}
		h.Set("Content-Type", ct)
		w, err := mw.CreatePart(h)
		if err != nil {
			return nil, "", err
		}
		_, _ = w.Write(data)
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return out.Bytes(), mw.FormDataContentType(), nil
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"", "\r", "", "\n", "")

func isFilePart(p *multipart.Part) bool {
	_, params, err := mime.ParseMediaType(p.Header.Get("Content-Disposition"))
	_, has := params["filename"]
	return err == nil && has
}

// cleanName keeps the base name of an uploaded file, without paths or
// control characters.
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
