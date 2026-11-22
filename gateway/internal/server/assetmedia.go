package server

import (
	"bytes"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/mediasig"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// Default maximum size of a streamed audio or video asset (512 MiB).
const defaultMediaMaxKB = 512 << 10

// mediaRange is the only Range a media asset passes on: one range, as
// players ask for it ("bytes=0-", "bytes=1048576-", "bytes=-500").
var mediaRange = regexp.MustCompile(`^bytes=(\d{1,15}-\d{0,15}|-\d{1,15})$`)

// mediaHeaders are the legacy headers a streamed media asset keeps.
var mediaHeaders = []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges",
	"ETag", "Last-Modified", "Cache-Control", "Expires"}

// totalSize matches the full size in a Content-Range ("bytes 0-99/12345").
var totalSize = regexp.MustCompile(`/(\d{1,15})$`)

// streamsMedia reports whether an asset request is read as streamed media:
// the file looks like audio or video and the prefix allows such a type.
func streamsMedia(asset *mapmodel.Asset, file string) bool {
	return mediasig.File(file) && slices.ContainsFunc(asset.Types, mediasig.Type)
}

// serveMediaAsset streams an audio or video asset, with byte ranges so that
// players can seek. The declared type must be allowed, the size within the
// limit and, when the response starts at the first byte, the signature that
// of a media container.
func (p *Public) serveMediaAsset(w http.ResponseWriter, r *http.Request, st *site.Site, m router.Match, asset *mapmodel.Asset, file string) {
	ss := sessionOf(r)
	req, err := st.Legacy.Build(r.Context(), mapmodel.LegacyRequest{Method: http.MethodGet, Path: asset.Prefix + file}, nil)
	if err != nil {
		p.block(w, r, st, http.StatusNotFound, "asset_name_rejected")
		return
	}
	if v := r.Header.Get("Range"); mediaRange.MatchString(v) {
		req.Header.Set("Range", v) // checked, then written again: never a list or junk
	}
	p.prepare(req, r, st, ss)
	if !p.acquireLegacy(r, st, m.Entry.ID) {
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	resp, err := st.Legacy.Open(req)
	if err != nil {
		st.Budget.Release()
		p.legacyFailure(w, r, st, m.Entry.ID, err)
		return
	}
	defer func() { resp.Close(); st.Budget.Release() }()

	switch resp.Status {
	case http.StatusOK, http.StatusPartialContent:
	case http.StatusRequestedRangeNotSatisfiable:
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			w.Header().Set("Content-Range", cr)
		}
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	default:
		writePage(w, http.StatusNotFound, pageNotFound)
		return
	}

	maxKB := asset.MaxSizeKB
	if maxKB <= 0 {
		maxKB = defaultMediaMaxKB
	}
	limit := int64(maxKB) << 10
	ct := resp.Header.Get("Content-Type")
	head, reason := checkMedia(asset, ct, resp, limit)
	if reason != "" {
		p.Log.WarnContext(r.Context(), "asset rejected", slog.Group("detail",
			slog.String("prefix", asset.Prefix), slog.String("reason", reason), slog.String("content_type", ct)))
		p.block(w, r, st, http.StatusNotFound, reason)
		return
	}

	h := w.Header()
	for _, k := range mediaHeaders {
		if v := resp.Header.Get(k); v != "" {
			h.Set(k, v)
		}
	}
	securityHeaders(h)
	w.WriteHeader(resp.Status)
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, io.MultiReader(bytes.NewReader(head), io.LimitReader(resp.Body, limit-int64(len(head)))))
	}
	p.Events.Emit(p.event(r, st, "request.served", "", resp.Status, telemetry.Ptr(m.Entry.ID)))
}

// checkMedia checks a streamed media response and returns the bytes it read
// to do so, or the rejection reason.
func checkMedia(asset *mapmodel.Asset, contentType string, resp *legacyclient.Stream, limit int64) ([]byte, string) {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !mediasig.Type(mediaType) || !slices.Contains(asset.Types, mediaType) {
		return nil, "asset_type_not_allowed"
	}
	if size := fullSize(resp); size > limit {
		return nil, "asset_too_large"
	}
	// A range in the middle of the file has no signature to read: its type is
	// still checked, and the browser never runs it (nosniff).
	if resp.Status == http.StatusPartialContent && !strings.HasPrefix(resp.Header.Get("Content-Range"), "bytes 0-") {
		return nil, ""
	}
	head := make([]byte, mediasig.HeadSize)
	n, _ := io.ReadFull(resp.Body, head)
	head = head[:n]
	if !mediasig.Match(head) {
		return nil, "asset_content_mismatch"
	}
	return head, ""
}

// fullSize is the size of the whole file, or 0 if the legacy server does not say.
func fullSize(resp *legacyclient.Stream) int64 {
	if m := totalSize.FindStringSubmatch(resp.Header.Get("Content-Range")); m != nil {
		n, _ := strconv.ParseInt(m[1], 10, 64)
		return n
	}
	if resp.Status == http.StatusOK {
		n, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
		return n
	}
	return 0
}
