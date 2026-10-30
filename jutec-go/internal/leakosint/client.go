package leakosint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const apiURL = "https://leakosintapi.com/"

type Client struct {
	Token  string
	HTTP   *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		Token: token,
		HTTP:  &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Request / Response structs ---

type apiRequest struct {
	Token   string `json:"token"`
	Request string `json:"request"`
	Lang    string `json:"lang"`
}

// KomInfoEntry adalah satu baris data dari sumber "KomInfo Indonesia".
type KomInfoEntry struct {
	Phone    string `json:"Phone"`
	NIK      string `json:"Passport"`
	Provider string `json:"Provider"`
	RegDate  string `json:"RegDate"`
}

// DukcapilEntry adalah satu baris data dari sumber "Dukcapil".
type DukcapilEntry struct {
	Phone     string `json:"Phone"`
	NIK       string `json:"Passport"`
	FullName  string `json:"FullName"`
	Gender    string `json:"Gender"`
	BirthDate string `json:"BDay"`
	Address   string `json:"Address"`
	District  string `json:"District"`
	Location  string `json:"Location"`
	Region    string `json:"Region"`
}

// LookupResult berisi hasil parsing dari LeakOSINT API.
type LookupResult struct {
	Phone    string          // nomor yang di-query
	KomInfo  []KomInfoEntry
	Dukcapil []DukcapilEntry
}

// normalizePhone mengubah nomor HP Indonesia ke format 628xxx yang dipakai LeakOSINT.
// 08xxx → 628xxx, +628xxx → 628xxx, format lain tidak diubah.
func normalizePhone(phone string) string {
	if len(phone) > 1 && phone[0] == '+' {
		phone = phone[1:] // strip leading +
	}
	if len(phone) > 1 && phone[0] == '0' {
		phone = "62" + phone[1:]
	}
	return phone
}

// Lookup melakukan query ke LeakOSINT API dan parse hasilnya.
func (c *Client) Lookup(phone string) (*LookupResult, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("leakosint token not configured")
	}

	phone = normalizePhone(phone)

	payload, _ := json.Marshal(apiRequest{
		Token:   c.Token,
		Request: phone,
		Lang:    "id",
	})

	resp, err := c.HTTP.Post(apiURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("leakosint request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("leakosint api error %d: %s", resp.StatusCode, string(body))
	}

	return parseResponse(phone, body)
}

// parseResponse extracts KomInfo Indonesia and Dukcapil data from the raw API response.
func parseResponse(phone string, body []byte) (*LookupResult, error) {
	// Struktur response: {"List": {"KomInfo Indonesia": {"Data": [...]}, "Dukcapil": {"Data": [...]}}}
	var raw struct {
		List map[string]json.RawMessage `json:"List"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := &LookupResult{Phone: phone}

	// Parse KomInfo Indonesia
	if komRaw, ok := raw.List["KomInfo Indonesia"]; ok {
		var source struct {
			Data []KomInfoEntry `json:"Data"`
		}
		if err := json.Unmarshal(komRaw, &source); err == nil {
			result.KomInfo = source.Data
			log.Printf("[LeakOSINT] Phone %s: Found %d KomInfo entries", phone, len(source.Data))
		}
	}

	// Parse Dukcapil
	if dukRaw, ok := raw.List["Dukcapil"]; ok {
		var source struct {
			Data []DukcapilEntry `json:"Data"`
		}
		if err := json.Unmarshal(dukRaw, &source); err == nil {
			result.Dukcapil = source.Data
			log.Printf("[LeakOSINT] Phone %s: Found %d Dukcapil entries", phone, len(source.Data))
		}
	}

	return result, nil
}
