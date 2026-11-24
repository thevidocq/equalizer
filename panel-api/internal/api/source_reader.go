package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/sourceschema"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// The source reader's side: it authenticates with its site's token and
// only ever sees its own site.

type readerKey struct{}

func (s *Server) sourceReader(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(tok) < 20 {
			fail(w, http.StatusUnauthorized, "unauthorized", "")
			return
		}
		sum := sha256.Sum256([]byte(tok))
		app, err := s.Store.SourceApp(r.Context(), hex.EncodeToString(sum[:]))
		if errors.Is(err, store.ErrNotFound) {
			fail(w, http.StatusUnauthorized, "unauthorized", "")
			return
		}
		if err != nil {
			s.storeError(w, r, err)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), readerKey{}, app)))
	}
}

func readerApp(r *http.Request) string { app, _ := r.Context().Value(readerKey{}).(string); return app }

func (s *Server) sourceHello(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version   string `json:"version"`
		PublicKey string `json:"public_key"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.PublicKey = strings.TrimSpace(in.PublicKey)
	if len(in.Version) > 40 || len(in.PublicKey) > 800 ||
		in.PublicKey != "" && (!strings.HasPrefix(in.PublicKey, "ssh-ed25519 ") || strings.ContainsAny(in.PublicKey, "\n\r")) {
		fail(w, http.StatusUnprocessableEntity, "invalid", "version or public_key")
		return
	}
	if err := s.Store.SourceHello(r.Context(), readerApp(r), in.Version, in.PublicKey); err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type sourceJob struct {
	ID       int64   `json:"id"`
	Method   string  `json:"method"`
	Host     *string `json:"host"`
	Port     *int    `json:"port"`
	Username *string `json:"username"`
	Path     string  `json:"path"`
	Share    *string `json:"share"`
	Password *string `json:"password"`
}

func (s *Server) sourceClaim(w http.ResponseWriter, r *http.Request) {
	app := readerApp(r)
	c, err := s.Store.SourceReaderOf(r.Context(), app)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	read, err := s.Store.ClaimSourceRead(r.Context(), app)
	if errors.Is(err, store.ErrNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if c.Method == nil || c.Path == nil {
		msg := "the source reader is not set up: choose how it reaches the files"
		_ = s.Store.FinishSourceRead(r.Context(), app, read.ID, nil, &msg)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	job := sourceJob{ID: read.ID, Method: *c.Method, Host: c.Host, Port: c.Port, Username: c.Username, Path: *c.Path, Share: c.Share}
	if len(c.Password) > 0 {
		pw, err := s.MFA.OpenFor("source:"+app, c.Password)
		if err != nil {
			msg := "the saved password cannot be read: save it again"
			_ = s.Store.FinishSourceRead(r.Context(), app, read.ID, nil, &msg)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		p := string(pw)
		job.Password = &p
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) sourceResult(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	var in struct {
		Summary json.RawMessage `json:"summary"`
		Error   *string         `json:"error"`
	}
	if !decode(w, r, &in) {
		return
	}
	var failure *string
	switch {
	case in.Error != nil:
		msg := *in.Error
		if len(msg) > 1000 {
			msg = msg[:1000]
		}
		failure = &msg
	case len(in.Summary) == 0 || string(in.Summary) == "null":
		fail(w, http.StatusUnprocessableEntity, "invalid", "a summary or an error")
		return
	default:
		if err := sourceschema.Validate(in.Summary); err != nil {
			// The summary is refused, never stored: the read fails.
			msg := "the summary does not follow its schema: " + err.Error()
			if len(msg) > 1000 {
				msg = msg[:1000]
			}
			failure = &msg
			s.Log.WarnContext(r.Context(), "source summary refused", slog.Int64("read", id))
		}
	}
	summary := in.Summary
	if failure != nil {
		summary = nil
	}
	if err := s.Store.FinishSourceRead(r.Context(), readerApp(r), id, summary, failure); err != nil {
		s.storeError(w, r, err)
		return
	}
	if failure != nil && in.Error == nil {
		fail(w, http.StatusUnprocessableEntity, "invalid", *failure)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// sourceCounts tells what a read found, without the summary itself.
type sourceCounts struct {
	Files      int    `json:"files"`
	Routes     int    `json:"routes"`
	Inputs     int    `json:"inputs"`
	Risks      int    `json:"risks"`
	Components int    `json:"components"`
	Stack      string `json:"stack"`
}

func readView(read store.SourceRead) *sourceReadView {
	v := &sourceReadView{SourceRead: read}
	if len(read.Summary) == 0 {
		return v
	}
	var sum struct {
		Stack string `json:"stack"`
		Files struct {
			Read int `json:"read"`
		} `json:"files"`
		Routes, Inputs, Risks, Components []json.RawMessage
	}
	if json.Unmarshal(read.Summary, &sum) == nil {
		v.Counts = &sourceCounts{Files: sum.Files.Read, Routes: len(sum.Routes), Inputs: len(sum.Inputs), Risks: len(sum.Risks),
			Components: len(sum.Components), Stack: sum.Stack}
	}
	return v
}
