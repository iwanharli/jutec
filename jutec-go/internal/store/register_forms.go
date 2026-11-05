package store

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// FormField: field form yang sudah dibakukan — Key untuk filter & pola, Raw = label asli di situs.
type FormField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Raw   string `json:"raw"`
}

// RegisterForm: satu form pendaftaran dari auth_surface situs judol (dibaca pasif saat scan).
type RegisterForm struct {
	Kind        string      `json:"kind"` // register | login
	SiteID      int         `json:"site_id"`
	URL         string      `json:"url"`
	Priority    string      `json:"priority"`
	Confidence  float64     `json:"confidence"`
	Category    string      `json:"category"`
	Lifecycle   string      `json:"lifecycle"`
	Verified    string      `json:"verified"` // judol | "" (dikonfirmasi mesin)
	PageURL     string      `json:"page_url"`
	Pages       int         `json:"pages"` // jumlah halaman situs tempat form yang sama muncul
	Action      string      `json:"action"`
	ActionHost  string      `json:"action_host"`
	Method      string      `json:"method"`
	Fields      []FormField `json:"fields"`
	BankOptions []string    `json:"bank_options"`
	HasCaptcha  bool        `json:"has_captcha"`
	HasOTP      bool        `json:"has_otp"`
	NeedsPhone  bool        `json:"needs_phone"`
	NeedsBank   bool        `json:"needs_bank"`
	ExternalAct bool        `json:"external_action"`
	Referrals   []string    `json:"referral_codes"`
	Pattern     string      `json:"pattern"`       // sidik jari struktur form (field baku + tujuan submit)
	PatternSize int         `json:"pattern_sites"` // jumlah situs lain dengan pola sama
}

type RegisterFormFilter struct {
	Kind     string // register (bawaan) | login
	Scope    string // verified (hanya keputusan analis) | valid (analis + bukti kuat mesin, default)
	Query    string // domain / action
	Fields   []string
	Bank     string
	Captcha  string // yes | no
	OTP      string
	Phone    string
	NeedBank string
	External string
	Pattern  string
	Live     bool
}

type Facet struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
	Count int    `json:"count"`
}

// PatternFacet: kelompok situs dengan form identik — indikasi kuat platform/operator yang sama.
type PatternFacet struct {
	ID     string   `json:"id"`
	Sites  int      `json:"sites"`
	Fields []string `json:"fields"` // label field baku
	Host   string   `json:"host"`   // tujuan submit
	Sample []string `json:"sample"` // contoh domain
}

type RegisterFormResult struct {
	Forms    []RegisterForm `json:"forms"`
	Total    int            `json:"total"`
	Sites    int            `json:"sites"`
	Pool     int            `json:"pool"`
	Fields   []Facet        `json:"fields"`
	Banks    []Facet        `json:"banks"`
	Hosts    []Facet        `json:"hosts"`
	Patterns []PatternFacet `json:"patterns"`
	Counts   map[string]int `json:"counts"` // jumlah form per jenis (register/login) sebelum filter
	Stats    map[string]int `json:"stats"`
}

// fieldRules: label/nama field → jenis baku. Urutan penting (aturan pertama yang cocok dipakai).
var fieldRules = []struct {
	key, label string
	re         *regexp.Regexp
}{
	{"confirm_password", "Konfirmasi password", regexp.MustCompile(`(?i)konfirm|confirm|ulang|sekali lagi|re.?pass|password2|retype|verify.?pass`)},
	{"password", "Password", regexp.MustCompile(`(?i)pass|sandi|kata kunci`)},
	{"pin", "PIN", regexp.MustCompile(`(?i)\bpin\b`)},
	{"account_name", "Nama rekening", regexp.MustCompile(`(?i)(nama|name).{0,10}(rek|account|akun|pemilik)|account.?name|acc.?name|atas nama`)},
	{"account_number", "Nomor rekening", regexp.MustCompile(`(?i)(no|nomor|number|num).{0,10}(rek|account|akun)|acc(ount)?.?(no|num)|norek|rekening`)},
	{"bank", "Bank / e-wallet", regexp.MustCompile(`(?i)bank|ewallet|e-wallet|metode|payment`)},
	{"referral", "Kode referral", regexp.MustCompile(`(?i)ref+er|referal|upline|kode.?ajak|invite|aff`)},
	{"phone", "No. HP / WA", regexp.MustCompile(`(?i)phone|telp|telepon|telpon|whatsapp|\bwa\b|\bhp\b|handphone|mobile|no.?hp`)},
	{"email", "Email", regexp.MustCompile(`(?i)e-?mail|surel`)},
	{"captcha", "Captcha", regexp.MustCompile(`(?i)captcha|verifikasi|kode keamanan|security code|validation`)},
	{"otp", "OTP", regexp.MustCompile(`(?i)\botp\b|kode sms|sms code`)},
	{"agree", "Setuju syarat", regexp.MustCompile(`(?i)agree|setuju|terms|syarat|tnc`)},
	{"birth", "Tanggal lahir", regexp.MustCompile(`(?i)lahir|birth|dob`)},
	{"fullname", "Nama lengkap", regexp.MustCompile(`(?i)full.?name|nama lengkap|\bnama\b|\bname\b`)},
	{"username", "Username", regexp.MustCompile(`(?i)user|login|akun|account|id pengguna`)},
	{"currency", "Mata uang", regexp.MustCompile(`(?i)currency|mata uang`)},
	{"game", "Pilihan game", regexp.MustCompile(`(?i)game|produk|product`)},
}

var mojibake = strings.NewReplacer("Â ", " ", " ", " ", "Â", "", "â€™", "'", "â€“", "-")

// canonicalField memetakan label/nama field mentah ke jenis baku ("" bila tak dikenali).
func canonicalField(raw string) FormField {
	clean := strings.Join(strings.Fields(mojibake.Replace(raw)), " ")
	// Nama kontrol ASP.NET (ctl00$SubContent$rg_bankname) → bagian terakhir saja
	probe := clean
	if i := strings.LastIndexAny(probe, "$"); i >= 0 {
		probe = probe[i+1:]
	}
	probe = strings.NewReplacer("_", " ", "-", " ").Replace(probe)
	for _, r := range fieldRules {
		if r.re.MatchString(probe) {
			return FormField{Key: r.key, Label: r.label, Raw: clean}
		}
	}
	return FormField{Key: "other:" + NormalizeField(probe), Label: clean, Raw: clean}
}

// NormalizeField menyederhanakan nama field agar varian (user_name, Username, "Nama Pengguna") terkelompok.
func NormalizeField(f string) string {
	f = strings.ToLower(strings.TrimSpace(f))
	f = strings.NewReplacer("_", " ", "-", " ", "[", " ", "]", " ").Replace(f)
	return strings.Join(strings.Fields(f), " ")
}

func yn(want string, v bool) bool {
	return want == "" || (want == "yes") == v
}

// isLoginOnly: "form daftar" yang hanya berisi username/password (+captcha) sebenarnya form login.
func isLoginOnly(fields []FormField) bool {
	for _, f := range fields {
		switch f.Key {
		case "username", "password", "captcha", "agree":
		default:
			return false
		}
	}
	return true
}

func formPattern(fields []FormField, host string) string {
	keys := map[string]bool{}
	for _, f := range fields {
		if !strings.HasPrefix(f.Key, "other:") {
			keys[f.Key] = true
		}
	}
	list := make([]string, 0, len(keys))
	for k := range keys {
		list = append(list, k)
	}
	sort.Strings(list)
	sum := sha1.Sum([]byte(strings.Join(list, ",") + "|" + host))
	return hex.EncodeToString(sum[:])[:10]
}

func (db *PostgresDB) RegisterForms(ctx context.Context, f RegisterFormFilter) (*RegisterFormResult, error) {
	cond := `s.verified_status = 'judol' OR (COALESCE(s.verified_status,'') <> 'bukan_judol' AND site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND ` + StrongEvidenceSQL + `)`
	if f.Scope == "verified" {
		cond = `s.verified_status = 'judol'`
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT s.id, s.url, COALESCE(s.priority,'Low'), COALESCE(s.confidence,0), `+siteCategorySQL+`,
		       site_lifecycle(s.reach_status, s.status, s.offline_since), COALESCE(s.verified_status,''), s.auth_surface
		FROM sites s
		WHERE s.auth_surface IS NOT NULL AND (`+cond+`)
		ORDER BY s.verified_status IS NULL, s.confidence DESC NULLS LAST`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 1. Kumpulkan semua form daftar (sebelum filter) — dibakukan, form login & duplikat dibuang
	var all []RegisterForm
	pool := map[int]bool{}
	for rows.Next() {
		var base RegisterForm
		var raw []byte
		if err := rows.Scan(&base.SiteID, &base.URL, &base.Priority, &base.Confidence, &base.Category, &base.Lifecycle, &base.Verified, &raw); err != nil {
			return nil, err
		}
		var as struct {
			Forms []struct {
				Type        string   `json:"type"`
				PageURL     string   `json:"page_url"`
				Action      string   `json:"action"`
				Method      string   `json:"method"`
				Fields      []string `json:"fields"`
				BankOptions []string `json:"bank_options"`
				HasCaptcha  bool     `json:"has_captcha"`
				HasOTP      bool     `json:"has_otp"`
				NeedsPhone  bool     `json:"needs_phone"`
				NeedsBank   bool     `json:"needs_bank"`
				ExternalAct bool     `json:"external_action"`
			} `json:"forms"`
			Referrals []string `json:"referral_codes"`
		}
		if json.Unmarshal(raw, &as) != nil {
			continue
		}
		perSite := map[string]int{} // pola → indeks di all (gabungkan form sama di banyak halaman)
		for _, af := range as.Forms {
			if af.Type != "register" && af.Type != "login" {
				continue // ganti password dll.
			}
			r := base
			r.PageURL, r.Action, r.Method = af.PageURL, af.Action, strings.ToUpper(af.Method)
			r.HasCaptcha, r.HasOTP, r.NeedsPhone, r.NeedsBank, r.ExternalAct = af.HasCaptcha, af.HasOTP, af.NeedsPhone, af.NeedsBank, af.ExternalAct
			seen := map[string]bool{}
			for _, x := range af.Fields {
				ff := canonicalField(x)
				if ff.Raw == "" || seen[ff.Key] {
					continue
				}
				seen[ff.Key] = true
				r.Fields = append(r.Fields, ff)
			}
			if len(r.Fields) == 0 {
				continue
			}
			// "Form daftar" yang hanya berisi username/password sebenarnya form login
			r.Kind = af.Type
			if r.Kind == "register" && isLoginOnly(r.Fields) {
				r.Kind = "login"
			}
			for _, b := range af.BankOptions {
				r.BankOptions = append(r.BankOptions, strings.ToUpper(strings.TrimSpace(mojibake.Replace(b))))
			}
			if r.BankOptions == nil {
				r.BankOptions = []string{}
			}
			r.Referrals = as.Referrals
			if r.Referrals == nil {
				r.Referrals = []string{}
			}
			if u, err := url.Parse(r.Action); err == nil {
				r.ActionHost = strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
			}
			r.Pattern = r.Kind[:1] + formPattern(r.Fields, r.ActionHost)
			r.Pages = 1
			if i, ok := perSite[r.Pattern]; ok {
				all[i].Pages++
				continue
			}
			perSite[r.Pattern] = len(all)
			all = append(all, r)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	kind := f.Kind
	if kind != "login" {
		kind = "register"
	}
	counts := map[string]int{}
	for _, r := range all {
		counts[r.Kind]++
		if r.Kind == kind {
			pool[r.SiteID] = true
		}
	}

	// 2. Ukuran tiap pola (jumlah situs berbeda) — dihitung dari semua data, bukan hasil filter
	patSites := map[string]map[int]bool{}
	for _, r := range all {
		if patSites[r.Pattern] == nil {
			patSites[r.Pattern] = map[int]bool{}
		}
		patSites[r.Pattern][r.SiteID] = true
	}

	// 3. Filter
	res := &RegisterFormResult{Forms: []RegisterForm{}, Stats: map[string]int{}, Pool: len(pool), Counts: counts}
	fieldN, fieldLabel, bankN, hostN := map[string]int{}, map[string]string{}, map[string]int{}, map[string]int{}
	sites := map[int]bool{}
	q := strings.ToLower(strings.TrimSpace(f.Query))
	for _, r := range all {
		r.PatternSize = len(patSites[r.Pattern])
		if r.Kind != kind {
			continue
		}
		if f.Live && r.Lifecycle == "inactive" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.URL+" "+r.Action+" "+r.PageURL), q) {
			continue
		}
		if f.Pattern != "" && r.Pattern != f.Pattern {
			continue
		}
		if !yn(f.Captcha, r.HasCaptcha) || !yn(f.OTP, r.HasOTP) || !yn(f.Phone, r.NeedsPhone) || !yn(f.NeedBank, r.NeedsBank) || !yn(f.External, r.ExternalAct) {
			continue
		}
		keys := map[string]bool{}
		for _, x := range r.Fields {
			keys[x.Key] = true
		}
		ok := true
		for _, want := range f.Fields {
			if !keys[want] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if f.Bank != "" {
			hit := false
			for _, b := range r.BankOptions {
				if strings.EqualFold(b, f.Bank) {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
		}

		res.Forms = append(res.Forms, r)
		sites[r.SiteID] = true
		for _, x := range r.Fields {
			if !strings.HasPrefix(x.Key, "other:") {
				fieldN[x.Key]++
				fieldLabel[x.Key] = x.Label
			}
		}
		for _, b := range r.BankOptions {
			bankN[b]++
		}
		if r.ActionHost != "" {
			hostN[r.ActionHost]++
		}
		for k, v := range map[string]bool{"captcha": r.HasCaptcha, "otp": r.HasOTP, "phone": r.NeedsPhone, "bank": r.NeedsBank, "external": r.ExternalAct} {
			if v {
				res.Stats[k]++
			}
		}
	}
	// Form dari pola besar lebih dulu: paling informatif untuk menghubungkan situs
	sort.SliceStable(res.Forms, func(i, j int) bool { return res.Forms[i].PatternSize > res.Forms[j].PatternSize })

	res.Total, res.Sites = len(res.Forms), len(sites)
	res.Fields = facets(fieldN, 40)
	for i := range res.Fields {
		res.Fields[i].Label = fieldLabel[res.Fields[i].Value]
	}
	res.Banks, res.Hosts = facets(bankN, 40), facets(hostN, 20)

	// 4. Pola yang dipakai ≥2 situs (dari hasil filter)
	seenPat := map[string]bool{}
	for _, r := range res.Forms {
		if r.PatternSize < 2 || seenPat[r.Pattern] {
			continue
		}
		seenPat[r.Pattern] = true
		p := PatternFacet{ID: r.Pattern, Sites: r.PatternSize, Host: r.ActionHost, Fields: []string{}, Sample: []string{}}
		for _, x := range r.Fields {
			if !strings.HasPrefix(x.Key, "other:") {
				p.Fields = append(p.Fields, x.Label)
			}
		}
		for _, o := range all {
			if o.Pattern == r.Pattern && len(p.Sample) < 4 {
				p.Sample = append(p.Sample, hostOf(o.URL))
			}
		}
		res.Patterns = append(res.Patterns, p)
	}
	sort.SliceStable(res.Patterns, func(i, j int) bool { return res.Patterns[i].Sites > res.Patterns[j].Sites })
	if res.Patterns == nil {
		res.Patterns = []PatternFacet{}
	}
	return res, nil
}

func facets(m map[string]int, limit int) []Facet {
	out := make([]Facet, 0, len(m))
	for k, v := range m {
		out = append(out, Facet{Value: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
