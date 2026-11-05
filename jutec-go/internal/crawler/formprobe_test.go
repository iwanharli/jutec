package crawler

import (
	"os"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Menjalankan Chrome headless — aktifkan dengan LIVE=1.
func TestProbeFlowLocal(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	users := map[string]string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/daftar", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			r.ParseForm()
			if _, ok := users[r.FormValue("user_name")]; ok {
				fmt.Fprint(w, `<div class="alert">Username sudah terdaftar</div>`)
				return
			}
			users[r.FormValue("user_name")] = r.FormValue("pass")
			w.Header().Set("Set-Cookie", "laravel_session=abc")
			fmt.Fprint(w, `<div class="alert alert-success">Pendaftaran berhasil, silakan login</div>`)
			return
		}
		fmt.Fprint(w, `<form method="post" action="/daftar"><input name="user_name" placeholder="Username"><input type="password" name="pass" placeholder="Password">
		<input type="password" name="pass2" placeholder="Konfirmasi Password"><select name="bank"><option value="">Pilih bank</option><option value="1">BCA</option><option value="2">DANA</option></select>
		<input name="acc_name" placeholder="Nama Rekening"><input name="acc_no" placeholder="Nomor Rekening"><input name="hp" placeholder="No HP / WhatsApp">
		<input type="checkbox" name="agree"><button type="submit">Daftar</button></form>`)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			r.ParseForm()
			if users[r.FormValue("u")] == r.FormValue("p") {
				fmt.Fprint(w, `<a href="/deposit">Deposit</a> <a href="/logout">Logout</a> Saldo: 0`)
				return
			}
		}
		fmt.Fprint(w, `<form method="post" action="/login"><input name="u"><input type="password" name="p"><button>Masuk</button></form>`)
	})
	mux.HandleFunc("/deposit", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<h3>Transfer ke rekening berikut</h3><p>BCA 0000000002 a.n. PT CONTOH</p><p>DANA 080000000002</p>`)
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		mux.ServeHTTP(w, r)
	}))
	defer srv.Close()
	r := NewCrawler().ProbeForm(srv.URL+"/daftar", "/daftar", srv.URL+"/login", ProbeProfile{Username: "jt000001", Password: "Rahasia123", Phone: "081200000000", FullName: "Uji Coba", Accounts: []ProbeAccount{{Bank: "BRI", Number: "111", Name: "UJI BRI"}}})
	t.Logf("status=%s outcome=%s reason=%q msg=%q filled=%v missing=%v platform=%v", r.Status, r.Outcome, r.Reason, r.Message, r.Filled, r.Missing, r.Platform)
	for _, x := range r.Exchanges {
		t.Logf("  %s %s %d req=%q", x.Method, x.URL, x.Status, x.Request)
	}
	if r.Login != nil {
		t.Logf("login=%s reason=%q deposit=%v artefak=%v", r.Login.Status, r.Login.Reason, r.Login.DepositURLs, r.Login.Artifacts)
	}
}
