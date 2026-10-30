package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type GoogleScraper struct {
	APIKey string
	Pages  int

	// SerpApiKey (serpapi.com) dipakai sebagai cadangan saat Serper habis kredit / kena limit.
	SerpApiKey string

	mu             sync.Mutex
	serperDownTill time.Time // selama ini Serper dilewati agar kuota SerpApi tak terbuang percobaan gagal
}

// serperCooldown: lama Serper dilewati setelah membalas error kuota/limit.
const serperCooldown = time.Hour

func NewGoogleScraper(apiKey string, pages int) *GoogleScraper {
	if pages <= 0 {
		pages = 5
	}
	return &GoogleScraper{
		APIKey: apiKey,
		Pages:  pages,
	}
}

type SerperResponse struct {
	Organic []struct {
		Link string `json:"link"`
	} `json:"organic"`
}

func (s *GoogleScraper) Search(keyword string) ([]string, error) {
	return s.SearchPages(keyword, s.Pages)
}

// SearchPages mengambil beberapa halaman dari Serper API.
// Setiap halaman = 1 API credit, menghasilkan ~10 organic results.
// pages=0 berarti tanpa limit (sampai habis, max 10 halaman safety).
func (s *GoogleScraper) SearchPages(keyword string, pages int) ([]string, error) {
	if s.APIKey == "" && s.SerpApiKey == "" {
		return nil, fmt.Errorf("serper api key is not configured")
	}

	if pages <= 0 {
		pages = 10 // safety max
	}

	var allRawURLs []string

	for page := 1; page <= pages; page++ {
		if !s.serperAvailable() {
			links, ok := s.serpApiPage(keyword, page)
			if !ok || len(links) == 0 {
				break
			}
			allRawURLs = append(allRawURLs, links...)
			if len(links) < 10 {
				break
			}
			continue
		}

		payload := map[string]interface{}{
			"q":    keyword,
			"num":  10,
			"page": page,
		}

		jsonData, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "https://google.serper.dev/search", bytes.NewBuffer(jsonData))
		req.Header.Add("X-API-KEY", s.APIKey)
		req.Header.Add("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("❌ [Serper] Page %d error for '%s': %v", page, keyword, err)
			break
		}

		body, _ := io.ReadAll(res.Body)
		res.Body.Close()

		if res.StatusCode != 200 {
			log.Printf("❌ [Serper] Page %d status %d for '%s'", page, res.StatusCode, keyword)
			// 400 "Not enough credits", 402/403 kuota habis, 429 rate limit → pindah ke SerpApi
			if isQuotaStatus(res.StatusCode) && s.SerpApiKey != "" {
				s.markSerperDown()
				page-- // ulangi halaman yang sama via SerpApi
				continue
			}
			break
		}

		var result SerperResponse
		if err := json.Unmarshal(body, &result); err != nil {
			break
		}

		if len(result.Organic) == 0 {
			break // tidak ada hasil lagi
		}

		for _, item := range result.Organic {
			allRawURLs = append(allRawURLs, item.Link)
		}

		// Jika hasil < 10, berarti sudah halaman terakhir
		if len(result.Organic) < 10 {
			break
		}
	}

	cleaned := s.cleanURLs(allRawURLs)
	log.Printf("🔎 [Serper] Keyword: '%s' | Raw: %d | Unique: %d", keyword, len(allRawURLs), len(cleaned))
	return cleaned, nil
}

func isQuotaStatus(code int) bool {
	return code == 400 || code == 401 || code == 402 || code == 403 || code == 429
}

func (s *GoogleScraper) serperAvailable() bool {
	if s.APIKey == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Now().After(s.serperDownTill)
}

func (s *GoogleScraper) markSerperDown() {
	s.mu.Lock()
	s.serperDownTill = time.Now().Add(serperCooldown)
	s.mu.Unlock()
	log.Printf("⚠️  [Serper] Kuota/limit habis — beralih ke SerpApi selama %s", serperCooldown)
}

// serpApiPage mengambil satu halaman hasil Google lewat SerpApi (serpapi.com).
func (s *GoogleScraper) serpApiPage(keyword string, page int) ([]string, bool) {
	if s.SerpApiKey == "" {
		return nil, false
	}
	q := url.Values{
		"engine":  {"google"},
		"q":       {keyword},
		"num":     {"10"},
		"start":   {fmt.Sprint((page - 1) * 10)},
		"api_key": {s.SerpApiKey},
	}
	res, err := http.Get("https://serpapi.com/search.json?" + q.Encode())
	if err != nil {
		log.Printf("❌ [SerpApi] Page %d error for '%s': %v", page, keyword, err)
		return nil, false
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	var result struct {
		Error          string `json:"error"`
		OrganicResults []struct {
			Link string `json:"link"`
		} `json:"organic_results"`
	}
	if err := json.Unmarshal(body, &result); err != nil || res.StatusCode != 200 || result.Error != "" {
		log.Printf("❌ [SerpApi] Page %d status %d for '%s': %s", page, res.StatusCode, keyword, result.Error)
		return nil, false
	}
	links := make([]string, 0, len(result.OrganicResults))
	for _, r := range result.OrganicResults {
		links = append(links, r.Link)
	}
	log.Printf("🔁 [SerpApi] Page %d '%s' → %d hasil", page, keyword, len(links))
	return links, true
}

func (s *GoogleScraper) cleanURLs(input []string) []string {
	unique := make(map[string]bool)
	var cleaned []string

	for _, raw := range input {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}

		host := u.Host
		ignore := []string{"google.com", "youtube.com", "facebook.com", "twitter.com", "instagram.com", "linkedin.com", "apple.com", "microsoft.com"}
		
		isIgnored := false
		for _, ign := range ignore {
			if strings.Contains(host, ign) {
				isIgnored = true
				break
			}
		}

		if !isIgnored {
			domain := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			if !unique[domain] {
				unique[domain] = true
				cleaned = append(cleaned, domain)
			}
		}
	}

	return cleaned
}
