package virustotal

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://www.virustotal.com/api/v3"

// ErrRateLimit dikembalikan saat API mengembalikan 429. Caller tidak perlu retry segera.
var ErrRateLimit = errors.New("virustotal: rate limit exceeded")

// ErrNotFound dikembalikan saat resource belum pernah dianalisis VT.
var ErrNotFound = errors.New("virustotal: resource not found")

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

// AnalysisResult berisi ringkasan hasil analisis dari VirusTotal.
type AnalysisResult struct {
	Resource        string
	ResourceType    string // url, domain, apk
	MaliciousCount  int
	SuspiciousCount int
	HarmlessCount   int
	TotalEngines    int
	Categories      []string
	Permalink       string
	AnalysisDate    time.Time
}

func (c *Client) do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-apikey", c.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return c.HTTP.Do(req)
}

func checkStatus(resp *http.Response) error {
	switch resp.StatusCode {
	case 200, 201:
		return nil
	case 429:
		return ErrRateLimit
	case 404:
		return ErrNotFound
	default:
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("virustotal: status %d: %s", resp.StatusCode, string(b))
	}
}

// urlID menghitung ID URL untuk VT API: base64url tanpa padding.
func urlID(rawURL string) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString([]byte(rawURL)), "=")
}

// CheckURL mengambil hasil analisis URL yang sudah ada di VT.
// Jika belum ada → submit lalu return hasil awal.
func (c *Client) CheckURL(rawURL string) (*AnalysisResult, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("virustotal: api key not configured")
	}

	resp, err := c.do("GET", "/urls/"+urlID(rawURL), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		if errors.Is(err, ErrNotFound) {
			// Belum ada di VT — submit untuk future analysis
			return c.submitURL(rawURL)
		}
		return nil, err
	}

	return parseURLResponse(resp.Body, rawURL, "url")
}

// CheckDomain mengambil laporan domain dari VT.
func (c *Client) CheckDomain(domain string) (*AnalysisResult, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("virustotal: api key not configured")
	}

	resp, err := c.do("GET", "/domains/"+domain, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return nil, err
	}

	return parseURLResponse(resp.Body, domain, "domain")
}

// submitURL mengirimkan URL ke VT untuk analisis. Mengembalikan hasil awal (mungkin kosong).
func (c *Client) submitURL(rawURL string) (*AnalysisResult, error) {
	body := strings.NewReader(url.Values{"url": {rawURL}}.Encode())
	resp, err := c.do("POST", "/urls", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return nil, err
	}

	// Submission diterima — kembalikan placeholder, hasil akan tersedia nanti
	return &AnalysisResult{
		Resource:     rawURL,
		ResourceType: "url",
	}, nil
}

// parseURLResponse mem-parse response VT untuk URL atau domain.
func parseURLResponse(r io.Reader, resource, resourceType string) (*AnalysisResult, error) {
	var body struct {
		Data struct {
			Attributes struct {
				LastAnalysisStats struct {
					Malicious  int `json:"malicious"`
					Suspicious int `json:"suspicious"`
					Harmless   int `json:"harmless"`
					Undetected int `json:"undetected"`
				} `json:"last_analysis_stats"`
				Categories       map[string]string `json:"categories"`
				LastAnalysisDate int64             `json:"last_analysis_date"`
			} `json:"attributes"`
			Links struct {
				Self string `json:"self"`
			} `json:"links"`
		} `json:"data"`
	}

	if err := json.NewDecoder(r).Decode(&body); err != nil {
		return nil, err
	}

	a := body.Data.Attributes
	total := a.LastAnalysisStats.Malicious + a.LastAnalysisStats.Suspicious +
		a.LastAnalysisStats.Harmless + a.LastAnalysisStats.Undetected

	cats := make([]string, 0, len(a.Categories))
	for _, v := range a.Categories {
		cats = append(cats, v)
	}

	analysisDate := time.Time{}
	if a.LastAnalysisDate > 0 {
		analysisDate = time.Unix(a.LastAnalysisDate, 0)
	}

	return &AnalysisResult{
		Resource:        resource,
		ResourceType:    resourceType,
		MaliciousCount:  a.LastAnalysisStats.Malicious,
		SuspiciousCount: a.LastAnalysisStats.Suspicious,
		HarmlessCount:   a.LastAnalysisStats.Harmless,
		TotalEngines:    total,
		Categories:      cats,
		Permalink:       body.Data.Links.Self,
		AnalysisDate:    analysisDate,
	}, nil
}
