// Package commoncrawl mencari URL di indeks arsip Common Crawl (index.commoncrawl.org) —
// dipakai untuk menemukan halaman judol yang disusupkan ke domain institusi tanpa lewat Google.
package commoncrawl

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const base = "https://index.commoncrawl.org"

// ErrEnd: halaman indeks di luar jangkauan / tak ada tangkapan lagi.
var ErrEnd = errors.New("commoncrawl: halaman habis")

type Record struct {
	URL       string `json:"url"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
}

var client = &http.Client{Timeout: 3 * time.Minute}

// LatestIndex: ID koleksi terbaru, mis. CC-MAIN-2026-43.
func LatestIndex(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", base+"/collinfo.json", nil)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var cols []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&cols); err != nil || len(cols) == 0 {
		return "", fmt.Errorf("collinfo: %v", err)
	}
	return cols[0].ID, nil
}

// Query membaca satu halaman indeks untuk pola URL (mis. "*.go.id") dengan filter regex URL.
func Query(ctx context.Context, index, pattern, urlRegex string, page int) ([]Record, error) {
	q := url.Values{}
	q.Set("url", pattern)
	q.Set("output", "json")
	q.Set("page", fmt.Sprint(page))
	q.Set("fl", "url,timestamp,status")
	if urlRegex != "" {
		q.Set("filter", "~url:"+urlRegex)
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", base+"/"+index+"-index?"+q.Encode(), nil)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == 404:
		return nil, nil // halaman ini tanpa hasil yang cocok filter
	case res.StatusCode == 400:
		return nil, ErrEnd
	case res.StatusCode != 200:
		return nil, fmt.Errorf("commoncrawl: HTTP %d", res.StatusCode)
	}
	var out []Record
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var r Record
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.URL != "" {
			out = append(out, r)
		}
	}
	return out, sc.Err()
}
