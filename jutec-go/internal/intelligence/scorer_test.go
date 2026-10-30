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
