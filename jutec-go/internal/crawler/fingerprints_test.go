package crawler

import "testing"

func TestExtractFingerprints(t *testing.T) {
	html := `<img src="//d33egg70nrp50s.cloudfront.net/Images/a.png"><link href="https://id-gaming.b-cdn.net/x.css">`
	api := []string{
		"https://jp-api.namesvr.dev/progressive-jackpot", // API backend pihak lain → diambil
		"https://indo303a.site/session/online",           // domain sendiri → dibuang
		"https://www.google-analytics.com/g/collect",     // layanan umum → dibuang
	}
	got := map[string]bool{}
	for _, f := range ExtractFingerprints(html, nil, api, "indo303a.site") {
		got[f.Kind+"|"+f.Value] = true
	}
	for _, want := range []string{"cdn|cloudfront:d33egg70nrp50s", "cdn|bunnycdn:id-gaming", "api|jp-api.namesvr.dev"} {
		if !got[want] {
			t.Errorf("sidik jari %q tidak ditemukan (hasil: %v)", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("harus tepat 3 sidik jari, dapat %d: %v", len(got), got)
	}
}

func TestInterestingInternalLinks(t *testing.T) {
	internal := []string{
		"https://situs.com/deposit", "https://situs.com/promo", "https://situs.com/about",
		"https://situs.com/assets/app.js", "https://situs.com/kontak#top", "https://lain.com/deposit",
	}
	got := interestingInternalLinks(internal, "https://situs.com/")
	if len(got) != 3 || got[0] != "https://situs.com/deposit" {
		t.Fatalf("hasil tak sesuai: %v (harus deposit, kontak, promo; tanpa about/js/domain lain)", got)
	}
}

func TestRegisterLinkByButtonText(t *testing.T) {
	html := `<a class="btn" href="https://jali.me/tripslot"><span>DAFTAR</span></a>
		<a href="https://jali.me/tripslot">LOGIN</a>
		<a href="https://berita.com/artikel">Baca selengkapnya</a>`
	l := ExtractLinks(html, "https://tripslot.everolyjewelry.site/")
	if len(l.Register) != 1 || l.Register[0] != "https://jali.me/tripslot" {
		t.Fatalf("tombol DAFTAR harus jadi link pendaftaran, dapat register=%v external=%v", l.Register, l.External)
	}
	for _, e := range l.External {
		if e == "https://jali.me/tripslot" {
			t.Errorf("link pendaftaran tidak boleh juga tercatat sebagai external")
		}
	}
	if !IsShortenerURL("https://jali.me/tripslot") {
		t.Errorf("jali.me harus dikenali sebagai shortener")
	}
}
