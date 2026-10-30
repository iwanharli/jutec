package crawler

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Identifier pengelola: kunci layanan pihak ketiga yang terdaftar atas AKUN pemilik situs
// (Google, Firebase, Telegram, Cloudflare, …). Satu kunci di banyak situs = satu akun pengelola,
// dan pemilik akunnya bisa dimintakan resmi ke penyedia layanan. Diambil pasif dari HTML & JS situs.
//
// Token rahasia (mis. token bot Telegram) TIDAK disimpan utuh — hanya bagian identitasnya (ID bot).
var ownerPatterns = []struct {
	Platform string
	Context  string // bila diisi, pola hanya dipakai jika teks memuat kata ini (cegah salah tangkap)
	Patterns []*regexp.Regexp
}{
	{Platform: "ReCaptcha", Patterns: []*regexp.Regexp{
		// Kunci bisa berakhiran "-"/"_" sehingga \b tidak dipakai; batasnya karakter non-kunci atau akhir teks
		regexp.MustCompile(`(?:data-sitekey|sitekey|render)["'\s:=]+["']?(6L[0-9A-Za-z_-]{38})(?:[^0-9A-Za-z_-]|$)`),
		regexp.MustCompile(`recaptcha/(?:api|enterprise)\.js\?(?:[^"'\s]*&)?render=(6L[0-9A-Za-z_-]{38})(?:[^0-9A-Za-z_-]|$)`),
	}},
	{Platform: "GoogleAPIKey", Patterns: []*regexp.Regexp{regexp.MustCompile(`\b(AIza[0-9A-Za-z_-]{35})(?:[^0-9A-Za-z_-]|$)`)}},
	{Platform: "FirebaseProject", Patterns: []*regexp.Regexp{
		regexp.MustCompile(`\b([a-z0-9][a-z0-9-]{3,40})\.firebaseapp\.com`),
		regexp.MustCompile(`\b([a-z0-9][a-z0-9-]{3,40})(?:-default-rtdb)?\.firebaseio\.com`),
		regexp.MustCompile(`projectId["']?\s*:\s*["']([a-z0-9][a-z0-9-]{3,40})["']`),
	}, Context: "firebase"},
	{Platform: "AdSense", Patterns: []*regexp.Regexp{regexp.MustCompile(`\bca-(pub-\d{16})\b`), regexp.MustCompile(`\b(pub-\d{16})\b`)}},
	// Token bot: <id>:<rahasia> → simpan ID bot saja
	{Platform: "TelegramBot", Patterns: []*regexp.Regexp{
		regexp.MustCompile(`\b(\d{8,10}):AA[0-9A-Za-z_-]{33}(?:[^0-9A-Za-z_-]|$)`),
		regexp.MustCompile(`api\.telegram\.org/bot(\d{8,10}):`),
	}},
	{Platform: "TelegramChat", Context: "api.telegram.org", Patterns: []*regexp.Regexp{
		regexp.MustCompile(`chat_id["']?\s*[:=]\s*["']?(-?\d{6,16})\b`),
	}},
	{Platform: "OneSignal", Context: "onesignal", Patterns: []*regexp.Regexp{
		regexp.MustCompile(`appId["']?\s*:\s*["']([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})["']`),
	}},
	{Platform: "Sentry", Patterns: []*regexp.Regexp{regexp.MustCompile(`https://([a-f0-9]{32})@[a-z0-9.]*sentry\.io/\d+`)}},
	{Platform: "CloudflareAnalytics", Context: "beacon.min.js", Patterns: []*regexp.Regexp{
		regexp.MustCompile(`["']token["']\s*:\s*["']([a-f0-9]{32})["']`),
	}},
	{Platform: "Clarity", Patterns: []*regexp.Regexp{regexp.MustCompile(`clarity\.ms/tag/([a-z0-9]{8,12})`)}},
	{Platform: "YandexMetrica", Patterns: []*regexp.Regexp{regexp.MustCompile(`\bym\(\s*(\d{6,10})\s*,\s*["']init`)}},
	{Platform: "TikTokPixel", Patterns: []*regexp.Regexp{regexp.MustCompile(`ttq\.load\(\s*["']([A-Z0-9]{18,24})["']`)}},
	{Platform: "Jivo", Patterns: []*regexp.Regexp{regexp.MustCompile(`code\.jivo(?:site\.com|\.ru)/widget/([A-Za-z0-9]{8,14})`)}},
	{Platform: "Tidio", Patterns: []*regexp.Regexp{regexp.MustCompile(`code\.tidio\.co/([a-z0-9]{32})\.js`)}},
	{Platform: "Zendesk", Patterns: []*regexp.Regexp{regexp.MustCompile(`zdassets\.com/ekr/snippet\.js\?key=([a-f0-9-]{36})`)}},
	{Platform: "Smartsupp", Patterns: []*regexp.Regexp{regexp.MustCompile(`_smartsupp\.key\s*=\s*["']([a-f0-9]{40})["']`)}},
}

// Kunci contoh di dokumentasi/template yang sering ikut tersalin — bukan milik operator
var ownerPlaceholders = map[string]bool{
	"6LeIxAcTAAAAAJcZVRqyHh4X1y_-Uq-3n_Nhnwm7": true, // kunci uji resmi reCAPTCHA
	"pub-0000000000000000":                     true,
	"your-project-id":                          true,
	"my-project":                               true,
}

// ExtractOwnerIDs mengambil identifier pengelola dari teks HTML/JS.
func ExtractOwnerIDs(text string) []TrackingID {
	lower := strings.ToLower(text)
	seen := map[string]bool{}
	var out []TrackingID
	for _, e := range ownerPatterns {
		if e.Context != "" && !strings.Contains(lower, e.Context) {
			continue
		}
		for _, re := range e.Patterns {
			for _, m := range re.FindAllStringSubmatch(text, -1) {
				v := strings.TrimSpace(m[1])
				if v == "" || ownerPlaceholders[v] || ownerPlaceholders[strings.ToLower(v)] {
					continue
				}
				if k := e.Platform + ":" + v; !seen[k] {
					seen[k] = true
					out = append(out, TrackingID{Platform: e.Platform, Value: v})
				}
			}
		}
	}
	return out
}

var reScriptSrc = regexp.MustCompile(`(?i)<script[^>]+src=["']([^"']+)["']`)

// Pustaka umum — isinya sama di jutaan situs, tidak perlu dipindai
var reCommonJS = regexp.MustCompile(`(?i)(jquery|bootstrap|swiper|slick|owl\.carousel|fontawesome|gtag/js|googletagmanager|recaptcha/api|cloudflare-static|polyfill|lazysizes|moment(\.min)?\.js|lodash|vue(\.runtime)?(\.min)?\.js|react(-dom)?)`)

// ScanOwnerScripts mengunduh file JS milik situs sendiri (domain sama, maks. 8 file, maks. 2 MB/file)
// lalu mencari identifier pengelola di dalamnya. Kunci layanan sering hanya ada di bundle JS.
func ScanOwnerScripts(html, pageURL string) []TrackingID {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	site := registrable(base.Hostname())
	client := &http.Client{Timeout: 12 * time.Second}
	seenURL := map[string]bool{}
	var out []TrackingID
	seen := map[string]bool{}
	n := 0
	for _, m := range reScriptSrc.FindAllStringSubmatch(html, -1) {
		if n >= 8 {
			break
		}
		ref, err := url.Parse(strings.TrimSpace(m[1]))
		if err != nil {
			continue
		}
		u := base.ResolveReference(ref)
		if (u.Scheme != "http" && u.Scheme != "https") || registrable(u.Hostname()) != site || reCommonJS.MatchString(u.Path) || seenURL[u.String()] {
			continue
		}
		seenURL[u.String()] = true
		n++
		req, _ := http.NewRequest("GET", u.String(), nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if resp.StatusCode != 200 {
			continue
		}
		js := string(body)
		for _, t := range append(ExtractOwnerIDs(js), ExtractTrackingIDs(js)...) {
			if k := t.Platform + ":" + t.Value; !seen[k] {
				seen[k] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// registrable: dua label terakhir host (cukup untuk membedakan "situs sendiri" vs CDN pihak ketiga).
func registrable(host string) string {
	parts := strings.Split(strings.TrimSuffix(strings.ToLower(host), "."), ".")
	if len(parts) <= 2 {
		return strings.Join(parts, ".")
	}
	// Domain negara dua tingkat (co.id, com.br, …)
	if l := len(parts); len(parts[l-2]) <= 3 && len(parts[l-1]) == 2 {
		return strings.Join(parts[l-3:], ".")
	}
	return strings.Join(parts[len(parts)-2:], ".")
}
