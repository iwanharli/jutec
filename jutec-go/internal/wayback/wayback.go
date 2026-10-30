// Package wayback mengambil snapshot lama situs dari Internet Archive (Wayback Machine) untuk
// menemukan rekening, kontak, dan link pendaftaran yang sudah dihapus dari versi sekarang.
package wayback

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"jutec-go/internal/crawler"
	"jutec-go/internal/store"
)

var client = &http.Client{Timeout: 30 * time.Second}

// Fetch mengambil hingga max snapshot (tersebar dari terlama ke terbaru) dan mengekstrak isinya.
func Fetch(host string, max int) ([]store.WaybackSnapshot, error) {
	q := url.Values{
		"url":      {host},
		"output":   {"json"},
		"fl":       {"timestamp,original"},
		"filter":   {"statuscode:200"},
		"collapse": {"timestamp:6"}, // satu snapshot per bulan
		"limit":    {"300"},
	}
	resp, err := getWithRetry("https://web.archive.org/cdx/search/cdx?" + q.Encode())
	if err != nil {
		return nil, fmt.Errorf("wayback CDX: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("wayback CDX: status %d", resp.StatusCode)
	}
	var rows [][]string
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("wayback CDX: %w", err)
	}
	if len(rows) <= 1 {
		return []store.WaybackSnapshot{}, nil // belum pernah diarsipkan
	}
	rows = rows[1:] // baris pertama = header
	picks := spread(len(rows), max)

	var out []store.WaybackSnapshot
	for _, i := range picks {
		ts, orig := rows[i][0], rows[i][1]
		// "id_" = HTML asli tanpa toolbar Wayback
		raw := fmt.Sprintf("https://web.archive.org/web/%sid_/%s", ts, orig)
		html := get(raw)
		if html == "" {
			continue
		}
		snap := store.WaybackSnapshot{Timestamp: ts, URL: fmt.Sprintf("https://web.archive.org/web/%s/%s", ts, orig)}
		for _, a := range crawler.ExtractArtifacts(html) {
			snap.Artifacts = append(snap.Artifacts, map[string]string{"type": a.Type, "value": a.Value, "name": a.Name})
		}
		links := crawler.ExtractLinks(html, orig)
		for _, r := range links.Register {
			if !strings.Contains(r, "web.archive.org") {
				snap.Register = append(snap.Register, r)
			}
		}
		for _, t := range crawler.ExtractTrackingIDs(html) {
			snap.Tracking = append(snap.Tracking, map[string]string{"platform": t.Platform, "value": t.Value})
		}
		out = append(out, snap)
		time.Sleep(1500 * time.Millisecond) // sopan terhadap archive.org
	}
	return out, nil
}

// spread memilih n indeks tersebar merata (termasuk yang pertama & terakhir).
func spread(total, n int) []int {
	if total <= n {
		idx := make([]int, total)
		for i := range idx {
			idx[i] = i
		}
		return idx
	}
	seen := map[int]bool{}
	var idx []int
	for k := 0; k < n; k++ {
		i := k * (total - 1) / (n - 1)
		if !seen[i] {
			seen[i] = true
			idx = append(idx, i)
		}
	}
	sort.Ints(idx)
	return idx
}

// getWithRetry: archive.org menolak request tanpa User-Agent / terlalu cepat (status 429/498) — coba ulang.
func getWithRetry(u string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; JutecResearch/1.0; +investigasi-judol)")
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode != 429 && resp.StatusCode != 498 && resp.StatusCode != 503 {
			return resp, nil
		}
		if err == nil {
			resp.Body.Close()
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(time.Duration(3*(attempt+1)) * time.Second)
	}
	return nil, lastErr
}

func get(u string) string {
	resp, err := getWithRetry(u)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return string(b)
}
