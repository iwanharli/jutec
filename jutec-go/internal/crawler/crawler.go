package crawler

import (
	"jutec-go/internal/browser"
	"context"
	"regexp"
	"sync"
	neturl "net/url"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"jutec-go/internal/netutil"

	cdpnetwork "github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type Crawler struct {
	Analyzer   *Analyzer
	httpClient *http.Client
}

func NewCrawler() *Crawler {
	// HTTP client khusus untuk redirect chain — tidak auto-follow redirect.
	// Tersambung lewat IP dari DoH: DNS ISP membelokkan situs yang diblokir ke halaman Internet Positif.
	client := netutil.HTTPClient(10 * time.Second)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // stop di setiap redirect, kita handle manual
	}
	return &Crawler{
		Analyzer:   NewAnalyzer(),
		httpClient: client,
	}
}

// ProcessURL crawls url dengan headless browser, menginterception network request,
// lalu menganalisis HTML + network signals menggunakan keywords dari DB.
func (c *Crawler) ProcessURL(url string, dbKeywords []WeightedKeyword, depth int) (*AnalysisResult, error) {
	return c.process(url, dbKeywords, depth, true)
}

// ProcessURLDesktop sama dengan ProcessURL tanpa pass profil HP — untuk sub-crawl agar scan tidak melambat.
func (c *Crawler) ProcessURLDesktop(url string, dbKeywords []WeightedKeyword, depth int) (*AnalysisResult, error) {
	return c.process(url, dbKeywords, depth, false)
}

func (c *Crawler) process(url string, dbKeywords []WeightedKeyword, depth int, mobilePass bool) (*AnalysisResult, error) {
	log.Printf("🌐 [Crawler D:%d] Processing: %s", depth, url)

	// 1. Rekam redirect chain via HTTP sebelum buka browser
	redirectChain := c.captureRedirectChain(url)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Situs yang diblokir Komdigi: DNS ISP (bahkan permintaan ke 1.1.1.1 dibajak) mengarah ke server
	// Internet Positif yang tak merespons HTTPS → crawl menggantung sampai timeout. Arahkan Chrome ke IP asli dari DoH.
	resolverRules, ispBlocked := dohResolverRules(ctx, url)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		// Iframe widget (LiveChat, Tawk…) satu proses dengan halaman → traffic & WebSocket-nya ikut tersadap
		chromedp.Flag("disable-site-isolation-trials", true),
		chromedp.Flag("disable-features", "IsolateOrigins,site-per-process"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)
	if resolverRules != "" {
		opts = append(opts, chromedp.Flag("host-resolver-rules", resolverRules))
	}

	allocCtx, allocCancel := browser.NewAllocator(ctx, opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx, chromedp.WithErrorf(func(s string, args ...interface{}) {
		msg := fmt.Sprintf(s, args...)
		if strings.Contains(msg, "unhandled node event") || strings.Contains(msg, "protocol error") {
			return // Suppress noise
		}
		log.Printf("🔴 [ChromeDP] %s", msg)
	}))
	defer taskCancel()

	// 2. Intercept network requests selama crawl
	capture := &networkCapture{}
	chromedp.ListenTarget(taskCtx, capture.listen)

	var htmlContent string
	var finalURL string
	err := chromedp.Run(taskCtx,
		cdpnetwork.Enable(),
		// Jangan pakai chromedp.Navigate: ia menunggu event "load" (semua gambar/CDN selesai),
		// yang di situs judol berat sering tak pernah terjadi → seluruh scan timeout.
		// Cukup tunggu DOM siap, lalu beri jeda untuk JS render.
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, _, errText, _, err := page.Navigate(url).Do(ctx)
			if err != nil {
				return err
			}
			if errText != "" {
				return fmt.Errorf("navigate: %s", errText)
			}
			return nil
		}),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(5*time.Second),
		chromedp.Location(&finalURL),
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		if ispBlocked {
			return nil, fmt.Errorf("diblokir ISP (Internet Positif) dan tidak terjangkau lewat IP asli: %w", err)
		}
		return nil, err
	}

	// Halaman blokir/tantangan proteksi bot (Cloudflare dll.) bukan konten situs — menilainya akan
	// membuat situs judol tampak bersih. Perlakukan sebagai gagal agar vonis sebelumnya tidak tertimpa.
	if reason := botBlockReason(htmlContent); reason != "" {
		return nil, fmt.Errorf("diblokir proteksi bot (%s) — konten situs tidak terbaca", reason)
	}

	// Widget chat mengirim pesan sambutan (berisi link alternatif/APK/Telegram) lewat WebSocket
	// beberapa detik setelah halaman siap — tunggu sampai muncul, maks. 10 detik.
	if capture.hasChatWidget() {
		for i := 0; i < 10 && len(capture.widgetLinks()) == 0; i++ {
			time.Sleep(time.Second)
		}
	}

	// Tambahkan finalURL jika browser diarahkan via JS/Meta refresh (bukan HTTP)
	if finalURL != "" && len(redirectChain) > 0 && finalURL != redirectChain[len(redirectChain)-1] {
		redirectChain = append(redirectChain, finalURL)
		log.Printf("  ↪️  [Redirect JS/Meta] Final: %s", finalURL)
	}

	// Link judol tersembunyi (SEO spam) — dihitung di browser karena butuh CSS terhitung
	hiddenSpam := detectHiddenSpam(taskCtx)

	// 3. Analisis konten
	networkSignals := capture.analyze()
	result := c.Analyzer.Analyze(htmlContent, dbKeywords, networkSignals)
	result.ISPBlocked = ispBlocked
	result.Links = ExtractLinks(htmlContent, finalURL)

	// Resolve shortener app links inline (timeout pendek per link)
	result.Links.Apps = c.resolveAppLinks(result.Links.Apps)
	result.RedirectChain = redirectChain
	result.TrackingIDs = ExtractTrackingIDs(htmlContent)
	// Identifier pengelola (reCAPTCHA, Firebase, bot Telegram, …) dari HTML & file JS milik situs
	result.TrackingIDs = mergeTracking(result.TrackingIDs, ExtractOwnerIDs(htmlContent))
	result.TrackingIDs = mergeTracking(result.TrackingIDs, ScanOwnerScripts(htmlContent, finalURL))
	result.MetaTokens = ExtractMetaTokens(htmlContent)
	result.SimHash = ComputeSimHash(htmlContent)
	result.AppPromo = DetectAppPromo(htmlContent)
	c.mergeWidgetLinks(result, capture, finalURL)

	// 3b. Penyusupan: cloaking (konten judol hanya untuk Googlebot / pengunjung dari Google) — dicek bila
	// versi pengunjung belum jelas judol. Versi cloaked digabung agar kata kunci & rekeningnya ikut tercatat.
	var inj *Injection
	if mobilePass && result.Confidence < 0.7 {
		var cloaked *AnalysisResult
		inj, cloaked = c.checkCloaking(ctx, finalURL, result, dbKeywords)
		if cloaked != nil {
			result.FoundKeywords = mergeUnique(result.FoundKeywords, cloaked.FoundKeywords)
			result.Artifacts = append(result.Artifacts, cloaked.Artifacts...)
			if cloaked.Confidence > result.Confidence {
				result.Confidence = cloaked.Confidence
			}
		}
	}
	result.Injection = finalizeInjection(inj, hiddenSpam)
	if result.Injection != nil && result.Injection.Detected {
		log.Printf("  🕳️  [Injection] %v · tersembunyi %d/%d · cloak %q", result.Injection.Signals, result.Injection.HiddenLinks, result.Injection.JudolLinks, result.Injection.CloakVia)
	}

	// 4. Pass kedua dengan profil HP Android: tombol APK / halaman "versi mobile" sering hanya
	// muncul di HP. Non-fatal — kegagalan di sini tidak membatalkan hasil desktop.
	// Semua HTML yang terkumpul — dipakai ulang untuk memetakan form login/daftar
	pages := map[string]string{finalURL: htmlContent}
	if mobilePass {
		if mobileHTML, err := c.fetchMobileHTML(taskCtx, url); err != nil {
			log.Printf("  📱 [Mobile] Dilewati: %v", err)
		} else {
			c.mergeMobile(result, mobileHTML, finalURL)
			pages[finalURL+"#mobile"] = mobileHTML
		}
	}
	// Ikuti shortener & link pendaftaran eksternal ke tujuan akhirnya
	// (mis. jali.me/tripslot → worthlzdtoto.store/?content=register…)
	c.resolveRegisterLinks(result.Links)

	// 5. Sidik jari infrastruktur (CDN, API backend, favicon) — untuk menghubungkan situs satu platform
	requests, apiCalls := capture.snapshot()
	host := ""
	if u, err := neturl.Parse(finalURL); err == nil {
		host = u.Hostname()
	}
	result.Fingerprints = ExtractFingerprints(htmlContent, requests, apiCalls, host)
	if fav := FaviconFingerprint(htmlContent, finalURL); fav != nil {
		result.Fingerprints = append(result.Fingerprints, *fav)
	}

	// 6. Halaman internal penting (deposit, bank, kontak, promo, lobby…) hanya untuk situs utama
	// yang sudah menunjukkan tanda judol — situs normal tak perlu ditelusuri lebih dalam.
	if mobilePass && looksGambling(result) {
		for u, h := range c.crawlInternalPages(result, finalURL) {
			pages[u] = h
		}
	}

	// 6b. Provider game baru dimuat di halaman lobby/game — buka satu dengan browser bila beranda kosong
	if mobilePass && looksGambling(result) && len(result.NetworkSignals.GameProviders) == 0 {
		c.probeLobbyProviders(taskCtx, result, finalURL)
	}

	// 7. Peta login & pendaftaran (pasif: hanya membaca HTML, form tak pernah dikirim)
	if mobilePass {
		as := c.collectAuthSurface(result, pages, finalURL)
		result.AuthSurface = as
		for _, code := range as.ReferralCodes {
			result.Fingerprints = mergeFingerprints(result.Fingerprints, []InfraFingerprint{{Kind: "referral", Value: strings.ToLower(code)}})
		}
		for _, f := range as.Forms {
			if u, err := neturl.Parse(f.Action); err == nil && f.ExternalAct && u.Hostname() != "" {
				result.Fingerprints = mergeFingerprints(result.Fingerprints, []InfraFingerprint{{Kind: "api", Value: strings.ToLower(u.Hostname())}})
			}
		}
		log.Printf("  🔐 [Auth] login:%d daftar:%d form:%d referral:%v", len(as.LoginURLs), len(as.RegisterURLs), len(as.Forms), as.ReferralCodes)
	}

	if result.AppPromo != nil && result.AppPromo.Detected {
		log.Printf("  📲 [AppPromo] Promosi aplikasi terdeteksi (%s), link: %d", strings.Join(result.AppPromo.Evidence, ", "), len(result.Links.Apps))
	}

	return result, nil
}

const mobileUserAgent = "Mozilla/5.0 (Linux; Android 13; SM-A536E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"

// fetchMobileHTML membuka URL di tab baru (browser yang sama) dengan emulasi HP Android.
func (c *Crawler) fetchMobileHTML(parent context.Context, url string) (string, error) {
	tabCtx, cancel := chromedp.NewContext(parent)
	defer cancel()
	tabCtx, cancelT := context.WithTimeout(tabCtx, 20*time.Second)
	defer cancelT()

	var html string
	err := chromedp.Run(tabCtx,
		emulation.SetUserAgentOverride(mobileUserAgent).WithPlatform("Android"),
		emulation.SetDeviceMetricsOverride(412, 915, 2.625, true),
		emulation.SetTouchEmulationEnabled(true),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, _, errText, _, err := page.Navigate(url).Do(ctx)
			if err == nil && errText != "" {
				err = fmt.Errorf("navigate: %s", errText)
			}
			return err
		}),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(4*time.Second),
		chromedp.OuterHTML("html", &html),
	)
	return html, err
}

// mergeMobile menambahkan link aplikasi, link, artefak, tracking & penanda promosi dari versi HP.
// Skor konten tetap dari versi desktop agar tidak dihitung dua kali.
func (c *Crawler) mergeMobile(result *AnalysisResult, mobileHTML, baseURL string) {
	result.MobileCrawled = true
	ml := ExtractLinks(mobileHTML, baseURL)
	before := len(result.Links.Apps)
	result.Links.Apps = mergeUnique(result.Links.Apps, c.resolveAppLinks(ml.Apps))
	result.Links.Register = mergeUnique(result.Links.Register, ml.Register)
	result.Links.External = mergeUnique(result.Links.External, ml.External)
	result.Links.Alternate = mergeUnique(result.Links.Alternate, ml.Alternate)
	result.Links.Iframes = mergeUnique(result.Links.Iframes, ml.Iframes)
	result.Links.Total = len(result.Links.Internal) + len(result.Links.External)

	result.Artifacts = uniqueArtifacts(append(result.Artifacts, ExtractArtifacts(mobileHTML)...))

	seen := map[string]bool{}
	for _, t := range result.TrackingIDs {
		seen[t.Platform+"|"+t.Value] = true
	}
	for _, t := range append(ExtractTrackingIDs(mobileHTML), ExtractOwnerIDs(mobileHTML)...) {
		if !seen[t.Platform+"|"+t.Value] {
			seen[t.Platform+"|"+t.Value] = true
			result.TrackingIDs = append(result.TrackingIDs, t)
		}
	}

	result.AppPromo = mergeAppPromo(result.AppPromo, DetectAppPromo(mobileHTML))
	log.Printf("  📱 [Mobile] +%d app link dari profil HP", len(result.Links.Apps)-before)
}

// resolveAppLinks menelusuri redirect untuk setiap app link dari URL shortener.
// Shortener URL (rebrand.ly, bit.ly, dll) diikuti hingga URL final, lalu re-check
// apakah URL final adalah app link. Timeout 3 detik per link.
func (c *Crawler) resolveAppLinks(appURLs []string) []string {
	resolved := make([]string, 0, len(appURLs))
	seen := make(map[string]bool)

	for _, rawURL := range appURLs {
		if seen[rawURL] {
			continue
		}

		finalURL := rawURL
		if IsShortenerURL(rawURL) {
			if dest := c.followRedirect(rawURL); dest != "" && dest != rawURL {
				log.Printf("  🔗 [AppResolve] %s → %s", rawURL, dest)
				finalURL = dest
			}
		}

		if !seen[finalURL] {
			seen[finalURL] = true
			resolved = append(resolved, finalURL)
		}
	}
	return resolved
}

// followRedirect mengikuti satu redirect HTTP dengan timeout 3 detik.
// Mengembalikan URL tujuan, atau string kosong jika gagal.
func (c *Crawler) followRedirect(rawURL string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", rawURL, nil)
	if err != nil {
		return ""
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ""
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc := resp.Header.Get("Location"); loc != "" {
			return loc
		}
	}
	return ""
}

// captureRedirectChain mengikuti redirect HTTP dan merekam semua URL dalam chain.
func (c *Crawler) captureRedirectChain(startURL string) []string {
	chain := []string{startURL}
	current := startURL

	for i := 0; i < 10; i++ {
		resp, err := c.httpClient.Head(current)
		if err != nil {
			// HEAD gagal → coba GET
			resp, err = c.httpClient.Get(current)
			if err != nil {
				break
			}
			resp.Body.Close()
		}

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			if loc == "" || loc == current {
				break
			}
			current = loc
			chain = append(chain, current)
		} else {
			break
		}
	}

	if len(chain) == 1 {
		return nil // tidak ada redirect, kembalikan nil agar response lebih bersih
	}
	return chain
}

// looksGambling: sinyal awal (sebelum scoring) yang cukup untuk menelusuri halaman internal.
func looksGambling(r *AnalysisResult) bool {
	return len(r.FoundKeywords) >= 2 || len(r.Artifacts) > 0 ||
		(r.AppPromo != nil && r.AppPromo.Detected) || len(r.NetworkSignals.GameProviders) > 0
}

// resolveRegisterLinks mengikuti redirect link pendaftaran eksternal & shortener, lalu menambahkan
// URL tujuan akhirnya. Tanpa ini, situs kasino "asli" di balik tombol DAFTAR tak pernah tercatat.
func (c *Crawler) resolveRegisterLinks(l *ExtractedLinks) {
	if l == nil {
		return
	}
	type target struct {
		url        string
		isRegister bool
	}
	var targets []target
	seen := map[string]bool{}
	for _, u := range l.Register {
		if !seen[u] && (IsShortenerURL(u) || strings.HasPrefix(u, "http")) {
			seen[u] = true
			targets = append(targets, target{u, true})
		}
	}
	for _, u := range l.External {
		if !seen[u] && IsShortenerURL(u) {
			seen[u] = true
			targets = append(targets, target{u, false})
		}
	}
	if len(targets) > 12 {
		targets = targets[:12]
	}

	finals := make([]string, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			if chain := c.captureRedirectChain(u); len(chain) > 1 {
				finals[i] = absoluteURL(chain[len(chain)-2], chain[len(chain)-1])
			}
		}(i, t.url)
	}
	wg.Wait()

	for i, f := range finals {
		if f == "" || f == targets[i].url {
			continue
		}
		log.Printf("  🔗 [Redirect] %s → %s", targets[i].url, f)
		if targets[i].isRegister || isRegisterURL(f) {
			l.Register = mergeUnique(l.Register, []string{f})
		} else {
			l.External = mergeUnique(l.External, []string{f})
		}
	}
}

// absoluteURL menyelesaikan header Location relatif terhadap URL sebelumnya.
func absoluteURL(prev, loc string) string {
	base, err := neturl.Parse(prev)
	if err != nil {
		return loc
	}
	ref, err := neturl.Parse(loc)
	if err != nil {
		return loc
	}
	return base.ResolveReference(ref).String()
}

// reLiveChatLicense: ID akun LiveChat dari URL widget (…/lc/main/19967948/… atau license_id=…)
var reLiveChatLicense = regexp.MustCompile(`(?:/lc/main/|license_id=|license=)(\d{6,10})`)

// mergeWidgetLinks memasukkan link dari pesan sambutan widget chat (LiveChat dll) ke hasil:
// diproses seperti <a href> agar APK, Telegram/WA, link pendaftaran & alternatif terklasifikasi.
func (c *Crawler) mergeWidgetLinks(result *AnalysisResult, capture *networkCapture, baseURL string) {
	links := capture.widgetLinks()
	if len(links) > 0 {
		var b strings.Builder
		for _, u := range links {
			b.WriteString(`<a href="` + u + `">` + u + "</a>\n")
		}
		synthetic := b.String()
		wl := ExtractLinks(synthetic, baseURL)
		result.Links.Apps = mergeUnique(result.Links.Apps, wl.Apps)
		result.Links.Register = mergeUnique(result.Links.Register, wl.Register)
		result.Links.External = mergeUnique(result.Links.External, wl.External)
		result.Artifacts = uniqueArtifacts(append(result.Artifacts, ExtractArtifacts(synthetic)...))
		result.WidgetLinks = links
		log.Printf("  💬 [Widget] %d link dari pesan sambutan chat: %v", len(links), links)
	}

	// Lisensi LiveChat: akun yang sama di banyak situs = operator/CS yang sama
	requests, _ := capture.snapshot()
	seen := map[string]bool{}
	for _, t := range result.TrackingIDs {
		seen[t.Platform+"|"+t.Value] = true
	}
	for _, r := range requests {
		if !strings.Contains(r, "livechat") && !strings.Contains(r, "files-text.com") {
			continue
		}
		if m := reLiveChatLicense.FindStringSubmatch(r); m != nil && !seen["LiveChat|"+m[1]] {
			seen["LiveChat|"+m[1]] = true
			result.TrackingIDs = append(result.TrackingIDs, TrackingID{Platform: "LiveChat", Value: m[1]})
		}
	}
}

var reLobbyPath = regexp.MustCompile(`(?i)/(slots?|casino|live|lobby|games?|arcade|fishing|tembak|sportsbook|sports?)(/|$)`)

// probeLobbyProviders membuka halaman lobby di tab baru dan merekam request jaringan untuk
// mendeteksi provider game (Pragmatic, PG Soft, …) yang tidak dimuat di beranda.
func (c *Crawler) probeLobbyProviders(parent context.Context, result *AnalysisResult, pageURL string) {
	var target string
	for _, l := range result.Links.Internal {
		if u, err := neturl.Parse(l); err == nil && reLobbyPath.MatchString(u.Path) {
			target = l
			break
		}
	}
	if target == "" {
		return
	}
	tabCtx, cancel := chromedp.NewContext(parent)
	defer cancel()
	tabCtx, cancelT := context.WithTimeout(tabCtx, 25*time.Second)
	defer cancelT()

	lobby := &networkCapture{}
	chromedp.ListenTarget(tabCtx, lobby.listen)
	var html string
	err := chromedp.Run(tabCtx, cdpnetwork.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, _, _, _, err := page.Navigate(target).Do(ctx)
			return err
		}),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(6*time.Second),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		log.Printf("  🎰 [Lobby] %s dilewati: %v", target, err)
		return
	}
	found := map[string]bool{}
	for _, p := range append(lobby.analyze().GameProviders, providersInHTML(html)...) {
		if !found[p] {
			found[p] = true
			result.NetworkSignals.GameProviders = append(result.NetworkSignals.GameProviders, p)
		}
	}
	result.NetworkSignals.TotalRequests += lobby.analyze().TotalRequests
	log.Printf("  🎰 [Lobby] %s → provider: %v", target, result.NetworkSignals.GameProviders)
}

// dohResolverRules: bila DNS sistem membelokkan domain ke halaman blokir ISP, kembalikan aturan
// --host-resolver-rules Chrome yang memetakan domain (dan varian www) ke IP asli dari DNS-over-HTTPS.
// Domain yang tidak diblokir tidak diubah agar perilaku crawl normal tetap sama.
func dohResolverRules(ctx context.Context, rawURL string) (rules string, blocked bool) {
	u, err := neturl.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if !netutil.ISPBlocked(host) {
		return "", false
	}
	dctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	ips, _, err := netutil.LookupDoH(dctx, host)
	if err != nil || len(ips) == 0 {
		return "", true
	}
	bare := strings.TrimPrefix(host, "www.")
	log.Printf("  🛡️  [Crawler] %s diblokir DNS ISP → dibuka lewat IP asli %s (DoH)", host, ips[0])
	return fmt.Sprintf("MAP %s %s, MAP www.%s %s", bare, ips[0], bare, ips[0]), true
}

// botBlockSignatures: penanda halaman blokir/tantangan dari layanan proteksi bot. Dicocokkan hanya
// pada halaman pendek (< 60 KB) agar situs asli yang kebetulan menyebut kata-kata ini tidak ikut tertolak.
var botBlockSignatures = []struct{ needle, reason string }{
	{"attention required! | cloudflare", "Cloudflare"},
	{"sorry, you have been blocked", "Cloudflare"},
	{"cf-error-details", "Cloudflare"},
	{"<title>just a moment...</title>", "Cloudflare challenge"},
	{"challenges.cloudflare.com/turnstile", "Cloudflare Turnstile"},
	{"cf-chl-", "Cloudflare challenge"},
	{"checking your browser before accessing", "pemeriksaan browser"},
	{"ddos protection by", "proteksi DDoS"},
	{"_incapsula_resource", "Imperva"},
	{"access denied</title>", "akses ditolak"},
}

func botBlockReason(html string) string {
	if len(html) > 60_000 {
		return ""
	}
	low := strings.ToLower(html)
	for _, s := range botBlockSignatures {
		if strings.Contains(low, s.needle) {
			return s.reason
		}
	}
	return ""
}
