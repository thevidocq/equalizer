package api

import (
	"errors"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// The source reader of a site: an optional component next to the
// connector that reads the site's files, read only, and sends a summary
// (schema/source/summary.schema.json). This file holds the tenant's side;
// source_reader.go holds the reader's.

type sourceReaderView struct {
	Configured  bool            `json:"configured"`
	Method      *string         `json:"method"`
	Host        *string         `json:"host"`
	Port        *int            `json:"port"`
	Username    *string         `json:"username"`
	Path        *string         `json:"path"`
	Share       *string         `json:"share"`
	Container   *string         `json:"container"`
	HasPassword bool            `json:"has_password"`
	Status      string          `json:"status"`
	PublicKey   *string         `json:"public_key"`
	Version     *string         `json:"version"`
	LastSeen    *time.Time      `json:"last_seen"`
	LastRead    *sourceReadView `json:"last_read"`
}

type sourceReadView struct {
	store.SourceRead
	Counts *sourceCounts `json:"counts"`
}

// sourceHeard is how long a reader stays ready after its last hello (it
// says hello every minute).
const sourceHeard = 2 * time.Minute

var (
	sourceHostRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,252}$`)
	sourceUserRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._@-]{0,63}$`)
	// The reader's own allowlist (source-reader/internal/fetch): the path
	// reaches the server's shell (rsync) and smbclient's command string.
	sourcePathRe = regexp.MustCompile(`^/[A-Za-z0-9._@+~ /-]{0,299}$`)
	sourceShrRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._$-]{0,79}$`)
	containerRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
)

func readerStatus(c store.SourceReader, now time.Time) string {
	switch {
	case c.Method == nil:
		return "not_set_up"
	case !c.HasToken || c.LastSeen == nil:
		return "not_installed"
	case now.Sub(*c.LastSeen) > sourceHeard:
		return "waiting"
	}
	return "ready"
}

// sourceReady tells whether the site's reader can read now.
func (s *Server) sourceReady(r *http.Request, app string) bool {
	c, err := s.Store.SourceReaderOf(r.Context(), app)
	return err == nil && readerStatus(c, time.Now()) == "ready"
}

func (s *Server) sourceView(r *http.Request, app string) (sourceReaderView, error) {
	c, err := s.Store.SourceReaderOf(r.Context(), app)
	if errors.Is(err, store.ErrNotFound) {
		return sourceReaderView{Status: "not_set_up"}, nil
	}
	if err != nil {
		return sourceReaderView{}, err
	}
	v := sourceReaderView{Configured: c.Method != nil, Method: c.Method, Host: c.Host, Port: c.Port, Username: c.Username,
		Path: c.Path, Share: c.Share, Container: c.Container, HasPassword: len(c.Password) > 0, Status: readerStatus(c, time.Now()),
		PublicKey: c.PublicKey, Version: c.Version, LastSeen: c.LastSeen}
	last, err := s.Store.LatestSourceRead(r.Context(), app)
	if err == nil {
		v.LastRead = readView(last)
	} else if !errors.Is(err, store.ErrNotFound) {
		return sourceReaderView{}, err
	}
	return v, nil
}

func (s *Server) getSource(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	v, err := s.sourceView(r, a.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type sourceUpdate struct {
	Method    string `json:"method"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Path      string `json:"path"`
	Share     string `json:"share"`
	Password  string `json:"password"`
	Container string `json:"container"`
}

// check normalises the update; a message when it is not acceptable.
func (in *sourceUpdate) check() string {
	in.Host, in.Username, in.Path, in.Share = strings.TrimSpace(in.Host), strings.TrimSpace(in.Username),
		strings.TrimSpace(in.Path), strings.TrimSpace(in.Share)
	if in.Path == "" {
		in.Path = "/"
	}
	if !sourcePathRe.MatchString(in.Path) || strings.Contains("/"+in.Path+"/", "/../") {
		return "path: an absolute folder: letters, digits, spaces and . _ @ + ~ - /, no .."
	}
	switch in.Method {
	case "local":
		in.Host, in.Port, in.Username, in.Share, in.Password = "", 0, "", "", ""
		in.Container = strings.TrimSpace(in.Container)
		if in.Container != "" && !containerRe.MatchString(in.Container) {
			return "container: the name of the site's Docker container"
		}
		if in.Container == "" && systemFolder(in.Path) {
			return "path: the site's folder, not the whole disk or a system folder"
		}
		return ""
	case "sftp":
		in.Share, in.Password = "", ""
		if in.Port == 0 {
			in.Port = 22
		}
	case "smb":
		if !sourceShrRe.MatchString(in.Share) {
			return "share: the name of the share"
		}
		if in.Port == 0 {
			in.Port = 445
		}
	default:
		return "method: sftp, smb or local"
	}
	switch {
	case !sourceHostRe.MatchString(in.Host):
		return "host: a name or an address"
	case !sourceUserRe.MatchString(in.Username):
		return "username: letters, digits, . _ @ -"
	case in.Port < 1 || in.Port > 65535:
		return "port: between 1 and 65535"
	case len(in.Password) > 200 || strings.ContainsAny(in.Password, "\x00\r\n"):
		return "password: at most 200 characters, on one line"
	}
	return ""
}

func (s *Server) putSource(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	var in sourceUpdate
	if !decode(w, r, &in) {
		return
	}
	if msg := in.check(); msg != "" {
		fail(w, http.StatusUnprocessableEntity, "invalid", msg)
		return
	}
	c := store.SourceReader{App: a.ID, Method: &in.Method, Path: &in.Path}
	if in.Method != "local" {
		c.Host, c.Port, c.Username = &in.Host, &in.Port, &in.Username
	}
	if in.Method == "smb" {
		c.Share = &in.Share
	}
	if in.Method == "local" && in.Container != "" {
		c.Container = &in.Container
	}
	// The saved password serves the same smb host and account only: an
	// empty one for another erases it.
	old, err := s.Store.SourceReaderOf(r.Context(), a.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.storeError(w, r, err)
		return
	}
	switch {
	case in.Password != "":
		c.Password = s.MFA.SealFor("source:"+a.ID, []byte(in.Password))
	case in.Method != "smb" || old.Host == nil || *old.Host != in.Host || old.Username == nil || *old.Username != in.Username:
		c.Password = []byte{}
	}
	if err := s.Store.SetSourceReader(r.Context(), c); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "source.configure", a.ID,
		map[string]any{"method": in.Method, "host": in.Host})
	s.getSource(w, r, u)
}

func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	if err := s.Store.DeleteSourceReader(r.Context(), a.ID); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "source.delete", a.ID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sourceToken(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	tok, hash := newToken()
	if err := s.Store.SetSourceToken(r.Context(), a.ID, hash); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "source.token", a.ID, nil)
	c, _ := s.Store.SourceReaderOf(r.Context(), a.ID)
	writeJSON(w, http.StatusCreated, map[string]string{"token": tok, "install": s.sourceInstall(tok, c)})
}

// sourceInstall is the command that starts the reader next to the
// connector. With the local method it mounts the site, read only: the
// volumes of its container, or its folder at the same path, so the folder
// saved in the panel is the one the reader reads.
func (s *Server) sourceInstall(token string, c store.SourceReader) string {
	tag := strings.TrimPrefix(s.Version, "v")
	if !releaseRe.MatchString(tag) {
		tag = "latest"
	}
	lines := []string{"docker run -d --name eq-source-reader --restart unless-stopped \\"}
	if c.Method != nil && *c.Method == "local" {
		switch {
		case c.Container != nil:
			lines = append(lines, "  --volumes-from "+*c.Container+":ro \\")
		case c.Path != nil:
			// Quoted: the path may hold spaces, and has no single quote.
			lines = append(lines, "  -v '"+*c.Path+":"+*c.Path+":ro' \\")
		}
		lines = append(lines, "  -e EQ_SOURCE_SITE=/ \\")
	}
	return strings.Join(append(lines,
		"  -e EQ_PANEL_URL="+s.PanelURL+" \\",
		"  -e EQ_SOURCE_TOKEN="+token+" \\",
		"  -v eq-source-reader:/var/lib/equalizer-source \\",
		"  ghcr.io/thevidocq/equalizer-source-reader:"+tag,
	), "\n")
}

var releaseRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// systemFolder is the whole disk or a folder of the system: mounting it in
// the reader would show it far more than the site.
func systemFolder(p string) bool {
	p = path.Clean(p)
	if p == "/" {
		return true
	}
	top := strings.SplitN(strings.TrimPrefix(p, "/"), "/", 2)[0]
	switch top {
	case "bin", "boot", "dev", "etc", "lib", "lib32", "lib64", "proc", "root", "run", "sbin", "sys", "usr":
		return true
	}
	return p == "/var" || p == "/home" || p == "/var/lib" || p == "/var/lib/docker" || p == "/opt"
}

func (s *Server) startSourceRead(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	if !s.sourceReady(r, a.ID) {
		fail(w, http.StatusConflict, "no_reader", "the source reader of this site is not set up or not answering")
		return
	}
	read, err := s.Store.QueueSourceRead(r.Context(), a.ID, nil, &u.ID)
	if errors.Is(err, store.ErrConflict) {
		fail(w, http.StatusConflict, "read_in_progress", "a read of this site's files is already going on")
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "source.read", a.ID, map[string]any{"id": read.ID})
	writeJSON(w, http.StatusAccepted, readView(read))
}
