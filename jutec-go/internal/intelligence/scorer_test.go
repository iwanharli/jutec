package intelligence

import (
	"testing"

	"jutec-go/internal/crawler"
)

// Kasus-kasus diambil dari salah deteksi & deteksi benar yang pernah ditemukan di data nyata.
// Bila aturan scorer diubah, test ini menjaga agar kesalahan lama tidak muncul lagi.

func analysis(conf float64, providers []string, arts ...crawler.TransactionArtifact) *crawler.AnalysisResult {
	return &crawler.AnalysisResult{Confidence: conf, Artifacts: arts, NetworkSignals: &crawler.NetworkSignals{GameProviders: providers}}
}

var (
	waContact = crawler.TransactionArtifact{Type: "Contact", Name: "WhatsApp", Value: "6281200000002"}
	bank      = crawler.TransactionArtifact{Type: "Bank", Name: "BCA", Value: "1234567891"}
	qris      = crawler.TransactionArtifact{Type: "QRIS", Name: "Toko", Value: "ID1023456789012"}
)

func TestScorerFalsePositives(t *testing.T) {
	cases := []struct {
		name               string
		a                  *crawler.AnalysisResult
		url                string
		domainRisk, simhash float64
	}{
		// otomotif.tempo.co: halaman berita mirip template judol + nomor WA dari kutipan artikel
		{"berita: template mirip + WA kutipan", analysis(0.4, nil, waContact, waContact), "https://otomotif.tempo.co", 0.2, 0.35},
		// apk.cafe / quora: kemiripan template saja, tanpa artefak
		{"platform: template mirip saja", analysis(0.4, nil), "https://plague-inc.apk.cafe", 0.2, 0.35},
		// healthline / bodhinyana: situs sah dengan kontak biasa
		{"situs sah: kontak biasa, konten lemah", analysis(0.23, nil, waContact), "https://www.healthline.com", 0.2, 0},
		// Platform resmi di trustedDomains tidak pernah judol
		{"domain tepercaya", analysis(0.9, []string{"pragmatic"}, bank), "https://www.dana.id", 0.5, 0.35},
		// Situs pemerintah: konten saja (mis. berita pemberantasan judol) tidak cukup
		{"pemerintah: konten saja", analysis(0.8, nil), "https://www.komdigi.go.id", 0.2, 0},
	}
	for _, c := range cases {
		s := Compute(c.a, c.domainRisk, c.url, c.simhash, 0)
		if s.IsGambling {
			t.Errorf("%s: ditandai judol (skor %.2f, %s) — seharusnya bukan", c.name, s.Final, s.Priority)
		}
		if s.Priority == "High" {
			t.Errorf("%s: prioritas High tanpa bukti kuat (skor %.2f)", c.name, s.Final)
		}
	}
}

func TestScorerTruePositives(t *testing.T) {
	cases := []struct {
		name               string
		a                  *crawler.AnalysisResult
		url                string
		domainRisk, simhash float64
	}{
		// Situs judol lengkap: provider game + rekening
		{"provider game + rekening", analysis(0.5, []string{"pragmatic", "pgsoft"}, bank), "https://worthlzdtoto.store", 0.6, 0},
		// Konten judol kuat + QRIS
		{"konten kuat + QRIS", analysis(0.8, nil, qris), "https://indo303a.site", 0.4, 0},
		// Subdomain bajakan: nama host berisi istilah judol + kontak
		{"host berisi istilah judol", analysis(0.3, nil, waContact), "https://slot-gacor.ohiophysicianwellness.org", 0.2, 0},
		// Subdomain pemerintah yang dibajak, dengan bukti teknis (provider game)
		{"subdomain .go.id dibajak + provider", analysis(0.6, []string{"pragmatic"}, bank), "https://slot.desa-contoh.go.id", 0.2, 0},
		// Template sama dengan situs judol + rekening = judol
		{"template judol + rekening", analysis(0.4, nil, bank), "https://tripslot.net", 0.4, 0.35},
	}
	for _, c := range cases {
		s := Compute(c.a, c.domainRisk, c.url, c.simhash, 0)
		if !s.IsGambling {
			t.Errorf("%s: tidak terdeteksi judol (skor %.2f, %s)", c.name, s.Final, s.Priority)
		}
		if s.Priority != "High" {
			t.Errorf("%s: prioritas %s, seharusnya High (skor %.2f)", c.name, s.Priority, s.Final)
		}
	}
}

// Kemiripan template tidak boleh dihitung sebagai provider game (bug yang memberi skor 0.97 ke tempo.co).
func TestTemplateIsNotGameProvider(t *testing.T) {
	s := Compute(analysis(0.4, nil, waContact), 0.2, "https://dunia.tempo.co", 0.35, 0)
	if s.Final >= 0.9 {
		t.Fatalf("skor %.2f — kemiripan template terbaca sebagai provider game", s.Final)
	}
}

// Landing page judol: konten kuat tanpa rekening/provider tetap judol (bukan "High tapi bukan judi")
func TestStrongContentLandingIsGambling(t *testing.T) {
	a := &crawler.AnalysisResult{Confidence: 0.75, FoundKeywords: []string{"slot", "gacor", "bandar", "togel"}, NetworkSignals: &crawler.NetworkSignals{}}
	s := Compute(a, 0.2, "https://indo303ku.pages.dev", 0, 0)
	if !s.IsGambling || s.Priority != "High" {
		t.Fatalf("landing page judol: gambling=%v prio=%s rule=%s", s.IsGambling, s.Priority, s.Rule)
	}
	// Situs institusi dengan konten serupa (mis. berita pemberantasan judol) tetap jalur ketat
	s = Compute(a, 0.2, "https://kominfo.go.id/berita", 0, 0)
	if s.IsGambling {
		t.Fatal("situs pemerintah tanpa bukti teknis tidak boleh divonis judol")
	}
}

func TestInjectedSites(t *testing.T) {
	// Halaman susupan di domain kampus: konten kuat di sub-path judol, tanpa rekening
	a := &crawler.AnalysisResult{Confidence: 0.8, FoundKeywords: []string{"slot gacor", "maxwin", "togel"}}
	if s := Compute(a, 0, "https://fkip.contoh.ac.id/wp-content/slot-gacor/", 0, 0); !s.IsGambling || s.Rule != "injected" {
		t.Errorf("halaman susupan ac.id harus judol (injected), dapat %+v", s)
	}
	// Berita pemberantasan judol di beranda pemerintah tetap bukan judol
	if s := Compute(a, 0, "https://kominfo.contoh.go.id/", 0, 0); s.IsGambling {
		t.Errorf("beranda go.id tanpa tanda susupan tidak boleh judol: %+v", s)
	}
	// Cloaking di domain biasa
	b := &crawler.AnalysisResult{Confidence: 0.3, Injection: &crawler.Injection{Detected: true, Signals: []string{"cloaking"}}}
	if s := Compute(b, 0, "https://rumahsakit-contoh.co.id/", 0, 0); !s.IsGambling || s.Priority != "High" {
		t.Errorf("cloaking harus judol High: %+v", s)
	}
}

func TestBankWithoutKeywordsNotGambling(t *testing.T) {
	// Situs hosting: rekening pembayaran + WhatsApp, tanpa kata kunci judi (kasus jagoweb.com)
	a := &crawler.AnalysisResult{Confidence: 0.4, FoundPatterns: []string{"whatsapp", "tele"},
		Artifacts: []crawler.TransactionArtifact{{Type: "Bank", Name: "BCA", Value: "0000000001"}, {Type: "Contact", Name: "WhatsApp", Value: "6280000000001"}}}
	if s := Compute(a, 0.2, "https://www.jagoweb.com", 0, 0); s.IsGambling {
		t.Errorf("situs hosting dengan rekening tanpa kata kunci judi tidak boleh judol: %+v", s)
	}
}

func TestHostnameIgnoredWhenContentClean(t *testing.T) {
	// gacor-6.warlordchicago.com: halaman terbaca (dialihkan ke steakhouse), 0 kata kunci
	a := &crawler.AnalysisResult{Links: &crawler.ExtractedLinks{Total: 23}, RedirectChain: []string{"https://gacor-6.warlordchicago.com", "https://barranchco.com/"}}
	if s := Compute(a, 0.2, "https://gacor-6.warlordchicago.com", 0, 0); s.IsGambling {
		t.Errorf("nama host saja dengan konten bersih tidak boleh judol: %+v", s)
	}
	// Konten tak terbaca (diblokir) — nama host tetap dihitung
	b := &crawler.AnalysisResult{Links: &crawler.ExtractedLinks{Total: 0}}
	if s := Compute(b, 0.2, "https://casino-n1.com", 0, 0); !s.IsGambling {
		t.Errorf("nama host judol dengan konten tak terbaca tetap judol: %+v", s)
	}
}
