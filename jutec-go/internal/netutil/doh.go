// Package netutil: resolusi DNS lewat DNS-over-HTTPS (DoH) agar unduhan tidak tertipu halaman blokir
// DNS ISP (Internet Positif / TrustPositif), plus diagnosis kegagalan yang bisa dibaca manusia.
package netutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var dohClient = &http.Client{Timeout: 8 * time.Second}

// ispBlockMarkers: nama host halaman blokir DNS ISP Indonesia
var ispBlockMarkers = []string{"internetpositif", "trustpositif", "nawala", "uzone.id", "blokir", "aduankonten"}

// LookupDoH mengembalikan IPv4 domain dari Cloudflare DoH. nx=true bila domain tidak punya data DNS (NXDOMAIN).
func LookupDoH(ctx context.Context, host string) (ips []string, nx bool, err error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		"https://cloudflare-dns.com/dns-query?type=A&name="+url.QueryEscape(host), nil)
	req.Header.Set("Accept", "application/dns-json")
	resp, err := dohClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	var out struct {
		Status int
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, false, err
	}
	for _, a := range out.Answer {
		if a.Type == 1 {
			ips = append(ips, a.Data)
		}
	}
	return ips, out.Status == 3, nil
}

// ISPBlocked: DNS sistem (milik ISP) membelokkan domain ke halaman blokir.
func ISPBlocked(host string) bool {
	if cname, err := net.LookupCNAME(host); err == nil && hasBlockMarker(cname) {
		return true
	}
	addrs, _ := net.LookupHost(host)
	for _, a := range addrs {
		if names, err := net.LookupAddr(a); err == nil {
			for _, n := range names {
				if hasBlockMarker(n) {
					return true
				}
			}
		}
	}
	return false
}

func hasBlockMarker(s string) bool {
	s = strings.ToLower(s)
	for _, m := range ispBlockMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// HTTPClient: koneksi memakai IP dari DoH (bukan DNS ISP); SNI/Host tetap nama domain aslinya.
func HTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil || net.ParseIP(host) != nil {
			return dialer.DialContext(ctx, network, addr)
		}
		ips, _, derr := LookupDoH(ctx, host)
		if derr != nil || len(ips) == 0 {
			return dialer.DialContext(ctx, network, addr) // DoH gagal → DNS biasa
		}
		var lastErr error
		for _, ip := range ips {
			c, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
			if err == nil {
				return c, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
	return &http.Client{Timeout: timeout, Transport: tr}
}

// Diagnose menerjemahkan kegagalan unduhan menjadi penjelasan yang bisa ditindaklanjuti.
func Diagnose(rawURL string, err error, status int) string {
	u, perr := url.Parse(rawURL)
	if perr != nil {
		return "URL tidak valid"
	}
	host := u.Hostname()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ips, nx, derr := LookupDoH(ctx, host)
	blocked := ISPBlocked(host)
	switch {
	case derr == nil && nx:
		return fmt.Sprintf("domain %s sudah tidak aktif (DNS kosong/NXDOMAIN) — kemungkinan dicopot operator%s",
			host, map[bool]string{true: "; juga ada di daftar blokir ISP (Internet Positif)", false: ""}[blocked])
	case status == 520 || status == 521 || status == 522 || status == 523 || status == 524:
		return fmt.Sprintf("server asal APK sedang mati/tidak merespons (Cloudflare %d) — dicoba lagi pada analisis berikutnya", status)
	case status >= 500:
		return fmt.Sprintf("server APK error (%d) — dicoba lagi pada analisis berikutnya", status)
	case status == 404 || status == 410:
		return fmt.Sprintf("file APK sudah dihapus dari server (%d)", status)
	case status == 403:
		return "akses ke file APK ditolak server (403) — mungkin dibatasi wilayah/perangkat"
	case blocked && len(ips) == 0:
		return "domain diblokir ISP (Internet Positif) dan IP aslinya tidak ditemukan"
	case err != nil && errors.Is(err, context.DeadlineExceeded) || err != nil && strings.Contains(err.Error(), "timeout"):
		return "server APK tidak merespons (timeout) — dicoba lagi pada analisis berikutnya"
	case err != nil:
		return err.Error()
	}
	return fmt.Sprintf("status %d", status)
}

// LookupDoHType mengembalikan data record DNS bertipe qtype (A, AAAA, MX, NS, TXT, CNAME) via Cloudflare DoH.
// Untuk MX, data berbentuk "10 mail.example.com." — pemanggil memotong prioritasnya.
func LookupDoHType(ctx context.Context, host, qtype string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		"https://cloudflare-dns.com/dns-query?type="+url.QueryEscape(qtype)+"&name="+url.QueryEscape(host), nil)
	req.Header.Set("Accept", "application/dns-json")
	resp, err := dohClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	want := map[string]int{"A": 1, "NS": 2, "CNAME": 5, "MX": 15, "TXT": 16, "AAAA": 28}[qtype]
	var res []string
	for _, a := range out.Answer {
		if a.Type == want {
			res = append(res, a.Data)
		}
	}
	return res, nil
}

// Reach: hasil pengecekan keterjangkauan sebuah situs.
type Reach struct {
	Status     string // online | blocked_alive | blocked | offline | nxdomain
	HTTPStatus int
	ISPBlocked bool
	NXDomain   bool
	Error      string
}

// CheckReach memeriksa DNS publik (DoH), DNS ISP (pengalihan ke halaman blokir), lalu HTTP via IP dari DoH.
// "blocked_alive" = diblokir DNS ISP tetapi servernya masih merespons (masih beroperasi lewat VPN/DNS lain).
func CheckReach(ctx context.Context, rawURL string) Reach {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return Reach{Status: "offline", Error: "URL tidak valid"}
	}
	host := u.Hostname()
	r := Reach{}
	if net.ParseIP(host) == nil {
		ips, nx, err := LookupDoH(ctx, host)
		if nx {
			return Reach{Status: "nxdomain", NXDomain: true}
		}
		if err == nil && len(ips) == 0 {
			if aaaa, _ := LookupDoHType(ctx, host, "AAAA"); len(aaaa) == 0 {
				return Reach{Status: "nxdomain", NXDomain: true}
			}
		}
		r.ISPBlocked = ISPBlocked(host)
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 Chrome/120 Mobile Safari/537.36")
	resp, err := HTTPClient(20 * time.Second).Do(req)
	alive := false
	if err != nil {
		r.Error = err.Error()
		if len(r.Error) > 200 {
			r.Error = r.Error[:200]
		}
	} else {
		r.HTTPStatus = resp.StatusCode
		resp.Body.Close()
		alive = resp.StatusCode < 500 && resp.StatusCode != 404 && resp.StatusCode != 410
	}
	switch {
	case r.ISPBlocked && alive:
		r.Status = "blocked_alive"
	case r.ISPBlocked:
		r.Status = "blocked"
	case alive:
		r.Status = "online"
	default:
		r.Status = "offline"
	}
	return r
}
