package legacyclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
	"github.com/thevidocq/equalizer/gateway/internal/upload"
)

// BuildUpload builds a multipart POST: the form values of legacy_request
// and the checked files, under their new names. files is keyed by the
// public field name used in legacy_request.files ("{{fields.doc}}").
func (c *Client) BuildUpload(ctx context.Context, lr mapmodel.LegacyRequest, vals placeholder.Values, files map[string]upload.File) (*http.Request, error) {
	if !strings.EqualFold(lr.Method, http.MethodPost) {
		return nil, errors.New("uploads are sent with POST")
	}
	plain := lr
	plain.Files, plain.Form = nil, nil
	req, err := c.Build(ctx, plain, vals) // path, query, headers
	if err != nil {
		return nil, err
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, key := range sortedKeys(lr.Form) {
		v, absent, err := placeholder.Resolve(lr.Form[key], vals)
		if err != nil {
			return nil, fmt.Errorf("form %q: %w", key, err)
		}
		if !absent {
			if err := mw.WriteField(key, v); err != nil {
				return nil, err
			}
		}
	}
	for _, key := range sortedKeys(lr.Files) {
		ns, name, ok := placeholder.Single(lr.Files[key])
		if !ok || ns != "fields" {
			return nil, fmt.Errorf("files %q: a single {{fields.x}} is expected", key)
		}
		f, present := files[name]
		if !present {
			continue
		}
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, key, f.Name))
		h.Set("Content-Type", f.Type)
		part, err := mw.CreatePart(h)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(f.Data); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	up, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL.String(), &body)
	if err != nil {
		return nil, err
	}
	up.Header = req.Header.Clone()
	up.Header.Set("Content-Type", mw.FormDataContentType())
	return up, nil
}
