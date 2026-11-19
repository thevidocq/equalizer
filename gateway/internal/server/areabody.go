package server

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	areabody "github.com/thevidocq/equalizer/gateway/internal/rebuild"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/upload"
)

// maxMemoryBody bounds the bodies rebuilt in memory (forms, JSON, XML,
// text). Multipart forms, which carry uploads, are rebuilt into a
// temporary file up to the area's own limit.
const maxMemoryBody = 16 << 20

// rebuiltBody is a request body ready for the legacy application.
type rebuiltBody struct {
	r     io.Reader
	size  int64
	ctype string
	file  *os.File
	held  int64 // bytes of bodyMemory to give back
}

func (b *rebuiltBody) close() {
	if b.held > 0 {
		bodyMemory.give(b.held)
		b.held = 0
	}
	if b.file != nil {
		_ = b.file.Close()
		_ = os.Remove(b.file.Name())
	}
}

// areaBody reads and rebuilds the request body within the area's limit.
func (p *Public) areaBody(w http.ResponseWriter, r *http.Request, st *site.Site, a *mapmodel.Area, cs string) (*rebuiltBody, int, string) {
	none := &rebuiltBody{size: 0}
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return none, 0, ""
	}
	limit := int64(a.MaxBodyKB) * 1024
	if limit == 0 {
		limit = mapmodel.DefaultAreaBodyKB * 1024
	}
	src := http.MaxBytesReader(w, r.Body, limit)
	ctype := r.Header.Get("Content-Type")
	if mt, params, err := mime.ParseMediaType(ctype); err == nil && mt == "multipart/form-data" {
		return spoolMultipart(src, params["boundary"], cs)
	} else if err == nil && upload.IsRawType(mt) {
		if _, params, err := mime.ParseMediaType(r.Header.Get("Content-Disposition")); err == nil && areabody.RunnableName(params["filename"]) {
			return nil, http.StatusUnsupportedMediaType, areabody.ReasonFileRefused
		}
		return spoolRaw(src, mt)
	}
	// The body and its rebuilt copy are held until the legacy application
	// has answered: reserve them, sized by Content-Length when there is one.
	held := int64(maxMemoryBody)
	if r.ContentLength >= 0 && r.ContentLength < held {
		held = r.ContentLength
	}
	held *= 2
	if !bodyMemory.take(held) {
		return nil, http.StatusServiceUnavailable, "busy"
	}
	b := &rebuiltBody{held: held}
	raw, err := io.ReadAll(io.LimitReader(src, maxMemoryBody+1))
	if err != nil || len(raw) > maxMemoryBody {
		b.close()
		return bodyError(err, len(raw) > maxMemoryBody)
	}
	body, out, err := areabody.Body(ctype, raw, cs)
	if err != nil || body == nil {
		b.close()
		if err != nil {
			return rebuildError(err)
		}
		return none, 0, ""
	}
	body = st.Inbound.Bytes(body) // the legacy application keeps its own address
	b.r, b.size, b.ctype = bytes.NewReader(body), int64(len(body)), out
	return b, 0, ""
}

// spoolMultipart rebuilds a multipart form into a temporary file, removed
// once the legacy application has answered.
func spoolMultipart(src io.Reader, boundary, cs string) (*rebuiltBody, int, string) {
	f, err := os.CreateTemp("", "eq-upload-*")
	if err != nil {
		return nil, http.StatusInternalServerError, "spool_failed"
	}
	b := &rebuiltBody{file: f}
	ctype, err := areabody.Multipart(f, src, boundary, cs)
	if err != nil {
		b.close()
		var re *areabody.Error
		if errors.As(err, &re) {
			return rebuildError(err)
		}
		return bodyError(err, false)
	}
	size, err := f.Seek(0, io.SeekCurrent)
	if err == nil {
		_, err = f.Seek(0, io.SeekStart)
	}
	if err != nil {
		b.close()
		return nil, http.StatusInternalServerError, "spool_failed"
	}
	b.r, b.size, b.ctype = f, size, ctype
	return b, 0, ""
}

// spoolRaw keeps a file sent as the whole body (some JavaScript uploaders)
// in a temporary file, after checking that its bytes are what it claims.
func spoolRaw(src io.Reader, mt string) (*rebuiltBody, int, string) {
	head := make([]byte, 512)
	n, err := io.ReadFull(src, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return bodyError(err, false)
	}
	head = head[:n]
	if n == 0 {
		return &rebuiltBody{size: 0}, 0, ""
	}
	ctype, err := upload.CheckRaw(mt, head)
	if err != nil {
		return nil, http.StatusUnsupportedMediaType, "raw_type"
	}
	f, err := os.CreateTemp("", "eq-upload-*")
	if err != nil {
		return nil, http.StatusInternalServerError, "spool_failed"
	}
	b := &rebuiltBody{file: f, ctype: ctype}
	check := areabody.NewFileCheck(ctype) // PHP code anywhere in the file
	size, err := io.Copy(io.MultiWriter(check, f), io.MultiReader(bytes.NewReader(head), src))
	if err == nil {
		err = check.Done()
	}
	if err == nil {
		_, err = f.Seek(0, io.SeekStart)
	}
	if err != nil {
		b.close()
		var re *areabody.Error
		if errors.As(err, &re) {
			return nil, http.StatusUnsupportedMediaType, re.Reason
		}
		return bodyError(err, false)
	}
	b.r, b.size = f, size
	return b, 0, ""
}

func bodyError(err error, over bool) (*rebuiltBody, int, string) {
	var tooLarge *http.MaxBytesError
	if over || errors.As(err, &tooLarge) {
		return nil, http.StatusRequestEntityTooLarge, "body_too_large"
	}
	return nil, http.StatusBadRequest, "malformed_body"
}

func rebuildError(err error) (*rebuiltBody, int, string) {
	var re *areabody.Error
	if errors.As(err, &re) {
		if re.Reason == areabody.ReasonUnsupported || re.Reason == areabody.ReasonFileRefused {
			return nil, http.StatusUnsupportedMediaType, re.Reason
		}
		return nil, http.StatusBadRequest, re.Reason
	}
	return nil, http.StatusBadRequest, "malformed_body"
}

// rawDisposition rebuilds the Content-Disposition of a file sent as the
// whole body: "attachment" and a cleaned file name, nothing else.
func rawDisposition(r *http.Request) string {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !upload.IsRawType(mt) {
		return ""
	}
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Disposition"))
	if err != nil {
		return ""
	}
	name := areabody.CleanName(params["filename"])
	if name == "" {
		return ""
	}
	return mime.FormatMediaType("attachment", map[string]string{"filename": name})
}
