package crawler

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// maxInternalPages: batas halaman internal yang diambil per situs.
const maxInternalPages = 6

// reInternalInteresting: path yang biasanya memuat rekening, QRIS, kontak, atau daftar provider game.
var reInternalInteresting = regexp.MustCompile(`(?i)(deposit|depo|bank|rekening|payment|bayar|qris|withdraw|promo|bonus|kontak|contact|hubungi|cs|livechat|rtp|slot|casino|lobby|game|register|regist|daftar|cara|panduan|tutorial|referral|vip)`)

// interestingInternalLinks memilih link internal paling relevan (path cocok pola, tanpa duplikat path).
func interestingInternalLinks(internal []string, pageURL string) []string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	type cand struct {
		url   string
		score int
	}
	seen := map[string]bool{strings.TrimRight(base.Path, "/"): true}
	var cands []cand
	for _, raw := range internal {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || !strings.EqualFold(rootDomain(u.Hostname()), rootDomain(base.Hostname())) {
			continue
		}
		path := strings.TrimRight(u.Path, "/")
		if path == "" || seen[path] || isAssetPath(path) {
			continue
		}
		matches := reInternalInteresting.FindAllString(path, -1)
		if len(matches) == 0 {
			continue
		}
		seen[path] = true
		u.Fragment = ""
		// Halaman pembayaran/kontak paling berharga → skor lebih tinggi
		score := len(matches)
		if regexp.MustCompile(`(?i)deposit|bank|rekening|payment|qris|kontak|contact`).MatchString(path) {
			score += 3
		}
		cands = append(cands, cand{u.String(), score})
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	var out []string
	for _, c := range cands {
		if len(out) == maxInternalPages {
			break
		}
		out = append(out, c.url)
	}
	return out
}

func isAssetPath(p string) bool {
	return regexp.MustCompile(`(?i)\.(js|css|png|jpe?g|gif|webp|svg|ico|woff2?|ttf|mp3|mp4|pdf|zip|apk)$`).MatchString(p)
}

// fetchHTML mengambil halaman lewat HTTP biasa (tanpa browser) dengan profil HP.
func fetchHTML(client *http.Client, pageURL string) string {
	h, _ := fetchHTMLFinal(client, pageURL)
	return h
}

// fetchHTMLFinal juga mengembalikan URL akhir setelah redirect — dasar yang benar untuk alamat relatif
// (halaman daftar sering dibuka lewat shortener yang mengalihkan ke domain lain).
func fetchHTMLFinal(client *http.Client, pageURL string) (string, string) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", mobileUserAgent)
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9")
	res, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "html") {
		return "", ""
	}
	b, _ := io.ReadAll(io.LimitReader(res.Body, 3*1024*1024))
	return string(b), res.Request.URL.String()
}

// crawlInternalPages mengambil halaman internal penting secara paralel dan menggabungkan
// artefak, tracking ID, link aplikasi, promosi APK, provider game & sidik jari CDN ke result.
// Mengembalikan HTML tiap halaman (url → html) untuk dipakai ulang, mis. pemetaan form login/daftar.
func (c *Crawler) crawlInternalPages(result *AnalysisResult, pageURL string) map[string]string {
	got := map[string]string{}
	targets := interestingInternalLinks(result.Links.Internal, pageURL)
	if len(targets) == 0 {
		return got
	}
	client := &http.Client{Timeout: 10 * time.Second}

	htmls := make([]string, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, t string) {
			defer wg.Done()
			htmls[i] = fetchHTML(client, t)
		}(i, t)
	}
	wg.Wait()

	beforeArt, beforeApps := len(result.Artifacts), len(result.Links.Apps)
	providers := map[string]bool{}
	for _, p := range result.NetworkSignals.GameProviders {
		providers[p] = true
	}
	host := ""
	if u, err := url.Parse(pageURL); err == nil {
		host = u.Hostname()
	}

	for i, html := range htmls {
		if html == "" {
			continue
		}
		result.InternalPages = append(result.InternalPages, targets[i])
		got[targets[i]] = html
		result.Artifacts = append(result.Artifacts, ExtractArtifacts(html)...)
		ml := ExtractLinks(html, targets[i])
		result.Links.Apps = mergeUnique(result.Links.Apps, c.resolveAppLinks(ml.Apps))
		result.Links.Register = mergeUnique(result.Links.Register, ml.Register)
		result.Links.External = mergeUnique(result.Links.External, ml.External)
		result.Links.Alternate = mergeUnique(result.Links.Alternate, ml.Alternate)
		result.TrackingIDs = mergeTracking(result.TrackingIDs, ExtractTrackingIDs(html))
		result.TrackingIDs = mergeTracking(result.TrackingIDs, ExtractOwnerIDs(html))
		result.AppPromo = mergeAppPromo(result.AppPromo, DetectAppPromo(html))
		result.Fingerprints = mergeFingerprints(result.Fingerprints, ExtractFingerprints(html, nil, nil, host))
		for _, p := range providersInHTML(html) {
			if !providers[p] {
				providers[p] = true
				result.NetworkSignals.GameProviders = append(result.NetworkSignals.GameProviders, p)
			}
		}
	}
	result.Artifacts = uniqueArtifacts(result.Artifacts)
	log.Printf("  📄 [Internal] %d/%d halaman diambil · +%d artefak · +%d app link · provider: %v",
		len(result.InternalPages), len(targets), len(result.Artifacts)-beforeArt, len(result.Links.Apps)-beforeApps,
		result.NetworkSignals.GameProviders)
	return got
}

func mergeTracking(a, b []TrackingID) []TrackingID {
	seen := map[string]bool{}
	for _, t := range a {
		seen[t.Platform+"|"+t.Value] = true
	}
	for _, t := range b {
		if !seen[t.Platform+"|"+t.Value] {
			seen[t.Platform+"|"+t.Value] = true
			a = append(a, t)
		}
	}
	return a
}

func mergeFingerprints(a, b []InfraFingerprint) []InfraFingerprint {
	seen := map[string]bool{}
	for _, f := range a {
		seen[f.Kind+"|"+f.Value] = true
	}
	for _, f := range b {
		if !seen[f.Kind+"|"+f.Value] {
			seen[f.Kind+"|"+f.Value] = true
			a = append(a, f)
		}
	}
	return a
}
