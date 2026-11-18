package server

import (
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
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

	// The client query never reaches the legacy application: an asset is just its path.
	req, err := st.Legacy.Build(r.Context(), mapmodel.LegacyRequest{Method: http.MethodGet, Path: asset.Prefix + file}, nil)
	if err != nil {
		p.block(w, r, st, http.StatusNotFound, "asset_name_rejected")
		return
	}
	ss.apply(req) // cookies set by assets are ignored
	if !st.Budget.Acquire(r.Context()) {
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
	if reason := checkAsset(asset, ct, resp, maxKB); reason != "" {
		p.Log.WarnContext(r.Context(), "asset rejected", slog.Group("detail",
			slog.String("prefix", asset.Prefix), slog.String("reason", reason), slog.String("content_type", ct)))
		p.block(w, r, st, http.StatusNotFound, reason)
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
	securityHeaders(h)
	if strings.HasPrefix(ct, "image/svg") {
		// An SVG opened directly must not be able to run scripts.
		h.Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'; img-src data:")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(resp.Body)
	}
	p.Events.Emit(p.event(r, st, "request.served", "", http.StatusOK, telemetry.Ptr(m.Entry.ID)))
}

func validAssetFile(file string) bool {
	if !assetFileRe.MatchString(file) || strings.HasPrefix(file, "/") || strings.Contains(file, "//") {
		return false
	}
	for _, seg := range strings.Split(file, "/") {
		if seg == "." || seg == ".." || seg == "" {
			return false
		}
	}
	return true
}

// checkAsset returns the rejection reason, or "" if the asset is fine.
func checkAsset(asset *mapmodel.Asset, contentType string, resp *legacyclient.Response, maxKB int) string {
	if len(resp.Body) > maxKB<<10 {
		return "asset_too_large"
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !slices.Contains(asset.Types, mediaType) {
		return "asset_type_not_allowed"
	}
	if strings.HasPrefix(mediaType, "image/") && mediaType != "image/svg+xml" && mediaType != "image/x-icon" &&
		mediaType != "image/vnd.microsoft.icon" {
		if !strings.HasPrefix(http.DetectContentType(resp.Body), "image/") {
			return "asset_content_mismatch"
		}
	}
	return ""
}
