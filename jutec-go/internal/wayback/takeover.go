package wayback

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
)

// Takeover: domain yang dulunya situs sah (bisnis, instansi, sekolah) lalu dipakai situs judol —
// biasanya domain kedaluwarsa yang didaftarkan ulang operator untuk menumpang reputasinya di Google.
type Takeover struct {
	Detected   bool      `json:"detected"`
	Domain     string    `json:"domain"`
	FirstSeen  string    `json:"first_seen"`            // snapshot tertua (YYYYMMDDhhmmss)
	LegitTitle string    `json:"legit_title,omitempty"` // judul situs sah pada masa lalu
	LegitURL   string    `json:"legit_url,omitempty"`   // snapshot Wayback situs sah terakhir
	LegitUntil string    `json:"legit_until,omitempty"` // snapshot sah terakhir
	JudolSince string    `json:"judol_since,omitempty"` // snapshot judol pertama (perkiraan waktu diambil alih)
	JudolURL   string    `json:"judol_url,omitempty"`
	Snapshots  int       `json:"snapshots"` // jumlah bulan terarsip
	CheckedAt  time.Time `json:"checked_at"`
}

var (
	reTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reJudolTxt = regexp.MustCompile(`(?i)\b(slot|gacor|togel|maxwin|judi|bandar|toto|casino|poker|sbobet|scatter|jackpot|deposit pulsa|link alternatif)\b`)
)

type snapInfo struct {
	ts, url, title string
	judol          bool
}

func inspect(ts, orig string) *snapInfo {
	page := get(fmt.Sprintf("https://web.archive.org/web/%sid_/%s", ts, orig))
	if page == "" {
		return nil
	}
	s := &snapInfo{ts: ts, url: fmt.Sprintf("https://web.archive.org/web/%s/%s", ts, orig)}
	if m := reTitle.FindStringSubmatch(page); m != nil {
		s.title = strings.TrimSpace(html.UnescapeString(m[1]))
		if len(s.title) > 160 {
			s.title = s.title[:160]
		}
	}
	// Judol bila judulnya bertema judi atau halaman memuat banyak istilah judi
	s.judol = reJudolTxt.MatchString(s.title) || len(reJudolTxt.FindAllString(page, 30)) >= 8
	return s
}

// CheckTakeover membandingkan arsip lama & baru beranda domain. Terdeteksi bila snapshot lama bersih
// (situs sah) dan snapshot setelahnya judol. Mengambil maks. ±8 snapshot (bisection pada riwayat).
func CheckTakeover(domain string) (*Takeover, error) {
	t := &Takeover{Domain: domain, CheckedAt: time.Now()}
	q := fmt.Sprintf("https://web.archive.org/cdx/search/cdx?url=%s&output=json&fl=timestamp,original&filter=!statuscode:[45]..&collapse=timestamp:6&limit=400", domain)
	resp, err := getWithRetry(q)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("wayback CDX: status %d", resp.StatusCode)
	}
	var rows [][]string
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	if len(rows) <= 2 {
		return t, nil // terlalu sedikit arsip untuk menilai riwayat
	}
	rows = rows[1:]
	t.Snapshots, t.FirstSeen = len(rows), rows[0][0]

	at := func(i int) *snapInfo {
		s := inspect(rows[i][0], rows[i][1])
		time.Sleep(1200 * time.Millisecond)
		return s
	}
	// Snapshot tertua yang terbaca (coba 3 pertama)
	var first *snapInfo
	lo := 0
	for ; lo < len(rows) && lo < 3 && first == nil; lo++ {
		first = at(lo)
	}
	lo--
	if first == nil || first.judol || first.title == "" {
		return t, nil // sejak awal judol / tak terbaca → bukan pengambilalihan
	}
	last := at(len(rows) - 1)
	if last == nil || !last.judol {
		return t, nil // arsip terbaru belum judol (mungkin baru saja diambil alih, atau arsip tertinggal)
	}
	// Cari batas sah→judol dengan bisection (maks. ±5 langkah)
	good, bad := first, last
	hi := len(rows) - 1
	for step := 0; step < 5 && hi-lo > 1; step++ {
		mid := (lo + hi) / 2
		s := at(mid)
		if s == nil {
			break
		}
		if s.judol {
			hi, bad = mid, s
		} else {
			lo = mid
			if s.title != "" {
				good = s
			} else {
				good = &snapInfo{ts: s.ts, url: s.url, title: good.title}
			}
		}
	}
	t.Detected = true
	t.LegitTitle, t.LegitURL, t.LegitUntil = good.title, good.url, good.ts
	t.JudolSince, t.JudolURL = bad.ts, bad.url
	return t, nil
}
