package useragent

import "testing"

func TestCanonical(t *testing.T) {
	for in, want := range map[string]string{
		"": "",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1": SafariIPhone,
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Mobile Safari/537.36":                       ChromeAndroid,
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36 Edg/130.0":                   EdgeWindows,
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15":                      SafariMac,
		"sqlmap/1.8' OR 1=1 -- () { :; }; /bin/bash -c id":                                                                                        ChromeWindows,
	} {
		if got := Canonical(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}
