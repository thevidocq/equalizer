package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// apiError is the error body of the contract.
type apiError struct {
	Error   string   `json:"error"`
	Message string   `json:"message,omitempty"`
	Details []string `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func list[T any](items []T) map[string]any {
	if items == nil {
		items = []T{}
	}
	return map[string]any{"items": items, "next_cursor": nil}
}

func fail(w http.ResponseWriter, status int, code, message string, details ...string) {
	writeJSON(w, status, apiError{Error: code, Message: message, Details: details})
}

// decode reads a JSON body; unknown fields are an error.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, "body_too_large", "")
			return false
		}
		fail(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return false
	}
	return true
}

// storeError answers a store error with the right status.
func (s *Server) storeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(w, http.StatusNotFound, "not_found", "")
	case errors.Is(err, store.ErrConflict):
		fail(w, http.StatusConflict, "already_exists", "")
	case errors.Is(err, store.ErrState):
		fail(w, http.StatusConflict, "wrong_state", "the version is not in a state that allows this action")
	default:
		s.Log.ErrorContext(r.Context(), "store error", slog.String("error", err.Error()))
		fail(w, http.StatusInternalServerError, "internal", "")
	}
}

func forbidden(w http.ResponseWriter) { fail(w, http.StatusForbidden, "forbidden", "") }

func pathInt(r *http.Request, name string) (int, bool) {
	n, err := strconv.Atoi(r.PathValue(name))
	return n, err == nil && n > 0
}
