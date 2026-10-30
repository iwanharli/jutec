package intelligence

import (
	"net/url"
	"strings"

	"jutec-go/internal/crawler"
)

// Score adalah hasil final analisis gabungan dari semua sinyal.
type Score struct {
	Final      float64   `json:"final"`
	IsGambling bool      `json:"is_gambling"`
	Priority   string    `json:"priority"`
	Breakdown  Breakdown `json:"breakdown"`
	Rule       string    `json:"rule,omitempty"` // aturan yang menentukan prioritas (lihat switch di Compute)
}

// Breakdown menampilkan kontribusi masing-masing sinyal ke skor akhir.
type Breakdown struct {
	ContentScore    float64 `json:"content"`    // keyword + pattern + artifact + URL path
	NetworkScore    float64 `json:"network"`    // game provider API calls + template cluster
	DomainScore     float64 `json:"domain"`     // WHOIS: umur domain, privacy, TLD
	URLPathScore    float64 `json:"url_path"`   // kata judi di path/hostname URL
	TLDRisk         float64 `json:"tld_risk"`   // TLD berisiko tinggi (.top, .xyz, .cc, dll)
	SimHashBoost    float64 `json:"simhash"`    // kesamaan template dengan situs judol diketahui
	EnrichmentBoost float64 `json:"enrichment"` // URLScan verdict + VirusTotal malicious engines
	HostnameScore   float64 `json:"hostname"`   // istilah judol kuat di nama hostname/subdomain
}

// trustedDomains adalah platform/layanan resmi yang TIDAK PERNAH ditandai sebagai situs judol,
// meskipun user-generated content di dalamnya mengandung sinyal judol.
var trustedDomains = map[string]bool{
	// Platform UGC — judol sering posting di sini tapi domain-nya sendiri bukan judol
	"take.app": true, "bio.link": true, "linktree.com": true, "beacons.ai": true,
	"carrd.co": true, "systeme.io": true, "taplink.cc": true,
	"blogspot.com": true, "wordpress.com": true, "tumblr.com": true,
	"wix.com": true, "weebly.com": true, "squarespace.com": true,
	"notion.site": true, "sites.google.com": true,
	"heylink.me": true, "linktr.ee": true, "linkin.bio": true,
	// Dompet & payment resmi Indonesia
	"dana.id": true, "gopay.co.id": true, "ovo.id": true,
	"shopeepay.co.id": true, "linkaja.id": true, "bca.co.id": true,
	"bri.co.id": true, "mandiri.co.id": true, "bni.co.id": true,
	// Media & news
	"dw.com": true, "bbc.com": true, "reuters.com": true,
	"detik.com": true, "kompas.com": true, "cnnindonesia.com": true,
	"kompasiana.com": true, "tribunnews.com": true, "liputan6.com": true,
	"okezone.com": true, "merdeka.com": true, "suara.com": true,
	// Platform dev & tools
	"github.com": true, "gitlab.com": true, "vercel.app": true,
	"netlify.app": true, "apkpure.com": true, "aptoide.com": true,
	// Ad networks & analytics (bukan judol)
	"amillionads.com": true, "adserver.online": true,
	// Domain marketplace — halaman parkir/jual domain, bukan operator judol
	"expireddomains.com": true, "afternic.com": true, "dan.com": true,
	"sedo.com": true, "hugedomains.com": true, "godaddy.com": true,
	// Gambling regulator / research (bukan operator)
	"gamblingcommission.gov.uk": true, "kominfo.go.id": true,
}

// trustedTLDSuffixes adalah sufiks TLD pemerintah dan akademik yang tidak pernah
// menjadi operator judol (meski kontennya mungkin mengandung kata judi).
var trustedTLDSuffixes = []string{
	// Indonesia
	".go.id", ".ac.id", ".gov.id", ".mil.id", ".sch.id", ".desa.id",
	// International government
	".gov", ".gov.au", ".gov.uk", ".gov.ph", ".gov.co", ".gov.sg",
	".gov.my", ".gov.in", ".gov.br", ".gouv.fr",
	// Academic
	".edu", ".edu.br", ".edu.mx", ".edu.my", ".edu.ph", ".edu.co",
	".edu.au", ".edu.sg", ".edu.hk",
	".ac.uk", ".ac.nz", ".ac.jp", ".ac.id", // .ac.id already above but harmless
}

// gamblingPathTerms adalah kata-kata judi yang sering muncul di path/hostname URL.
var gamblingPathTerms = []string{
	"slot", "slots", "daftar", "deposit", "withdraw", "rtp",
	"togel", "toto", "casino", "poker", "baccarat", "roulette",
	"jackpot", "bonus", "promo", "spin", "taruhan", "betting",
	"sportsbook", "livecasino", "live-casino", "pragmatic",
	"gacor", "maxwin", "scatter",
}

// strongHostTerms adalah istilah judol yang tidak ambigu jika muncul di hostname/subdomain.
// Berbeda dari gamblingPathTerms, istilah ini hampir mustahil dipakai domain legitimate.
// Ketika ditemukan di hostname → hostnameScore = 0.40, langsung memenuhi threshold IsGambling.
var strongHostTerms = []string{
	"casino", "poker", "togel", "toto", "baccarat", "roulette",
	"sportsbook", "livecasino", "gacor", "maxwin",
	"judol", "judi", "taruhan", "betting", "jackpot", "scatter",
}

// riskyTLDs adalah TLD yang sangat umum dipakai operator judol.
// Nilai = floor minimum untuk domain score jika TLD cocok.
var riskyTLDs = map[string]float64{
	".top":    0.50,
	".vip":    0.45,
	".cc":     0.45,
	".pw":     0.50,
	".xyz":    0.40,
	".live":   0.40,
	".icu":    0.40,
	".online": 0.35,
	".site":   0.35,
	".biz":    0.30,
	".ws":     0.30,
}

// computeURLSignals menganalisis URL untuk sinyal judi dari path dan TLD.
func computeURLSignals(rawURL string) (pathScore, tldRisk float64) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}

	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(u.Path)
	combined := host + path

	hits := 0
	for _, term := range gamblingPathTerms {
		if strings.Contains(combined, term) {
			hits++
		}
	}
	pathScore = float64(hits) * 0.15
	if pathScore > 0.45 {
		pathScore = 0.45
	}

	for tld, risk := range riskyTLDs {
		if strings.HasSuffix(host, tld) {
			tldRisk = risk
			break
		}
	}
	return
}

// artifactQuality menghitung kualitas artifacts berdasarkan tipe.
// Bank/E-Wallet/Contact = sinyal kuat judol. Crypto = sinyal lemah (banyak false positive).
func artifactQuality(artifacts []crawler.TransactionArtifact) (hasStrong bool, hasWeak bool) {
	for _, a := range artifacts {
		switch a.Type {
		case "Bank", "E-Wallet", "VirtualAccount", "QRIS":
			hasStrong = true
		// Kontak (Discord/WA/TG) & nomor HP umum di situs biasa — hanya sinyal pendukung,
		// agar situs seperti chess.com tidak ter-flag judol hanya karena punya link Discord.
		case "Crypto", "Phone", "Contact":
			hasWeak = true
		}
	}
	return
}

// Compute menghitung composite score dari semua sinyal.
//
// Bobot: content 45% · network 30% · domain 25%
// Sinyal tambahan: URL path terms, TLD risk, SimHash cluster match, enrichment (VT+URLScan).
func Compute(analysis *crawler.AnalysisResult, domainRisk float64, rawURL string, simHashBoost float64, enrichmentBoost float64) *Score {
	// Trusted domain — platform resmi atau UGC host, tidak pernah judol
	u, err := url.Parse(rawURL)
	if err != nil || u == nil || u.Host == "" {
		return &Score{Final: 0.0, IsGambling: false, Priority: "Low"}
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if trustedDomains[host] {
		return &Score{Final: 0.0, IsGambling: false, Priority: "Low"}
	}

	// Cek apakah ini domain pemerintah / akademik
	isGovEdu := false
	for _, sfx := range trustedTLDSuffixes {
		if strings.HasSuffix(host, sfx) {
			isGovEdu = true
			break
		}
	}

	urlPathScore, tldRisk := computeURLSignals(rawURL)

	// Hostname mengandung istilah judol kuat = domain sengaja didaftarkan untuk judol.
	// Ini menangkap situs yang diblokir chromedp sehingga konten tidak bisa dibaca,
	// tapi nama domainnya sendiri sudah jelas (casino-n1.com, scatterhitam.systeme.io, dll).
	hostnameScore := 0.0
	for _, term := range strongHostTerms {
		if strings.Contains(host, term) {
			hostnameScore = 0.40
			break
		}
	}

	contentScore := analysis.Confidence + urlPathScore
	if contentScore > 1.0 {
		contentScore = 1.0
	}

	networkScore := 0.0
	if analysis.NetworkSignals != nil && len(analysis.NetworkSignals.GameProviders) > 0 {
		networkScore = float64(len(analysis.NetworkSignals.GameProviders)) * 0.3
		if networkScore > 1.0 {
			networkScore = 1.0
		}
	}
	if simHashBoost > networkScore {
		networkScore = simHashBoost
	}

	domainScore := domainRisk
	if tldRisk > domainScore {
		domainScore = tldRisk
	}

	final := contentScore*0.45 + networkScore*0.30 + domainScore*0.25 + enrichmentBoost + hostnameScore
	if final > 1.0 {
		final = 1.0
	}

	priority := "Low"
	rule := "low"
	// Hanya provider game yang benar-benar terdeteksi — networkScore juga memuat simHashBoost,
	// yang sudah diwakili hasTemplateMatch (halaman berita generik bisa mirip template judol).
	hasGameProvider := analysis.NetworkSignals != nil && len(analysis.NetworkSignals.GameProviders) > 0
	hasStrongArtifact, hasWeakArtifact := artifactQuality(analysis.Artifacts)
	// Crypto-only artifacts tanpa konten judol = sinyal lemah, tidak cukup untuk High
	hasArtifacts := hasStrongArtifact || (hasWeakArtifact && contentScore >= 0.5)
	hasStrongContent := contentScore >= 0.7
	hasTemplateMatch := simHashBoost > 0
	hasEnrichmentSignal := enrichmentBoost >= 0.20
	hasHostnameSignal := hostnameScore > 0
	// Situs sah yang disusupi: cloaking atau spam link judol tersembunyi. Konten judol di sub-path
	// domain institusi (/wp-content/slot-gacor/) dengan konten kuat juga tanda susupan.
	injected := analysis.Injection != nil && analysis.Injection.Detected
	injectedPath := isGovEdu && urlPathScore > 0 && contentScore >= 0.5

	switch {
	case injected || injectedPath:
		priority = "High"
		rule = "injected"
		if final < 0.8 {
			final = 0.8
		}
	case hasGameProvider && hasArtifacts:
		priority = "High"
		rule = "game_provider+artifact"
		final = 0.97
	case hasStrongContent && hasArtifacts:
		priority = "High"
		rule = "content+artifact"
		if final < 0.88 {
			final = 0.88
		}
	case hasTemplateMatch && hasArtifacts:
		priority = "High"
		rule = "template+artifact"
		if final < 0.85 {
			final = 0.85
		}
	case hasEnrichmentSignal && hasArtifacts:
		priority = "High"
		rule = "enrichment+artifact"
		if final < 0.82 {
			final = 0.82
		}
	case hasHostnameSignal && hasArtifacts:
		priority = "High"
		rule = "hostname+artifact"
		if final < 0.80 {
			final = 0.80
		}
	case final >= 0.75 || hasGameProvider || hasStrongContent || hasEnrichmentSignal:
		priority = "High"
		rule = "strong_signal"
	// Kemiripan template saja (tanpa artefak) bisa kebetulan pada halaman generik — cukup Medium
	case final >= 0.45 || tldRisk >= 0.4 || urlPathScore >= 0.3 || hasHostnameSignal || hasTemplateMatch:
		priority = "Medium"
		rule = "medium_signal"
	}

	// IsGambling:
	// - gov/edu: hanya jika ada bukti teknis/finansial kuat (bukan sekadar konten/keyword)
	// - domain biasa: threshold 0.4, strong artifact, atau hostname mengandung istilah judol kuat
	var isGambling bool
	if isGovEdu {
		isGambling = hasGameProvider || hasStrongArtifact || hasTemplateMatch || injected || injectedPath
	} else {
		// "Rekening" tanpa konten judi sama sekali (situs bank, berita keuangan) bukan bukti judol
		// Konten judi kuat (≥70% = minimal 2 kata kunci) cukup — landing page judol sering tanpa rekening/provider
		isGambling = final >= 0.4 || hasStrongContent || (hasStrongArtifact && contentScore >= 0.2) || hasHostnameSignal || injected
	}
	// High hanya untuk situs yang dinilai judol; sinyal kuat tanpa vonis judol (mis. berita
	// pemberantasan judol di situs pemerintah) cukup Medium agar tetap ditinjau tanpa terlihat pasti judol.
	if !isGambling && priority == "High" {
		priority = "Medium"
		rule = "not_gambling_cap"
	}

	return &Score{
		Final:      final,
		IsGambling: isGambling,
		Priority:   priority,
		Rule:       rule,
		Breakdown: Breakdown{
			ContentScore:    contentScore,
			NetworkScore:    networkScore,
			DomainScore:     domainScore,
			URLPathScore:    urlPathScore,
			TLDRisk:         tldRisk,
			SimHashBoost:    simHashBoost,
			EnrichmentBoost: enrichmentBoost,
			HostnameScore:   hostnameScore,
		},
	}
}
