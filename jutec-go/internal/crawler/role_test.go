package crawler

import "testing"

func TestClassifyRole(t *testing.T) {
	// Landing: tombol daftar ke domain lain (lewat shortener), form di halaman tujuan
	landing := &AnalysisResult{
		Links: &ExtractedLinks{Register: []string{"https://ronic.link/daftarwarga88", "https://warga88zone.tokobecak.online/Register.aspx"}},
		AuthSurface: &AuthSurface{Forms: []AuthForm{{Type: "register", PageURL: "https://warga88zone.tokobecak.online/Register.aspx",
			Action: "https://warga88zone.tokobecak.online/Register.aspx", ExternalAct: true}}},
	}
	role, targets := ClassifyRole(landing, "https://sbobet88.ltd/")
	if role != RoleLanding || len(targets) != 1 || targets[0] != "warga88zone.tokobecak.online" {
		t.Fatalf("landing: role=%s targets=%v", role, targets)
	}

	// Situs utama: form daftar di halamannya sendiri, mengirim ke dirinya sendiri, minta rekening
	operator := &AnalysisResult{AuthSurface: &AuthSurface{Forms: []AuthForm{{Type: "register",
		PageURL: "https://warga88zone.tokobecak.online/", Action: "https://warga88zone.tokobecak.online/Register.aspx",
		BankOptions: []string{"BCA", "DANA"}}}}}
	if role, _ := ClassifyRole(operator, "https://warga88zone.tokobecak.online/"); role != RoleOperator {
		t.Fatalf("operator: role=%s", role)
	}

	// Provider game saja sudah cukup sebagai situs utama
	lobby := &AnalysisResult{NetworkSignals: &NetworkSignals{GameProviders: []string{"Pragmatic"}}}
	if role, _ := ClassifyRole(lobby, "https://slot.example"); role != RoleOperator {
		t.Fatalf("lobby: role=%s", role)
	}

	// Tidak ada sinyal → kosong
	if role, _ := ClassifyRole(&AnalysisResult{}, "https://x.example"); role != "" {
		t.Fatalf("kosong: role=%s", role)
	}
}

func TestResolveRelativeFileNotDomain(t *testing.T) {
	if got := resolveURL("Register.aspx", "https://warga88zone.tokobecak.online/"); got != "https://warga88zone.tokobecak.online/Register.aspx" {
		t.Errorf("Register.aspx → %s", got)
	}
	if got := resolveURL("login.php?x=1", "https://a.com/m/"); got != "https://a.com/m/login.php?x=1" {
		t.Errorf("login.php → %s", got)
	}
	if got := resolveURL("tripslot.org/daftar", "https://a.com/"); got != "https://tripslot.org/daftar" {
		t.Errorf("domain tanpa skema → %s", got)
	}
}
