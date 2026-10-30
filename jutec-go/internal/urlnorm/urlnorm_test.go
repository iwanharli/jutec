package urlnorm

import "testing"

func TestCanonical(t *testing.T) {
	same := []string{
		"https://tripslot.everolyjewelry.site", "https://tripslot.everolyjewelry.site/", "HTTPS://TripSlot.EverolyJewelry.Site/",
		"https://tripslot.everolyjewelry.site:443/", "https://tripslot.everolyjewelry.site/#daftar", "tripslot.everolyjewelry.site", "https://tripslot.everolyjewelry.site\\",
	}
	for _, s := range same {
		if got, _ := Canonical(s); got != "https://tripslot.everolyjewelry.site" {
			t.Errorf("Canonical(%q) = %q", s, got)
		}
	}
	if got, _ := Canonical("https://indo303a.site/versi-mobile/"); got != "https://indo303a.site/versi-mobile" {
		t.Errorf("path: %q", got)
	}
	if got, _ := Canonical("http://situs.com:8080/"); got != "http://situs.com:8080" {
		t.Errorf("port non-bawaan harus dipertahankan: %q", got)
	}
	if got, _ := Canonical("https://situs.com/?ref=abc"); got != "https://situs.com?ref=abc" {
		t.Errorf("query: %q", got)
	}
	for _, bad := range []string{"notaurl", "ftp://situs.com", ""} {
		if _, err := Canonical(bad); err == nil {
			t.Errorf("%q seharusnya ditolak", bad)
		}
	}
}

func TestAppURL(t *testing.T) {
	cases := map[string]string{
		`https://apps.apple.com/us/app/bitcoin-com-wallet-buy-sell/id1252903728\`:        "https://apps.apple.com/app/id1252903728",
		`https://apps.apple.com/app/apple-store/id1252903728?pt=118718303&ct=x`:          "https://apps.apple.com/app/id1252903728",
		`https://play.google.com/store/apps/details?id=com.bitcoin.mwallet&referrer=abc`: "https://play.google.com/store/apps/details?id=com.bitcoin.mwallet",
		`https://play.google.com/store/apps/details?hl=id&amp;id=com.bitcoin.mwallet`:    "https://play.google.com/store/apps/details?id=com.bitcoin.mwallet",
		`https://vpn-app.s3.ap-southeast-3.amazonaws.com/vpn.apk`:                        "https://vpn-app.s3.ap-southeast-3.amazonaws.com/vpn.apk",
	}
	for in, want := range cases {
		if got := AppURL(in); got != want {
			t.Errorf("AppURL(%q) = %q, mau %q", in, got, want)
		}
	}
}
