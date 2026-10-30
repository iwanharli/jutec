package crawler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Fingerprint adalah penanda infrastruktur yang praktis unik per platform/operator:
// ID distribusi CDN, hash favicon, dan host API backend. Situs white-label dari platform
// yang sama hampir selalu berbagi penanda ini meski domain & tampilannya berbeda.
type InfraFingerprint struct {
	Kind  string `json:"kind"`  // cdn | favicon | api
	Value string `json:"value"` // mis. cloudfront:d33egg70nrp50s, sha256:ab12…, jp-api.namesvr.dev
}

var (
	reCloudFront = regexp.MustCompile(`(?i)\b([a-z0-9]{12,16})\.cloudfront\.net`)
	reBunnyCDN   = regexp.MustCompile(`(?i)\b([a-z0-9][a-z0-9-]{2,62})\.b-cdn\.net`)
	reIconLink   = regexp.MustCompile(`(?i)<link[^>]+rel=["'][^"']*icon[^"']*["'][^>]*>`)
	reHref       = regexp.MustCompile(`(?i)href=["']([^"']+)["']`)
)

// apiHostDeny: layanan umum pihak ketiga yang bukan backend operator.
var apiHostDeny = []string{
	"google", "gstatic", "doubleclick", "facebook", "fbcdn", "cloudflareinsights", "cloudflare.com",
	"hotjar", "tawk.to", "livechatinc", "crisp.chat", "jsdelivr", "cdnjs", "jquery", "youtube",
	"twitter", "tiktok", "clarity.ms", "sentry", "newrelic", "onesignal", "recaptcha", "fonts.",
}

// ExtractFingerprints mengumpulkan sidik jari dari HTML + request jaringan.
// siteHost dipakai untuk membuang request ke domain situs itu sendiri.
func ExtractFingerprints(html string, requests, apiCalls []string, siteHost string) []InfraFingerprint {
	seen := map[string]bool{}
	var out []InfraFingerprint
	add := func(kind, value string) {
		key := kind + "|" + value
		if value != "" && !seen[key] {
			seen[key] = true
			out = append(out, InfraFingerprint{Kind: kind, Value: value})
		}
	}

	corpus := html + "\n" + strings.Join(requests, "\n")
	for _, m := range reCloudFront.FindAllStringSubmatch(corpus, -1) {
		add("cdn", "cloudfront:"+strings.ToLower(m[1]))
	}
	for _, m := range reBunnyCDN.FindAllStringSubmatch(corpus, -1) {
		add("cdn", "bunnycdn:"+strings.ToLower(m[1]))
	}

	siteRoot := rootDomain(siteHost)
	for _, raw := range apiCalls {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if rootDomain(host) == siteRoot || isDeniedAPIHost(host) {
			continue
		}
		add("api", host)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Kind+out[i].Value < out[j].Kind+out[j].Value })
	return out
}

func isDeniedAPIHost(host string) bool {
	for _, d := range apiHostDeny {
		if strings.Contains(host, d) {
			return true
		}
	}
	return isCommonCDN(host)
}

// rootDomain: "jp-api.namesvr.dev" → "namesvr.dev" (cukup untuk membedakan domain sendiri vs pihak lain).
func rootDomain(host string) string {
	parts := strings.Split(strings.TrimPrefix(strings.ToLower(host), "www."), ".")
	if len(parts) <= 2 {
		return strings.Join(parts, ".")
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// FaviconFingerprint mengunduh favicon situs dan mengembalikan hash-nya.
// Favicon identik antar domain = template/brand yang sama.
func FaviconFingerprint(html, pageURL string) *InfraFingerprint {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	iconURL := base.ResolveReference(&url.URL{Path: "/favicon.ico"}).String()
	if tag := reIconLink.FindString(html); tag != "" {
		if m := reHref.FindStringSubmatch(tag); m != nil && !strings.HasPrefix(m[1], "data:") {
			if ref, err := url.Parse(strings.TrimSpace(m[1])); err == nil {
				iconURL = base.ResolveReference(ref).String()
			}
		}
	}

	client := &http.Client{Timeout: 6 * time.Second}
	req, _ := http.NewRequest("GET", iconURL, nil)
	req.Header.Set("User-Agent", mobileUserAgent)
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	ct := strings.ToLower(res.Header.Get("Content-Type"))
	if res.StatusCode != 200 || strings.Contains(ct, "html") {
		return nil // halaman error/404 yang dibalas HTML bukan favicon
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 512*1024))
	if err != nil || len(data) < 64 {
		return nil
	}
	sum := sha256.Sum256(data)
	return &InfraFingerprint{Kind: "favicon", Value: "sha256:" + hex.EncodeToString(sum[:])[:32]}
}
