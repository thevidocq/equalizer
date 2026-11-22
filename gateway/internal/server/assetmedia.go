package server

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

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
// limit and the file must begin with a media container signature, whatever
// range is asked. A stream holds its legacy budget slot while it is read,
// within the site's media share and two streams per visitor.
func (p *Public) serveMediaAsset(w http.ResponseWriter, r *http.Request, st *site.Site, m router.Match, asset *mapmodel.Asset, file string) {
	ss := sessionOf(r)
	lreq := mapmodel.LegacyRequest{Method: http.MethodGet, Path: asset.Prefix + file}
	req, err := st.Legacy.Build(r.Context(), lreq, nil)
	if err != nil {
		p.block(w, r, st, http.StatusNotFound, "asset_name_rejected")
		return
	}
	asked := r.Header.Get("Range")
	if !mediaRange.MatchString(asked) {
		asked = ""
	}
	if asked != "" {
		req.Header.Set("Range", asked) // checked, then written again: never a list or junk
	}
	p.prepare(req, r, st, ss)
	// A stream holds a legacy worker as long as the player reads, so streams
	// are bounded twice: half the site's budget at most, two per visitor.
	ip := p.IPs.IP(r)
	if !p.media.take(st.Model.Meta.App, ip) {
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	defer p.media.give(st.Model.Meta.App, ip)
	if !st.Media.Acquire(r.Context()) {
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	defer st.Media.Release()
	// With a budget of one slot a stream would hold the whole site: it takes
	// none (the media share, one stream, bounds the legacy server's load).
	holds := st.Model.Defaults.LegacyBudget.MaxConcurrency != 1
	if holds && !p.acquireLegacy(r, st, m.Entry.ID) || !holds && !p.legacyRateAllowed(r, st, m.Entry.ID) {
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	release := func() {
		if holds {
			st.Budget.Release()
		}
	}
	resp, err := st.Legacy.Open(req)
	if err != nil {
		release()
		p.legacyFailure(w, r, st, m.Entry.ID, err)
		return
	}
	defer func() { resp.Close(); release() }()

	ct := resp.Header.Get("Content-Type")
	switch resp.Status {
	case http.StatusOK, http.StatusPartialContent:
	case http.StatusRequestedRangeNotSatisfiable:
		// Only for a file the map would serve: otherwise the 416 would tell
		// the size of any file under the prefix.
		if asked == "" || p.probeMedia(r, st, ss, lreq, asset) != "" {
			writePage(w, http.StatusNotFound, pageNotFound)
			return
		}
		if cr := resp.Header.Get("Content-Range"); totalSize.MatchString(cr) && strings.HasPrefix(cr, "bytes */") {
			w.Header().Set("Content-Range", cr)
		}
		securityHeaders(w.Header())
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		p.Events.Emit(p.event(r, st, "request.served", "", http.StatusRequestedRangeNotSatisfiable, telemetry.Ptr(m.Entry.ID)))
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
	head, reason := checkMedia(asset, ct, resp, limit, asked != "")
	if (reason == "" && head == nil) || reason == "probe" { // a range in the middle or too short: read the start apart
		reason = p.probeMedia(r, st, ss, lreq, asset)
	}
	if reason != "" {
		p.refuseAsset(w, r, st, asset, reason, ct)
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
		switch copyMedia(w, io.MultiReader(bytes.NewReader(head), resp.Body), limit) {
		case mediaTooLong:
			p.Events.Emit(p.event(r, st, "request.blocked", "asset_too_large", resp.Status, telemetry.Ptr(m.Entry.ID)))
			p.abortStream(r)
			return
		case mediaCut:
			p.Events.Emit(p.event(r, st, "legacy.error", "stream_cut", http.StatusBadGateway, telemetry.Ptr(m.Entry.ID)))
			p.abortStream(r)
			return
		}
	}
	p.Events.Emit(p.event(r, st, "request.served", "", resp.Status, telemetry.Ptr(m.Entry.ID)))
}

// mediaWriteTimeout drops a client that has not taken a chunk for this long.
const mediaWriteTimeout = 60 * time.Second

// How a media body went.
const (
	mediaSent    = iota // whole, or the client went away
	mediaTooLong        // longer than the limit, which the legacy server had not announced
	mediaCut            // the legacy server stopped before the end
)

// copyMedia sends a media body. A stalled client is dropped; a body longer
// than limit, or one the legacy server cut, is reported so the caller fails
// the response: never a short file taken as whole.
func copyMedia(w http.ResponseWriter, body io.Reader, limit int64) int {
	rc := http.NewResponseController(w)
	buf := make([]byte, 32<<10)
	var sent int64
	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			sent += int64(n)
			if sent > limit {
				return mediaTooLong
			}
			_ = rc.SetWriteDeadline(time.Now().Add(mediaWriteTimeout))
			if _, err := w.Write(buf[:n]); err != nil {
				return mediaSent
			}
		}
		switch {
		case rerr == io.EOF:
			return mediaSent
		case rerr != nil:
			return mediaCut
		}
	}
}

// abortStream fails a response already started: the client sees a broken
// transfer. A preview keeps its verdict from the event instead.
func (p *Public) abortStream(r *http.Request) {
	if _, try := r.Context().Value(tryCaptureKey{}).(*tryCapture); try {
		return
	}
	panic(http.ErrAbortHandler)
}

// mediaTypeAllowed reports whether a declared type is an audio or video type
// of the prefix.
func mediaTypeAllowed(asset *mapmodel.Asset, contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediasig.Type(mediaType) && slices.Contains(asset.Types, mediaType)
}

// checkMedia checks a streamed media response and returns the bytes it read
// to do so, or the rejection reason. A 206 that does not start at the first
// byte returns neither: its signature must be read apart (probeMedia).
func checkMedia(asset *mapmodel.Asset, contentType string, resp *legacyclient.Stream, limit int64, ranged bool) ([]byte, string) {
	if !mediaTypeAllowed(asset, contentType) {
		return nil, "asset_type_not_allowed"
	}
	if size := fullSize(resp); size > limit {
		return nil, "asset_too_large"
	}
	cr := resp.Header.Get("Content-Range")
	if resp.Status == http.StatusPartialContent {
		if !ranged || !strings.HasPrefix(cr, "bytes ") {
			return nil, "asset_content_mismatch" // a range nobody asked for, or one it does not describe
		}
		if !strings.HasPrefix(cr, "bytes 0-") {
			return nil, ""
		}
	}
	head := make([]byte, mediasig.HeadSize)
	n, _ := io.ReadFull(resp.Body, head)
	head = head[:n]
	if n < mediasig.HeadSize && resp.Status == http.StatusPartialContent {
		return head, "probe" // a first range shorter than the signature (bytes=0-1): read it apart
	}
	if !mediasig.Match(head) {
		return nil, "asset_content_mismatch"
	}
	return head, ""
}

// probeMedia reads the type and the first bytes of a media file with a
// request of its own: for a range that starts in the middle, and before a
// 416, so that the range a client chooses never skips the checks nor tells
// the size of a file the map would not serve.
func (p *Public) probeMedia(r *http.Request, st *site.Site, ss *sessionState, lreq mapmodel.LegacyRequest, asset *mapmodel.Asset) string {
	req, err := st.Legacy.Build(r.Context(), lreq, nil)
	if err != nil {
		return "asset_content_mismatch"
	}
	req.Header.Set("Range", "bytes=0-"+strconv.Itoa(mediasig.HeadSize-1))
	p.prepare(req, r, st, ss)
	resp, err := st.Legacy.Open(req)
	if err != nil {
		return "asset_content_mismatch"
	}
	defer resp.Close()
	if resp.Status != http.StatusOK && resp.Status != http.StatusPartialContent {
		return "asset_content_mismatch"
	}
	if !mediaTypeAllowed(asset, resp.Header.Get("Content-Type")) {
		return "asset_type_not_allowed"
	}
	head := make([]byte, mediasig.HeadSize)
	n, _ := io.ReadFull(resp.Body, head)
	if !mediasig.Match(head[:n]) {
		return "asset_content_mismatch"
	}
	return ""
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
