package tron

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// USDT-ERC20 lewat Etherscan API v2. Paket gratis Etherscan hanya mencakup jaringan Ethereum
// (BSC/BEP20 butuh paket berbayar), jadi alamat 0x diprofilkan di Ethereum saja.
const (
	etherscanURL     = "https://api.etherscan.io/v2/api"
	USDTERC20        = "0xdAC17F958D2ee523a2206206994597C13D831ec7"
	etherscanPerPage = 1000
)

// IsEVM: alamat 0x… (Ethereum/BSC).
func IsEVM(addr string) bool {
	return strings.HasPrefix(strings.ToLower(addr), "0x") && len(addr) == 42
}

// ERC20Profile meringkas transfer USDT-ERC20 terbaru sebuah alamat (maks. maxPages×1000).
func ERC20Profile(address string, maxPages int) (*Profile, error) {
	key := os.Getenv("ETHERSCAN_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("kunci Etherscan belum diisi")
	}
	self := strings.ToLower(address)
	p := &Profile{Address: address, Network: "ERC20", Complete: true, FetchedAt: time.Now()}
	in, out := map[string]*Counterparty{}, map[string]*Counterparty{}
	hc := &http.Client{Timeout: 30 * time.Second}
	for page := 1; page <= maxPages; page++ {
		q := url.Values{"chainid": {"1"}, "module": {"account"}, "action": {"tokentx"}, "contractaddress": {USDTERC20},
			"address": {address}, "page": {fmt.Sprint(page)}, "offset": {fmt.Sprint(etherscanPerPage)}, "sort": {"desc"}, "apikey": {key}}
		resp, err := hc.Get(etherscanURL + "?" + q.Encode())
		if err != nil {
			return nil, err
		}
		var res struct {
			Status  string          `json:"status"`
			Message string          `json:"message"`
			Result  json.RawMessage `json:"result"`
		}
		err = json.NewDecoder(resp.Body).Decode(&res)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var txs []struct {
			From, To, Value, TimeStamp, TokenDecimal string
		}
		if res.Status != "1" {
			if strings.Contains(strings.ToLower(res.Message), "no transactions") {
				break
			}
			var msg string
			json.Unmarshal(res.Result, &msg)
			if strings.Contains(strings.ToLower(msg), "rate limit") {
				return nil, ErrRateLimit
			}
			return nil, fmt.Errorf("etherscan: %s %s", res.Message, msg)
		}
		json.Unmarshal(res.Result, &txs)
		for _, t := range txs {
			dec := 6
			fmt.Sscan(t.TokenDecimal, &dec)
			amt := amount(t.Value, dec)
			var sec int64
			fmt.Sscan(t.TimeStamp, &sec)
			ts := sec * 1000
			p.Transfers++
			if ts > p.LastAt {
				p.LastAt = ts
			}
			if p.FirstAt == 0 || ts < p.FirstAt {
				p.FirstAt = ts
			}
			if strings.ToLower(t.To) == self {
				p.TotalIn += amt
				add(in, t.From, amt, ts)
			} else if strings.ToLower(t.From) == self {
				p.TotalOut += amt
				add(out, t.To, amt, ts)
			}
		}
		if len(txs) < etherscanPerPage {
			break
		}
		if page == maxPages {
			p.Complete = false
		}
		time.Sleep(400 * time.Millisecond) // batas gratis 3 kueri/detik
	}
	p.TopSenders, p.TopReceivers = top(in, 10), top(out, 10)
	return p, nil
}

// AnyProfile memilih jaringan dari bentuk alamat: T… → TRC20 (TronGrid), 0x… → ERC20 (Etherscan).
func (c *Client) AnyProfile(address string, maxPages int) (*Profile, error) {
	if IsEVM(address) {
		return ERC20Profile(address, maxPages)
	}
	return c.USDTProfile(address, maxPages)
}
