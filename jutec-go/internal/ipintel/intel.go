// Package ipintel mengumpulkan informasi PASIF tentang sebuah IP server: pemilik blok IP (RDAP),
// lokasi & ASN (ip-api), port/hostname yang pernah teramati (Shodan InternetDB), dan reverse DNS.
// Tidak ada pemindaian aktif ke server target.
package ipintel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Intel struct {
	IP          string            `json:"ip"`
	Version     int               `json:"version"`
	ReverseDNS  []string          `json:"reverse_dns"`
	Country     string            `json:"country"`
	CountryCode string            `json:"country_code"`
	Region      string            `json:"region"`
	City        string            `json:"city"`
	ISP         string            `json:"isp"`
	Org         string            `json:"org"`
	ASN         string            `json:"asn"`     // "AS13335"
	ASName      string            `json:"as_name"` // "CLOUDFLARENET"
	Hosting     bool              `json:"hosting"` // IP pusat data / hosting
	Proxy       bool              `json:"proxy"`   // proxy / VPN / Tor
	CDN         string            `json:"cdn"`     // "Cloudflare" bila IP milik CDN (IP asli tersembunyi)
	NetName     string            `json:"net_name"`
	NetRange    string            `json:"net_range"`
	Registry    string            `json:"registry"` // RIR: APNIC, ARIN, RIPE, …
	RegOrg      string            `json:"reg_org"`  // organisasi pemegang blok IP
	AbuseEmails []string          `json:"abuse_emails"`
	Ports       []int             `json:"ports"`
	Hostnames   []string          `json:"hostnames"`
	Tags        []string          `json:"tags"`
	Vulns       []string          `json:"vulns"`
	Errors      map[string]string `json:"errors,omitempty"`
	FetchedAt   time.Time         `json:"fetched_at"`
}

var httpc = &http.Client{Timeout: 15 * time.Second}

// ip-api gratis dibatasi 45 permintaan/menit
var ipapiMu sync.Mutex
var ipapiLast time.Time

// Lookup menjalankan semua sumber secara paralel. Kegagalan satu sumber tidak menggagalkan yang lain.
func Lookup(ctx context.Context, ip string) (*Intel, error) {
	addr := net.ParseIP(ip)
	if addr == nil {
		return nil, fmt.Errorf("IP tidak valid: %q", ip)
	}
	if addr.IsPrivate() || addr.IsLoopback() || addr.IsUnspecified() {
		return nil, fmt.Errorf("IP privat/lokal tidak bisa dilacak: %s", ip)
	}
	in := &Intel{IP: ip, Version: 4, Errors: map[string]string{}, FetchedAt: time.Now().UTC(),
		ReverseDNS: []string{}, AbuseEmails: []string{}, Ports: []int{}, Hostnames: []string{}, Tags: []string{}, Vulns: []string{}}
	if addr.To4() == nil {
		in.Version = 6
	}
	if IsCloudflare(addr) {
		in.CDN = "Cloudflare"
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	run := func(name string, f func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := f(); err != nil {
				mu.Lock()
				in.Errors[name] = err.Error()
				mu.Unlock()
			}
		}()
	}
	run("ipapi", func() error { return ipAPI(ctx, in, &mu) })
	run("rdap", func() error { return rdap(ctx, in, &mu) })
	run("internetdb", func() error { return internetDB(ctx, in, &mu) })
	run("rdns", func() error {
		names, err := net.DefaultResolver.LookupAddr(ctx, ip)
		mu.Lock()
		for _, n := range names {
			in.ReverseDNS = append(in.ReverseDNS, strings.TrimSuffix(n, "."))
		}
		mu.Unlock()
		if err != nil && !strings.Contains(err.Error(), "no such host") {
			return err
		}
		return nil
	})
	wg.Wait()
	if in.CDN == "" && strings.Contains(strings.ToUpper(in.ASName+in.Org), "CLOUDFLARE") {
		in.CDN = "Cloudflare"
	}
	return in, nil
}

func getJSON(ctx context.Context, url string, v any) (int, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/json, application/rdap+json")
	req.Header.Set("User-Agent", "jutec-ipintel/1")
	resp, err := httpc.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return resp.StatusCode, fmt.Errorf("status %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return 200, json.Unmarshal(b, v)
}

func ipAPI(ctx context.Context, in *Intel, mu *sync.Mutex) error {
	ipapiMu.Lock()
	if wait := 1400*time.Millisecond - time.Since(ipapiLast); wait > 0 {
		time.Sleep(wait)
	}
	ipapiLast = time.Now()
	ipapiMu.Unlock()
	var r struct {
		Status, Message, Country, CountryCode, RegionName, City, ISP, Org, AS, ASName string
		Hosting, Proxy                                                                bool
	}
	// Layanan gratis ip-api hanya tersedia lewat HTTP
	if _, err := getJSON(ctx, "http://ip-api.com/json/"+in.IP+
		"?fields=status,message,country,countryCode,regionName,city,isp,org,as,asname,hosting,proxy", &r); err != nil {
		return err
	}
	if r.Status != "success" {
		return fmt.Errorf("ip-api: %s", r.Message)
	}
	mu.Lock()
	defer mu.Unlock()
	in.Country, in.CountryCode, in.Region, in.City = r.Country, r.CountryCode, r.RegionName, r.City
	in.ISP, in.Org, in.ASName, in.Hosting, in.Proxy = r.ISP, r.Org, r.ASName, r.Hosting, r.Proxy
	if f := strings.Fields(r.AS); len(f) > 0 {
		in.ASN = f[0]
	}
	return nil
}

// rdap: data registri resmi blok IP (rdap.org meneruskan ke RIR yang berwenang).
func rdap(ctx context.Context, in *Intel, mu *sync.Mutex) error {
	var r struct {
		Name, Handle, StartAddress, EndAddress, Port43 string
		Cidr0                                          []struct {
			V4prefix, V6prefix string
			Length             int
		} `json:"cidr0_cidrs"`
		Entities []rdapEntity
	}
	if _, err := getJSON(ctx, "https://rdap.org/ip/"+in.IP, &r); err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	in.NetName = r.Name
	if len(r.Cidr0) > 0 {
		c := r.Cidr0[0]
		in.NetRange = fmt.Sprintf("%s%s/%d", c.V4prefix, c.V6prefix, c.Length)
	} else if r.StartAddress != "" {
		in.NetRange = r.StartAddress + " – " + r.EndAddress
	}
	in.Registry = registryOf(r.Port43)
	emails := map[string]bool{}
	var walk func(es []rdapEntity)
	walk = func(es []rdapEntity) {
		for _, e := range es {
			fn, email := e.vcard()
			for _, role := range e.Roles {
				switch role {
				case "abuse":
					if email != "" {
						emails[strings.ToLower(email)] = true
					}
				case "registrant":
					if in.RegOrg == "" {
						in.RegOrg = fn
					}
				}
			}
			walk(e.Entities)
		}
	}
	walk(r.Entities)
	for e := range emails {
		in.AbuseEmails = append(in.AbuseEmails, e)
	}
	sort.Strings(in.AbuseEmails)
	return nil
}

type rdapEntity struct {
	Roles      []string        `json:"roles"`
	VcardArray json.RawMessage `json:"vcardArray"`
	Entities   []rdapEntity    `json:"entities"`
}

// vcard mengambil nama (fn) & email dari jCard RDAP: ["vcard", [["fn",{},"text","Nama"], ["email",{},"text","a@b"]]]
func (e rdapEntity) vcard() (fn, email string) {
	var arr []json.RawMessage
	if json.Unmarshal(e.VcardArray, &arr) != nil || len(arr) < 2 {
		return
	}
	var props [][]json.RawMessage
	if json.Unmarshal(arr[1], &props) != nil {
		return
	}
	for _, p := range props {
		if len(p) < 4 {
			continue
		}
		var key, val string
		json.Unmarshal(p[0], &key)
		json.Unmarshal(p[3], &val)
		switch key {
		case "fn":
			fn = val
		case "email":
			email = val
		}
	}
	return
}

func registryOf(port43 string) string {
	p := strings.ToLower(port43)
	for k, v := range map[string]string{"apnic": "APNIC", "arin": "ARIN", "ripe": "RIPE NCC", "lacnic": "LACNIC", "afrinic": "AFRINIC", "idnic": "IDNIC"} {
		if strings.Contains(p, k) {
			return v
		}
	}
	return port43
}

// internetDB: data pasif Shodan (port & hostname yang pernah teramati) — gratis, tanpa kunci.
func internetDB(ctx context.Context, in *Intel, mu *sync.Mutex) error {
	var r struct {
		Ports     []int    `json:"ports"`
		Hostnames []string `json:"hostnames"`
		Tags      []string `json:"tags"`
		Vulns     []string `json:"vulns"`
	}
	code, err := getJSON(ctx, "https://internetdb.shodan.io/"+in.IP, &r)
	if code == 404 {
		return nil // belum pernah teramati
	}
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	in.Ports, in.Hostnames, in.Tags, in.Vulns = nz(r.Ports), nzs(r.Hostnames), nzs(r.Tags), nzs(r.Vulns)
	return nil
}

func nz(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}
func nzs(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// Rentang IP Cloudflare (cloudflare.com/ips). IP di sini = proxy CDN, bukan server asli operator.
var cfNets = func() []*net.IPNet {
	var out []*net.IPNet
	for _, c := range []string{
		"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22", "141.101.64.0/18", "108.162.192.0/18",
		"190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
		"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
		"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32", "2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
	} {
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}()

func IsCloudflare(ip net.IP) bool {
	for _, n := range cfNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
