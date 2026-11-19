package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/thevidocq/equalizer/panel-api/internal/mapcheck"
)

// mapChecked asks a gateway whether the map would load. strict (publish):
// a gateway that does not answer refuses too; otherwise (a draft) it is only
// logged, since a draft is not live.
func (s *Server) mapChecked(ctx context.Context, doc json.RawMessage, strict bool) error {
	if s.MapCheck == nil {
		return nil
	}
	err := s.MapCheck.Check(ctx, doc)
	if errors.Is(err, mapcheck.ErrUnavailable) && !strict {
		s.Log.WarnContext(ctx, "map not checked by a gateway", "error", err.Error())
		return nil
	}
	return err
}

func (s *Server) checkMap(w http.ResponseWriter, r *http.Request, doc json.RawMessage, strict bool) bool {
	err := s.mapChecked(r.Context(), doc, strict)
	if err == nil {
		return true
	}
	if !s.mapCheckFail(w, err) {
		s.storeError(w, r, err)
	}
	return false
}

// mapCheckFail answers the errors of the map check; false for any other.
func (s *Server) mapCheckFail(w http.ResponseWriter, err error) bool {
	var inc *mapcheck.Inconsistent
	switch {
	case errors.As(err, &inc):
		fail(w, http.StatusUnprocessableEntity, "map_inconsistent", "The gateway would refuse this map: "+inc.Reason)
	case errors.Is(err, mapcheck.ErrUnavailable):
		fail(w, http.StatusServiceUnavailable, "map_unchecked", "No gateway could check the map now: try again in a moment.")
	default:
		return false
	}
	return true
}
