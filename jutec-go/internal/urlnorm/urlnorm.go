// Package urlnorm menentukan bentuk kanonik URL situs, agar satu situs tidak tersimpan dua kali
// karena beda penulisan ("https://Situs.com/", "https://situs.com:443", "https://situs.com#x").
package urlnorm

import (
	"errors"
	"net/url"
	"strings"
)

var ErrInvalid = errors.New("invalid url: harus http(s) dengan hostname valid")

// Canonical: scheme & host huruf kecil, tanpa port bawaan, tanpa fragmen, tanpa "/" di akhir path.
// Scheme kosong dianggap https. Path & query dipertahankan (bisa beda halaman).
func Canonical(raw string) (string, error) {
	// "\" sisa escape JSON/JS (mis. "https://situs.com\") bukan bagian URL
	raw = strings.TrimRight(strings.TrimSpace(raw), `\`)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw, ErrInvalid
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if (u.Scheme != "http" && u.Scheme != "https") || !strings.Contains(u.Hostname(), ".") {
		return raw, ErrInvalid
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if p := u.Port(); p != "" && !(u.Scheme == "http" && p == "80") && !(u.Scheme == "https" && p == "443") {
		host += ":" + p
	}
	u.Host = host
	u.Fragment, u.RawFragment = "", ""
	u.User = nil
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u.String(), nil
}

// Key: kunci pembanding di database — harus sama persis dengan indeks unik
// sites_url_canonical_idx: lower(rtrim(url, '/')).
func Key(u string) string { return strings.ToLower(strings.TrimRight(u, "/")) }
