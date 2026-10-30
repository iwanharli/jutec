// Package evidence mengambil bukti forensik sebuah halaman: HTML asli, screenshot satu halaman penuh
// (desktop & HP), dan log jaringan. Setiap file ditulis sekali (baca-saja), di-hash SHA-256, lalu
// diikat dalam manifest.json yang hash-nya juga dicatat — sehingga perubahan file apa pun terdeteksi.
package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const (
	ToolVersion = "jutec-evidence/1"
	DesktopUA   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	MobileUA    = "Mozilla/5.0 (Linux; Android 13; SM-A536E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"
)

type File struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size_bytes"`
}

type NetEntry struct {
	URL      string  `json:"url"`
	Method   string  `json:"method"`
	Status   int64   `json:"status,omitempty"`
	MIME     string  `json:"mime,omitempty"`
	RemoteIP string  `json:"remote_ip,omitempty"`
	Type     string  `json:"type,omitempty"`
	At       float64 `json:"at"` // detik sejak navigasi dimulai
}

type Manifest struct {
	Tool         string    `json:"tool"`
	CaptureID    int       `json:"capture_id"`
	SiteID       int       `json:"site_id"`
	RequestedURL string    `json:"requested_url"`
	FinalURL     string    `json:"final_url"`
	HTTPStatus   int64     `json:"http_status"`
	ServerIP     string    `json:"server_ip"`
	Redirects    []string  `json:"redirects"`
	UserAgent    string    `json:"user_agent"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	Host         string    `json:"capture_host"`
	RequestedBy  string    `json:"requested_by"`
	Files        []File    `json:"files"`
}

type Result struct {
	Manifest       Manifest
	ManifestSHA256 string
	Files          []File // termasuk manifest.json
}

// Capture memuat url lalu menyimpan buktinya ke dir (dibuat; tidak boleh sudah ada).
func Capture(ctx context.Context, dir string, captureID, siteID int, url, requestedBy string) (*Result, error) {
	if err := os.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
		return nil, err
	}
	if err := os.Mkdir(dir, 0o750); err != nil {
		return nil, fmt.Errorf("folder bukti sudah ada / tidak bisa dibuat: %w", err)
	}
	host, _ := os.Hostname()
	m := Manifest{Tool: ToolVersion, CaptureID: captureID, SiteID: siteID, RequestedURL: url, UserAgent: DesktopUA,
		StartedAt: time.Now().UTC(), Host: host, RequestedBy: requestedBy, Redirects: []string{}}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true), chromedp.Flag("disable-gpu", true), chromedp.Flag("no-sandbox", true),
		chromedp.Flag("ignore-certificate-errors", true), chromedp.UserAgent(DesktopUA), chromedp.WindowSize(1366, 900))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	tab, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()
	tab, cancelTimeout := context.WithTimeout(tab, 150*time.Second)
	defer cancelTimeout()

	// Log jaringan + dokumen utama (status, IP server, rantai redirect)
	var mu sync.Mutex
	entries := map[network.RequestID]*NetEntry{}
	var order []network.RequestID
	start := time.Now()
	var mainID network.RequestID
	chromedp.ListenTarget(tab, func(ev interface{}) {
		mu.Lock()
		defer mu.Unlock()
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if e.RedirectResponse != nil && e.Type == network.ResourceTypeDocument && (mainID == "" || e.RequestID == mainID) {
				m.Redirects = append(m.Redirects, e.RedirectResponse.URL)
			}
			if mainID == "" && e.Type == network.ResourceTypeDocument {
				mainID = e.RequestID
			}
			if _, ok := entries[e.RequestID]; !ok {
				order = append(order, e.RequestID)
			}
			entries[e.RequestID] = &NetEntry{URL: e.Request.URL, Method: e.Request.Method, Type: string(e.Type), At: time.Since(start).Seconds()}
		case *network.EventResponseReceived:
			if n := entries[e.RequestID]; n != nil {
				n.Status, n.MIME, n.RemoteIP = e.Response.Status, e.Response.MimeType, e.Response.RemoteIPAddress
			}
			if e.RequestID == mainID {
				m.HTTPStatus, m.ServerIP = e.Response.Status, e.Response.RemoteIPAddress
			}
		}
	})

	var html, finalURL string
	var desktopPNG, mobilePNG []byte
	err := chromedp.Run(tab,
		network.Enable(),
		// Navigasi tanpa menunggu event "load": banyak situs judol tak pernah memicunya (widget chat,
		// long-polling, iklan) sehingga chromedp.Navigate menggantung sampai batas waktu. Cukup tunggu body siap.
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, _, errText, _, err := page.Navigate(url).Do(ctx)
			if err == nil && errText != "" {
				err = fmt.Errorf("%s", errText)
			}
			return err
		}),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(6*time.Second), // beri waktu skrip/popup judol tampil
		chromedp.Location(&finalURL),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
		chromedp.FullScreenshot(&desktopPNG, 100), // kualitas 100 → PNG tanpa kompresi lossy
	)
	if err != nil {
		return nil, fmt.Errorf("gagal memuat halaman: %w", err)
	}
	// Tampilan HP (banyak situs judol menampilkan konten berbeda di HP)
	_ = chromedp.Run(tab,
		emulation.SetDeviceMetricsOverride(412, 915, 2.6, true),
		emulation.SetUserAgentOverride(MobileUA).WithPlatform("Android"),
		chromedp.Reload(),
		chromedp.Sleep(3*time.Second),
		chromedp.FullScreenshot(&mobilePNG, 100),
	)
	m.FinalURL = finalURL

	mu.Lock()
	netLog := make([]NetEntry, 0, len(order))
	for _, id := range order {
		netLog = append(netLog, *entries[id])
	}
	mu.Unlock()
	sort.SliceStable(netLog, func(i, j int) bool { return netLog[i].At < netLog[j].At })
	netJSON, _ := json.MarshalIndent(netLog, "", "  ")

	type item struct {
		name, kind string
		data       []byte
	}
	items := []item{{"page.html", "html", []byte(html)}, {"screenshot-desktop.png", "screenshot", desktopPNG}}
	if len(mobilePNG) > 0 {
		items = append(items, item{"screenshot-mobile.png", "screenshot", mobilePNG})
	}
	items = append(items, item{"network.json", "network", netJSON})

	for _, it := range items {
		f, err := writeOnce(dir, it.name, it.kind, it.data)
		if err != nil {
			return nil, err
		}
		m.Files = append(m.Files, f)
	}
	m.FinishedAt = time.Now().UTC()
	manJSON, _ := json.MarshalIndent(m, "", "  ")
	mf, err := writeOnce(dir, "manifest.json", "manifest", manJSON)
	if err != nil {
		return nil, err
	}
	return &Result{Manifest: m, ManifestSHA256: mf.SHA256, Files: append(append([]File{}, m.Files...), mf)}, nil
}

// writeOnce menulis file baru (gagal bila sudah ada) lalu menjadikannya baca-saja.
func writeOnce(dir, name, kind string, data []byte) (File, error) {
	p := filepath.Join(dir, name)
	fh, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o440)
	if err != nil {
		return File{}, err
	}
	if _, err := fh.Write(data); err != nil {
		fh.Close()
		return File{}, err
	}
	if err := fh.Sync(); err != nil {
		fh.Close()
		return File{}, err
	}
	fh.Close()
	sum := sha256.Sum256(data)
	return File{Name: name, Kind: kind, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))}, nil
}

// HashFile menghitung ulang SHA-256 file di disk (untuk verifikasi).
func HashFile(path string) (string, int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), int64(len(b)), nil
}
