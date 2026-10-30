package crawler

import (
	"net/url"
	"regexp"
	"strings"
)

type ExtractedLinks struct {
	Internal  []string `json:"internal"`  // link dalam domain yang sama
	External  []string `json:"external"`  // link ke domain lain
	Iframes   []string `json:"iframes"`   // iframe src
	Images    []string `json:"images"`    // img/source src dari domain eksternal
	Alternate []string `json:"alternate"` // canonical/alternate/amphtml — domain terkait operator
	Apps      []string `json:"apps"`      // .apk/.ipa + app store links
	Register  []string `json:"register"`  // halaman registrasi/daftar judol
	Total     int      `json:"total"`
}

// registerKeywords adalah kata kunci yang menandakan halaman registrasi judol.
var registerKeywords = []string{
	"register", "regist", "daftar", "signup", "sign-up",
	"join", "pendaftaran", "create-account", "new-account",
	"member", "mendaftar",
}

func isRegisterURL(rawURL string) bool {
	lower := strings.ToLower(strings.Split(rawURL, "?")[0])
	for _, kw := range registerKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

var (
	reAnchor      = regexp.MustCompile(`(?i)<a[^>]+href=["']([^"'#\s][^"']*)["']`)
	// <a href="…">teks</a> — untuk mengenali tombol pendaftaran dari teksnya
	reAnchorWithText = regexp.MustCompile(`(?is)<a[^>]+href=["']([^"'#\s][^"']*)["'][^>]*>(.{0,300}?)</a>`)
	reRegisterText   = regexp.MustCompile(`(?i)\b(daftar|register|registrasi|login|log\s?in|masuk|join|sign\s?up|gabung)\b`)
	reIframe      = regexp.MustCompile(`(?i)<iframe[^>]+src=["']([^"']+)["']`)
	reImg         = regexp.MustCompile(`(?i)<(?:img|source|video|audio)[^>]+src=["']([^"']+)["']`)
	reDataAttr    = regexp.MustCompile(`(?i)\bdata-(?:href|url|src|link|action)=["']([^"']+)["']`)
	reScriptURL   = regexp.MustCompile(`(?i)["'](https?://[a-zA-Z0-9][a-zA-Z0-9\-\.]+\.[a-zA-Z]{2,}[^"'\s<>]{0,200})["']`)
	reProtocolRel = regexp.MustCompile(`(?i)(?:href|src)=["'](//[a-zA-Z0-9][^"']+)["']`)

	// <link rel="canonical/alternate/amphtml"> — domain lain milik operator yang sama
	reLinkRelFirst  = regexp.MustCompile(`(?i)<link[^>]+rel=["'](?:canonical|alternate|amphtml)["'][^>]+href=["']([^"']+)["']`)
	reLinkHrefFirst = regexp.MustCompile(`(?i)<link[^>]+href=["']([^"']+)["'][^>]+rel=["'](?:canonical|alternate|amphtml)["']`)
)

// ExtractLinks mengambil semua link dari HTML dan mengkategorikannya.
// Menangani: <a href>, <iframe src>, <img src>, data attributes, JS strings, protocol-relative URLs.
func ExtractLinks(html, rawBaseURL string) *ExtractedLinks {
	baseDomain := extractDomain(rawBaseURL)

	internalSeen  := make(map[string]bool)
	externalSeen  := make(map[string]bool)
	iframeSeen    := make(map[string]bool)
	imagesSeen    := make(map[string]bool)
	appsSeen      := make(map[string]bool)
	registerSeen  := make(map[string]bool)

	addLink := func(raw string) {
		link := strings.TrimSpace(raw)
		if skip(link) || isImageURL(link) {
			return
		}
		link = resolveURL(link, rawBaseURL)
		if link == "" || isImageURL(link) {
			return
		}
		// Unwrap link wrapper (l.wl.co, l.facebook.com, t.co, dll)
		link = unwrapURL(link)
		if isAppURL(link) {
			appsSeen[link] = true
			return
		}
		if isRegisterURL(link) {
			registerSeen[link] = true
			return
		}
		// IP:port langsung — selalu external, tandai khusus
		if isIPPortURL(link) {
			externalSeen[link] = true
			return
		}
		domain := extractDomain(link)
		if domain == "" || domain == baseDomain {
			internalSeen[link] = true
		} else {
			externalSeen[link] = true
		}
	}

	// 1. <a href="...">
	for _, m := range reAnchor.FindAllStringSubmatch(html, -1) {
		addLink(m[1])
	}

	// 1b. Tombol berteks DAFTAR/LOGIN/REGISTER → link pendaftaran apa pun URL-nya
	// (mis. href="https://jali.me/tripslot" yang tak mengandung kata "register").
	for _, m := range reAnchorWithText.FindAllStringSubmatch(html, -1) {
		text := reHTMLTag.ReplaceAllString(m[2], " ")
		if !reRegisterText.MatchString(text) {
			continue
		}
		link := strings.TrimSpace(m[1])
		if skip(link) {
			continue
		}
		if link = unwrapURL(resolveURL(link, rawBaseURL)); link == "" || isAppURL(link) {
			continue
		}
		if d := extractDomain(link); d != "" && d != baseDomain {
			delete(externalSeen, link)
			registerSeen[link] = true
		}
	}

	// 2. <iframe src="...">
	for _, m := range reIframe.FindAllStringSubmatch(html, -1) {
		link := strings.TrimSpace(m[1])
		if !skip(link) {
			link = resolveURL(link, rawBaseURL)
			if link != "" {
				iframeSeen[link] = true
			}
		}
	}

	// 3. <img src>, <source src>, <video src>, <audio src>
	for _, m := range reImg.FindAllStringSubmatch(html, -1) {
		link := strings.TrimSpace(m[1])
		if !skip(link) {
			link = resolveURL(link, rawBaseURL)
			if link != "" {
				domain := extractDomain(link)
				if domain != "" && domain != baseDomain {
					imagesSeen[link] = true
				}
			}
		}
	}

	// 4. data-href, data-url, data-src, data-link, data-action
	for _, m := range reDataAttr.FindAllStringSubmatch(html, -1) {
		addLink(m[1])
	}

	// 5. Protocol-relative URLs: href="//domain.com/..."
	for _, m := range reProtocolRel.FindAllStringSubmatch(html, -1) {
		addLink("https:" + m[1])
	}

	// 6. URLs dalam <script> tag (JS strings)
	//    Skip gambar dan CDN library umum — hanya ambil link bermakna
	for _, m := range reScriptURL.FindAllStringSubmatch(html, -1) {
		link := m[1]
		if isImageURL(link) {
			continue
		}
		if isAppURL(link) {
			appsSeen[link] = true
			continue
		}
		domain := extractDomain(link)
		if domain != "" && domain != baseDomain && !isCommonCDN(domain) {
			externalSeen[link] = true
		}
	}

	// 7. <link rel="canonical/alternate/amphtml"> — domain terkait operator
	alternateSeen := make(map[string]bool)
	processAlternate := func(raw string) {
		link := strings.TrimSpace(raw)
		if skip(link) || isImageURL(link) {
			return
		}
		link = resolveURL(link, rawBaseURL)
		if link == "" {
			return
		}
		domain := extractDomain(link)
		if domain != "" && domain != baseDomain {
			alternateSeen[link] = true
		}
	}
	for _, m := range reLinkRelFirst.FindAllStringSubmatch(html, -1) {
		processAlternate(m[1])
	}
	for _, m := range reLinkHrefFirst.FindAllStringSubmatch(html, -1) {
		processAlternate(m[1])
	}

	// Link pendaftaran tidak dihitung ganda sebagai external (langkah lain bisa menambahkannya lagi)
	for l := range registerSeen {
		delete(externalSeen, l)
	}

	internal  := mapKeys(internalSeen)
	external  := mapKeys(externalSeen)
	iframes   := mapKeys(iframeSeen)
	images    := mapKeys(imagesSeen)
	alternate := mapKeys(alternateSeen)
	apps      := mapKeys(appsSeen)
	register  := mapKeys(registerSeen)

	return &ExtractedLinks{
		Internal:  internal,
		External:  external,
		Iframes:   iframes,
		Images:    images,
		Alternate: alternate,
		Apps:      apps,
		Register:  register,
		Total:     len(internal) + len(external) + len(register),
	}
}

// appExts adalah ekstensi file aplikasi mobile yang sering dipakai operator judol.
var appExts = []string{".apk", ".ipa", ".aab"}

// appStoreDomains adalah domain app store / app catalog yang tidak perlu di-crawl sebagai situs judol.
var appStoreDomains = []string{
	"uptodown.com", "apk.cafe", "apkpure.com", "apkpure.net",
	"aptoide.com", "apkmirror.com", "apkmonk.com", "apkcombo.com",
	"mob.org", "androidappsapk.co",
}

// knownShorteners adalah domain URL shortener yang umum dipakai untuk distribusi APK judol.
var knownShorteners = []string{
	"rebrand.ly", "bit.ly", "s.id", "tinyurl.com", "t.ly",
	"short.io", "rb.gy", "cutt.ly", "shorturl.at", "is.gd",
	"gg.gg", "v.gd", "tiny.cc",
	// Umum di situs judol Indonesia (tombol DAFTAR/LOGIN anti-blokir)
	"jali.me", "jaga.link", "bit.do", "ow.ly", "kutt.it", "shorturl.asia",
	"cutt.us", "u.to", "clck.ru", "goo.su", "tny.im", "linkr.it", "bitly.ws",
}

// wrapperDomains adalah domain yang menyembunyikan URL asli di query param.
// Contoh: l.wl.co/l/?u=https://47.254.231.232:1232/
var wrapperDomains = []string{
	"l.wl.co", "wl.co",         // WhatsApp link wrapper
	"lnkd.in",                   // LinkedIn
	"out.reddit.com",            // Reddit
	"l.facebook.com",            // Facebook
	"lm.facebook.com",           // Facebook mobile
	"t.co",                      // Twitter/X
}

// wrapperParams adalah nama query param yang berisi URL asli pada link wrapper.
var wrapperParams = []string{"u", "url", "redirect", "dest", "destination", "to", "target", "link", "q"}

// unwrapURL mengekstrak URL asli dari link wrapper jika ada.
// Contoh: https://l.wl.co/l/?u=https://47.254.231.232:1232/ → https://47.254.231.232:1232/
func unwrapURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Cek apakah domain adalah wrapper yang dikenal
	host := strings.TrimPrefix(u.Hostname(), "www.")
	isWrapper := false
	for _, wd := range wrapperDomains {
		if host == wd || strings.HasSuffix(host, "."+wd) {
			isWrapper = true
			break
		}
	}
	if !isWrapper {
		return rawURL
	}

	// Coba ekstrak URL asli dari query params
	q := u.Query()
	for _, param := range wrapperParams {
		if val := q.Get(param); val != "" {
			// Validasi bahwa nilai param adalah URL valid
			if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") {
				return val
			}
		}
	}
	return rawURL
}

// isIPPortURL mendeteksi URL dengan IP address langsung + port non-standar.
// Contoh: https://47.254.231.232:1232/ — teknik evasion umum judol.
func isIPPortURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" || port == "80" || port == "443" {
		return false
	}
	// Cek apakah host adalah IP address (bukan domain)
	parts := strings.Split(host, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 3 {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// appHintKeywords adalah kata-kata yang menunjukkan sebuah short link mengarah ke app.
var appHintKeywords = []string{
	"apk", "install", "app", "download", "unduh", "android", "ios", "mobile",
}

// installPagePatterns adalah pola URL halaman landing download aplikasi.
var installPagePatterns = []string{
	"_install.html", "install.html", "/install/", "download.html",
	"/download-app", "/get-app", "/unduh-app", "/app-download",
}

// isAppURL mendeteksi link aplikasi mobile: file APK/IPA, app store, install page,
// dan URL shortener yang mengandung hint kata kunci app.
func isAppURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	noQuery := strings.Split(lower, "?")[0]

	// File langsung (.apk, .ipa, .aab)
	for _, ext := range appExts {
		if strings.HasSuffix(noQuery, ext) {
			return true
		}
	}

	// App store resmi dan app catalog
	if strings.Contains(lower, "play.google.com/store/apps") ||
		strings.Contains(lower, "apps.apple.com/") {
		return true
	}
	for _, d := range appStoreDomains {
		if strings.Contains(lower, d) {
			return true
		}
	}

	// Halaman install/download (bukan shortener)
	for _, p := range installPagePatterns {
		if strings.Contains(noQuery, p) {
			return true
		}
	}

	// URL shortener + slug mengandung kata kunci app
	for _, shortener := range knownShorteners {
		if strings.Contains(lower, shortener) {
			for _, kw := range appHintKeywords {
				if strings.Contains(lower, kw) {
					return true
				}
			}
			break
		}
	}

	return false
}

// IsShortenerURL mengembalikan true jika URL adalah dari domain shortener yang dikenal.
// Dipakai oleh crawler untuk memutuskan apakah perlu di-resolve.
func IsShortenerURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	for _, s := range knownShorteners {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// commonCDNs adalah domain CDN/library umum yang tidak perlu disimpan sebagai intelligence.
var commonCDNs = []string{
	"jquery.com", "googleapis.com", "gstatic.com", "cloudflare.com",
	"bootstrapcdn.com", "fontawesome.com", "jsdelivr.net", "unpkg.com",
	"cdnjs.cloudflare.com", "cdn.jsdelivr.net", "google-analytics.com",
	"googletagmanager.com", "doubleclick.net", "googlesyndication.com",
}

var imageExts = []string{
	".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg",
	".ico", ".bmp", ".avif", ".tiff", ".woff", ".woff2", ".ttf",
}

// imageCDNPrefixes adalah hostname CDN gambar yang URL-nya tidak selalu punya ekstensi gambar
// (menggunakan hash path). Semua URL dari domain ini di-skip.
var imageCDNPrefixes = []string{
	"img.utdstc.com", "utdstc.com",
	"img.apkpure.com", "image.apkpure.com",
	"img.uptodown.com", "imganuncios.mitula.net",
	"s.ntv.io", "ntv.io",
	"utt.impactcdn.com", "impactcdn.com",
	"static.apkpure.net",
	"secure.gravatar.com", "i.gravatar.com",
	"i.imgur.com", "i.ibb.co",
}

func isImageURL(rawURL string) bool {
	lower := strings.ToLower(strings.Split(rawURL, "?")[0])
	// Cek ekstensi
	for _, ext := range imageExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	// Cek image CDN host — URL hash tanpa ekstensi tetap adalah gambar
	for _, cdn := range imageCDNPrefixes {
		if strings.Contains(lower, cdn) {
			return true
		}
	}
	return false
}

func isCommonCDN(domain string) bool {
	for _, cdn := range commonCDNs {
		if strings.HasSuffix(domain, cdn) || domain == cdn {
			return true
		}
	}
	return false
}

func skip(link string) bool {
	if link == "" {
		return true
	}
	lower := strings.ToLower(link)
	return strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "data:")
}

func resolveURL(link, base string) string {
	// Bersihkan karakter tidak valid di akhir URL (backslash, spasi, dll)
	link = strings.TrimRight(link, "\\/\t\r\n")

	// Protocol-relative
	if strings.HasPrefix(link, "//") {
		link = "https:" + link
	}

	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link
	}

	// Bare domain detection: "domain.com/path" tanpa protocol
	// Heuristic: ada titik di bagian pertama dan tidak dimulai dengan "/"
	if !strings.HasPrefix(link, "/") && !strings.HasPrefix(link, ".") {
		parts := strings.SplitN(link, "/", 2)
		if strings.Contains(parts[0], ".") && !strings.Contains(parts[0], " ") {
			return "https://" + link
		}
	}

	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	ref, err := url.Parse(link)
	if err != nil {
		return ""
	}
	return b.ResolveReference(ref).String()
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	return host
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
