package ipintel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"jutec-go/internal/validin"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"jutec-go/internal/netutil"
)

// OriginCandidate: IP non-CDN yang terhubung dengan domain — kandidat server asli di balik Cloudflare.
type OriginCandidate struct {
	IP       string   `json:"ip"`
	CDN      bool     `json:"cdn"`      // true = masih IP CDN (bukan kandidat origin)
	Evidence []string `json:"evidence"` // mis. "A mail.x.com", "MX → mx1.x.com", "URLScan 2025-03-02"
	Sources  []string `json:"sources"`  // subdomain | mx | urlscan | dns_history | apex
	LastSeen string   `json:"last_seen,omitempty"`
}

type OriginReport struct {
	Domain     string            `json:"domain"`
	BehindCDN  bool              `json:"behind_cdn"`
	Subdomains []string          `json:"subdomains"` // subdomain yang diperiksa (dari CT log + umum)
	Candidates []OriginCandidate `json:"candidates"` // non-CDN, urut bukti terbanyak
	CDNIPs     []string          `json:"cdn_ips"`    // IP CDN yang teramati
	Notes      []string          `json:"notes"`
	CheckedAt  time.Time         `json:"checked_at"`
}

var commonSubs = []string{"direct", "origin", "mail", "webmail", "smtp", "ftp", "cpanel", "whm", "server", "panel", "admin",
	"api", "dev", "staging", "backend", "m", "wap", "app", "old", "ns1", "ns2", "vpn", "remote", "db"}

// FindOrigin mencari kandidat IP asli domain. urlscanKey boleh kosong (sumber URLScan dilewati).
func FindOrigin(ctx context.Context, domain, urlscanKey string) (*OriginReport, error) {
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), "www.")
	if domain == "" || strings.ContainsAny(domain, "/ ") {
		return nil, fmt.Errorf("domain tidak valid")
	}
	rep := &OriginReport{Domain: domain, CheckedAt: time.Now().UTC(), Subdomains: []string{}, Candidates: []OriginCandidate{}, CDNIPs: []string{}, Notes: []string{}}

	type hit struct{ evidence, source, seen string }
	var mu sync.Mutex
	hits := map[string][]hit{}
	add := func(ip, evidence, source, seen string) {
		mu.Lock()
		hits[ip] = append(hits[ip], hit{evidence, source, seen})
		mu.Unlock()
	}

	// 1. Daftar host: apex, www, subdomain dari CT log, subdomain umum
	hosts := map[string]bool{domain: true, "www." + domain: true}
	ct, err := ctSubdomains(ctx, domain)
	if err != nil {
		rep.Notes = append(rep.Notes, "CT log (crt.sh) tidak bisa diakses: "+err.Error())
	}
	for _, h := range ct {
		hosts[h] = true
	}
	for _, s := range commonSubs {
		hosts[s+"."+domain] = true
	}
	list := make([]string, 0, len(hosts))
	for h := range hosts {
		list = append(list, h)
	}
	sort.Strings(list)
	if len(list) > 120 {
		list = list[:120]
	}

	// 2. Resolusi A/AAAA (via DoH — tak terpengaruh blokir DNS ISP)
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, h := range list {
		wg.Add(1)
		sem <- struct{}{}
		go func(h string) {
			defer wg.Done()
			defer func() { <-sem }()
			found := false
			for _, t := range []string{"A", "AAAA"} {
				recs, _ := netutil.LookupDoHType(ctx, h, t)
				for _, ip := range recs {
					if net.ParseIP(ip) != nil {
						src := "subdomain"
						if h == domain || h == "www."+domain {
							src = "apex"
						}
						add(ip, t+" "+h, src, "")
						found = true
					}
				}
			}
			if found {
				mu.Lock()
				rep.Subdomains = append(rep.Subdomains, h)
				mu.Unlock()
			}
		}(h)
	}
	wg.Wait()

	// 3. MX → IP server email
	if mx, err := netutil.LookupDoHType(ctx, domain, "MX"); err == nil {
		for _, rec := range mx {
			f := strings.Fields(rec)
			host := strings.TrimSuffix(f[len(f)-1], ".")
			if host == "" {
				continue
			}
			ips, _ := netutil.LookupDoHType(ctx, host, "A")
			for _, ip := range ips {
				add(ip, "MX → "+host, "mx", "")
			}
		}
	}

	// 4. Riwayat IP dari URLScan (scan lama sebelum memakai CDN)
	if urlscanKey != "" {
		if hist, err := urlscanHistory(ctx, domain, urlscanKey); err != nil {
			rep.Notes = append(rep.Notes, "Riwayat URLScan tidak tersedia: "+err.Error())
		} else {
			for _, h := range hist {
				add(h.ip, "URLScan "+h.seen[:10]+" ("+h.host+")", "urlscan", h.seen)
			}
		}
	}

	// 4b. Riwayat DNS dari Validin: IP yang dipakai domain sebelum pindah ke CDN
	if validin.Configured() {
		if hist, err := validin.AHistory(domain); err != nil {
			rep.Notes = append(rep.Notes, "Riwayat DNS Validin tidak tersedia: "+err.Error())
		} else {
			for _, h := range hist {
				seen := ""
				if h.LastSeen > 0 {
					seen = time.Unix(h.LastSeen, 0).UTC().Format(time.RFC3339)
				}
				ev := "Riwayat DNS (Validin)"
				if seen != "" {
					ev += " s.d. " + seen[:10]
				}
				add(h.IP, ev, "dns_history", seen)
			}
		}
	}

	// 5. Pisahkan IP CDN & kandidat origin
	for ip, hs := range hits {
		c := OriginCandidate{IP: ip, CDN: IsCloudflare(net.ParseIP(ip))}
		seenEv, seenSrc := map[string]bool{}, map[string]bool{}
		for _, h := range hs {
			if !seenEv[h.evidence] {
				seenEv[h.evidence] = true
				c.Evidence = append(c.Evidence, h.evidence)
			}
			if !seenSrc[h.source] {
				seenSrc[h.source] = true
				c.Sources = append(c.Sources, h.source)
			}
			if h.seen > c.LastSeen {
				c.LastSeen = h.seen
			}
		}
		sort.Strings(c.Evidence)
		if c.CDN {
			rep.CDNIPs = append(rep.CDNIPs, ip)
			for _, s := range c.Sources {
				if s == "apex" {
					rep.BehindCDN = true
				}
			}
			continue
		}
		rep.Candidates = append(rep.Candidates, c)
	}
	sort.Strings(rep.CDNIPs)
	sort.Strings(rep.Subdomains)
	sort.Slice(rep.Candidates, func(i, j int) bool {
		if len(rep.Candidates[i].Evidence) != len(rep.Candidates[j].Evidence) {
			return len(rep.Candidates[i].Evidence) > len(rep.Candidates[j].Evidence)
		}
		return rep.Candidates[i].IP < rep.Candidates[j].IP
	})
	if rep.BehindCDN && len(rep.Candidates) == 0 {
		rep.Notes = append(rep.Notes, "Situs di balik Cloudflare dan belum ada IP asli yang bocor lewat subdomain, MX, atau riwayat URLScan.")
	}
	return rep, nil
}

// ctSubdomains: nama host *.domain dari sertifikat di Certificate Transparency log.
func ctSubdomains(ctx context.Context, domain string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://crt.sh/?q="+url.QueryEscape("%."+domain)+"&output=json", nil)
	req.Header.Set("User-Agent", "jutec-ipintel/1")
	resp, err := (&http.Client{Timeout: 40 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var entries []struct {
		NameValue string `json:"name_value"`
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, e := range entries {
		for _, n := range strings.Split(e.NameValue, "\n") {
			n = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(n)), "*.")
			if (n == domain || strings.HasSuffix(n, "."+domain)) && !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out, nil
}

type urlscanHit struct{ ip, host, seen string }

func urlscanHistory(ctx context.Context, domain, key string) ([]urlscanHit, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://urlscan.io/api/v1/search/?size=100&q="+url.QueryEscape("domain:"+domain), nil)
	req.Header.Set("API-Key", key)
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var r struct {
		Results []struct {
			Task struct{ Time string }       `json:"task"`
			Page struct{ IP, Domain string } `json:"page"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	var out []urlscanHit
	for _, x := range r.Results {
		h := strings.ToLower(x.Page.Domain)
		if x.Page.IP == "" || len(x.Task.Time) < 10 || !(h == domain || strings.HasSuffix(h, "."+domain)) {
			continue
		}
		out = append(out, urlscanHit{ip: x.Page.IP, host: h, seen: x.Task.Time})
	}
	return out, nil
}
