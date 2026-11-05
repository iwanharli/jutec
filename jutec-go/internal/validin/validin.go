// Package validin membaca riwayat DNS (IP yang pernah dipakai domain) dari Validin — dipakai mencari
// IP asli situs di balik Cloudflare: IP lama sebelum situs pindah ke CDN sering masih server asalnya.
// Butuh VALIDIN_API_KEY (akun gratis berkuota harian).
package validin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

var (
	ErrNotConfigured = errors.New("validin belum dikonfigurasi")
	ErrRateLimit     = errors.New("validin: kuota/rate limit")
	client           = &http.Client{Timeout: 30 * time.Second}
)

func Configured() bool { return os.Getenv("VALIDIN_API_KEY") != "" }

type Record struct {
	IP        string `json:"ip"`
	FirstSeen int64  `json:"first_seen"`
	LastSeen  int64  `json:"last_seen"`
}

// AHistory: semua IPv4 yang pernah tercatat untuk domain.
func AHistory(domain string) ([]Record, error) {
	if !Configured() {
		return nil, ErrNotConfigured
	}
	req, _ := http.NewRequest("GET", "https://app.validin.com/api/axon/domain/dns/history/"+domain+"/A", nil)
	req.Header.Set("Authorization", "BEARER "+os.Getenv("VALIDIN_API_KEY"))
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == 429 || res.StatusCode == 402 {
		return nil, ErrRateLimit
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("validin: HTTP %d", res.StatusCode)
	}
	var body struct {
		Records map[string][]struct {
			Value     string `json:"value"`
			FirstSeen int64  `json:"first_seen"`
			LastSeen  int64  `json:"last_seen"`
		} `json:"records"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}
	var out []Record
	for _, rs := range body.Records {
		for _, r := range rs {
			out = append(out, Record{IP: r.Value, FirstSeen: r.FirstSeen, LastSeen: r.LastSeen})
		}
	}
	return out, nil
}
