package crawler

import (
	"context"
	"fmt"
	"golang.org/x/net/publicsuffix"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"jutec-go/internal/netutil"

	"github.com/chromedp/chromedp"
)

// Injection: tanda situs sah yang DISUSUPI konten judol (bukan situs judol milik operator).
//   - cloaking: konten judol hanya disajikan ke Googlebot / pengunjung dari Google
//   - hidden_links: link judol ke domain lain disembunyikan dari pengunjung (SEO spam)
type Injection struct {
	Detected       bool     `json:"detected"`
	Signals        []string `json:"signals"`
	Cloaking       bool     `json:"cloaking"`
	CloakVia       string   `json:"cloak_via,omitempty"` // googlebot | referer
	CloakKeywords  []string `json:"cloak_keywords,omitempty"`
	CloakRedirect  string   `json:"cloak_redirect,omitempty"` // tujuan redirect yang hanya muncul untuk Googlebot/referer Google
	CloakTitle     string   `json:"cloak_title,omitempty"`    // judul halaman versi tersembunyi (biasanya memuat merek, mis. "ROKOKBET | …")
	CloakBrand     string   `json:"cloak_brand,omitempty"`    // merek judol yang dipromosikan
	CloakTargets   []string `json:"cloak_targets,omitempty"`  // URL judol (domain / pemendek URL) dari versi tersembunyi — kandidat situs operator
	ParentURL      string   `json:"parent_url,omitempty"`     // parasite SEO: beranda domain induk yang bersih
	ParentTitle    string   `json:"parent_title,omitempty"`   // judul beranda induk (identitas pemilik sah)
	SiblingHosts   []string `json:"sibling_hosts,omitempty"`  // subdomain saudara bernama judol yang terdaftar di DNS (gacor-1…N)
	SubdomainAbuse bool     `json:"-"`
	HiddenLinks    int      `json:"hidden_links"` // link judol tersembunyi ke domain lain
	JudolLinks     int      `json:"judol_links"`  // semua link judol ke domain lain
	HiddenSamples  []string `json:"hidden_samples,omitempty"`
	HiddenHosts    []string `json:"hidden_hosts,omitempty"`
}

// hiddenSpamJS menghitung link bertema judol ke domain lain dan berapa yang disembunyikan dari pengunjung
// (display:none, visibility, opacity 0, font sangat kecil, digeser jauh keluar layar, kotak 0×0).
const hiddenSpamJS = `(() => {
  const re = /slot|gacor|togel|toto|casino|kasino|judi|maxwin|scatter|bandar|poker|sbobet|pragmatic|mahjong|zeus|olympus|sabung|baccarat|roulette/i;
  const hidden = el => {
    for (let e = el; e && e !== document.documentElement; e = e.parentElement) {
      const s = getComputedStyle(e);
      if (s.display === 'none' || s.visibility === 'hidden' || parseFloat(s.opacity) === 0 || parseFloat(s.fontSize) < 2) return true;
      if ((s.position === 'absolute' || s.position === 'fixed') && (parseFloat(s.left) < -500 || parseFloat(s.top) < -500 || parseFloat(s.textIndent) < -500)) return true;
      if (parseFloat(s.textIndent) < -500) return true;
      if (s.overflow === 'hidden' && (e.clientHeight <= 2 || e.clientWidth <= 2)) return true;
    }
    return false;
  };
  const host = location.hostname.replace(/^www\./, '');
  let judol = 0; const hid = []; const hosts = new Set();
  for (const a of document.querySelectorAll('a[href]')) {
    let h = ''; try { h = new URL(a.href).hostname.replace(/^www\./, ''); } catch (e) { continue; }
    if (!h || h === host) continue;
    const t = ((a.textContent || '') + ' ' + a.href);
    if (!re.test(t)) continue;
    judol++;
    if (hidden(a)) { hosts.add(h); if (hid.length < 12) hid.push(((a.textContent || '').trim().slice(0, 60) || '(tanpa teks)') + ' → ' + a.href.slice(0, 140)); }
  }
  return { judol, hidden: [...hosts].length ? hid.length : 0, samples: hid, hosts: [...hosts].slice(0, 20) };
})()`

type hiddenSpamResult struct {
	Judol   int      `json:"judol"`
	Hidden  int      `json:"hidden"`
	Samples []string `json:"samples"`
	Hosts   []string `json:"hosts"`
}

// detectHiddenSpam dijalankan di tab yang halamannya sudah dimuat.
func detectHiddenSpam(ctx context.Context) *hiddenSpamResult {
	var r hiddenSpamResult
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := chromedp.Run(cctx, chromedp.Evaluate(hiddenSpamJS, &r)); err != nil {
		return nil
	}
	return &r
}

const googlebotUA = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

// fetchAs mengambil HTML mentah dengan UA/referer tertentu. Mengembalikan HTML & URL akhir.
func fetchAs(ctx context.Context, rawURL, ua, referer string) (string, string) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	res, err := netutil.HTTPClient(15 * time.Second).Do(req)
	if err != nil {
		return "", ""
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return "", res.Request.URL.String()
	}
	b, _ := io.ReadAll(io.LimitReader(res.Body, 3<<20))
	return string(b), res.Request.URL.String()
}

// checkCloaking membandingkan halaman versi pengunjung (sudah dianalisis) dengan versi untuk
// Googlebot dan pengunjung dari Google. Bila hanya versi "Google" yang berisi judol → cloaking.
// Mengembalikan analisis versi cloaked (untuk digabung ke hasil) atau nil.
func (c *Crawler) checkCloaking(ctx context.Context, rawURL string, base *AnalysisResult, kws []WeightedKeyword) (*Injection, *AnalysisResult) {
	baseKW := len(base.FoundKeywords)
	baseHost := hostOnly(rawURL)
	for _, v := range []struct{ via, ua, ref string }{
		{"googlebot", googlebotUA, ""},
		{"referer", "Mozilla/5.0 (Linux; Android 13; SM-A536E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36", "https://www.google.com/"},
	} {
		htmlText, final := fetchAs(ctx, rawURL, v.ua, v.ref)
		inj := &Injection{CloakVia: v.via}
		// Redirect ke domain lain yang hanya terjadi untuk Googlebot/pengunjung Google (sneaky redirect)
		if fh := hostOnly(final); fh != "" && baseHost != "" && fh != baseHost && !strings.HasSuffix(fh, "."+baseHost) && !strings.HasSuffix(baseHost, "."+fh) {
			inj.CloakRedirect = final
		}
		if htmlText == "" {
			if inj.CloakRedirect != "" {
				inj.Cloaking = true
				return inj, nil
			}
			continue
		}
		cloaked := c.Analyzer.Analyze(htmlText, kws, nil)
		inj.CloakTitle, inj.CloakBrand = cloakTitleBrand(htmlText)
		inj.CloakTargets = cloakTargets(htmlText, final, baseHost)
		// Tombol DAFTAR sering ada di halaman AMP/login sisipan lain di domain korban — ikuti 1 lompatan
		for _, u := range cloakInternalHops(htmlText, final, baseHost) {
			if hopHTML, hopFinal := fetchAs(ctx, u, v.ua, v.ref); hopHTML != "" {
				inj.CloakTargets = mergeUnique(inj.CloakTargets, cloakTargets(hopHTML, hopFinal, baseHost))
			}
		}
		// Versi "Google" berisi ≥2 kata kunci judol lebih banyak, sementara versi pengunjung (hampir) bersih
		if len(cloaked.FoundKeywords) >= 2 && baseKW <= 1 && len(cloaked.FoundKeywords) >= baseKW+2 {
			inj.Cloaking = true
			inj.CloakKeywords = cloaked.FoundKeywords
			if len(inj.CloakKeywords) > 15 {
				inj.CloakKeywords = inj.CloakKeywords[:15]
			}
			return inj, cloaked
		}
		if inj.CloakRedirect != "" && cloaked.Confidence >= 0.4 {
			inj.Cloaking = true
			inj.CloakKeywords = cloaked.FoundKeywords
			return inj, cloaked
		}
	}
	return nil, nil
}

var (
	reTitleTag   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reJudolHost  = regexp.MustCompile(`(?i)slot|gacor|toto|togel|bet|casino|kasino|judi|poker|4d|88|303|777|maxwin|jp|hoki|win|bola|sbo|qq|domino|lotto`)
	reBrandSplit = regexp.MustCompile(`\s*[|:\-–—]\s*`)
)

// cloakTitleBrand: judul versi tersembunyi + merek (bagian judul berhuruf kapital / sebelum "|").
func cloakTitleBrand(page string) (title, brand string) {
	m := reTitleTag.FindStringSubmatch(page)
	if m == nil {
		return "", ""
	}
	title = strings.TrimSpace(html.UnescapeString(m[1]))
	if len(title) > 200 {
		title = title[:200]
	}
	parts := reBrandSplit.Split(title, -1)
	// Merek biasanya ditulis kapital ("ROKOKBET | …"), atau satu kata di depan judul ("Indo303 - …")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 3 && len(p) <= 30 && p == strings.ToUpper(p) && strings.ContainsAny(p, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			return title, p
		}
	}
	if p := strings.TrimSpace(parts[0]); len(p) >= 3 && len(p) <= 30 && !strings.Contains(p, " ") {
		return title, p
	}
	return title, ""
}

// cloakTargets: tujuan judol dari versi tersembunyi — pemendek URL (utuh, diselesaikan browser saat scan)
// dan domain judol yang ditautkan (bukan domain korban, bukan CDN/umum).
func cloakTargets(page, pageURL, selfHost string) []string {
	links := ExtractLinks(page, pageURL)
	seen := map[string]bool{}
	var out []string
	add := func(raw string, force bool) {
		h := hostOnly(raw)
		if h == "" || h == selfHost || strings.HasSuffix(h, "."+selfHost) || isCommonHost(h) {
			return
		}
		target := "https://" + h
		if IsShortenerURL(raw) {
			target = raw // slug pemendek = identitas tujuan
		} else if !force && !reJudolHost.MatchString(h) {
			return
		}
		if !seen[target] {
			seen[target] = true
			out = append(out, target)
		}
	}
	for _, l := range links.Register {
		add(l, true)
	}
	for _, l := range append(append([]string{}, links.External...), links.Alternate...) {
		add(l, false)
	}
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

var reInternalHop = regexp.MustCompile(`(?i)<link[^>]+rel=["']amphtml["'][^>]*href=["']([^"']+)["']|<a[^>]+class=["'][^"']*(?:login|daftar|register)[^"']*["'][^>]*href=["']([^"']+)["']|<a[^>]+href=["']([^"']+)["'][^>]*class=["'][^"']*(?:login|daftar|register)[^"']*["']`)

// cloakInternalHops: halaman AMP/login sisipan lain di domain korban (maks. 2).
func cloakInternalHops(page, pageURL, selfHost string) []string {
	var out []string
	seen := map[string]bool{strings.TrimRight(pageURL, "/"): true}
	for _, m := range reInternalHop.FindAllStringSubmatch(page, -1) {
		raw := m[1] + m[2] + m[3]
		u := resolveURL(raw, pageURL)
		if u == "" || hostOnly(u) != selfHost || seen[strings.TrimRight(u, "/")] {
			continue
		}
		seen[strings.TrimRight(u, "/")] = true
		out = append(out, u)
		if len(out) == 2 {
			break
		}
	}
	return out
}

func hostOnly(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// finalizeInjection menyusun sinyal. Link tersembunyi hanya dihitung penyusupan bila jelas spam:
// ≥3 link judol tersembunyi ke domain lain dan sebagian besar link judol memang disembunyikan
// (menu dropdown situs judol asli juga tersembunyi, tapi mengarah ke domainnya sendiri).
func finalizeInjection(inj *Injection, hs *hiddenSpamResult) *Injection {
	if hs != nil && hs.Judol > 0 {
		if inj == nil {
			inj = &Injection{}
		}
		inj.JudolLinks, inj.HiddenLinks, inj.HiddenSamples, inj.HiddenHosts = hs.Judol, hs.Hidden, hs.Samples, hs.Hosts
	}
	if inj == nil {
		return nil
	}
	if inj.Cloaking {
		inj.Signals = append(inj.Signals, "cloaking")
	}
	if inj.ParentURL != "" && !inj.SubdomainAbuse {
		inj.Signals = append(inj.Signals, "parasite")
	}
	if inj.SubdomainAbuse {
		inj.Signals = append(inj.Signals, "subdomain_abuse")
	}
	if inj.HiddenLinks >= 3 && float64(inj.HiddenLinks) >= 0.6*float64(inj.JudolLinks) {
		inj.Signals = append(inj.Signals, "hidden_links")
	}
	inj.Detected = len(inj.Signals) > 0
	return inj
}

// isCommonHost: domain umum (CDN, media sosial, mesin pencari, platform blog) yang bukan situs judol.
func isCommonHost(h string) bool {
	for _, d := range []string{"google.", "gstatic.com", "googleusercontent.com", "googleapis.com", "facebook.com", "fb.com", "twitter.com", "x.com",
		"instagram.com", "youtube.com", "tiktok.com", "linkedin.com", "cloudflare.com", "cloudflareinsights.com", "jsdelivr.net", "schema.org",
		"w3.org", "wordpress.", "wp.com", "blogger.com", "blogspot.com", "shopify.com", "wa.me", "whatsapp.com", "t.me", "telegram.", "apple.com",
		"microsoft.com", "bing.com", "yahoo.com", "wikipedia.org", "gravatar.com", "fontawesome.com", "bootstrapcdn.com", "jquery.com"} {
		if strings.Contains(h, d) {
			return true
		}
	}
	return false
}

// DetectParasite: halaman judol di subdomain/path domain sah yang berandanya bersih (parasite SEO,
// mis. nenekcantik.tokosepatujambi.online, cctv.dishubmaros.com, edimancoro.or.id/brosur).
// Domain platform hosting gratis (web.app, pages.dev, github.io…) tercakup daftar suffix publik,
// sehingga subdomainnya dianggap domain sendiri dan tidak diperiksa sebagai parasit.
func (c *Crawler) DetectParasite(ctx context.Context, pageURL string, kws []WeightedKeyword) *Injection {
	u, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	root, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil || reJudolHost.MatchString(strings.Split(root, ".")[0]) {
		return nil // domain induk sendiri bernama judol → milik operator, bukan korban
	}
	onSub := host != root
	onPath := strings.Trim(u.Path, "/") != ""
	if !onSub && !onPath {
		return nil
	}
	homeHTML, final := fetchAs(ctx, "https://"+root+"/", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "")
	if homeHTML == "" || botBlockReason(homeHTML) != "" {
		return nil // beranda tak terbaca — tidak bisa memastikan
	}
	if fh := hostOnly(final); fh != "" && fh != root && !strings.HasSuffix(fh, "."+root) {
		return nil // beranda dialihkan ke domain lain (mis. ke situs judol) — bukan pola parasit
	}
	home := c.Analyzer.Analyze(homeHTML, kws, nil)
	if len(home.FoundKeywords) > 1 {
		return nil // beranda juga berisi judi
	}
	title, _ := cloakTitleBrand(homeHTML)
	return &Injection{Signals: []string{"parasite"}, ParentURL: "https://" + root, ParentTitle: title}
}

var reNumberedLabel = regexp.MustCompile(`^(.*?[a-z])(-?)(\d{1,3})$`)

// DetectSubdomainAbuse: subdomain bernama judol (gacor-6.toko.com) di domain induk bernama sah, sementara
// halamannya sendiri tidak menampilkan judi — pola DNS/akun hosting dibobol lalu ditambahi subdomain judol.
// Subdomain saudara bernomor (gacor-1…20) dicek lewat DNS; yang masih menampilkan judol jadi target scan.
func (c *Crawler) DetectSubdomainAbuse(ctx context.Context, pageURL string, r *AnalysisResult, kws []WeightedKeyword) *Injection {
	if len(r.FoundKeywords) > 0 {
		return nil // halaman berisi judi — ditangani analisis biasa / parasite SEO
	}
	u, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	root, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil || host == root {
		return nil
	}
	label := strings.TrimSuffix(host, "."+root)
	if !reJudolHost.MatchString(label) || reJudolHost.MatchString(strings.Split(root, ".")[0]) {
		return nil
	}
	inj := &Injection{SubdomainAbuse: true, ParentURL: "https://" + root}
	if homeHTML, _ := fetchAs(ctx, "https://"+root+"/", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", ""); homeHTML != "" {
		inj.ParentTitle, _ = cloakTitleBrand(homeHTML)
	}
	// Saudara bernomor: gacor-6 → gacor-1…20
	if m := reNumberedLabel.FindStringSubmatch(label); m != nil {
		for i := 1; i <= 20; i++ {
			sib := fmt.Sprintf("%s%s%d.%s", m[1], m[2], i, root)
			if sib == host {
				continue
			}
			ips, _, err := netutil.LookupDoH(ctx, sib)
			if err != nil || len(ips) == 0 {
				continue
			}
			inj.SiblingHosts = append(inj.SiblingHosts, sib)
			if page, _ := fetchAs(ctx, "https://"+sib+"/", googlebotUA, ""); page != "" {
				if a := c.Analyzer.Analyze(page, kws, nil); len(a.FoundKeywords) >= 2 {
					inj.CloakTargets = append(inj.CloakTargets, "https://"+sib)
				}
			}
		}
	}
	return inj
}
