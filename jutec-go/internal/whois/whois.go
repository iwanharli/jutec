package whois

import (
	"strings"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

// suspectTLDs adalah TLD yang sering dipakai situs judol karena murah dan mudah didapat.
var suspectTLDs = map[string]bool{
	".xyz": true, ".club": true, ".online": true, ".fun": true,
	".site": true, ".click": true, ".live": true, ".monster": true,
	".cyou": true, ".mom": true, ".top": true, ".icu": true,
	".vip": true, ".cam": true, ".biz": true, ".pw": true,
	".website": true, ".space": true, ".store": true, ".link": true,
	".win": true, ".bet": true, ".casino": true, ".poker": true,
}

type DomainIntel struct {
	DomainAgeDays int     `json:"domain_age_days"` // -1 jika tidak diketahui
	Registrar     string  `json:"registrar"`
	IsPrivate     bool    `json:"is_private"`     // privacy protection aktif
	TLD           string  `json:"tld"`
	IsSuspectTLD  bool    `json:"is_suspect_tld"`
	RiskScore     float64 `json:"risk_score"`
}

func Lookup(rawURL string) (*DomainIntel, error) {
	domain := cleanDomain(rawURL)

	raw, err := whois.Whois(domain)
	if err != nil {
		return fallback(domain), nil
	}

	result, err := whoisparser.Parse(raw)
	if err != nil {
		return fallback(domain), nil
	}

	intel := &DomainIntel{}

	// Umur domain
	if result.Domain != nil && result.Domain.CreatedDate != "" {
		if t, err := time.Parse(time.RFC3339, result.Domain.CreatedDate); err == nil {
			intel.DomainAgeDays = int(time.Since(t).Hours() / 24)
		} else {
			intel.DomainAgeDays = -1
		}
	} else {
		intel.DomainAgeDays = -1
	}

	// Registrar
	if result.Registrar != nil {
		intel.Registrar = result.Registrar.Name
	}

	// Privacy protection: cek apakah data registrant disembunyikan
	if result.Registrant != nil {
		combined := strings.ToLower(result.Registrant.Name + " " + result.Registrant.Organization)
		intel.IsPrivate = isPrivacyProtected(combined)
	} else {
		intel.IsPrivate = true // tidak ada info registrant = kemungkinan besar diproteksi
	}

	// TLD
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		intel.TLD = "." + parts[len(parts)-1]
		intel.IsSuspectTLD = suspectTLDs[intel.TLD]
	}

	intel.RiskScore = computeRisk(intel)
	return intel, nil
}

func isPrivacyProtected(registrantInfo string) bool {
	keywords := []string{
		"privacy", "proxy", "redacted", "whoisguard",
		"withheld", "protected", "private", "guard", "anonymous",
	}
	for _, kw := range keywords {
		if strings.Contains(registrantInfo, kw) {
			return true
		}
	}
	return false
}

// computeRisk menghitung risk score 0.0–1.0 berdasarkan sinyal domain.
func computeRisk(intel *DomainIntel) float64 {
	score := 0.0

	// Domain sangat muda = red flag (judol sering ganti domain karena kena blokir)
	if intel.DomainAgeDays >= 0 && intel.DomainAgeDays < 30 {
		score += 0.4
	} else if intel.DomainAgeDays >= 0 && intel.DomainAgeDays < 90 {
		score += 0.3
	} else if intel.DomainAgeDays >= 0 && intel.DomainAgeDays < 180 {
		score += 0.15
	}

	// Privacy protection aktif
	if intel.IsPrivate {
		score += 0.2
	}

	// TLD mencurigakan
	if intel.IsSuspectTLD {
		score += 0.2
	}

	if score > 1.0 {
		score = 1.0
	}
	return score
}

func fallback(domain string) *DomainIntel {
	parts := strings.Split(domain, ".")
	tld := ""
	if len(parts) >= 2 {
		tld = "." + parts[len(parts)-1]
	}
	intel := &DomainIntel{
		DomainAgeDays: -1,
		TLD:           tld,
		IsSuspectTLD:  suspectTLDs[tld],
		IsPrivate:     true,
	}
	intel.RiskScore = computeRisk(intel)
	return intel
}

func cleanDomain(raw string) string {
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	return strings.Split(raw, "/")[0]
}
