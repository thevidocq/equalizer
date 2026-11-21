package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// Paths that a legacy server reads as another path ("//admin/" as
// "/admin/" on Apache, "/admin;x/" on Tomcat, "/admin./" and "/Admin/" on
// Windows) must not step around an area with roles.
func TestAreaPathsAsServersReadThem(t *testing.T) {
	var mu sync.Mutex
	var hits []string
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.URL.EscapedPath())
		mu.Unlock()
		_, _ = io.WriteString(w, "ok")
	}))
	defer legacy.Close()
	u, _ := url.Parse(legacy.URL)
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "intra", Version: 1, Status: "published", PublicDomain: "intra.example"},
		Legacy: mapmodel.Legacy{BaseURL: legacy.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil, "admin": nil}},
		Areas: []mapmodel.Area{
			{ID: "site", Prefix: "/"},
			{ID: "admin", Prefix: "/admin/", Roles: []string{"admin"}},
		},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}
	for target, want := range map[string]int{
		"/news":           http.StatusOK,
		"/admin/users":    http.StatusForbidden,
		"//admin/users":   http.StatusForbidden,
		"/admin;x/users":  http.StatusForbidden,
		"/admin./users":   http.StatusForbidden,
		"/admin%20/users": http.StatusForbidden,
		"/Admin/users":    http.StatusBadRequest,
		"/ADMIN/users":    http.StatusBadRequest,
	} {
		mu.Lock()
		hits = nil
		mu.Unlock()
		req := httptest.NewRequest("GET", "http://intra.example"+target, nil)
		req.Host = "intra.example"
		rec := httptest.NewRecorder()
		p.Handler().ServeHTTP(rec, req)
		mu.Lock()
		reached := len(hits) > 0
		mu.Unlock()
		if rec.Code != want {
			t.Errorf("%s: %d, want %d", target, rec.Code, want)
		}
		if want != http.StatusOK && reached {
			t.Errorf("%s reached the legacy application", target)
		}
	}
}
