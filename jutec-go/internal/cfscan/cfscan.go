// Package cfscan memakai Cloudflare URL Scanner (Radar) sebagai cadangan gratis URLScan.io:
// screenshot, request jaringan, IP, teknologi & vonis. Butuh CLOUDFLARE_ACCOUNT_ID + CLOUDFLARE_API_TOKEN
// (token dengan izin "URL Scanner"). Hasil dipetakan ke bentuk urlscan.ScanResult agar disimpan di tempat sama.
package cfscan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"jutec-go/internal/urlscan"
)

var (
	ErrNotConfigured = errors.New("cloudflare url scanner belum dikonfigurasi")
	ErrRateLimit     = errors.New("cloudflare url scanner: rate limit")
	client           = &http.Client{Timeout: 30 * time.Second}
)

func Configured() bool {
	return os.Getenv("CLOUDFLARE_ACCOUNT_ID") != "" && os.Getenv("CLOUDFLARE_API_TOKEN") != ""
}

func base() string {
	return "https://api.cloudflare.com/client/v4/accounts/" + os.Getenv("CLOUDFLARE_ACCOUNT_ID") + "/urlscanner/v2"
}

func do(method, path string, body any, out any) (int, error) {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, base()+path, rd)
	req.Header.Set("Authorization", "Bearer "+os.Getenv("CLOUDFLARE_API_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode == 429 {
		return 429, ErrRateLimit
	}
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode, nil
}

// Scan mengirim URL (visibilitas Unlisted) lalu menunggu hasil maks. ±90 detik.
func Scan(rawURL string) (*urlscan.ScanResult, error) {
	if !Configured() {
		return nil, ErrNotConfigured
	}
	var sub struct {
		UUID string `json:"uuid"`
	}
	code, err := do("POST", "/scan", map[string]any{"url": rawURL, "visibility": "Unlisted"}, &sub)
	if err != nil {
		return nil, err
	}
	if code >= 300 || sub.UUID == "" {
		return nil, fmt.Errorf("cloudflare url scanner: submit HTTP %d", code)
	}
	for i := 0; i < 9; i++ {
		time.Sleep(10 * time.Second)
		var r struct {
			Task struct {
				Time string `json:"time"`
			} `json:"task"`
			Verdicts struct {
				Overall struct {
					Malicious  bool     `json:"malicious"`
					Categories []string `json:"categories"`
				} `json:"overall"`
			} `json:"verdicts"`
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
		code, err := do("GET", "/result/"+sub.UUID, nil, &r)
		if err != nil {
			return nil, err
		}
		if code == 404 {
			continue // masih diproses
		}
		if code != 200 {
			return nil, fmt.Errorf("cloudflare url scanner: result HTTP %d", code)
		}
		out := &urlscan.ScanResult{UUID: "cf:" + sub.UUID, Verdict: "benign", Categories: r.Verdicts.Overall.Categories, ScannedAt: time.Now()}
		if t, err := time.Parse(time.RFC3339, r.Task.Time); err == nil {
			out.ScannedAt = t
		}
		if r.Verdicts.Overall.Malicious {
			out.Verdict, out.Malicious, out.Score = "malicious", true, 100
		}
		for _, a := range r.Meta.Processors.Wappa.Data {
			out.Technologies = append(out.Technologies, a.App)
		}
		return out, nil
	}
	return nil, fmt.Errorf("cloudflare url scanner: hasil belum siap (%s)", sub.UUID)
}
