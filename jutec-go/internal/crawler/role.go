package crawler

import (
	"net/url"
	"sort"
	"strings"
)

// Peran situs dalam jaringan judol:
//   - RoleOperator: situs utama — punya form login/daftar sendiri, lobi/provider game, atau meminta rekening.
//     Di sinilah akun pemain, deposit, dan rekening berada → target utama penindakan.
//   - RoleLanding: pintu masuk — halaman promosi/SEO yang tombol DAFTAR/LOGIN-nya mengarah ke domain lain.
//     Murah, banyak, dan cepat diganti; nilainya ada pada ke mana ia mengarah.
const (
	RoleOperator = "operator"
	RoleLanding  = "landing"
)

// ClassifyRole menentukan peran situs dan host tujuan tombol daftar/login di domain lain.
// role kosong = sinyal tidak cukup (mis. situs tidak terbaca atau bukan situs judol).
func ClassifyRole(r *AnalysisResult, siteURL string) (role string, targets []string) {
	if r == nil {
		return "", nil
	}
	u, err := url.Parse(siteURL)
	if err != nil || u.Hostname() == "" {
		return "", nil
	}
	siteRoot := rootDomain(u.Hostname())

	// Form login/daftar yang ada DI HALAMAN situs ini dan mengirim ke situs ini sendiri
	ownForm, asksBank := false, false
	targetSet := map[string]bool{}
	addTarget := func(raw string) {
		t, err := url.Parse(raw)
		if err != nil || t.Hostname() == "" || IsShortenerURL(raw) {
			return
		}
		if h := strings.TrimPrefix(strings.ToLower(t.Hostname()), "www."); rootDomain(h) != siteRoot {
			targetSet[h] = true
		}
	}
	if as := r.AuthSurface; as != nil {
		for _, f := range as.Forms {
			if f.Type != "login" && f.Type != "register" {
				continue
			}
			p, _ := url.Parse(f.PageURL)
			onOwnPage := p != nil && rootDomain(p.Hostname()) == siteRoot
			switch {
			case onOwnPage && !f.ExternalAct:
				ownForm = true
				asksBank = asksBank || len(f.BankOptions) > 0 || f.NeedsBank
			case f.ExternalAct:
				addTarget(f.Action)
			}
		}
		for _, l := range append(append([]string{}, as.RegisterURLs...), as.LoginURLs...) {
			addTarget(l)
		}
	}
	if r.Links != nil {
		for _, l := range r.Links.Register {
			addTarget(l)
		}
	}
	for h := range targetSet {
		targets = append(targets, h)
	}
	sort.Strings(targets)

	hasProvider := r.NetworkSignals != nil && len(r.NetworkSignals.GameProviders) > 0
	switch {
	case ownForm || hasProvider || asksBank:
		return RoleOperator, targets
	case len(targets) > 0:
		return RoleLanding, targets
	}
	return "", targets
}
