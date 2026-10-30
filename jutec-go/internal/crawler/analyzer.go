package crawler

import (
	"strings"
)

type AnalysisResult struct {
	IsGambling     bool                  `json:"is_gambling"`
	Confidence     float64               `json:"confidence"`
	Priority       string                `json:"priority"`
	FoundKeywords  []string              `json:"found_keywords"`
	FoundPatterns  []string              `json:"found_patterns"`
	NetworkSignals *NetworkSignals       `json:"network_signals"`
	Fingerprint    *Fingerprint          `json:"fingerprint"`
	Artifacts      []TransactionArtifact `json:"artifacts"`
	Links          *ExtractedLinks       `json:"links"`
	TrackingIDs    []TrackingID          `json:"tracking_ids"`
	MetaTokens     []MetaToken           `json:"meta_tokens"`
	SimHash        string                `json:"simhash"`
	RedirectChain  []string              `json:"redirect_chain,omitempty"`
	AppPromo       *AppPromo             `json:"app_promo,omitempty"`
	MobileCrawled  bool                  `json:"mobile_crawled"` // profil HP Android ikut di-crawl
	Fingerprints   []InfraFingerprint         `json:"fingerprints,omitempty"`   // sidik jari CDN/favicon/API
	InternalPages  []string              `json:"internal_pages,omitempty"` // halaman internal yang ikut dianalisis
	AuthSurface    *AuthSurface          `json:"auth_surface,omitempty"`   // peta login & pendaftaran
	WidgetLinks    []string              `json:"widget_links,omitempty"`   // link dari pesan sambutan widget chat
}

type WeightedKeyword struct {
	Keyword string
	Weight  float64
}

type Analyzer struct {
	Patterns []string
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		Patterns: []string{"livechat", "whatsapp", "tele", "rtp-live"},
	}
}

// Analyze menganalisis HTML + network signals menggunakan keywords dari DB.
func (a *Analyzer) Analyze(html string, keywords []WeightedKeyword, network *NetworkSignals) *AnalysisResult {
	// htmlLower hanya untuk keyword/pattern matching — ExtractArtifacts butuh HTML asli
	// agar address crypto case-sensitive (TRON: T...) dan tag HTML bisa di-strip dengan benar.
	htmlLower := strings.ToLower(html)

	var foundKeywords []string
	keywordScore := 0.0
	for _, kw := range keywords {
		if strings.Contains(htmlLower, strings.ToLower(kw.Keyword)) {
			foundKeywords = append(foundKeywords, kw.Keyword)
			keywordScore += kw.Weight
		}
	}

	var foundPatterns []string
	for _, pt := range a.Patterns {
		if strings.Contains(htmlLower, pt) {
			foundPatterns = append(foundPatterns, pt)
		}
	}

	fingerprint := GenerateFingerprint(htmlLower)
	artifacts := ExtractArtifacts(html)

	// Network signal boost: setiap game provider terdeteksi = tambah 0.3
	networkBoost := 0.0
	if network != nil && len(network.GameProviders) > 0 {
		networkBoost = 0.3
	}

	confidence := keywordScore + networkBoost
	if confidence > 1.0 {
		confidence = 1.0
	}

	priority := "Low"
	hasPayment        := len(artifacts) > 0
	hasGameProvider   := network != nil && len(network.GameProviders) > 0
	hasStrongKeywords := keywordScore >= 0.4
	hasSomeKeywords   := keywordScore >= 0.2 // minimal 1 keyword terdeteksi

	if (hasStrongKeywords || hasGameProvider) && hasPayment {
		priority = "High"
		confidence = 0.95
	} else if hasStrongKeywords || hasGameProvider || len(foundPatterns) > 2 {
		priority = "Medium"
		if confidence < 0.6 {
			confidence = 0.6
		}
	} else if hasSomeKeywords || len(foundPatterns) >= 1 || hasPayment {
		priority = "Medium"
		if confidence < 0.4 {
			confidence = 0.4
		}
	}

	return &AnalysisResult{
		IsGambling:     confidence > 0.4,
		Confidence:     confidence,
		Priority:       priority,
		FoundKeywords:  foundKeywords,
		FoundPatterns:  foundPatterns,
		NetworkSignals: network,
		Fingerprint:    fingerprint,
		Artifacts:      artifacts,
	}
}

// AnalyzeOCRText khusus untuk teks mentah dari gambar (tanpa network signal/HTML)
func (a *Analyzer) AnalyzeOCRText(text string, keywords []WeightedKeyword) *AnalysisResult {
	text = strings.ToLower(text)

	var foundKeywords []string
	keywordScore := 0.0
	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw.Keyword)) {
			foundKeywords = append(foundKeywords, kw.Keyword)
			keywordScore += kw.Weight
		}
	}

	artifacts := ExtractArtifacts(text)

	// Batasi skor maksimal
	finalScore := keywordScore
	if finalScore > 1.0 {
		finalScore = 1.0
	}

	priority := "Low"
	isGambling := finalScore >= 0.5 || len(artifacts) > 0

	if finalScore >= 0.8 || (finalScore >= 0.5 && len(artifacts) > 0) {
		priority = "High"
	} else if finalScore >= 0.5 {
		priority = "Medium"
	}

	return &AnalysisResult{
		IsGambling:    isGambling,
		Confidence:    finalScore,
		Priority:      priority,
		FoundKeywords: foundKeywords,
		Artifacts:     artifacts,
	}
}
