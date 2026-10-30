package crawler

import (
	"regexp"
	"strings"
	"sync"

	cdpnetwork "github.com/chromedp/cdproto/network"
)

// gameProviders maps known domain substrings to provider/platform names.
// Mencakup: game providers internasional + platform hosting judol Indonesia.
var gameProviders = map[string]string{
	// Game Providers
	"pragmaticplay.com":               "Pragmatic Play",
	"pragmatic-external-platform.com": "Pragmatic Play",
	"pg-api.com":                      "PG Soft",
	"pgsoft.com":                      "PG Soft",
	"pgsoftgaming.com":                "PG Soft",
	"habanero.com":                    "Habanero",
	"evolutiongaming.com":             "Evolution Gaming",
	"evolution.com":                   "Evolution Gaming",
	"netent.com":                      "NetEnt",
	"microgaming.co.uk":               "Microgaming",
	"playtech.com":                    "Playtech",
	"spadegaming.com":                 "Spade Gaming",
	"cq9gaming.com":                   "CQ9",
	"jdb168.com":                      "JDB",
	"fastspin.com":                    "Fastspin",
	"simpleplay.net":                  "Simple Play",
	"nextspin.io":                     "Nextspin",
	"onetouch.io":                     "OneTouch",
	"relax-gaming.com":                "Relax Gaming",
	"hacksaw-gaming.com":              "Hacksaw Gaming",
	// Judol Hosting Platforms (Indonesia)
	// AsiaSigma: platform builder judol, UUID unik per operator di CDN URL
	"cloudstoragesigma02.com": "AsiaSigma Platform",
	"cloudstoragesigma.com":   "AsiaSigma Platform",
	// GigaGaming: platform builder judol, brand code unik per operator
	"sitestatic.net":       "GigaGaming Platform",
	"cdn.sitestatic.net":   "GigaGaming Platform",
	"files.sitestatic.net": "GigaGaming Platform",
}

// NetworkSignals berisi hasil intercept request browser selama crawl.
type NetworkSignals struct {
	GameProviders      []string `json:"game_providers"`
	TotalRequests      int      `json:"total_requests"`
	SuspiciousRequests int      `json:"suspicious_requests"`
}

// networkCapture mengumpulkan semua URL request yang terjadi selama navigasi.
type networkCapture struct {
	mu       sync.Mutex
	requests []string
	apiCalls []string // URL request XHR/Fetch/WebSocket — backend API yang dipakai situs
	wsURLs   []string // URL di dalam pesan WebSocket (mis. tombol sambutan widget LiveChat)
}

// reWSURL: URL di payload JSON WebSocket (bisa ter-escape: https:\/\/…)
var reWSURL = regexp.MustCompile(`https?:\\?/\\?/[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}[^"'\s\\<>]*`)

// widgetNoise: aset internal penyedia widget, bukan link milik operator
var widgetNoise = regexp.MustCompile(`(?i)livechatinc\.com|files-text\.com|text\.com/|tawk\.to|crisp\.chat|gstatic|googleapis|cloudflare`)

func (nc *networkCapture) listen(ev interface{}) {
	if e, ok := ev.(*cdpnetwork.EventWebSocketFrameReceived); ok {
		nc.mu.Lock()
		for _, u := range reWSURL.FindAllString(e.Response.PayloadData, -1) {
			u = strings.ReplaceAll(u, `\/`, "/")
			if !widgetNoise.MatchString(u) {
				nc.wsURLs = append(nc.wsURLs, u)
			}
		}
		nc.mu.Unlock()
		return
	}
	if e, ok := ev.(*cdpnetwork.EventRequestWillBeSent); ok {
		nc.mu.Lock()
		nc.requests = append(nc.requests, e.Request.URL)
		switch e.Type {
		case cdpnetwork.ResourceTypeXHR, cdpnetwork.ResourceTypeFetch, cdpnetwork.ResourceTypeWebSocket, cdpnetwork.ResourceTypeEventSource:
			nc.apiCalls = append(nc.apiCalls, e.Request.URL)
		}
		nc.mu.Unlock()
	}
}

func (nc *networkCapture) analyze() *NetworkSignals {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	found := make(map[string]bool)
	suspicious := 0

	for _, reqURL := range nc.requests {
		lower := strings.ToLower(reqURL)
		for domain, name := range gameProviders {
			if strings.Contains(lower, domain) {
				if !found[name] {
					found[name] = true
				}
				suspicious++
				break
			}
		}
	}

	providers := make([]string, 0, len(found))
	for name := range found {
		providers = append(providers, name)
	}

	return &NetworkSignals{
		GameProviders:      providers,
		TotalRequests:      len(nc.requests),
		SuspiciousRequests: suspicious,
	}
}

// widgetLinks mengembalikan URL unik dari pesan WebSocket widget.
func (nc *networkCapture) widgetLinks() []string {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return mergeUnique(nil, nc.wsURLs)
}

// hasChatWidget: situs memuat widget chat yang biasa mengirim pesan sambutan berisi link.
func (nc *networkCapture) hasChatWidget() bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	for _, r := range nc.requests {
		if strings.Contains(r, "livechatinc.com") || strings.Contains(r, "tawk.to") || strings.Contains(r, "crisp.chat") {
			return true
		}
	}
	return false
}

// snapshot mengembalikan salinan semua request & API call (aman dipanggil setelah crawl).
func (nc *networkCapture) snapshot() (requests, apiCalls []string) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return append([]string(nil), nc.requests...), append([]string(nil), nc.apiCalls...)
}

// providersInHTML mendeteksi provider game dari URL yang tertulis di HTML (script/img/iframe),
// untuk halaman yang diambil tanpa browser (tak ada network capture).
func providersInHTML(html string) []string {
	lower := strings.ToLower(html)
	found := map[string]bool{}
	var out []string
	for domain, name := range gameProviders {
		if !found[name] && strings.Contains(lower, domain) {
			found[name] = true
			out = append(out, name)
		}
	}
	return out
}
