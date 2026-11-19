package forward

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
)

// handOver tries the other replicas when this one holds no tunnel of the
// connector. It returns false when none has it (the caller answers 502).
// The peer checks token, tenant and destination again.
func (h *Handler) handOver(w http.ResponseWriter, r *http.Request, id string) bool {
	peers := h.Peers(r.Context())
	if len(peers) == 0 {
		return false
	}
	body, size, done, err := keep(r.Body)
	if err != nil {
		return false
	}
	defer done()
	for _, p := range peers {
		if _, err := body.Seek(0, io.SeekStart); err != nil {
			return false
		}
		out, err := http.NewRequestWithContext(r.Context(), r.Method, "http://"+p+r.URL.RequestURI(), io.NopCloser(body))
		if err != nil {
			continue
		}
		out.ContentLength = size
		out.Header = r.Header.Clone()
		for k := range hopByHop {
			out.Header.Del(k)
		}
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
			if hopByHop[http.CanonicalHeaderKey(k)] {
				continue
			}
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

// hopByHop headers belong to one connection and are never passed on.
var hopByHop = map[string]bool{
	"Connection": true, "Keep-Alive": true, "Proxy-Authenticate": true, "Proxy-Authorization": true,
	"Te": true, "Trailer": true, "Transfer-Encoding": true, "Upgrade": true, "Proxy-Connection": true,
}

// keep holds a request body to send it again: in memory when small, in a
// temporary file otherwise (uploads).
func keep(src io.Reader) (io.ReadSeeker, int64, func(), error) {
	head, err := io.ReadAll(io.LimitReader(src, memoryHandover+1))
	if err != nil {
		return nil, 0, nil, err
	}
	if len(head) <= memoryHandover {
		return bytes.NewReader(head), int64(len(head)), func() {}, nil
	}
	f, err := os.CreateTemp("", "eq-handover-*")
	if err != nil {
		return nil, 0, nil, err
	}
	done := func() { _ = f.Close(); _ = os.Remove(f.Name()) }
	n, err := io.Copy(f, io.MultiReader(bytes.NewReader(head), io.LimitReader(src, MaxHandover+1-int64(len(head)))))
	if err != nil || n > MaxHandover {
		done()
		return nil, 0, nil, errors.New("body too large to hand over")
	}
	return f, n, done, nil
}
