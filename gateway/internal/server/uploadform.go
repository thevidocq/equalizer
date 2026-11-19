package server

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	areabody "github.com/thevidocq/equalizer/gateway/internal/rebuild"
	"github.com/thevidocq/equalizer/gateway/internal/upload"
	"github.com/thevidocq/equalizer/gateway/internal/validate"
)

// MaxUploadBody bounds a multipart submission, files included.
const MaxUploadBody = 50 << 20

// readSubmission reads an urlencoded body, or a multipart one when the form
// declares files. Only declared file fields are read, one file each, each
// bounded by its max_size_kb; their names and types from the browser are
// ignored.
func readSubmission(w http.ResponseWriter, r *http.Request, form *mapmodel.Form) (url.Values, map[string][]byte, int, string) {
	mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	_, files := splitFiles(form.Fields)
	if err != nil || mt != "multipart/form-data" || len(files) == 0 {
		vals, status, reason := readForm(w, r)
		return vals, nil, status, reason
	}
	limit := int64(MaxFormBody)
	for _, f := range files {
		limit += int64(f.MaxSizeKB) * 1024
	}
	mr := multipart.NewReader(http.MaxBytesReader(w, r.Body, min(limit, MaxUploadBody)), params["boundary"])
	vals, blobs := url.Values{}, map[string][]byte{}
	text := 0
	for parts := 0; ; parts++ {
		if parts >= areabody.MaxFields {
			return nil, nil, http.StatusBadRequest, areabody.ReasonTooMany
		}
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return vals, blobs, 0, ""
		}
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				return nil, nil, http.StatusRequestEntityTooLarge, "body_too_large"
			}
			return nil, nil, http.StatusBadRequest, "malformed_body"
		}
		name := part.FormName()
		if spec, isFile := files[name]; isFile && part.FileName() != "" {
			if _, dup := blobs[name]; dup {
				return nil, nil, http.StatusBadRequest, "duplicate_file"
			}
			data, err := io.ReadAll(io.LimitReader(part, int64(spec.MaxSizeKB)*1024+1))
			if err != nil {
				return nil, nil, http.StatusBadRequest, "malformed_body"
			}
			if len(data) > 0 {
				blobs[name] = data
			}
			continue
		}
		if part.FileName() != "" {
			return nil, nil, http.StatusBadRequest, "undeclared_file"
		}
		v, err := io.ReadAll(io.LimitReader(part, MaxFormBody+1))
		text += len(v)
		if err != nil || text > MaxFormBody {
			return nil, nil, http.StatusRequestEntityTooLarge, "body_too_large"
		}
		vals.Add(name, string(v))
	}
}

// splitFiles separates the file fields from the others.
func splitFiles(fields map[string]mapmodel.FieldSpec) (map[string]mapmodel.FieldSpec, map[string]mapmodel.FieldSpec) {
	text, files := map[string]mapmodel.FieldSpec{}, map[string]mapmodel.FieldSpec{}
	for k, v := range fields {
		if v.Type == "file" {
			files[k] = v
		} else {
			text[k] = v
		}
	}
	return text, files
}

// checkFiles applies type, size and rebuild to every file.
func (p *Public) checkFiles(r *http.Request, specs map[string]mapmodel.FieldSpec, blobs map[string][]byte) (map[string]upload.File, error) {
	out := map[string]upload.File{}
	for name, spec := range specs {
		data, present := blobs[name]
		if !present {
			if spec.Required == nil || *spec.Required {
				return nil, &validate.Error{Field: name, Reason: validate.ReasonMissing}
			}
			continue
		}
		f, err := upload.Check(name, spec, data)
		if err != nil {
			var ue *upload.Error
			if errors.As(err, &ue) {
				return nil, &validate.Error{Field: name, Reason: ue.Reason}
			}
			return nil, err
		}
		out[name] = f
	}
	return out, nil
}
