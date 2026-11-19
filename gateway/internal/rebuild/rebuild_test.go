package rebuild

import (
	"bytes"
	"errors"
	"mime/multipart"
	"strings"
	"testing"
)

func reason(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Reason
	}
	return ""
}

func TestForms(t *testing.T) {
	out, ct, err := Body("application/x-www-form-urlencoded", []byte("b=2&a%5B%5D=1&a%5B%5D=%C3%A8&empty="), "")
	if err != nil || ct != "application/x-www-form-urlencoded" || string(out) != "b=2&a%5B%5D=1&a%5B%5D=%C3%A8&empty=" {
		t.Fatalf("form: %q %q %v", out, ct, err)
	}
	latin, _, err := Body("application/x-www-form-urlencoded", []byte("q=%C3%A8"), "iso-8859-1")
	if err != nil || string(latin) != "q=%E8" {
		t.Fatalf("latin-1: %q %v", latin, err)
	}
	if _, _, err := Body("application/x-www-form-urlencoded", []byte("q=a%00b"), ""); reason(err) != ReasonNUL {
		t.Errorf("NUL: %v", err)
	}
	if q, err := Query("x=1&y=%2F&x=2", ""); err != nil || q != "x=1&y=%2F&x=2" {
		t.Errorf("query: %q %v", q, err)
	}
	if _, err := Query("a=%zz", ""); reason(err) != ReasonMalformed {
		t.Errorf("malformed query: %v", err)
	}
}

func TestJSONAndXML(t *testing.T) {
	out, _, err := Body("application/json", []byte(` {"z": 1.50, "a": [true, null, "x\u0041"]} `), "")
	if err != nil || string(out) != `{"z":1.50,"a":[true,null,"xA"]}` {
		t.Fatalf("json: %s %v", out, err)
	}
	for _, bad := range []string{`{"a":1}{"b":2}`, `{"a":}`, strings.Repeat("[", 200) + strings.Repeat("]", 200)} {
		if _, _, err := Body("application/json", []byte(bad), ""); err == nil {
			t.Errorf("json accepted: %.30s", bad)
		}
	}
	x, _, err := Body("text/xml", []byte(`<?xml version="1.0"?><methodCall><!-- c --><methodName>wp.getPosts</methodName><p a="1&amp;2">x &lt; y</p></methodCall>`), "")
	if err != nil || !bytes.Contains(x, []byte(`<methodName>wp.getPosts</methodName><p a="1&amp;2">x &lt; y</p>`)) || bytes.Contains(x, []byte("<!--")) {
		t.Fatalf("xml: %s %v", x, err)
	}
	soap, _, _ := Body("application/soap+xml", []byte(`<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body/></soap:Envelope>`), "")
	if !bytes.Contains(soap, []byte("<soap:Envelope xmlns:soap=")) {
		t.Errorf("namespace prefix lost: %s", soap)
	}
	xxe := `<?xml version="1.0"?><!DOCTYPE r [<!ENTITY x SYSTEM "file:///etc/passwd">]><r>&x;</r>`
	if _, _, err := Body("application/xml", []byte(xxe), ""); reason(err) != ReasonXMLDoctype {
		t.Errorf("XXE accepted")
	}
}

func TestMultipartAndOthers(t *testing.T) {
	var in bytes.Buffer
	mw := multipart.NewWriter(&in)
	_ = mw.WriteField("title", "Hello")
	fw, _ := mw.CreateFormFile("doc", `C:\Users\me\..\report "final".pdf`)
	_, _ = fw.Write([]byte("%PDF-1.4"))
	ew, _ := mw.CreateFormFile("empty", "")
	_, _ = ew.Write(nil)
	_ = mw.Close()
	out, ct, err := Body(mw.FormDataContentType(), in.Bytes(), "")
	if err != nil || !strings.HasPrefix(ct, "multipart/form-data; boundary=") || strings.Contains(ct, mw.Boundary()) {
		t.Fatalf("multipart: %q %v", ct, err)
	}
	for _, want := range []string{`name="title"`, "Hello", `filename="report final.pdf"`, "%PDF-1.4", `name="empty"; filename=""`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("multipart lacks %q:\n%s", want, out)
		}
	}
	if _, _, err := Body("application/octet-stream", []byte{1, 2, 3}, ""); reason(err) != ReasonUnsupported {
		t.Errorf("binary: %v", err)
	}
	if out, ct, err := Body("", nil, ""); out != nil || ct != "" || err != nil {
		t.Errorf("empty body")
	}
}
