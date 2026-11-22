package server

import (
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/fontsig"
	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/origin"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// File names allowed for assets: no placeholders, no leftover %.
var assetFileRe = regexp.MustCompile(`^[A-Za-z0-9._\-/ @+~,()!]{1,512}$`)

// Default maximum asset size (10 MiB).
const defaultAssetMaxKB = 10 << 10

// serveAsset serves a legacy static file after checking its name, declared
// type and, for raster images, actual type.
func (p *Public) serveAsset(w http.ResponseWriter, r *http.Request, st *site.Site, m router.Match) {
	asset, ok := st.AssetByPrefix(m.Entry.Pattern)
	if !ok {
		writePage(w, http.StatusInternalServerError, pageError)
		return
	}
	ss := sessionOf(r)
	if len(asset.Roles) > 0 && !site.RoleAllowed(ss.role(st), asset.Roles) {
		p.block(w, r, st, http.StatusForbidden, "authz_denied")
		return
	}
	file := m.Params["file"]
	if !validAssetFile(file) {
		p.block(w, r, st, http.StatusNotFound, "asset_name_rejected")
		return
	}
	if streamsMedia(asset, file) {
		p.serveMediaAsset(w, r, st, m, asset, file)
		return
	}

	// The client query never reaches the legacy application: an asset is just its path.
	req, err := st.Legacy.Build(r.Context(), mapmodel.LegacyRequest{Method: http.MethodGet, Path: asset.Prefix + file}, nil)
	if err != nil {
		p.block(w, r, st, http.StatusNotFound, "asset_name_rejected")
		return
	}
	p.prepare(req, r, st, ss) // cookies set by assets are ignored
	if !p.acquireLegacy(r, st, m.Entry.ID) {
		writePage(w, http.StatusServiceUnavailable, pageUnavailable)
		return
	}
	resp, err := st.Legacy.Do(req)
	st.Budget.Release()
	if err != nil {
		p.legacyFailure(w, r, st, m.Entry.ID, err)
		return
	}
	if resp.Status != http.StatusOK {
		writePage(w, http.StatusNotFound, pageNotFound)
		return
	}

	maxKB := asset.MaxSizeKB
	if maxKB <= 0 {
		maxKB = defaultAssetMaxKB
	}
	ct := resp.Header.Get("Content-Type")
	served, reason := checkAsset(asset, ct, resp, maxKB)
	if reason != "" {
		p.refuseAsset(w, r, st, asset, reason, ct)
		return
	}

	h := w.Header()
	for k, vs := range resp.Header {
		if k == "Content-Disposition" {
			continue
		}
		for _, v := range vs {
			h.Add(k, v)
		}
	}
	if served != "" {
		h.Set("Content-Type", served) // a font sent with a generic or old type
	}
	securityHeaders(h)
	// The media type as checkAsset read it: "Text/HTML" is HTML too.
	if mt, _, _ := mime.ParseMediaType(ct); mt == "image/svg+xml" || mt == "text/html" {
		// An SVG, or an HTML fragment a page's scripts load as a template
		// (Magento's /static/…/*.html), opened directly must not be able to
		// run scripts.
		h.Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'; img-src data:")
	}
	body := resp.Body
	if st.Origin != nil && origin.Text(ct) {
		body = st.Origin.Bytes(body)
		h.Del("Content-Length")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
	p.Events.Emit(p.event(r, st, "request.served", "", http.StatusOK, telemetry.Ptr(m.Entry.ID)))
}

// refuseAsset refuses a legacy file. A type or a size the map does not allow
// yet is also reported to discovery, which studies the file and proposes it;
// a file whose content contradicts its type is not.
func (p *Public) refuseAsset(w http.ResponseWriter, r *http.Request, st *site.Site, asset *mapmodel.Asset, reason, ct string) {
	p.Log.WarnContext(r.Context(), "asset rejected", slog.Group("detail",
		slog.String("prefix", asset.Prefix), slog.String("reason", reason), slog.String("content_type", ct)))
	if reason == "asset_type_not_allowed" || reason == "asset_too_large" {
		p.Unmapped.Record(st.Model.Meta.App, http.MethodGet, r.URL.Path, p.IPs.IP(r))
	}
	p.block(w, r, st, http.StatusNotFound, reason)
}

func validAssetFile(file string) bool {
	if !assetFileRe.MatchString(file) || strings.HasPrefix(file, "/") || strings.Contains(file, "//") {
		return false
	}
	for _, seg := range strings.Split(file, "/") {
		if seg == "" || dotSegment(seg) { // also ".. " and "...", which Windows reads as ".."
			return false
		}
	}
	return true
}

// checkAsset returns the rejection reason, or "" if the asset is fine. When
// the asset is a font declared with a generic or old media type, served is
// the canonical type to send instead.
func checkAsset(asset *mapmodel.Asset, contentType string, resp *legacyclient.Response, maxKB int) (served, reason string) {
	if len(resp.Body) > maxKB<<10 {
		return "", "asset_too_large"
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !slices.Contains(asset.Types, mediaType) {
		if font := fontsig.Resolve(strings.ToLower(mediaType), resp.Body, asset.Types); err == nil && font != "" {
			return font, ""
		}
		return "", "asset_type_not_allowed"
	}
	if strings.HasPrefix(mediaType, "image/") && mediaType != "image/svg+xml" && mediaType != "image/x-icon" &&
		mediaType != "image/vnd.microsoft.icon" {
		if !strings.HasPrefix(http.DetectContentType(resp.Body), "image/") {
			return "", "asset_content_mismatch"
		}
	}
	return "", ""
}
