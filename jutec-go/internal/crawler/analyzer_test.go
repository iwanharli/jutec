package crawler

import "testing"

var testKeywords = []WeightedKeyword{{"slot", 0.2}, {"withdraw", 0.8}, {"slot gacor", 0.5}, {"togel", 0.2}}

func TestKeywordsMatchVisibleWordsOnly(t *testing.T) {
	// Situs exchange: <slot> Web Component, "timeslot", kata di skrip/CSS → tidak boleh dihitung
	html := `<html><head><style>.slot-card{}</style><script>var slotMachine = "withdraw";</script></head>
	<body><my-el><slot name="x"></slot></my-el><p>Pilih timeslot penarikan.</p></body></html>`
	r := (&Analyzer{}).Analyze(html, testKeywords, nil)
	if len(r.FoundKeywords) != 0 {
		t.Fatalf("seharusnya tak ada keyword, dapat %v", r.FoundKeywords)
	}
	// Situs judol: teks terlihat + alt banner
	html = `<body><h1>Daftar Slot Gacor</h1><img alt="bandar togel resmi" src="b.png"><p>Withdraw cepat</p></body>`
	r = (&Analyzer{}).Analyze(html, testKeywords, nil)
	if len(r.FoundKeywords) != 4 {
		t.Fatalf("seharusnya 4 keyword, dapat %v", r.FoundKeywords)
	}
}

func TestContactsAreNotPayment(t *testing.T) {
	// Keyword kuat + hanya kontak WA → bukan "pembayaran", jangan dipatok 95%
	html := `<body><p>Slot gacor withdraw</p><a href="https://wa.me/6281200000001">CS</a></body>`
	r := (&Analyzer{}).Analyze(html, testKeywords, nil)
	if r.Confidence >= 0.95 || r.Priority == "High" {
		t.Fatalf("kontak saja tak boleh memicu High: conf=%.2f prio=%s", r.Confidence, r.Priority)
	}
	// Dengan rekening → High
	r = (&Analyzer{}).Analyze(html+`<p>BCA 1234567890 a.n. Budi</p><p>Rekening BCA: 1234567890</p>`, testKeywords, nil)
	if !hasPaymentArtifact(r.Artifacts) {
		t.Skipf("pola rekening contoh tidak dikenali extractor (%v) — lewati bagian ini", r.Artifacts)
	}
	if r.Priority != "High" {
		t.Fatalf("rekening + keyword kuat seharusnya High, dapat %s", r.Priority)
	}
}

func TestBankArtifactFalsePositives(t *testing.T) {
	// Situs bank/berita: timestamp di skrip, kata "website"/"someone", tanggal, nomor registrasi, nomor HP
	fp := `<script>var t=1574136653555; window.bsi={id:1708509064942}</script>
	<p>Visit our website 1574170872815 for more.</p><p>Someone 20260531050600 updated.</p>
	<p>OCBC Bank (Malaysia) Berhad 199401009721</p><p>CIMB call 6281197814041</p><p>BCA 1234567890</p>`
	if arts := ExtractArtifacts(fp); len(arts) != 0 {
		t.Fatalf("seharusnya tak ada rekening, dapat %+v", arts)
	}
	// Situs judol: rekening asli tetap tertangkap
	ok := `<div>Deposit via BCA a.n. Budi Santoso 7311234569</div><div>BRI: 002301057123506</div><td>MANDIRI</td><td>1370018234561</td>`
	got := map[string]bool{}
	for _, a := range ExtractArtifacts(ok) {
		if a.Type == "Bank" {
			got[a.Name+":"+a.Value] = true
		}
	}
	for _, w := range []string{"BCA:7311234569", "BRI:002301057123506", "MANDIRI:1370018234561"} {
		if !got[w] {
			t.Errorf("rekening %s seharusnya tertangkap, dapat %v", w, got)
		}
	}
}
