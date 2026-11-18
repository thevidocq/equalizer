package forward

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
)

// handOver tries the other replicas when this one holds no tunnel of the
// connector. It returns false when none has it (the caller answers 502).
// The peer checks token, tenant and destination again.
func (h *Handler) handOver(w http.ResponseWriter, r *http.Request, id string) bool {
	peers := h.Peers(r.Context())
	if len(peers) == 0 {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxHandover+1))
	if err != nil || len(body) > MaxHandover {
		return false
	}
	for _, p := range peers {
		out, err := http.NewRequestWithContext(r.Context(), r.Method, "http://"+p+r.URL.RequestURI(), bytes.NewReader(body))
		if err != nil {
			continue
		}
		out.Header = r.Header.Clone()
		out.Header.Set(HeaderHop, "1")
		resp, err := h.peer.Do(out)
		if err != nil {
			continue
		}
		if resp.Header.Get(HeaderError) == "connector_unavailable" {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			continue
		}
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		resp.Body.Close()
		h.Log.DebugContext(r.Context(), "request handed to another replica", slog.Group("detail", slog.String("connector_id", id)))
		return true
	}
	return false
}
