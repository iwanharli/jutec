package crawler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	neturl "net/url"
	"strings"
	"sync"
	"time"

	"jutec-go/internal/browser"

	cdpnetwork "github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Uji Form Terkendali: mengisi form pendaftaran dengan data dummy milik unit lalu mengirimnya — atas
// perintah analis, satu situs per uji. Form bercaptcha/OTP dilewati (tidak bisa diotomasi tanpa manusia).

// ProbeProfile: data dummy dari Pengaturan (probe_*). Username dibuat unik per uji kecuali Fixed.
type ProbeProfile struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	FullName      string `json:"fullname"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Bank          string `json:"bank"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	Referral      string `json:"referral"`
	// Rekening uji milik unit per bank/e-wallet — dipilih sesuai pilihan bank di form situs
	Accounts []ProbeAccount `json:"accounts"`
}

type ProbeAccount struct {
	Bank   string `json:"bank"`
	Number string `json:"number"`
	Name   string `json:"name"`
}

type ProbeExchange struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Type    string            `json:"type"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"` // header respons yang informatif (Server, X-Powered-By, Set-Cookie…)
	Request string            `json:"request,omitempty"` // isi POST (sudah memuat data dummy)
	Body    string            `json:"body,omitempty"`    // potongan respons (maks. 8 KB)
}

// ProbeLogin: hasil masuk dengan akun dummy yang baru didaftarkan + halaman deposit yang dibuka.
type ProbeLogin struct {
	Status      string                `json:"status"` // logged_in | failed | skipped
	Reason      string                `json:"reason,omitempty"`
	Message     string                `json:"message,omitempty"`
	FinalURL    string                `json:"final_url,omitempty"`
	DepositURLs []string              `json:"deposit_urls,omitempty"`
	Artifacts   []TransactionArtifact `json:"artifacts"` // rekening/e-wallet/QRIS penampung setelah login
	Exchanges   []ProbeExchange       `json:"exchanges"`
	ShotAfter   []byte                `json:"-"`
	ShotDeposit []byte                `json:"-"`
	HTMLDeposit string                `json:"-"`
}

type ProbeResult struct {
	Status     string                `json:"status"`  // submitted | skipped | error
	Outcome    string                `json:"outcome"` // accepted | rejected | unknown
	Reason     string                `json:"reason,omitempty"`
	Message    string                `json:"message,omitempty"` // pesan yang tampil setelah submit
	PageURL    string                `json:"page_url"`
	FinalURL   string                `json:"final_url,omitempty"`
	Filled     map[string]string     `json:"filled"`  // field baku → nilai yang diisikan
	Missing    []string              `json:"missing"` // field wajib tanpa data dummy
	Exchanges  []ProbeExchange       `json:"exchanges"`
	Artifacts  []TransactionArtifact `json:"artifacts"` // rekening/kontak di halaman setelah daftar
	Platform   []string              `json:"platform"`  // petunjuk perangkat lunak backend
	Login      *ProbeLogin           `json:"login,omitempty"`
	ShotBefore []byte                `json:"-"`
	ShotAfter  []byte                `json:"-"`
	HTMLAfter  string                `json:"-"`
}

// probeFillJS: cari form daftar di halaman, isi field sesuai jenisnya, kembalikan ringkasan.
// Jenis field dikenali dari name/id/placeholder/label — sejalan dengan fieldRules di store.
const probeFillJS = `((profile, actionHint) => {
  const rules = [
    ['captcha', /captcha|kode keamanan|security code|validation/i], ['otp', /\botp\b|kode sms|sms code/i],
    ['confirm_password', /konfirm|confirm|ulang|sekali lagi|re.?pass|password2|retype/i], ['password', /pass|sandi/i],
    ['account_name', /(nama|name).{0,10}(rek|account|akun|pemilik)|account.?name|acc.?name|atas nama/i],
    ['account_number', /(no|nomor|number|num).{0,10}(rek|account|akun)|acc(ount)?.?(no|num)|norek|rekening/i],
    ['bank', /bank|ewallet|e-wallet|metode|payment/i], ['referral', /ref+er|referal|upline|invite|aff/i],
    ['phone', /phone|telp|telepon|telpon|whatsapp|\bwa\b|\bhp\b|handphone|mobile/i], ['email', /e-?mail/i],
    ['agree', /agree|setuju|terms|syarat/i], ['username', /user|login|akun|account/i],
    ['fullname', /full.?name|nama lengkap|\bnama\b|\bname\b/i],
  ];
  const kindOf = el => {
    const lab = el.id && document.querySelector('label[for="' + el.id + '"]');
    const txt = [el.name, el.id, el.placeholder, el.getAttribute('aria-label'), lab && lab.innerText].filter(Boolean).join(' ').replace(/[_\-$]/g, ' ');
    for (const [k, re] of rules) if (re.test(txt)) return k;
    return '';
  };
  const forms = [...document.querySelectorAll('form')];
  const score = f => {
    const ins = [...f.querySelectorAll('input,select,textarea')];
    const ks = ins.map(kindOf);
    let s = ks.filter(k => ['confirm_password','account_number','account_name','bank','phone','email','referral'].includes(k)).length;
    const pw = f.querySelectorAll('input[type=password]').length;
    if (pw >= 2) s += 3; else if (pw === 1) s += 1;       // 2 kolom sandi = ciri khas form daftar
    if (pw >= 1 && ins.length >= 3) s += 1;               // form bersandi dengan beberapa field
    if (actionHint && (f.action || '').includes(actionHint)) s += 5;
    return s;
  };
  const form = forms.sort((a, b) => score(b) - score(a))[0];
  if (!form || score(form) === 0) return { error: 'form pendaftaran tidak ditemukan di halaman' };
  const filled = {}, missing = [], blockers = [];
  const setVal = (el, v) => {
    const proto = el.tagName === 'SELECT' ? HTMLSelectElement.prototype : el.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, 'value').set.call(el, v);
    el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true }));
  };
  if (form.querySelector('.g-recaptcha,.h-captcha,.cf-turnstile,iframe[src*="recaptcha"],iframe[src*="hcaptcha"],img[src*="captcha" i]')) blockers.push('captcha');
  // Dropdown bank diproses lebih dulu: pilih bank yang rekening ujinya terdaftar, lalu pakai nomor & nama rekening itu
  const accts = (profile.accounts || []).filter(a => a.bank && a.number);
  const norm = t => (t || '').toLowerCase().replace(/[^a-z0-9]/g, '');
  const els = [...form.querySelectorAll('input,select,textarea')];
  for (const el of els) {
    if (el.tagName !== 'SELECT' || el.disabled || kindOf(el) !== 'bank' || (!accts.length && !profile.phone)) continue;
    const opts = [...el.options].filter(o => o.value && !/pilih|select|--/i.test(o.text));
    let matched = false;
    for (const a of accts) {
      const o = opts.find(o => norm(o.text).includes(norm(a.bank)) || norm(a.bank).includes(norm(o.text)));
      if (o) { profile.bank = o.text.trim(); profile.account_number = a.number; profile.account_name = a.name || profile.account_name || profile.fullname; matched = true; break; }
    }
    // E-money: nomor akunnya = nomor HP dummy — dipakai bila tak ada rekening bank yang cocok
    if (!matched && profile.phone) {
      const w = opts.find(o => /dana|ovo|gopay|go-pay|linkaja|link aja|shopee|sakuku|jenius|isaku/i.test(o.text));
      if (w) { profile.bank = w.text.trim(); profile.account_number = profile.phone; profile.account_name = profile.account_name || profile.fullname || profile.username; }
    }
  }
  for (const el of els) {
    const t = (el.type || '').toLowerCase();
    if (['hidden','submit','button','image','reset','file'].includes(t) || el.disabled) continue;
    const k = kindOf(el);
    if (k === 'captcha' || k === 'otp') { blockers.push(k); continue; }
    if (t === 'checkbox' || t === 'radio') { if (!el.checked && (k === 'agree' || t === 'checkbox')) { el.click(); filled[k || 'checkbox'] = 'dicentang'; } continue; }
    if (el.tagName === 'SELECT') {
      const opts = [...el.options].filter(o => o.value && !/pilih|select|--/i.test(o.text));
      const want = (profile.bank || '').toLowerCase();
      const o = opts.find(o => want && o.text.toLowerCase().includes(want)) || opts[0];
      if (o) { setVal(el, o.value); filled[k || 'select'] = o.text.trim(); }
      continue;
    }
    const v = k === 'confirm_password' ? profile.password : profile[k];
    if (v) { setVal(el, v); filled[k] = k.includes('password') ? '••••••' : v; }
    else if (el.required || k) missing.push(k || (el.name || el.placeholder || 'field'));
  }
  // Fallback: isi username & password walau label tak dikenali (hindari submit kosong — kasus slotindo62)
  const txt = [...form.querySelectorAll('input')].filter(e => !['hidden','submit','button','image','reset','file','checkbox','radio'].includes((e.type||'').toLowerCase()) && !e.disabled);
  if (!filled.username) {
    const u = txt.find(e => !e.value && ['text','',null].includes((e.type||'').toLowerCase()) && !/pass|captcha|otp|kode/i.test([e.name,e.id,e.placeholder].join(' ')));
    if (u) { setVal(u, profile.username); filled.username = profile.username; }
  }
  const pws = txt.filter(e => (e.type||'').toLowerCase() === 'password' && !e.value);
  if (!filled.password && pws[0]) { setVal(pws[0], profile.password); filled.password = '\u2022\u2022\u2022\u2022\u2022\u2022'; }
  if (!filled.confirm_password && pws[1]) { setVal(pws[1], profile.password); filled.confirm_password = '\u2022\u2022\u2022\u2022\u2022\u2022'; }
  const ready = !!(filled.username && filled.password);
  return { filled, missing, blockers: [...new Set(blockers)], ready, action: form.action || '', idx: forms.indexOf(form) };
})`

// probeSubmitJS: kirim form terpilih (tombol submit asli bila ada, agar handler JS situs ikut jalan).
const probeSubmitJS = `((idx) => {
  const f = [...document.querySelectorAll('form')][idx];
  if (!f) return false;
  const btn = f.querySelector('button[type=submit],input[type=submit],button:not([type])');
  if (btn) btn.click(); else if (f.requestSubmit) f.requestSubmit(); else f.submit();
  return true;
})`

// probeMessageJS: pesan yang tampil setelah submit (alert, toast, sweetalert, teks error/sukses).
const probeMessageJS = `(() => {
  const sel = '.alert,.toast,.swal2-popup,.swal-modal,.error,.errors,.invalid-feedback,.help-block,.notification,.message,[role=alert],.modal.show,.text-danger,.text-success';
  const t = [...document.querySelectorAll(sel)].map(e => e.innerText.trim()).filter(x => x && x.length < 400);
  return [...new Set(t)].slice(0, 6).join(' | ');
})()`

// probeStateJS: keadaan halaman setelah submit — untuk mendeteksi sukses / minta OTP / pindah ke area member.
const probeStateJS = `(() => {
  const low = s => (s || '').toLowerCase();
  const url = location.href;
  const body = document.body ? document.body.innerText : '';
  const bodyLow = low(body).slice(0, 4000);
  const hasPassword = !!document.querySelector('input[type=password]');
  const member = /logout|log out|keluar|saldo|deposit|withdraw|my account|akun saya|dashboard|lobby|member area/.test(bodyLow);
  const otp = /kode (otp|verifikasi|sms)|verifikasi nomor|verifikasi (hp|whatsapp|wa|sms|akun)|one.?time password|\botp\b|masukkan kode|enter code|verification code|cek (sms|whatsapp)/.test(bodyLow)
    || /(verif|otp|confirm)/.test((low(url).split('?')[0].split('/').pop()) || '');
  return { url, member, otp, hasPassword, text: body.slice(0, 800) };
})()`

var platformHeaders = []string{"Server", "X-Powered-By", "X-AspNet-Version", "X-Generator", "Set-Cookie", "Via", "CF-Ray", "X-Cache"}

// ProbeForm menjalankan satu uji pendaftaran. actionHint = bagian URL action form (dari auth_surface).
func (c *Crawler) ProbeForm(pageURL, actionHint, loginURL string, prof ProbeProfile) *ProbeResult {
	res := &ProbeResult{PageURL: pageURL, Status: "error", Outcome: "unknown", Filled: map[string]string{}, Missing: []string{}, Exchanges: []ProbeExchange{}, Artifacts: []TransactionArtifact{}, Platform: []string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	rules, _ := dohResolverRules(ctx, pageURL)
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", true), chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (Linux; Android 13; SM-A536E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"))
	if rules != "" {
		opts = append(opts, chromedp.Flag("host-resolver-rules", rules))
	}
	alloc, ac := browser.NewAllocator(ctx, opts...)
	defer ac()
	tab, tc := chromedp.NewContext(alloc)
	defer tc()

	// Rekam pertukaran setelah submit: semua request non-GET + dokumen/XHR, beserta isi respons
	var mu sync.Mutex
	recording := false
	ex := map[cdpnetwork.RequestID]*ProbeExchange{}
	var order []cdpnetwork.RequestID
	chromedp.ListenTarget(tab, func(ev any) {
		mu.Lock()
		defer mu.Unlock()
		if !recording {
			return
		}
		switch e := ev.(type) {
		case *cdpnetwork.EventRequestWillBeSent:
			t := string(e.Type)
			if e.Request.Method == "GET" && t != "Document" && t != "XHR" && t != "Fetch" {
				return
			}
			x := &ProbeExchange{URL: e.Request.URL, Method: e.Request.Method, Type: t}
			if e.Request.HasPostData {
				for _, en := range e.Request.PostDataEntries {
					if b, err := base64.StdEncoding.DecodeString(en.Bytes); err == nil {
						x.Request += string(b)
					} else {
						x.Request += en.Bytes
					}
				}
			}
			if _, ok := ex[e.RequestID]; !ok {
				order = append(order, e.RequestID)
			}
			ex[e.RequestID] = x
		case *cdpnetwork.EventResponseReceivedExtraInfo:
			// Set-Cookie hanya tersedia di sini (Chrome menyembunyikannya dari Response.headers)
			if x := ex[e.RequestID]; x != nil {
				if x.Headers == nil {
					x.Headers = map[string]string{}
				}
				for k, v := range e.Headers {
					if strings.EqualFold(k, "Set-Cookie") {
						x.Headers["Set-Cookie"] = fmt.Sprint(v)
					}
				}
			}
		case *cdpnetwork.EventResponseReceived:
			if x := ex[e.RequestID]; x != nil {
				x.Status = int(e.Response.Status)
				if x.Headers == nil {
					x.Headers = map[string]string{}
				}
				for _, h := range platformHeaders {
					for k, v := range e.Response.Headers {
						if strings.EqualFold(k, h) {
							x.Headers[h] = fmt.Sprint(v)
						}
					}
				}
			}
		}
	})

	var fill struct {
		Error    string            `json:"error"`
		Filled   map[string]string `json:"filled"`
		Missing  []string          `json:"missing"`
		Blockers []string          `json:"blockers"`
		Ready    bool              `json:"ready"`
		Action   string            `json:"action"`
		Idx      int               `json:"idx"`
	}
	profJSON, _ := json.Marshal(prof)
	hintJSON, _ := json.Marshal(actionHint)
	err := chromedp.Run(tab, cdpnetwork.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error { _, _, _, _, err := page.Navigate(pageURL).Do(ctx); return err }),
		chromedp.WaitReady("body", chromedp.ByQuery), chromedp.Sleep(5*time.Second),
		chromedp.Evaluate(fmt.Sprintf("%s(%s, %s)", probeFillJS, profJSON, hintJSON), &fill),
	)
	if err != nil {
		res.Reason = "halaman tidak bisa dibuka: " + err.Error()
		return res
	}
	if fill.Error != "" {
		res.Status, res.Reason = "skipped", fill.Error
		return res
	}
	res.Filled, res.Missing = fill.Filled, fill.Missing
	if res.Missing == nil {
		res.Missing = []string{}
	}
	if len(fill.Blockers) > 0 {
		res.Status, res.Reason = "skipped", "form memakai "+strings.Join(fill.Blockers, " & ")+" — tidak bisa diotomasi"
		return res
	}
	if !fill.Ready {
		res.Status, res.Reason = "skipped", "field username/password tidak terbaca — form tidak dikirim agar tidak submit kosong"
		return res
	}
	_ = chromedp.Run(tab, chromedp.CaptureScreenshot(&res.ShotBefore))

	// collect: ambil isi respons pertukaran yang tercatat sejak reset terakhir, lalu kosongkan catatan
	collect := func() []ProbeExchange {
		mu.Lock()
		ids := append([]cdpnetwork.RequestID{}, order...)
		mu.Unlock()
		out := []ProbeExchange{}
		for _, id := range ids {
			var body []byte
			_ = chromedp.Run(tab, chromedp.ActionFunc(func(ctx context.Context) error {
				b, err := cdpnetwork.GetResponseBody(id).Do(ctx)
				body = b
				return err
			}))
			mu.Lock()
			if x := ex[id]; x != nil {
				if len(body) > 8192 {
					body = body[:8192]
				}
				x.Body = string(body)
				out = append(out, *x)
			}
			mu.Unlock()
		}
		mu.Lock()
		ex, order = map[cdpnetwork.RequestID]*ProbeExchange{}, nil
		mu.Unlock()
		return out
	}

	mu.Lock()
	recording = true
	mu.Unlock()
	var sent bool
	if err := chromedp.Run(tab, chromedp.Evaluate(fmt.Sprintf("%s(%d)", probeSubmitJS, fill.Idx), &sent), chromedp.Sleep(8*time.Second)); err != nil || !sent {
		res.Reason = "gagal mengirim form"
		return res
	}
	res.Status = "submitted"

	res.Exchanges = collect()

	var msg string
	var st probeState
	_ = chromedp.Run(tab, chromedp.Location(&res.FinalURL), chromedp.Evaluate(probeMessageJS, &msg),
		chromedp.Evaluate(probeStateJS, &st),
		chromedp.OuterHTML("html", &res.HTMLAfter, chromedp.ByQuery), chromedp.CaptureScreenshot(&res.ShotAfter))
	res.Message = msg
	res.Artifacts = ExtractArtifacts(res.HTMLAfter)
	if res.Artifacts == nil {
		res.Artifacts = []TransactionArtifact{}
	}
	res.Outcome = probeOutcome(res, st, pageURL)
	if res.Outcome == "otp_required" {
		res.Reason = "Pendaftaran minta verifikasi OTP/SMS — tidak bisa dilanjutkan otomatis"
	}

	// Masuk dengan akun dummy (kecuali pendaftaran jelas ditolak), lalu buka halaman deposit
	if res.Outcome != "rejected" && res.Outcome != "otp_required" && prof.Username != "" && prof.Password != "" {
		res.Login = c.probeLogin(tab, loginURL, pageURL, prof, collect)
	}
	res.Platform = probePlatform(res)
	return res
}

// probeLoginJS: isi form login (field username/HP + password) lalu kirim. Captcha → dilewati.
const probeLoginJS = `((u, p) => {
  const forms = [...document.querySelectorAll('form')].filter(f => f.querySelector('input[type=password]'));
  const f = forms.sort((a, b) => a.querySelectorAll('input').length - b.querySelectorAll('input').length)[0];
  if (!f) return { error: 'form login tidak ditemukan' };
  if (f.querySelector('.g-recaptcha,.h-captcha,.cf-turnstile,img[src*="captcha" i],input[name*="captcha" i]')) return { error: 'form login memakai captcha — tidak bisa diotomasi' };
  const set = (el, v) => { Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(el, v);
    el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })); };
  const user = [...f.querySelectorAll('input')].find(i => !['password','hidden','checkbox','radio','submit','button'].includes((i.type || '').toLowerCase()));
  if (!user) return { error: 'field username tidak ditemukan' };
  set(user, u); set(f.querySelector('input[type=password]'), p);
  const btn = f.querySelector('button[type=submit],input[type=submit],button:not([type])');
  if (btn) btn.click(); else if (f.requestSubmit) f.requestSubmit(); else f.submit();
  return { ok: true };
})`

// probeDepositJS: link menu deposit/setor dana setelah login.
const probeDepositJS = `(() => [...document.querySelectorAll('a[href]')]
  .filter(a => /deposit|setor|isi saldo|topup|top up|transfer|bank|rekening/i.test(a.innerText + ' ' + a.getAttribute('href')))
  .map(a => a.href).filter(h => h.startsWith('http')).filter((h, i, all) => all.indexOf(h) === i).slice(0, 3))()`

func (c *Crawler) probeLogin(tab context.Context, loginURL, regURL string, prof ProbeProfile, collect func() []ProbeExchange) *ProbeLogin {
	lg := &ProbeLogin{Status: "failed", Artifacts: []TransactionArtifact{}, Exchanges: []ProbeExchange{}}
	target := loginURL
	if target == "" {
		target = regURL
	}
	var r struct {
		Error string `json:"error"`
		OK    bool   `json:"ok"`
	}
	u, _ := json.Marshal(prof.Username)
	p, _ := json.Marshal(prof.Password)
	collect() // buang catatan sebelum login
	err := chromedp.Run(tab, chromedp.ActionFunc(func(ctx context.Context) error {
		// Banyak situs langsung masuk setelah daftar: cek dulu apakah form login masih ada
		_, _, _, _, err := page.Navigate(target).Do(ctx)
		return err
	}), chromedp.WaitReady("body", chromedp.ByQuery), chromedp.Sleep(4*time.Second),
		chromedp.Evaluate(fmt.Sprintf("%s(%s, %s)", probeLoginJS, u, p), &r), chromedp.Sleep(7*time.Second))
	if err != nil {
		lg.Reason = "halaman login tidak bisa dibuka: " + err.Error()
		return lg
	}
	if r.Error != "" && !strings.Contains(r.Error, "tidak ditemukan") {
		lg.Status, lg.Reason = "skipped", r.Error
		return lg
	}
	lg.Exchanges = collect()
	var html string
	_ = chromedp.Run(tab, chromedp.Location(&lg.FinalURL), chromedp.Evaluate(probeMessageJS, &lg.Message),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery), chromedp.CaptureScreenshot(&lg.ShotAfter))
	low := strings.ToLower(html)
	// Tanda sudah masuk: tidak ada lagi kolom password + ada menu akun
	if !strings.Contains(low, `type="password"`) && (strings.Contains(low, "logout") || strings.Contains(low, "keluar") || strings.Contains(low, "saldo") || strings.Contains(low, "deposit")) {
		lg.Status = "logged_in"
	} else {
		lg.Reason = "masih ada form login setelah submit — akun belum aktif atau kredensial ditolak"
		lg.Artifacts = append(lg.Artifacts, ExtractArtifacts(html)...)
		return lg
	}
	lg.Artifacts = append(lg.Artifacts, ExtractArtifacts(html)...)

	// Halaman deposit: rekening/e-wallet/QRIS penampung biasanya baru tampil di sini
	_ = chromedp.Run(tab, chromedp.Evaluate(probeDepositJS, &lg.DepositURLs))
	for i, d := range lg.DepositURLs {
		var h string
		var shot []byte
		if err := chromedp.Run(tab, chromedp.ActionFunc(func(ctx context.Context) error {
			_, _, _, _, err := page.Navigate(d).Do(ctx)
			return err
		}), chromedp.WaitReady("body", chromedp.ByQuery), chromedp.Sleep(4*time.Second),
			chromedp.OuterHTML("html", &h, chromedp.ByQuery), chromedp.CaptureScreenshot(&shot)); err != nil {
			continue
		}
		lg.Artifacts = append(lg.Artifacts, ExtractArtifacts(h)...)
		if i == 0 {
			lg.ShotDeposit, lg.HTMLDeposit = shot, h
		}
	}
	lg.Exchanges = append(lg.Exchanges, collect()...)
	return lg
}

type probeState struct {
	URL         string `json:"url"`
	Member      bool   `json:"member"`
	OTP         bool   `json:"otp"`
	HasPassword bool   `json:"hasPassword"`
	Text        string `json:"text"`
}

func anyContains(text string, keys ...string) bool {
	for _, k := range keys {
		if strings.Contains(text, k) {
			return true
		}
	}
	return false
}

// probeOutcome menilai hasil dari pesan layar, respons XHR/JSON, perubahan URL, dan keadaan halaman.
// accepted | rejected | otp_required | unknown
func probeOutcome(r *ProbeResult, st probeState, regURL string) string {
	text := strings.ToLower(r.Message + " " + st.Text)
	var jsonBody string
	for _, x := range r.Exchanges {
		b := strings.ToLower(x.Body)
		text += " " + b
		if strings.Contains(x.Type, "XHR") || strings.Contains(x.Type, "Fetch") || strings.Contains(b, "{") {
			jsonBody += " " + b
		}
	}
	// 1. Respons JSON (API pendaftaran situs modern)
	switch {
	case anyContains(jsonBody, `"success":true`, `"status":true`, `"status":"success`, `"status":1`, `"code":200`, `"error":false`):
		return "accepted"
	case anyContains(jsonBody, `"success":false`, `"status":false`, `"error":true`, `"status":"error`, `"errors":`):
		if anyContains(text, "otp", "verifikasi", "verification") {
			return "otp_required"
		}
		return "rejected"
	}
	// 2. Minta OTP/verifikasi nomor
	if st.OTP || anyContains(text, "kode otp", "kode verifikasi", "verifikasi nomor", "verifikasi whatsapp", "verifikasi sms", "verification code", "one time password", "masukkan kode") {
		return "otp_required"
	}
	// 3. Sudah di area member / pindah dari halaman daftar
	regPath, curPath := regURL, st.URL
	if u, err := neturl.Parse(regURL); err == nil {
		regPath = u.Path
	}
	if u, err := neturl.Parse(st.URL); err == nil {
		curPath = u.Path
	}
	if st.Member && !st.HasPassword {
		return "accepted"
	}
	if curPath != regPath && anyContains(strings.ToLower(curPath), "member", "dashboard", "account", "home", "lobby", "beranda", "main") {
		return "accepted"
	}
	// 4. Pesan eksplisit
	switch {
	case anyContains(text, "sudah terdaftar", "sudah digunakan", "already", "tidak valid", "invalid", "gagal", "wajib", "required", "salah", "tidak boleh kosong"):
		return "rejected"
	case anyContains(text, "berhasil", "sukses", "success", "selamat", "registrasi selesai", "pendaftaran selesai"):
		return "accepted"
	}
	return "unknown"
}

func probePlatform(r *ProbeResult) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, x := range r.Exchanges {
		for _, h := range []string{"Server", "X-Powered-By", "X-AspNet-Version", "X-Generator"} {
			if v := x.Headers[h]; v != "" {
				add(h + ": " + v)
			}
		}
		c := strings.ToLower(x.Headers["Set-Cookie"])
		for k, v := range map[string]string{"asp.net_sessionid": "ASP.NET", "laravel_session": "Laravel", "phpsessid": "PHP", "ci_session": "CodeIgniter", "connect.sid": "Node.js (Express)", "jsessionid": "Java"} {
			if strings.Contains(c, k) {
				add("Sesi: " + v)
			}
		}
	}
	return out
}
