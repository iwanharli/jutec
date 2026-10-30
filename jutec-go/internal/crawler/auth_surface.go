package crawler

import (
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// AuthSurface memetakan halaman & form login/pendaftaran situs secara PASIF:
// hanya membaca HTML — form tidak pernah diisi atau dikirim.
type AuthSurface struct {
	LoginURLs     []string   `json:"login_urls,omitempty"`
	RegisterURLs  []string   `json:"register_urls,omitempty"`
	Forms         []AuthForm `json:"forms,omitempty"`
	ReferralCodes []string   `json:"referral_codes,omitempty"` // dari ?ref=, ?reff=, ?referral=, ?aff=…
}

type AuthForm struct {
	Type        string   `json:"type"`   // login | register | change_password
	PageURL     string   `json:"page_url"`
	Action      string   `json:"action"` // URL tujuan submit (absolut)
	Method      string   `json:"method"`
	Fields      []string `json:"fields"`                 // nama/label field yang diminta
	BankOptions []string `json:"bank_options,omitempty"` // pilihan bank/e-wallet di dropdown
	HasCaptcha  bool     `json:"has_captcha"`
	HasOTP      bool     `json:"has_otp"`      // verifikasi OTP / kode SMS / WA
	NeedsPhone  bool     `json:"needs_phone"`  // nomor HP/WA wajib
	NeedsBank   bool     `json:"needs_bank"`   // nomor rekening diminta saat daftar
	ExternalAct bool     `json:"external_action"` // submit ke domain lain
	SeenOn      []string `json:"seen_on,omitempty"` // halaman lain tempat form yang sama muncul (popup/header)
}

var (
	reLoginWord    = regexp.MustCompile(`(?i)\b(login|log\s?in|masuk|signin|sign\s?in)\b`)
	reRegisterWord = regexp.MustCompile(`(?i)(register|registrasi|regist|daftar|signup|sign[-_\s]?up|join|create.?account|pendaftaran)`)
	reRegisterHint = regexp.MustCompile(`(?i)(confirm|konfirmasi|repassword|re_password|password2|bank|rekening|account.?(no|number)|norek|referral|referal|reff|upline|phone|telepon|whatsapp|no_?hp|email)`)
	reCaptcha      = regexp.MustCompile(`(?i)captcha|recaptcha|hcaptcha|turnstile`)
	reChangePass   = regexp.MustCompile(`(?i)(password|sandi)\s*(baru|lama)|new.?password|old.?password|current.?password|ganti.?(password|sandi)|change.?password|reset.?password`)
	reOTP          = regexp.MustCompile(`(?i)\botp\b|kode.?verifikasi|verification.?code|sms.?code|kode.?sms`)
	rePhoneField   = regexp.MustCompile(`(?i)phone|telepon|telp|whatsapp|no_?hp|mobile|handphone`)
	reBankField    = regexp.MustCompile(`(?i)bank|rekening|account.?(no|number|name)|norek|ewallet|e-wallet`)
	reReferralKeys = regexp.MustCompile(`(?i)^(ref|reff|referral|referal|refcode|ref_code|aff|affiliate|upline|kode_ref|invite|invitation)$`)
	reBankName     = regexp.MustCompile(`(?i)\b(bca|bni|bri|mandiri|cimb|danamon|permata|bsi|btn|mega|maybank|ocbc|panin|jago|seabank|blu|neo|dana|ovo|gopay|linkaja|shopeepay|qris|pulsa|telkomsel|xl|axis|tri|indosat|usdt|bitcoin)\b`)
)

// collectAuthSurface menggabungkan URL login/daftar dari link + form di semua HTML yang tersedia,
// lalu mengambil halaman daftar/login (maks. 3) untuk membaca form-nya.
func (c *Crawler) collectAuthSurface(result *AnalysisResult, pages map[string]string, siteURL string) *AuthSurface {
	as := &AuthSurface{}
	siteHost := ""
	if u, err := url.Parse(siteURL); err == nil {
		siteHost = u.Hostname()
	}

	// URL pendaftaran (termasuk tujuan shortener) & login dari link yang sudah diekstrak
	for _, r := range result.Links.Register {
		if strings.HasPrefix(r, "http") && !IsShortenerURL(r) {
			as.RegisterURLs = mergeUnique(as.RegisterURLs, []string{r})
		}
	}
	for _, l := range append(append([]string{}, result.Links.Internal...), result.Links.External...) {
		if u, err := url.Parse(l); err == nil && reLoginWord.MatchString(u.Path+" "+u.RawQuery) && !isAssetPath(u.Path) {
			as.LoginURLs = mergeUnique(as.LoginURLs, []string{l})
		}
	}

	// Ambil halaman daftar/login yang belum ada di tangan (maks. 3, paralel)
	var targets []string
	for _, u := range append(append([]string{}, as.RegisterURLs...), as.LoginURLs...) {
		if _, ok := pages[u]; !ok && len(targets) < 3 {
			targets = append(targets, u)
		}
	}
	client := &http.Client{Timeout: 10 * time.Second}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, t := range targets {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			if h := fetchHTML(client, t); h != "" {
				mu.Lock()
				pages[t] = h
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()

	// Parse form di setiap halaman
	// Form yang sama sering ada di layout/popup setiap halaman — gabungkan berdasarkan jenis + field,
	// catat halaman lain tempat form itu muncul. Halaman diproses terurut agar hasil stabil.
	pageURLs := make([]string, 0, len(pages))
	for u := range pages {
		pageURLs = append(pageURLs, u)
	}
	sort.Strings(pageURLs)
	formIdx := map[string]int{}
	for _, pageURL := range pageURLs {
		for _, f := range parseAuthForms(pages[pageURL], pageURL, siteHost) {
			key := f.Type + "|" + normalizedFields(f.Fields)
			if i, ok := formIdx[key]; ok {
				if clean := strings.SplitN(pageURL, "#", 2)[0]; clean != strings.SplitN(as.Forms[i].PageURL, "#", 2)[0] {
					as.Forms[i].SeenOn = mergeUnique(as.Forms[i].SeenOn, []string{clean})
				}
				continue
			}
			formIdx[key] = len(as.Forms)
			as.Forms = append(as.Forms, f)
		}
	}
	typeRank := map[string]int{"register": 0, "login": 1, "change_password": 2}
	sort.SliceStable(as.Forms, func(i, j int) bool { return typeRank[as.Forms[i].Type] < typeRank[as.Forms[j].Type] })

	// Kode referral/afiliasi dari query string URL daftar & login
	for _, u := range append(append([]string{}, as.RegisterURLs...), as.LoginURLs...) {
		as.ReferralCodes = mergeUnique(as.ReferralCodes, referralCodes(u))
	}
	return as
}

func referralCodes(raw string) []string {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	var out []string
	for k, vs := range u.Query() {
		if reReferralKeys.MatchString(k) {
			for _, v := range vs {
				if v = strings.TrimSpace(v); v != "" && len(v) <= 64 {
					out = append(out, v)
				}
			}
		}
	}
	return out
}

// parseAuthForms membaca semua <form> yang punya field password lalu mengklasifikasikannya.
func parseAuthForms(page, pageURL, siteHost string) []AuthForm {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil
	}
	base, _ := url.Parse(pageURL)
	var forms []AuthForm

	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "form" {
			if f, ok := readForm(n, base, siteHost); ok {
				forms = append(forms, f)
			}
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)
	return forms
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func readForm(form *html.Node, base *url.URL, siteHost string) (AuthForm, bool) {
	f := AuthForm{Method: strings.ToUpper(attr(form, "method"))}
	if f.Method == "" {
		f.Method = "GET"
	}
	if base != nil {
		f.PageURL = base.String()
		act := attr(form, "action")
		if ref, err := url.Parse(strings.TrimSpace(act)); err == nil {
			f.Action = base.ResolveReference(ref).String()
			if h := ref.Hostname(); h != "" && rootDomain(h) != rootDomain(siteHost) {
				f.ExternalAct = true
			}
		}
	}

	hasPassword, passwordCount := false, 0
	var signals strings.Builder
	signals.WriteString(attr(form, "id") + " " + attr(form, "name") + " " + attr(form, "class") + " " + attr(form, "action") + " ")
	seenField := map[string]bool{}
	banks := map[string]bool{}

	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "input", "select", "textarea":
				typ := strings.ToLower(attr(n, "type"))
				if typ == "hidden" || typ == "submit" || typ == "button" {
					if reCaptcha.MatchString(attr(n, "name")) {
						f.HasCaptcha = true
					}
					break
				}
				name := firstNonEmpty(attr(n, "name"), attr(n, "id"), attr(n, "placeholder"))
				if typ == "password" {
					hasPassword = true
					passwordCount++
				}
				label := strings.TrimSpace(firstNonEmpty(attr(n, "placeholder"), name))
				if label != "" && !seenField[strings.ToLower(label)] && len(f.Fields) < 25 {
					seenField[strings.ToLower(label)] = true
					f.Fields = append(f.Fields, label)
				}
				signals.WriteString(name + " " + attr(n, "placeholder") + " ")
				if n.Data == "select" {
					// Dropdown bank: dikenali dari nama field ATAU dari isinya (≥3 opsi nama bank/e-wallet),
					// karena sebagian situs memberi nama field generik (mis. "payment", "method_id").
					var opts []string
					for _, o := range descendants(n, "option") {
						if txt := strings.TrimSpace(textOf(o)); txt != "" && reBankName.MatchString(txt) {
							opts = append(opts, strings.ToUpper(txt))
						}
					}
					if reBankField.MatchString(name) || len(opts) >= 3 {
						for _, o := range opts {
							banks[o] = true
						}
					}
				}
				// Pilihan bank berbentuk radio: <input type="radio" name="bank…" value="BCA">
				if typ == "radio" && reBankField.MatchString(name) {
					if v := strings.TrimSpace(attr(n, "value")); v != "" && reBankName.MatchString(v) {
						banks[strings.ToUpper(v)] = true
					}
				}
			case "button", "a", "label", "span":
				signals.WriteString(textOf(n) + " ")
			}
			if reCaptcha.MatchString(attr(n, "class") + " " + attr(n, "id") + " " + attr(n, "src")) {
				f.HasCaptcha = true
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(form)

	if !hasPassword {
		return f, false // bukan form autentikasi
	}
	sig := signals.String()
	fieldSig := strings.Join(f.Fields, " ")
	hasPersonal := reBankField.MatchString(fieldSig) || rePhoneField.MatchString(fieldSig) || strings.Contains(strings.ToLower(fieldSig), "email")
	switch {
	case reChangePass.MatchString(fieldSig) && !hasPersonal:
		f.Type = "change_password" // "Password Baru + Konfirmasi" bukan form daftar
	case passwordCount >= 2 || hasPersonal:
		f.Type = "register"
	case passwordCount == 1:
		f.Type = "login" // satu password tanpa data diri = login, walau ada tombol "Daftar" di dekatnya
	case reRegisterWord.MatchString(sig) && reRegisterHint.MatchString(sig):
		f.Type = "register"
	default:
		f.Type = "login"
	}
	f.HasOTP = reOTP.MatchString(sig)
	f.NeedsPhone = rePhoneField.MatchString(sig)
	f.NeedsBank = reBankField.MatchString(sig)
	for b := range banks {
		f.BankOptions = append(f.BankOptions, b)
	}
	sort.Strings(f.BankOptions)
	return f, true
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// normalizedFields: daftar field huruf kecil & terurut, agar "Password"/"password" dianggap sama.
func normalizedFields(fields []string) string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = strings.ToLower(strings.TrimSpace(f))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// descendants mengembalikan semua elemen bertag `tag` di bawah n (termasuk dalam <optgroup>).
func descendants(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type == html.ElementNode && ch.Data == tag {
				out = append(out, ch)
			}
			walk(ch)
		}
	}
	walk(n)
	return out
}
