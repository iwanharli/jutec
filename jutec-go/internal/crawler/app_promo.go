package crawler

import (
	"regexp"
	"strings"
)

// AppPromo menandai situs yang MEMPROMOSIKAN aplikasi meski link unduhnya tidak tersedia
// (mis. tombol "Unduh" ber-href="#" di template white-label, APK hanya dibagikan setelah login / via CS).
type AppPromo struct {
	Detected  bool     `json:"detected"`
	Platforms []string `json:"platforms,omitempty"` // android, ios
	Evidence  []string `json:"evidence,omitempty"`  // potongan penanda yang ditemukan
}

// appPromoMarkers: penanda promosi aplikasi di HTML. Label dipakai sebagai bukti yang mudah dibaca.
var appPromoMarkers = []struct {
	label    string
	platform string
	re       *regexp.Regexp
}{
	{"teks 'Download/Unduh APK'", "android", regexp.MustCompile(`(?i)(download|unduh)\s+(android\s+)?apk`)},
	{"teks 'Unduh/Download ... App'", "", regexp.MustCompile(`(?i)(download|unduh)\s*(<[^>]+>\s*)*[a-z0-9]{2,20}\s*(app|aplikasi)\b`)},
	{"bagian download-apk", "android", regexp.MustCompile(`(?i)class="[^"]*download[-_]apk`)},
	{"panduan instalasi APK", "android", regexp.MustCompile(`(?i)(apk[-_]install[-_]guide|panduan\s+instal(asi)?)`)},
	{"notifikasi download APK", "android", regexp.MustCompile(`(?i)download[-_]apk[-_]notification`)},
	{"logo Android", "android", regexp.MustCompile(`(?i)android[-_]logo|alt="android"`)},
	{"logo iOS / App Store", "ios", regexp.MustCompile(`(?i)(ios[-_]logo|app[-_]?store[-_]badge|alt="(ios|iphone)")`)},
}

// DetectAppPromo mencari penanda promosi aplikasi. Butuh ≥2 penanda berbeda agar kata "app"
// yang kebetulan muncul (mis. di nama file JS) tidak dianggap promosi.
func DetectAppPromo(html string) *AppPromo {
	p := &AppPromo{}
	platforms := map[string]bool{}
	for _, m := range appPromoMarkers {
		if m.re.MatchString(html) {
			p.Evidence = append(p.Evidence, m.label)
			if m.platform != "" {
				platforms[m.platform] = true
			}
		}
	}
	p.Detected = len(p.Evidence) >= 2
	if !p.Detected {
		return &AppPromo{}
	}
	for pl := range platforms {
		p.Platforms = append(p.Platforms, pl)
	}
	return p
}

// mergeAppPromo menggabungkan hasil deteksi desktop & mobile.
func mergeAppPromo(a, b *AppPromo) *AppPromo {
	if a == nil || !a.Detected {
		return b
	}
	if b == nil || !b.Detected {
		return a
	}
	seen := map[string]bool{}
	out := &AppPromo{Detected: true}
	for _, e := range append(a.Evidence, b.Evidence...) {
		if !seen["e"+e] {
			seen["e"+e] = true
			out.Evidence = append(out.Evidence, e)
		}
	}
	for _, pl := range append(a.Platforms, b.Platforms...) {
		if !seen["p"+pl] {
			seen["p"+pl] = true
			out.Platforms = append(out.Platforms, pl)
		}
	}
	return out
}

// mergeUnique menambahkan item b ke a tanpa duplikat (urutan a dipertahankan).
func mergeUnique(a, b []string) []string {
	seen := make(map[string]bool, len(a))
	for _, s := range a {
		seen[strings.TrimSpace(s)] = true
	}
	for _, s := range b {
		if k := strings.TrimSpace(s); k != "" && !seen[k] {
			seen[k] = true
			a = append(a, s)
		}
	}
	return a
}
