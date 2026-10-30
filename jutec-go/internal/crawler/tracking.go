package crawler

import (
	"regexp"
	"strings"
)

type TrackingID struct {
	Platform string `json:"platform"`
	Value    string `json:"value"`
}

// MetaToken adalah token verifikasi domain dari <meta> tag HTML.
// Disimpan di tabel txt_tokens (sama seperti DNS TXT tokens).
type MetaToken struct {
	Service string `json:"service"`
	Token   string `json:"token"`
}

var trackingPatterns = []struct {
	Platform string
	Patterns []*regexp.Regexp
}{
	{
		Platform: "GoogleAnalytics",
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(UA-\d{4,12}-\d{1,4})`),
		},
	},
	{
		Platform: "GoogleTag",
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)['"](G-[A-Z0-9]{6,12})['"]`),
		},
	},
	{
		Platform: "GoogleTagManager",
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)['"](GTM-[A-Z0-9]{5,8})['"]`),
		},
	},
	{
		Platform: "FacebookPixel",
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)fbq\s*\(\s*['"]init['"]\s*,\s*['"](\d{10,20})['"]`),
			regexp.MustCompile(`(?i)facebook\.com/tr\?id=(\d{10,20})`),
		},
	},
	{
		Platform: "LiveChat",
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)license_id[=:]\s*['"]?(\d{6,12})`),
			regexp.MustCompile(`(?i)livechat\.com/chat-with/(\d{6,12})`),
		},
	},
	{
		Platform: "TawkTo",
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)tawk\.to/([a-f0-9]{24}/[a-z0-9\-]+)`),
			regexp.MustCompile(`(?i)s\d+\.tawk\.to/v1/([a-f0-9]{24})`),
		},
	},
	{
		Platform: "Crisp",
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)crisp\.chat/g/([a-f0-9\-]{36})`),
			regexp.MustCompile(`(?i)crisp_website_id['":\s=]+['"]([a-f0-9\-]{36})`),
		},
	},
	{
		Platform: "Hotjar",
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)hjid['":\s,]+(\d{5,10})`),
			regexp.MustCompile(`(?i)hotjar\.com/c/(\d{5,10})`),
		},
	},
	// Platform judol hosting — UUID/brand = identitas operator
	{
		Platform: "AsiaSigma",
		Patterns: []*regexp.Regexp{
			// cloudstoragesigma02.com/production/agent/{UUID}/...
			regexp.MustCompile(`(?i)cloudstoragesigma\d*\.com/production/agent/([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`),
		},
	},
	{
		Platform: "GigaGaming",
		Patterns: []*regexp.Regexp{
			// files.sitestatic.net/banners/timestamp__BRANDNAME__size.ext
			regexp.MustCompile(`(?i)sitestatic\.net/[^"'\s]*__([A-Z][A-Z0-9_]{2,30})__`),
		},
	},
}

// ExtractTrackingIDs mencari semua tracking ID dari HTML.
// Termasuk: GA, FB Pixel, LiveChat, Tawk.to, Crisp, Hotjar,
//           AsiaSigma operator UUID, GigaGaming brand code.
func ExtractTrackingIDs(html string) []TrackingID {
	seen := make(map[string]bool)
	var results []TrackingID

	for _, entry := range trackingPatterns {
		for _, re := range entry.Patterns {
			for _, m := range re.FindAllStringSubmatch(html, -1) {
				if len(m) < 2 {
					continue
				}
				val := strings.TrimSpace(m[1])
				if !validTrackingID(entry.Platform, val) {
					continue
				}
				key := entry.Platform + ":" + val
				if !seen[key] {
					seen[key] = true
					results = append(results, TrackingID{
						Platform: entry.Platform,
						Value:    val,
					})
				}
			}
		}
	}

	return results
}

// metaTokenPatterns mendefinisikan <meta> tag verification tokens.
// name attribute bisa sebelum atau sesudah content attribute.
var metaTokenPatterns = []struct {
	Service   string
	NameAttrs []string // nilai atribut name= yang dicocokan
}{
	{Service: "GoogleVerification", NameAttrs: []string{"google-site-verification"}},
	{Service: "FacebookVerification", NameAttrs: []string{"facebook-domain-verification"}},
	{Service: "MicrosoftVerification", NameAttrs: []string{"msvalidate.01"}},
	{Service: "YandexVerification", NameAttrs: []string{"yandex-verification"}},
	{Service: "AppleVerification", NameAttrs: []string{"apple-domain-verification"}},
}

var (
	// name sebelum content: <meta name="X" content="TOKEN">
	reMetaNameFirst = regexp.MustCompile(`(?i)<meta[^>]+name=["']([^"']+)["'][^>]+content=["']([^"']{10,200})["']`)
	// content sebelum name: <meta content="TOKEN" name="X">
	reMetaContentFirst = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']{10,200})["'][^>]+name=["']([^"']+)["']`)
)

// ExtractMetaTokens mengekstrak token verifikasi domain dari <meta> tag HTML.
// Hasilnya disimpan ke tabel txt_tokens (sama seperti DNS TXT records).
func ExtractMetaTokens(html string) []MetaToken {
	seen := make(map[string]bool)
	var results []MetaToken

	// Buat map dari name → service untuk lookup cepat
	nameToService := make(map[string]string)
	for _, p := range metaTokenPatterns {
		for _, name := range p.NameAttrs {
			nameToService[strings.ToLower(name)] = p.Service
		}
	}

	process := func(name, token string) {
		service, ok := nameToService[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return
		}
		token = strings.TrimSpace(token)
		key := service + ":" + token
		if !seen[key] {
			seen[key] = true
			results = append(results, MetaToken{Service: service, Token: token})
		}
	}

	// Pola 1: name sebelum content
	for _, m := range reMetaNameFirst.FindAllStringSubmatch(html, -1) {
		process(m[1], m[2])
	}

	// Pola 2: content sebelum name
	for _, m := range reMetaContentFirst.FindAllStringSubmatch(html, -1) {
		process(m[2], m[1])
	}

	return results
}
