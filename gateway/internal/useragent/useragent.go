// Package useragent rebuilds the browser's User-Agent for the legacy
// application. It never passes the string the client wrote (a classic way
// to reach SQL and log injections in old apps): it recognises the kind of
// device and browser and sends a fixed, ordinary string for it, so the
// legacy site still picks its mobile or desktop version.
package useragent

import "strings"

// Canonical strings, one per kind of browser.
const (
	ChromeWindows = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	EdgeWindows   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0"
	FirefoxDesk   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0"
	SafariMac     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15"
	ChromeMac     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	ChromeLinux   = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	ChromeAndroid = "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36"
	AndroidTablet = "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	SafariIPhone  = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1"
	SafariIPad    = "Mozilla/5.0 (iPad; CPU OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1"
	IE11          = "Mozilla/5.0 (Windows NT 10.0; WOW64; Trident/7.0; rv:11.0) like Gecko"
)

// Canonical returns the fixed string for the browser described by ua, or
// "" when ua is empty (the gateway's own User-Agent is used then).
func Canonical(ua string) string {
	if ua == "" {
		return ""
	}
	u := strings.ToLower(ua)
	has := func(s string) bool { return strings.Contains(u, s) }
	switch {
	case has("ipad"):
		return SafariIPad
	case has("iphone") || has("ipod"):
		return SafariIPhone
	case has("android") && has("mobile"):
		return ChromeAndroid
	case has("android"):
		return AndroidTablet
	case has("trident/") || has("msie "):
		return IE11
	case has("edg/"):
		return EdgeWindows
	case has("firefox/"):
		return FirefoxDesk
	case has("macintosh") && has("safari/") && !has("chrome/"):
		return SafariMac
	case has("macintosh"):
		return ChromeMac
	case has("linux") || has("x11"):
		return ChromeLinux
	}
	return ChromeWindows
}
