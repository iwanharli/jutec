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
