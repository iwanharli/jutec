package crawler

import "testing"

func TestResolveURLNonWebSchemes(t *testing.T) {
	base := "https://situs.com/"
	for _, l := range []string{"skype:live:.cid.622ff1ce6185033b?chat", "mailto:a@b.com", "tel:+62812", "whatsapp://send?phone=62812", "intent://x#Intent;end"} {
		if got := resolveURL(l, base); got != "" {
			t.Errorf("%q harus diabaikan, dapat %q", l, got)
		}
	}
	for l, want := range map[string]string{"contoh.com:8080/a": "https://contoh.com:8080/a", "contoh.com/daftar": "https://contoh.com/daftar", "https://x.com/": "https://x.com"} {
		if got := resolveURL(l, base); got != want {
			t.Errorf("%q → %q, mau %q", l, got, want)
		}
	}
}
