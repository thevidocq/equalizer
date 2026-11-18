package server

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/thevidocq/equalizer/gateway/internal/formmap"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
)

// bindingCookie ties form tokens to one browser. The __Host- prefix makes
// the browser accept it only over HTTPS, for this host, on path "/".
const bindingCookie = "__Host-eq_form"

// binder is the per-request pipeline.FormBinder: it matches legacy forms
// with the map and seals tokens bound to the browser's cookie, creating the
// cookie the first time a page contains a form.
type binder struct {
	forms  *formmap.Map
	sealer *formtoken.Sealer
	cookie string
	fresh  bool // the cookie was created for this response
	used   bool
}

func (p *Public) newBinder(r *http.Request, forms *formmap.Map) *binder {
	if p.Forms == nil || forms == nil {
		return nil
	}
	b := &binder{forms: forms, sealer: p.Forms}
	if c, err := r.Cookie(bindingCookie); err == nil && validBinding(c.Value) {
		b.cookie = c.Value
	}
	return b
}

func (b *binder) Match(action *url.URL, hidden map[string]string) (pipeline.FormMatch, bool) {
	m, ok := b.forms.Match(action, hidden)
	if !ok {
		return pipeline.FormMatch{}, false
	}
	return pipeline.FormMatch{FormID: m.FormID, Action: m.Action, Params: m.Params, Hidden: m.Hidden, Rename: m.Rename}, true
}

func (b *binder) Token(m pipeline.FormMatch, charset string) (string, error) {
	if b.cookie == "" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return "", err
		}
		b.cookie, b.fresh = base64.RawURLEncoding.EncodeToString(raw), true
	}
	b.used = true
	return b.sealer.Seal(formtoken.State{Form: m.FormID, Params: m.Params, Hidden: m.Hidden, Charset: charset}, b.cookie)
}

// setCookie sends the binding cookie if this response created it.
func (b *binder) setCookie(w http.ResponseWriter) {
	if b == nil || !b.used || !b.fresh {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: bindingCookie, Value: b.cookie, Path: "/",
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

// pipelineBinder avoids a typed nil inside the interface.
func (b *binder) pipelineBinder() pipeline.FormBinder {
	if b == nil {
		return nil
	}
	return b
}

func validBinding(v string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(v)
	return err == nil && len(raw) == 32
}
