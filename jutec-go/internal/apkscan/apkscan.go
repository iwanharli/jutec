// Package apkscan menganalisis APK secara STATIS: file diunduh lalu dibongkar sebagai arsip —
// tidak pernah di-install atau dijalankan. Hasilnya dipakai untuk menghubungkan situs lewat
// sertifikat penanda tangan (developer yang sama) dan domain/API yang tertanam di dalam kode.
package apkscan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"jutec-go/internal/netutil"

	"github.com/avast/apkparser"
	"github.com/avast/apkverifier"
)

const (
	maxAPKBytes  = 200 << 20 // 200 MB
	maxFileBytes = 25 << 20  // batas baca per file di dalam APK
)

type Result struct {
	SHA256        string    `json:"sha256"`
	SizeBytes     int64     `json:"size_bytes"`
	PackageName   string    `json:"package_name"`
	VersionName   string    `json:"version_name"`
	AppLabel      string    `json:"app_label"`
	CertSHA256    string    `json:"cert_sha256"`
	CertSubject   string    `json:"cert_subject"`
	CertIssuer    string    `json:"cert_issuer"`
	CertNotBefore time.Time `json:"cert_not_before"`
	Permissions   []string  `json:"permissions"`
	URLs          []string  `json:"urls"`
	Domains       []string  `json:"domains"`
}

var (
	reURL = regexp.MustCompile(`https?://[a-zA-Z0-9][a-zA-Z0-9.-]*\.[a-zA-Z]{2,}(?::\d+)?[^\s"'<>\\\x00-\x1f]*`)
	// Domain milik platform/SDK umum — bukan infrastruktur operator
	reNoise   = regexp.MustCompile(`(?i)(android\.com|google|gstatic|googleapis|firebase|crashlytics|schemas\.|w3\.org|apache\.org|xml\.org|mozilla|java\.(com|net)|oracle\.com|facebook\.com|fbcdn|apple\.com|microsoft|github\.com|jquery|cloudflare|example\.com|localhost|ietf\.org|unicode\.org|adobe\.com|openssl|bouncycastle|jetbrains|kotlinlang|reactnative|expo\.(io|dev)|sentry|onesignal|appsflyer|branch\.io|play\.google)`)
	rePackage = regexp.MustCompile(`\bpackage="([^"]+)"`)
	reVersion = regexp.MustCompile(`android:versionName="([^"]*)"`)
	reLabel   = regexp.MustCompile(`<application[^>]*android:label="([^"]*)"`)
	rePerm    = regexp.MustCompile(`<uses-permission[^>]*android:name="([^"]+)"`)
)

// Analyze mengunduh APK dari rawURL ke file sementara lalu membongkarnya.
func Analyze(rawURL string) (*Result, error) {
	path, sum, size, err := download(rawURL)
	if err != nil {
		return nil, err
	}
	defer os.Remove(path)

	res := &Result{SHA256: sum, SizeBytes: size}
	zr, err := apkparser.OpenZip(path)
	if err != nil {
		return res, fmt.Errorf("bukan arsip APK yang valid: %w", err)
	}
	defer zr.Close()

	// Manifest biner → XML teks
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if _, manifestErr := apkparser.ParseApkWithZip(zr, enc); manifestErr == nil {
		mf := buf.String()
		res.PackageName = first(rePackage, mf)
		res.VersionName = first(reVersion, mf)
		if l := first(reLabel, mf); l != "" && !strings.HasPrefix(l, "@") {
			res.AppLabel = l
		}
		for _, m := range rePerm.FindAllStringSubmatch(mf, -1) {
			res.Permissions = append(res.Permissions, strings.TrimPrefix(m[1], "android.permission."))
		}
	}

	// Sertifikat penanda tangan (skema v1/v2/v3) — identitas developer
	if certs, err := apkverifier.ExtractCerts(path, zr); err == nil && len(certs) > 0 && len(certs[0]) > 0 {
		c := certs[0][0]
		h := sha256.Sum256(c.Raw)
		res.CertSHA256 = hex.EncodeToString(h[:])
		res.CertSubject = c.Subject.String()
		res.CertIssuer = c.Issuer.String()
		res.CertNotBefore = c.NotBefore
	}

	// URL di dalam kode (classes*.dex) & aset (config JSON, HTML, JS)
	urls := map[string]bool{}
	for _, f := range zr.FilesOrdered {
		name := strings.ToLower(f.Name)
		if !(strings.HasSuffix(name, ".dex") || strings.HasPrefix(name, "assets/") || strings.HasPrefix(name, "res/raw/")) {
			continue
		}
		if err := f.Open(); err != nil {
			continue
		}
		data, _ := f.ReadAll(maxFileBytes)
		f.Close()
		for _, u := range reURL.FindAllString(string(data), -1) {
			u = strings.TrimRight(u, ".,;)")
			if !reNoise.MatchString(u) && len(u) < 300 {
				urls[u] = true
			}
		}
	}
	domains := map[string]bool{}
	for u := range urls {
		res.URLs = append(res.URLs, u)
		if p, err := url.Parse(u); err == nil && p.Hostname() != "" {
			domains[strings.ToLower(p.Hostname())] = true
		}
	}
	for d := range domains {
		res.Domains = append(res.Domains, d)
	}
	sort.Strings(res.URLs)
	sort.Strings(res.Domains)
	if len(res.URLs) > 300 {
		res.URLs = res.URLs[:300]
	}
	return res, nil
}

func download(rawURL string) (path, sum string, size int64, err error) {
	// Resolusi via DoH: domain yang diblokir DNS ISP tetap bisa diunduh untuk dianalisis
	client := netutil.HTTPClient(3 * time.Minute)
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 Chrome/120 Mobile Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, errors.New(netutil.Diagnose(rawURL, err, 0))
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", 0, errors.New(netutil.Diagnose(rawURL, nil, resp.StatusCode))
	}
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); strings.Contains(ct, "text/html") {
		return "", "", 0, fmt.Errorf("link mengarah ke halaman HTML, bukan file APK")
	}
	f, err := os.CreateTemp("", "jutec_apk_*.apk")
	if err != nil {
		return "", "", 0, err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxAPKBytes+1))
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		return "", "", 0, err
	}
	if n > maxAPKBytes {
		os.Remove(f.Name())
		return "", "", 0, fmt.Errorf("APK lebih dari %d MB", maxAPKBytes>>20)
	}
	return f.Name(), hex.EncodeToString(h.Sum(nil)), n, nil
}

func first(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}
