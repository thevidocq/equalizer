// Command attacklab runs a session of real WordPress and Joomla exploit
// payloads against the gateway and reports, for each, what reached the
// backend. The backend is an echo that records the exact request it is
// given, so the result measures what the gateway rebuilds and forwards, not
// whether a specific CMS version would execute it. Two maps are compared:
// "open" (one area "/" serving GET and POST, as the templates ship today)
// and "strict" (the area serves only GET/HEAD; POST and APIs go through
// precise entries), to show what a stricter map would change.
//
//	go run ./cmd/attacklab
package main

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"sync"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/server"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// seen is the last request the backend was given.
type seen struct {
	method, path, query, host, xff, origURL string
	body                                    []byte
	files                                   map[string]string // filename -> content
	reached                                 bool
}

type backend struct {
	mu   sync.Mutex
	last seen
}

func (b *backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s := seen{
		method: r.Method, path: r.URL.Path, query: r.URL.RawQuery,
		host: r.Host, xff: r.Header.Get("X-Forwarded-For"),
		origURL: r.Header.Get("X-Original-URL"), reached: true,
		files: map[string]string{},
	}
	if ct := r.Header.Get("Content-Type"); strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err == nil && r.MultipartForm != nil {
			for name, fhs := range r.MultipartForm.File {
				for _, fh := range fhs {
					f, _ := fh.Open()
					data, _ := io.ReadAll(f)
					s.files[name+":"+fh.Filename] = string(data)
				}
			}
		}
	} else {
		s.body, _ = io.ReadAll(r.Body)
	}
	b.mu.Lock()
	b.last = s
	b.mu.Unlock()
	w.Header().Set("Content-Type", "text/html")
	_, _ = io.WriteString(w, "<html><body>ok</body></html>")
}

func (b *backend) take() seen {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.last
	b.last = seen{}
	return s
}

type maps struct{ s *site.Site }

func (m maps) For(string) *site.Site { return m.s }
func (m maps) Ready() bool           { return true }

func tru() *bool             { v := true; return &v }
func f64(v float64) *float64 { return &v }

// buildGateway wires a Public in direct mode towards the backend.
func buildGateway(host, legacyURL string, strict bool) http.Handler {
	u, _ := url.Parse(legacyURL)
	methods := []string{"GET", "HEAD", "POST", "PUT", "DELETE"}
	if strict {
		methods = []string{"GET", "HEAD"}
	}
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "cms", Version: 1, Status: "published", PublicDomain: host},
		Legacy: mapmodel.Legacy{BaseURL: legacyURL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil, "admin": nil}},
		Areas: []mapmodel.Area{
			{ID: "site", Prefix: "/", Methods: methods},
			{ID: "admin", Prefix: "/wp-admin/", Roles: []string{"admin"}, Methods: methods},
			{ID: "jadmin", Prefix: "/administrator/", Roles: []string{"admin"}, Methods: methods},
		},
		// A precise API with a typed id (strict maps declare such entries
		// instead of letting POST and ids reach the app raw).
		API: []mapmodel.APIRoute{{
			ID: "post", Path: "/wp-json/wp/v2/posts/{id}", Method: "GET",
			Roles:         []string{"guest"},
			Params:        map[string]mapmodel.FieldSpec{"id": {Type: "int", Min: f64(1), Max: f64(1e9), Required: tru()}},
			LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/wp-json/wp/v2/posts/{{params.id}}"},
			Response:      mapmodel.APIResponse{Fields: map[string]mapmodel.FieldSpec{"id": {Type: "int"}}},
		}},
	}
	st, err := site.Build(m)
	if err != nil {
		panic(host + ": " + err.Error())
	}
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	return (&server.Public{
		Maps: maps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "attacklab"),
	}).Handler()
}

type attack struct {
	name string
	cve  string
	req  func(host string) *http.Request
	// verdict reads what the backend got (reached=false means blocked) and
	// returns one of: "PASSA", "RICOSTRUITO", "BLOCCATO".
	verdict func(status int, got seen) string
}

func blockedOr(check func(seen) string) func(int, seen) string {
	return func(status int, got seen) string {
		if !got.reached {
			return "BLOCCATO"
		}
		return check(got)
	}
}

func req(method, host, target, ctype, body string, hdr ...string) *http.Request {
	r := httptest.NewRequest(method, "http://"+host+target, strings.NewReader(body))
	r.Host = host
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	return r
}

func multipartReq(host, target, field, filename, content string) *http.Request {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	h.Set("Content-Type", "application/octet-stream")
	pw, _ := mw.CreatePart(h)
	_, _ = io.WriteString(pw, content)
	_ = mw.Close()
	r := httptest.NewRequest("POST", "http://"+host+target, &b)
	r.Host = host
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func hasFileWith(got seen, needle string) bool {
	for _, c := range got.files {
		if strings.Contains(c, needle) {
			return true
		}
	}
	return false
}

func main() {
	be := &backend{}
	srv := httptest.NewServer(be)
	defer srv.Close()

	const wp = "www.acme.test"
	const jm = "joomla.acme.test"

	attacks := []attack{
		{
			name: "wp2shell: batch route confusion + SQLi (via /wp-json)", cve: "CVE-2026-63030/60137",
			req: func(h string) *http.Request {
				body := `{"requests":[{"method":"POST","path":"/wp/v2/users"},{"method":"GET","path":"/wp/v2/posts?author_exclude[0]=1) UNION SELECT user_pass FROM wp_users-- -"}]}`
				return req("POST", h, "/wp-json/batch/v1", "application/json", body)
			},
			verdict: blockedOr(func(g seen) string {
				if strings.Contains(string(g.body), "UNION SELECT") {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "wp2shell via ?rest_route (query form of the batch)", cve: "CVE-2026-63030",
			req: func(h string) *http.Request {
				return req("POST", h, "/?rest_route=/batch/v1", "application/json",
					`{"requests":[{"path":"/wp/v2/posts?author_exclude[0]=1) UNION SELECT user_pass FROM wp_users-- -"}]}`)
			},
			verdict: blockedOr(func(g seen) string {
				if strings.Contains(string(g.body), "UNION SELECT") {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "XSS2Shell: reflected XSS on wp-login.php", cve: "CVE-2026-64638",
			req: func(h string) *http.Request {
				return req("GET", h, "/wp-login.php?redirect_to=%22%3E%3Cscript%3Ealert(1)%3C/script%3E&foo=bar", "", "")
			},
			verdict: blockedOr(func(g seen) string {
				if strings.Contains(g.query, "script") {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "PATH_INFO: /wp-login.php/x.php runs wp-login.php", cve: "classe",
			req: func(h string) *http.Request { return req("GET", h, "/wp-login.php/evil.php", "", "") },
			verdict: blockedOr(func(g seen) string {
				if strings.HasSuffix(g.path, "/evil.php") {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "Path traversal: /wp-content/../../../etc/passwd", cve: "classe",
			req:     func(h string) *http.Request { return req("GET", h, "/wp-content/../../../etc/passwd", "", "") },
			verdict: blockedOr(func(g seen) string { return "PASSA" }),
		},
		{
			name: "Upload webshell shell.php", cve: "classe upload RCE",
			req: func(h string) *http.Request {
				return multipartReq(h, "/wp-admin/admin-ajax.php", "file", "shell.php", "<?php system($_GET['c']); ?>")
			},
			verdict: blockedOr(func(g seen) string {
				if hasFileWith(g, "<?php") {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "Upload polyglot shell.php.png", cve: "classe upload RCE",
			req: func(h string) *http.Request {
				return multipartReq(h, "/wp-admin/admin-ajax.php", "file", "shell.php.png", "GIF89a<?php system($_GET['c']); ?>")
			},
			verdict: blockedOr(func(g seen) string {
				if hasFileWith(g, "<?php") {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "Upload shell.shtml (SSI)", cve: "CVE-2026-73373 (Joomla), classe",
			req: func(h string) *http.Request {
				return multipartReq(h, "/wp-admin/admin-ajax.php", "file", "shell.shtml", `<!--#exec cmd="id"-->`)
			},
			verdict: blockedOr(func(g seen) string {
				if len(g.files) > 0 {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "Host header injection (password reset poisoning)", cve: "classe (WP 4.6 CVE-2016-10033)",
			req: func(h string) *http.Request {
				return req("POST", h, "/wp-login.php?action=lostpassword", "application/x-www-form-urlencoded",
					"user_login=admin", "Host", "attacker.evil")
			},
			verdict: blockedOr(func(g seen) string {
				if strings.Contains(g.host, "attacker") {
					return "PASSA"
				}
				return "RICOSTRUITO (Host riscritto)"
			}),
		},
		{
			name: "X-Forwarded-For / X-Original-URL spoofing", cve: "classe",
			req: func(h string) *http.Request {
				return req("GET", h, "/", "", "", "X-Forwarded-For", "127.0.0.1", "X-Original-URL", "/wp-admin/")
			},
			verdict: blockedOr(func(g seen) string {
				if strings.Contains(g.xff, "127.0.0.1") || g.origURL != "" {
					return "PASSA"
				}
				return "RICOSTRUITO (header rimossi)"
			}),
		},
		{
			name: "SQLi in a typed id (/wp-json/wp/v2/posts/{id})", cve: "classe",
			req: func(h string) *http.Request {
				return req("GET", h, "/wp-json/wp/v2/posts/1%20OR%201=1", "", "")
			},
			verdict: blockedOr(func(g seen) string {
				if strings.Contains(g.path, "OR 1=1") || strings.Contains(g.query, "OR 1=1") {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "Joomla com_ajax abuse", cve: "CVE-2026-21629",
			req: func(h string) *http.Request {
				return req("GET", h, "/index.php?option=com_ajax&plugin=evil&format=raw", "", "")
			},
			verdict: blockedOr(func(g seen) string {
				if strings.Contains(g.query, "com_ajax") {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "Joomla JCE profiles.import", cve: "CVE-2026-48907",
			req: func(h string) *http.Request {
				return req("POST", h, "/index.php?option=com_jce&task=profiles.import", "application/x-www-form-urlencoded", "x=1")
			},
			verdict: blockedOr(func(g seen) string {
				if strings.Contains(g.query, "com_jce") {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "Joomla LFI via layout parameter", cve: "CVE-2026-40383",
			req: func(h string) *http.Request {
				return req("GET", h, "/index.php?option=com_users&view=login&layout=../../../../../../etc/passwd", "", "")
			},
			verdict: blockedOr(func(g seen) string {
				if strings.Contains(g.query, "passwd") {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
		{
			name: "Area con ruoli scavalcata: //administrator/", cve: "fix di oggi",
			req:     func(h string) *http.Request { return req("GET", h, "//administrator/index.php", "", "") },
			verdict: blockedOr(func(g seen) string { return "PASSA" }),
		},
		{
			name: "Area con ruoli scavalcata: /administrator;x/", cve: "fix di oggi",
			req:     func(h string) *http.Request { return req("GET", h, "/administrator;x/index.php", "", "") },
			verdict: blockedOr(func(g seen) string { return "PASSA" }),
		},
		{
			name: "JSON con chiavi duplicate (role confusion)", cve: "classe",
			req: func(h string) *http.Request {
				return req("POST", h, "/wp-json/wp/v2/users", "application/json", `{"role":"subscriber","role":"administrator"}`)
			},
			verdict: blockedOr(func(g seen) string {
				if strings.Count(string(g.body), "role") > 1 {
					return "PASSA"
				}
				return "RICOSTRUITO"
			}),
		},
	}

	type gw struct {
		open, strict http.Handler
	}
	gws := map[string]gw{
		wp: {buildGateway(wp, srv.URL, false), buildGateway(wp, srv.URL, true)},
		jm: {buildGateway(jm, srv.URL, false), buildGateway(jm, srv.URL, true)},
	}

	run := func(h http.Handler, host string, a attack) (int, string) {
		be.take() // reset
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, a.req(host))
		return rec.Code, a.verdict(rec.Code, be.take())
	}

	// Which CMS each attack targets (for the host).
	hostFor := func(a attack) string {
		if strings.HasPrefix(a.name, "Joomla") || strings.Contains(a.name, "administrator") {
			return jm
		}
		return wp
	}

	fmt.Printf("%-52s %-26s %-28s %s\n", "ATTACCO", "CVE", "MAPPA OGGI (area /)", "MAPPA STRICT")
	fmt.Println(strings.Repeat("-", 140))
	var passToday, passStrict int
	for _, a := range attacks {
		host := hostFor(a)
		g := gws[host]
		c1, v1 := run(g.open, host, a)
		c2, v2 := run(g.strict, host, a)
		if strings.HasPrefix(v1, "PASSA") {
			passToday++
		}
		if strings.HasPrefix(v2, "PASSA") {
			passStrict++
		}
		fmt.Printf("%-52s %-26s %-28s %s\n", trunc(a.name, 52), trunc(a.cve, 26),
			fmt.Sprintf("%d %s", c1, v1), fmt.Sprintf("%d %s", c2, v2))
	}
	fmt.Println(strings.Repeat("-", 140))
	fmt.Printf("Payload che raggiungono il backend intatti: oggi %d/%d, strict %d/%d\n",
		passToday, len(attacks), passStrict, len(attacks))
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
