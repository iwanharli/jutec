package crawler

import (
	"net/url"
	"regexp"
	"strings"
)

// reIDTerm: istilah judol khas pasar Indonesia. Situs judi tanpa satu pun istilah ini, tanpa nomor/rekening
// Indonesia, dan bukan domain .id adalah judi internasional (ulasan kasino, operator legal luar negeri,
// provider game) — tetap judi, tapi bukan target penindakan judol Indonesia.
// Harus sama dengan pola di migrasi 000042 (site_market).
var reIDTerm = regexp.MustCompile(`(?i)togel|gacor|judi|judol|bandar|pulsa|deposit dana|rungkad|maxwin|hoki|situs|agen|daftar|bocoran|paito|syair|prediksi|keluaran|alternatif|new member|taruhan|slot88|live draw|mix parlay`)

// reIDHost: nama domain bergaya judol Indonesia (halamannya kadang tak terbaca/kosong).
var reIDHost = regexp.MustCompile(`(?i)togel|toto|gacor|situs|slot88|maxwin|judi|bandar|hoki|4d`)

// Market: "id" (menyasar Indonesia) atau "intl".
func Market(r *AnalysisResult, rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		h := strings.ToLower(u.Hostname())
		if strings.HasSuffix(h, ".id") || reIDHost.MatchString(h) {
			return "id"
		}
	}
	for _, k := range r.FoundKeywords {
		if reIDTerm.MatchString(k) {
			return "id"
		}
	}
	for _, a := range r.Artifacts {
		switch a.Type {
		case "Bank", "E-Wallet", "QRIS", "VirtualAccount":
			return "id"
		case "Contact", "Phone":
			v := strings.TrimPrefix(a.Value, "+")
			if strings.HasPrefix(v, "628") || strings.HasPrefix(v, "08") {
				return "id"
			}
		}
	}
	return "intl"
}
