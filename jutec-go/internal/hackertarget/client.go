// Package hackertarget membungkus API reverse analytics HackerTarget:
// domain lain di internet yang memakai ID Google Analytics / AdSense yang sama.
package hackertarget

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const endpoint = "https://api.hackertarget.com/analyticslookup/"

// ErrQuota dikembalikan saat kuota harian habis.
var ErrQuota = errors.New("hackertarget: kuota API habis")

type Client struct {
	APIKey string // opsional — tanpa key memakai kuota gratis harian
	HTTP   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{APIKey: apiKey, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// ReverseAnalytics mengembalikan domain yang memakai ID yang sama. Output API berupa teks, satu domain per baris;
// galat juga dikirim sebagai teks biasa (mis. "error invalid input", "API count exceeded …").
func (c *Client) ReverseAnalytics(id string) ([]string, error) {
	q := url.Values{"q": {id}}
	if c.APIKey != "" {
		q.Set("apikey", c.APIKey)
	}
	resp, err := c.HTTP.Get(endpoint + "?" + q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	text := strings.TrimSpace(string(body))
	low := strings.ToLower(text)
	switch {
	case resp.StatusCode == 429 || strings.Contains(low, "api count exceeded"):
		return nil, ErrQuota
	case resp.StatusCode != 200:
		return nil, fmt.Errorf("hackertarget: HTTP %d", resp.StatusCode)
	case strings.HasPrefix(low, "error"):
		return nil, fmt.Errorf("hackertarget: %s", text)
	case strings.HasPrefix(low, "no results") || strings.HasPrefix(low, "no records") || text == "":
		return []string{}, nil
	}
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		// Format baris: "domain<TAB>jenis" (mis. "www.contoh.com	ga4") — ambil kolom pertama
		f := strings.FieldsFunc(line, func(r rune) bool { return r == '\t' || r == ',' || r == ' ' })
		if len(f) == 0 {
			continue
		}
		d := strings.ToLower(f[0])
		d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
		d = strings.TrimSuffix(d, "/")
		d = strings.TrimPrefix(d, "www.")
		if d == "" || !strings.Contains(d, ".") || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out, nil
}
