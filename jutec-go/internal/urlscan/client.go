package urlscan

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://urlscan.io/api/v1"

// ErrRateLimit dikembalikan saat API mengembalikan 429. Caller tidak perlu retry.
var ErrRateLimit = errors.New("urlscan: rate limit exceeded")

// ErrNotReady dikembalikan saat hasil scan belum siap setelah polling.
var ErrNotReady = errors.New("urlscan: scan not ready yet")

type Client struct {
	APIKey string
	HTTP   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		APIKey: apiKey,
		HTTP:   &http.Client{Timeout: 30 * time.Second},
	}
}

// ScanResult berisi hasil yang diparsing dari URLScan API.
type ScanResult struct {
	UUID          string
	Verdict       string // malicious, suspicious, benign, unknown
	Malicious     bool
	Score         int
	ScreenshotURL string
	Categories    []string
	Technologies  []string
	ScannedAt     time.Time
}

// SearchExisting mencari scan terbaru untuk domain tanpa menggunakan kuota.
// Mengembalikan nil jika belum pernah di-scan atau scan terlalu lama.
func (c *Client) SearchExisting(domain string) (*ScanResult, error) {
	q := url.QueryEscape("domain:" + domain)
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/search/?q=%s&size=1", baseURL, q), nil)
	req.Header.Set("API-Key", c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, ErrRateLimit
	}
	if resp.StatusCode != 200 {
		return nil, nil // tidak ada hasil, bukan error fatal
	}

	var body struct {
		Results []struct {
			ID   string `json:"_id"`
			Task struct {
				Time string `json:"time"`
			} `json:"task"`
			Verdicts struct {
				Overall struct {
					Score      int      `json:"score"`
					Malicious  bool     `json:"malicious"`
					Categories []string `json:"categories"`
					Tags       []string `json:"tags"`
				} `json:"overall"`
			} `json:"verdicts"`
			Screenshot string `json:"screenshot"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || len(body.Results) == 0 {
		return nil, nil
	}

	r := body.Results[0]
	scannedAt, _ := time.Parse(time.RFC3339, r.Task.Time)

	// Hanya pakai hasil yang tidak lebih dari 7 hari
	if time.Since(scannedAt) > 7*24*time.Hour {
		return nil, nil
	}

	verdict := "benign"
	if r.Verdicts.Overall.Malicious {
		verdict = "malicious"
	} else if r.Verdicts.Overall.Score > 40 {
		verdict = "suspicious"
	}

	return &ScanResult{
		UUID:          r.ID,
		Verdict:       verdict,
		Malicious:     r.Verdicts.Overall.Malicious,
		Score:         r.Verdicts.Overall.Score,
		ScreenshotURL: r.Screenshot,
		Categories:    r.Verdicts.Overall.Categories,
		ScannedAt:     scannedAt,
	}, nil
}

// Submit mengirimkan URL ke URLScan untuk di-scan. Mengembalikan UUID scan.
func (c *Client) Submit(rawURL string) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("urlscan: api key not configured")
	}

	body := strings.NewReader(`{"url":"` + rawURL + `","visibility":"unlisted"}`)
	req, _ := http.NewRequest("POST", baseURL+"/scan/", body)
	req.Header.Set("API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return "", ErrRateLimit
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("urlscan submit: status %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		UUID string `json:"uuid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.UUID, nil
}

// FetchResult mengambil hasil scan berdasarkan UUID.
// Mengembalikan ErrNotReady jika scan belum selesai.
func (c *Client) FetchResult(uuid string) (*ScanResult, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/result/%s/", baseURL, uuid), nil)
	req.Header.Set("API-Key", c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, ErrRateLimit
	}
	if resp.StatusCode == 404 {
		return nil, ErrNotReady
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("urlscan result: status %d", resp.StatusCode)
	}

	var body struct {
		Verdicts struct {
			Overall struct {
				Score      int      `json:"score"`
				Malicious  bool     `json:"malicious"`
				Categories []string `json:"categories"`
				Tags       []string `json:"tags"`
			} `json:"overall"`
		} `json:"verdicts"`
		Page struct {
			Domain string `json:"domain"`
		} `json:"page"`
		Task struct {
			Time       string `json:"time"`
			ScreenshotURL string `json:"screenshotURL"`
		} `json:"task"`
		Meta struct {
			Processors struct {
				Wappa struct {
					Data []struct {
						App string `json:"app"`
					} `json:"data"`
				} `json:"wappa"`
			} `json:"processors"`
		} `json:"meta"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	scannedAt, _ := time.Parse(time.RFC3339, body.Task.Time)

	verdict := "benign"
	if body.Verdicts.Overall.Malicious {
		verdict = "malicious"
	} else if body.Verdicts.Overall.Score > 40 {
		verdict = "suspicious"
	}

	techs := make([]string, 0)
	for _, t := range body.Meta.Processors.Wappa.Data {
		techs = append(techs, t.App)
	}

	return &ScanResult{
		UUID:          uuid,
		Verdict:       verdict,
		Malicious:     body.Verdicts.Overall.Malicious,
		Score:         body.Verdicts.Overall.Score,
		ScreenshotURL: body.Task.ScreenshotURL,
		Categories:    body.Verdicts.Overall.Categories,
		Technologies:  techs,
		ScannedAt:     scannedAt,
	}, nil
}

// ScanURL mencari scan existing dulu; jika tidak ada, submit baru lalu polling hasil.
// Graceful: rate limit dikembalikan sebagai error yang bisa di-cek caller.
func (c *Client) ScanURL(rawURL string) (*ScanResult, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	// 1. Cari scan existing (tidak pakai kuota)
	existing, err := c.SearchExisting(u.Hostname())
	if err != nil && !errors.Is(err, ErrRateLimit) {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	// Kuota "search" (1000/hari) terpisah dari kuota submit — kalau search habis, tetap submit scan baru



	// 2. Submit scan baru
	uuid, err := c.Submit(rawURL)
	if err != nil {
		return nil, err
	}

	// 3. Poll hasil (max 3x, interval 15 detik)
	for i := 0; i < 3; i++ {
		time.Sleep(15 * time.Second)
		result, err := c.FetchResult(uuid)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, ErrNotReady) {
			return nil, err
		}
	}

	// Scan terlalu lama — simpan UUID tanpa result, bisa di-fetch manual
	return &ScanResult{UUID: uuid, Verdict: "unknown"}, nil
}
