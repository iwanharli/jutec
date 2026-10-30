package service

import "testing"

func TestIsBlacklistedSuffix(t *testing.T) {
	bl := []string{"dw.com", "tempo.co", "googleads.g.doubleclick.net"}
	cases := map[string]bool{
		"https://www.dw.com/id":                 true,
		"dw.com":                                true,
		"https://kingdw.com":                    false, // dulu ikut terblokir (substring)
		"https://otomotif.tempo.co/read/1":      true,
		"https://tempo.com":                     false,
		"https://slot.tempo.co.xyz":             false,
		"googleads.g.doubleclick.net:443":       true,
		"https://ads.googleads.g.doubleclick.net": true,
	}
	for in, want := range cases {
		if got := isBlacklisted(in, bl); got != want {
			t.Errorf("isBlacklisted(%q) = %v, mau %v", in, got, want)
		}
	}
}
