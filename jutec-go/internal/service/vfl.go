package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"jutec-go/internal/store"
	"jutec-go/internal/telegram"
)

const (
	vflBot      = "VflOSINT_bot"
	vflGap      = 30 * time.Second // jeda minimal antar kueri ke bot
	VflDailyCap = 150              // batas kueri per 24 jam
	vflMaxDepth = 2                // nomor judol → NIK → nomor lain milik NIK itu (berhenti)
	vflMaxNIK   = 3                // NIK per nomor yang dilanjutkan
	vflRecheck  = 30 * 24 * time.Hour
)

var (
	ErrVflDailyCap = errors.New("batas harian kueri VflOSINT tercapai")

	vflMu   sync.Mutex // satu kueri ke bot pada satu waktu (proses ini)
	vflLast time.Time

	reVflPhone = regexp.MustCompile(`Telephone:\s*(\d{9,15})`)
	reVflReg   = regexp.MustCompile(`date of registration:\s*(\S+)`)
	reVflNIK   = regexp.MustCompile(`Passport number:\s*(\d{16})`)
	reVflProv  = regexp.MustCompile(`Provider:\s*(.+)`)
	reNIK      = regexp.MustCompile(`^\d{16}$`)
)

// VflRecord: satu blok KomInfo dari balasan bot.
type VflRecord struct {
	Phone, NIK, Provider, RegDate string
}

// ParseVflReply mengambil blok KomInfo dari balasan bot. Bot mencocokkan AWALAN, jadi hanya blok yang
// nomornya (untuk kueri nomor) atau NIK-nya (untuk kueri NIK) sama persis yang dipertahankan.
func ParseVflReply(msgs []telegram.BotMessage, query string) (recs []VflRecord, found bool) {
	isNIK := reNIK.MatchString(query)
	for _, m := range msgs {
		if strings.Contains(m.Text, "No results found") {
			return nil, false
		}
		if !strings.Contains(m.Text, "KomInfo") {
			continue
		}
		for _, block := range strings.Split(m.Text, "\n\n") {
			p, n := first(reVflPhone, block), first(reVflNIK, block)
			if p == "" || n == "" {
				continue
			}
			if (isNIK && n != query) || (!isNIK && p != query) {
				continue
			}
			recs = append(recs, VflRecord{Phone: p, NIK: n, Provider: strings.TrimSpace(first(reVflProv, block)), RegDate: first(reVflReg, block)})
		}
	}
	return recs, len(recs) > 0
}

func first(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

type VflService struct {
	tg *telegram.Client
	pg *store.PostgresDB
}

func NewVflService(apiID int, apiHash string, pg *store.PostgresDB) *VflService {
	return &VflService{tg: telegram.NewClient(apiID, apiHash), pg: pg}
}

// VflChainResult ringkasan satu rantai pengecekan.
type VflChainResult struct {
	Queries int         `json:"queries"`
	Records []VflRecord `json:"records"`
	Skipped []string    `json:"skipped,omitempty"`
}

// Chain mengecek nomor judol lalu otomatis mengecek NIK pemiliknya, lalu menyimpan nomor lain milik NIK itu.
// Kedalaman dibatasi agar tidak melebar ke orang yang tidak terkait.
func (s *VflService) Chain(ctx context.Context, artifactID int, phone string) (*VflChainResult, error) {
	phone = strings.TrimPrefix(strings.TrimSpace(phone), "+")
	res := &VflChainResult{}
	type item struct {
		q, parent string
		depth     int
	}
	queue := []item{{q: phone, depth: 0}}
	seen := map[string]bool{}
	var root *int
	if artifactID > 0 {
		root = &artifactID
	}
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		if seen[it.q] {
			continue
		}
		seen[it.q] = true
		if s.pg.VflQueriedWithin(ctx, it.q, vflRecheck) {
			res.Skipped = append(res.Skipped, it.q)
			// tetap lanjutkan rantai dari data yang sudah tersimpan
			if it.depth < 1 {
				rows, _ := s.pg.GetLeakKomInfoByPhone(ctx, it.q)
				for _, r := range rows {
					queue = append(queue, item{q: r.NIK, parent: it.q, depth: it.depth + 1})
				}
			}
			continue
		}
		recs, err := s.ask(ctx, it.q, it.depth, root, it.parent)
		res.Queries++
		if err != nil {
			if res.Queries == 1 {
				return res, err
			}
			log.Printf("⚠️  [VflOSINT] %s: %v — rantai dihentikan", it.q, err)
			break
		}
		res.Records = append(res.Records, recs...)
		if it.depth+1 >= vflMaxDepth { // NIK (depth 1) menghasilkan nomor lain → disimpan, tidak dicek lagi
			continue
		}
		niks := map[string]bool{}
		for _, r := range recs {
			if !niks[r.NIK] && len(niks) < vflMaxNIK {
				niks[r.NIK] = true
				queue = append(queue, item{q: r.NIK, parent: it.q, depth: it.depth + 1})
			}
		}
	}
	return res, nil
}

// ask mengirim satu kueri (dengan jeda & batas harian), mem-parse, menyimpan, dan mencatat audit.
func (s *VflService) ask(ctx context.Context, q string, depth int, root *int, parent string) ([]VflRecord, error) {
	vflMu.Lock()
	defer vflMu.Unlock()
	if s.pg.VflQueriesToday(ctx) >= VflDailyCap {
		return nil, ErrVflDailyCap
	}
	if wait := vflGap - time.Since(vflLast); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	kind := "phone"
	if reNIK.MatchString(q) {
		kind = "nik"
	}
	msgs, err := s.tg.AskBot(ctx, vflBot, q, 75*time.Second)
	vflLast = time.Now()
	if err != nil {
		s.pg.SaveVflQuery(ctx, q, kind, depth, root, parent, "error", 0, "", err.Error())
		return nil, err
	}
	var raw strings.Builder
	for _, m := range msgs {
		raw.WriteString(m.Text + "\n\n")
	}
	recs, found := ParseVflReply(msgs, q)
	status := "not_found"
	if found {
		status = "found"
	}
	for _, r := range recs {
		var aid *int
		if depth == 0 {
			aid = root // hanya nomor judol aslinya yang ditautkan ke artefak
		}
		if err := s.pg.SaveLeakKomInfoSource(ctx, r.Phone, r.NIK, r.Provider, r.RegDate, "VflOSINT", aid); err != nil {
			log.Printf("⚠️  [VflOSINT] simpan %s: %v", r.Phone, err)
		}
	}
	s.pg.SaveVflQuery(ctx, q, kind, depth, root, parent, status, len(recs), raw.String(), "")
	log.Printf("🕵️  [VflOSINT] %s %s (depth %d) → %s, %d data", kind, q, depth, status, len(recs))
	return recs, nil
}

// RunPending mengecek otomatis nomor judol yang belum pernah dicek (dipanggil terjadwal).
func (s *VflService) RunPending(ctx context.Context, limit int) (int, error) {
	list, err := s.pg.VflPendingPhones(ctx, limit)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, a := range list {
		if _, err := s.Chain(ctx, a.ID, a.Phone); err != nil {
			if errors.Is(err, telegram.ErrNotLoggedIn) || errors.Is(err, ErrVflDailyCap) || telegram.IsFlood(err) {
				return done, err
			}
			log.Printf("⚠️  [VflOSINT] %s: %v", a.Phone, err)
			continue
		}
		done++
	}
	return done, nil
}

func (r VflRecord) String() string { return fmt.Sprintf("%s/%s", r.Phone, r.NIK) }
