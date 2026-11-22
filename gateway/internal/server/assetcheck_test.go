package server

import (
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/legacyclient"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

func TestCheckAsset(t *testing.T) {
	asset := &mapmodel.Asset{Prefix: "/a/", Types: []string{"font/woff", "image/avif", "image/png"}}
	cases := []struct {
		name, ct, body, served, reason string
	}{
		{"font with no type", "", "wOFF\x00\x01\x00\x00", "font/woff", ""},
		{"font with an unreadable type", "font/;;", "wOFF\x00\x01\x00\x00", "font/woff", ""},
		{"no type, not a font", "", "<script>", "", "asset_type_not_allowed"},
		{"avif", "image/avif", "\x00\x00\x00\x1cftypavif\x00\x00\x00\x00", "", ""},
		{"avif that is a script", "image/avif", "<script>alert(1)</script>", "", "asset_content_mismatch"},
		{"png that is html", "image/png", "<html>", "", "asset_content_mismatch"},
		{"declared font that is html", "font/woff", "<html><script>", "", "asset_content_mismatch"},
		{"declared font, signature of another font", "font/woff", "wOF2\x00\x01", "", "asset_content_mismatch"},
		{"declared font", "font/woff", "wOFF\x00\x01", "", ""},
	}
	for _, c := range cases {
		served, reason := checkAsset(asset, c.ct, &legacyclient.Response{Body: []byte(c.body)}, 100)
		if served != c.served || reason != c.reason {
			t.Errorf("%s: (%q, %q), want (%q, %q)", c.name, served, reason, c.served, c.reason)
		}
	}
}
