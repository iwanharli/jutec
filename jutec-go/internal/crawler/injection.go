package crawler

import (
	"context"
	"io"
	"net/http"
	"net/url"
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
	HiddenLinks    int      `json:"hidden_links"`             // link judol tersembunyi ke domain lain
	JudolLinks     int      `json:"judol_links"`              // semua link judol ke domain lain
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
	if inj.HiddenLinks >= 3 && float64(inj.HiddenLinks) >= 0.6*float64(inj.JudolLinks) {
		inj.Signals = append(inj.Signals, "hidden_links")
	}
	inj.Detected = len(inj.Signals) > 0
	return inj
}
