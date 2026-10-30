// Package tron membaca riwayat transfer USDT-TRC20 sebuah alamat dari TronGrid dan meringkasnya
// menjadi profil: volume masuk/keluar, rentang aktif, dan lawan transaksi terbesar.
package tron

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"time"
)

const (
	baseURL = "https://api.trongrid.io"
	// USDTContract adalah kontrak resmi Tether di jaringan TRON.
	USDTContract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	pageSize     = 200
)

var ErrRateLimit = errors.New("trongrid: rate limit")

type Client struct {
	APIKey string // opsional (header TRON-PRO-API-KEY) — tanpa key rate limit lebih ketat
	HTTP   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{APIKey: apiKey, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

type transfer struct {
	TxID      string `json:"transaction_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Value     string `json:"value"`
	Timestamp int64  `json:"block_timestamp"`
	TokenInfo struct {
		Decimals int `json:"decimals"`
	} `json:"token_info"`
}

// Counterparty adalah alamat lawan transaksi beserta total nilainya.
type Counterparty struct {
	Address string  `json:"address"`
	Amount  float64 `json:"amount"` // USDT
	Count   int     `json:"count"`
	LastAt  int64   `json:"last_at"`
}

// Profile adalah ringkasan aktivitas USDT satu alamat dari sampel transfer terbaru.
type Profile struct {
	Address      string         `json:"address"`
	Network      string         `json:"network,omitempty"` // TRC20 | ERC20
	Transfers    int            `json:"transfers"`     // jumlah transfer dalam sampel
	Complete     bool           `json:"complete"`      // false = riwayat lebih panjang dari sampel
	TotalIn      float64        `json:"total_in"`      // USDT
	TotalOut     float64        `json:"total_out"`     // USDT
	FirstAt      int64          `json:"first_at"`      // ms, transfer tertua dalam sampel
	LastAt       int64          `json:"last_at"`       // ms
	TopSenders   []Counterparty `json:"top_senders"`   // pengirim terbesar (deposit pemain / setoran)
	TopReceivers []Counterparty `json:"top_receivers"` // penerima terbesar (tujuan pengumpulan dana)
	FetchedAt    time.Time      `json:"fetched_at"`
}

// USDTProfile mengambil hingga maxPages×200 transfer USDT terbaru lalu meringkasnya.
func (c *Client) USDTProfile(address string, maxPages int) (*Profile, error) {
	p := &Profile{Address: address, Network: "TRC20", Complete: true, FetchedAt: time.Now()}
	in, out := map[string]*Counterparty{}, map[string]*Counterparty{}
	fingerprint := ""
	for page := 0; page < maxPages; page++ {
		q := url.Values{"limit": {fmt.Sprint(pageSize)}, "contract_address": {USDTContract}, "only_confirmed": {"true"}}
		if fingerprint != "" {
			q.Set("fingerprint", fingerprint)
		}
		var res struct {
			Data    []transfer `json:"data"`
			Success bool       `json:"success"`
			Error   string     `json:"error"`
			Meta    struct {
				Fingerprint string `json:"fingerprint"`
			} `json:"meta"`
		}
		if err := c.get(fmt.Sprintf("%s/v1/accounts/%s/transactions/trc20?%s", baseURL, address, q.Encode()), &res); err != nil {
			return nil, err
		}
		if !res.Success && res.Error != "" {
			return nil, fmt.Errorf("trongrid: %s", res.Error)
		}
		for _, t := range res.Data {
			amt := amount(t.Value, t.TokenInfo.Decimals)
			p.Transfers++
			if p.LastAt == 0 || t.Timestamp > p.LastAt {
				p.LastAt = t.Timestamp
			}
			if p.FirstAt == 0 || t.Timestamp < p.FirstAt {
				p.FirstAt = t.Timestamp
			}
			if t.To == address {
				p.TotalIn += amt
				add(in, t.From, amt, t.Timestamp)
			} else if t.From == address {
				p.TotalOut += amt
				add(out, t.To, amt, t.Timestamp)
			}
		}
		fingerprint = res.Meta.Fingerprint
		if fingerprint == "" || len(res.Data) < pageSize {
			break
		}
		if page == maxPages-1 {
			p.Complete = false
		}
		time.Sleep(300 * time.Millisecond) // sopan terhadap rate limit tanpa key
	}
	p.TopSenders = top(in, 10)
	p.TopReceivers = top(out, 10)
	return p, nil
}

func (c *Client) get(u string, v any) error {
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 || resp.StatusCode == 403 {
		return ErrRateLimit
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("trongrid: HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func amount(raw string, decimals int) float64 {
	v, ok := new(big.Float).SetString(raw)
	if !ok {
		return 0
	}
	if decimals == 0 {
		decimals = 6 // USDT-TRC20
	}
	f, _ := new(big.Float).Quo(v, new(big.Float).SetFloat64(pow10(decimals))).Float64()
	return f
}

func pow10(n int) float64 {
	r := 1.0
	for i := 0; i < n; i++ {
		r *= 10
	}
	return r
}

func add(m map[string]*Counterparty, addr string, amt float64, ts int64) {
	c := m[addr]
	if c == nil {
		c = &Counterparty{Address: addr}
		m[addr] = c
	}
	c.Amount += amt
	c.Count++
	if ts > c.LastAt {
		c.LastAt = ts
	}
}

func top(m map[string]*Counterparty, n int) []Counterparty {
	out := make([]Counterparty, 0, len(m))
	for _, c := range m {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	if len(out) > n {
		out = out[:n]
	}
	return out
}
