package store

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
)

// RegisterForm: satu form pendaftaran dari auth_surface situs judol (dibaca pasif saat scan).
type RegisterForm struct {
	SiteID      int      `json:"site_id"`
	URL         string   `json:"url"`
	Priority    string   `json:"priority"`
	Confidence  float64  `json:"confidence"`
	Category    string   `json:"category"`
	Lifecycle   string   `json:"lifecycle"`
	Verified    string   `json:"verified"` // judol | "" (dikonfirmasi mesin)
	PageURL     string   `json:"page_url"`
	Action      string   `json:"action"`
	ActionHost  string   `json:"action_host"`
	Method      string   `json:"method"`
	Fields      []string `json:"fields"`
	BankOptions []string `json:"bank_options"`
	HasCaptcha  bool     `json:"has_captcha"`
	HasOTP      bool     `json:"has_otp"`
	NeedsPhone  bool     `json:"needs_phone"`
	NeedsBank   bool     `json:"needs_bank"`
	ExternalAct bool     `json:"external_action"`
	Referrals   []string `json:"referral_codes"`
}

type RegisterFormFilter struct {
	Scope    string // verified (hanya keputusan analis) | valid (analis + bukti kuat mesin, default)
	Query    string // domain / action
	Fields   []string
	Bank     string
	Captcha  string // yes | no
	OTP      string
	Phone    string
	NeedBank string
	External string
	Live     bool
}

type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type RegisterFormResult struct {
	Forms  []RegisterForm `json:"forms"`
	Total  int            `json:"total"`  // form sesuai filter
	Sites  int            `json:"sites"`  // situs unik sesuai filter
	Pool   int            `json:"pool"`   // situs valid yang punya form daftar (sebelum filter)
	Fields []Facet        `json:"fields"` // frekuensi field di hasil filter
	Banks  []Facet        `json:"banks"`
	Hosts  []Facet        `json:"hosts"` // domain tujuan submit
	Stats  map[string]int `json:"stats"` // captcha/otp/phone/bank/external di hasil filter
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

func (db *PostgresDB) RegisterForms(ctx context.Context, f RegisterFormFilter) (*RegisterFormResult, error) {
	cond := `s.verified_status = 'judol' OR (s.is_gambling AND COALESCE(s.verified_status,'') <> 'bukan_judol' AND ` + StrongEvidenceSQL + `)`
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

	res := &RegisterFormResult{Forms: []RegisterForm{}, Stats: map[string]int{}}
	fieldN, bankN, hostN := map[string]int{}, map[string]int{}, map[string]int{}
	sites := map[int]bool{}
	q := strings.ToLower(strings.TrimSpace(f.Query))
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
		counted := false
		for _, af := range as.Forms {
			if af.Type != "register" {
				continue
			}
			if !counted {
				res.Pool++
				counted = true
			}
			r := base
			r.PageURL, r.Action, r.Method = af.PageURL, af.Action, strings.ToUpper(af.Method)
			r.HasCaptcha, r.HasOTP, r.NeedsPhone, r.NeedsBank, r.ExternalAct = af.HasCaptcha, af.HasOTP, af.NeedsPhone, af.NeedsBank, af.ExternalAct
			r.Fields, r.BankOptions, r.Referrals = af.Fields, af.BankOptions, as.Referrals
			if r.Fields == nil {
				r.Fields = []string{}
			}
			if r.BankOptions == nil {
				r.BankOptions = []string{}
			}
			if r.Referrals == nil {
				r.Referrals = []string{}
			}
			if u, err := url.Parse(r.Action); err == nil {
				r.ActionHost = strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
			}

			// Filter
			if f.Live && r.Lifecycle == "inactive" {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(r.URL+" "+r.Action+" "+r.PageURL), q) {
				continue
			}
			if !yn(f.Captcha, r.HasCaptcha) || !yn(f.OTP, r.HasOTP) || !yn(f.Phone, r.NeedsPhone) || !yn(f.NeedBank, r.NeedsBank) || !yn(f.External, r.ExternalAct) {
				continue
			}
			norm := map[string]bool{}
			for _, x := range r.Fields {
				norm[NormalizeField(x)] = true
			}
			ok := true
			for _, want := range f.Fields {
				if !norm[NormalizeField(want)] {
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
					if strings.EqualFold(strings.TrimSpace(b), f.Bank) {
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
			for k := range norm {
				if k != "" {
					fieldN[k]++
				}
			}
			for _, b := range r.BankOptions {
				bankN[strings.ToUpper(strings.TrimSpace(b))]++
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
	}
	res.Total, res.Sites = len(res.Forms), len(sites)
	res.Fields, res.Banks, res.Hosts = facets(fieldN, 40), facets(bankN, 30), facets(hostN, 20)
	return res, rows.Err()
}

func facets(m map[string]int, limit int) []Facet {
	out := make([]Facet, 0, len(m))
	for k, v := range m {
		out = append(out, Facet{k, v})
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
