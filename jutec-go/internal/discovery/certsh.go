package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CertScraper struct {
	client *http.Client
}

func NewCertScraper() *CertScraper {
	return &CertScraper{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type certEntry struct {
	NameValue string `json:"name_value"`
}

// Search mencari domain dari Certificate Transparency logs via crt.sh.
// keyword bisa berupa domain pattern (misal: "slot-gacor") atau wildcard ("%.judol.com").
func (s *CertScraper) Search(keyword string) ([]string, error) {
	apiURL := fmt.Sprintf("https://crt.sh/?q=%%%s%%&output=json", url.QueryEscape(keyword))

	resp, err := s.client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("crt.sh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crt.sh returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var entries []certEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse crt.sh response: %w", err)
	}

	log.Printf("CT Discovery: crt.sh returned %d cert entries for keyword '%s'", len(entries), keyword)
	return extractUniqueDomains(entries), nil
}

var ignoredDomains = []string{
	"google.com", "youtube.com", "facebook.com", "cloudflare.com",
	"amazonaws.com", "microsoft.com", "apple.com", "akamai.com",
	"fastly.net", "github.com", "twitter.com", "instagram.com",
}

func extractUniqueDomains(entries []certEntry) []string {
	unique := make(map[string]bool)

	for _, e := range entries {
		// NameValue bisa berisi beberapa domain dipisah newline
		for _, name := range strings.Split(e.NameValue, "\n") {
			name = strings.TrimSpace(name)
			name = strings.TrimPrefix(name, "*.") // hapus wildcard prefix

			if name == "" || strings.Contains(name, " ") || !strings.Contains(name, ".") {
				continue
			}

			ignored := false
			for _, ign := range ignoredDomains {
				if strings.Contains(name, ign) {
					ignored = true
					break
				}
			}

			if !ignored && !unique[name] {
				unique[name] = true
			}
		}
	}

	domains := make([]string, 0, len(unique))
	for d := range unique {
		domains = append(domains, "https://"+d)
	}
	return domains
}
