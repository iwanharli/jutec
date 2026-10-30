package crawler

import "testing"

func TestParseAuthForms(t *testing.T) {
	page := `
	<form id="FrmLogin" method="post" action="/">
	  <input type="text" name="username" placeholder="Username"><input type="password" name="password">
	  <button>Login</button>
	</form>
	<form id="register-form" method="post" action="https://api.lain.com/member/register">
	  <input name="username"><input type="password" name="password"><input type="password" name="confirm_password">
	  <input name="phone" placeholder="Nomor WhatsApp">
	  <select name="bank_name"><option>Pilih Bank</option><option>BCA</option><option>DANA</option><option>OVO</option></select>
	  <input name="account_number" placeholder="No Rekening"><input name="referral" placeholder="Kode Referral">
	  <div class="g-recaptcha"></div><button>Daftar</button>
	</form>
	<form action="/search"><input name="q"></form>`
	forms := parseAuthForms(page, "https://situs.com/register", "situs.com")
	if len(forms) != 2 {
		t.Fatalf("harus 2 form autentikasi (form pencarian diabaikan), dapat %d: %+v", len(forms), forms)
	}
	login, reg := forms[0], forms[1]
	if login.Type != "login" || login.Action != "https://situs.com/" {
		t.Errorf("form login salah: %+v", login)
	}
	if reg.Type != "register" || !reg.ExternalAct || !reg.NeedsPhone || !reg.NeedsBank || !reg.HasCaptcha {
		t.Errorf("form daftar salah: %+v", reg)
	}
	if len(reg.BankOptions) != 3 {
		t.Errorf("pilihan bank harus BCA, DANA, OVO; dapat %v", reg.BankOptions)
	}
}

func TestReferralCodes(t *testing.T) {
	got := referralCodes("https://worthlzdtoto.store/?content=register&ref=reffmaxwin")
	if len(got) != 1 || got[0] != "reffmaxwin" {
		t.Fatalf("kode referral harus [reffmaxwin], dapat %v", got)
	}
}

func TestAuthFormClassificationWorthlzdtoto(t *testing.T) {
	page := `
	<form><input name="username" placeholder="Username"><input type="password" name="password">
	  <input name="captcha" placeholder="Verifikasi"><input type="checkbox" name="remember"><a href="/register">Daftar</a></form>
	<form><input type="password" placeholder="Password Baru (Min 6 karakter)"><input type="password" placeholder="Konfirmasi Password Baru"></form>
	<form><input name="username"><input type="password" name="p1"><input type="password" name="p2">
	  <input name="email" placeholder="Email"><input name="telp" placeholder="Telpon"><select name="bank"><option>BCA</option></select></form>`
	got := map[string]int{}
	for _, f := range parseAuthForms(page, "https://worthlzdtoto.store/", "worthlzdtoto.store") {
		got[f.Type]++
	}
	if got["login"] != 1 || got["change_password"] != 1 || got["register"] != 1 {
		t.Fatalf("harus 1 login, 1 ganti password, 1 daftar; dapat %v", got)
	}
}

func TestBankOptionsGenericSelectAndRadio(t *testing.T) {
	page := `<form><input name="u"><input type="password" name="p"><input type="password" name="p2">
	  <select name="method_id"><optgroup label="Bank"><option>BCA</option><option>BRI</option></optgroup><option>DANA</option></select>
	  <input type="radio" name="bank_radio" value="Mandiri"><input type="radio" name="bank_radio" value="OVO"></form>`
	f := parseAuthForms(page, "https://x.com/register", "x.com")
	if len(f) != 1 || len(f[0].BankOptions) != 5 {
		t.Fatalf("harus 5 pilihan (select generik + radio), dapat %+v", f)
	}
}
