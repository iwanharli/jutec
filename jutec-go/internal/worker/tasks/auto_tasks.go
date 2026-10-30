package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"sync"
	"time"

	"jutec-go/internal/ipintel"
	"jutec-go/internal/netutil"
	"jutec-go/internal/store"
	"jutec-go/internal/wayback"

	"github.com/hibiken/asynq"
)

// Proses otomatis untuk situs judol prioritas High. Masing-masing dibatasi jumlahnya per putaran
// agar tidak membebani Chrome, crt.sh, atau archive.org.
const (
	TypeAutoEvidence = "auto:evidence" // ambil bukti forensik situs aktif (baru / rekening berganti)
	TypeAutoCluster  = "auto:cluster"  // bangun ulang klaster operator
	TypeAutoOrigin   = "auto:origin"   // cari IP asli situs di balik Cloudflare
	TypeAutoWayback  = "auto:wayback"  // ambil arsip Wayback situs yang belum pernah diambil
	TypeAutoReach    = "auto:reach"    // cek keterjangkauan & blokir ISP situs judol

	autoActor = "system (otomatis)"
)

type AutoHandler struct {
	PG         *store.PostgresDB
	Client     *asynq.Client
	URLScanKey string
}

func (h *AutoHandler) Evidence(ctx context.Context, _ *asynq.Task) error {
	targets, err := h.PG.AutoEvidenceTargets(ctx, 3)
	if err != nil {
		log.Printf("⚠️  [Auto] bukti: %v", err)
		return nil
	}
	for _, t := range targets {
		capID, err := h.PG.CreateEvidenceCapture(ctx, t.ID, t.URL, "", autoActor)
		if err != nil {
			continue
		}
		payload, _ := json.Marshal(EvidencePayload{CaptureID: capID, SiteID: t.ID, URL: t.URL, RequestedBy: autoActor})
		if _, err := h.Client.Enqueue(asynq.NewTask(TypeEvidenceCapture, payload, asynq.MaxRetry(0), asynq.Timeout(4*time.Minute)),
			asynq.Queue("default")); err != nil {
			h.PG.FailEvidenceCapture(ctx, capID, "gagal diantrekan: "+err.Error())
			continue
		}
		h.PG.Audit(ctx, autoActor, "", "auto.evidence.queued", fmt.Sprintf("evidence:%d", capID), 202,
			map[string]any{"site_id": t.ID, "url": t.URL, "reason": t.Reason})
		log.Printf("🧾 [Auto] bukti #%d diantrekan untuk %s (%s)", capID, t.URL, t.Reason)
	}
	return nil
}

func (h *AutoHandler) Cluster(ctx context.Context, _ *asynq.Task) error {
	n, err := h.PG.DiscoverOperators(ctx)
	if err != nil {
		log.Printf("⚠️  [Auto] klaster operator: %v", err)
		return nil
	}
	log.Printf("🕸️  [Auto] klaster operator diperbarui: %d operator", n)
	return nil
}

func (h *AutoHandler) Origin(ctx context.Context, _ *asynq.Task) error {
	cands, err := h.PG.AutoOriginCandidates(ctx, 30)
	if err != nil {
		log.Printf("⚠️  [Auto] IP asli: %v", err)
		return nil
	}
	done := 0
	for _, t := range cands {
		if done >= 3 { // crt.sh lambat — 3 situs per putaran
			break
		}
		u, err := url.Parse(t.URL)
		if err != nil || u.Hostname() == "" || net.ParseIP(u.Hostname()) != nil {
			continue
		}
		ips, _ := h.PG.SiteIPs(ctx, t.ID)
		behindCF := false
		for _, ip := range ips {
			if p := net.ParseIP(ip); p != nil && ipintel.IsCloudflare(p) {
				behindCF = true
			}
		}
		if !behindCF {
			// Tidak memakai CDN: IP publik = server langsung. Catat agar tidak dipilih ulang tiap putaran.
			h.PG.SaveSiteOrigin(ctx, t.ID, &ipintel.OriginReport{Domain: u.Hostname(), CheckedAt: time.Now().UTC(),
				Subdomains: []string{}, Candidates: []ipintel.OriginCandidate{}, CDNIPs: []string{},
				Notes: []string{"Situs tidak memakai Cloudflare — IP publiknya adalah server langsung."}})
			continue
		}
		rctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		rep, err := ipintel.FindOrigin(rctx, u.Hostname(), h.URLScanKey)
		cancel()
		if err != nil {
			log.Printf("⚠️  [Auto] IP asli %s: %v", u.Hostname(), err)
			continue
		}
		h.PG.SaveSiteOrigin(ctx, t.ID, rep)
		ips2 := make([]string, 0, len(rep.Candidates))
		for _, c := range rep.Candidates {
			ips2 = append(ips2, c.IP)
		}
		h.PG.Audit(ctx, autoActor, "", "auto.origin.checked", fmt.Sprintf("site:%d", t.ID), 200,
			map[string]any{"domain": u.Hostname(), "candidates": ips2})
		if len(ips2) > 0 {
			log.Printf("🎯 [Auto] %s: %d kandidat IP asli %v", u.Hostname(), len(ips2), ips2)
		}
		done++
		time.Sleep(5 * time.Second)
	}
	return nil
}

func (h *AutoHandler) Wayback(ctx context.Context, _ *asynq.Task) error {
	targets, err := h.PG.AutoWaybackTargets(ctx, 3)
	if err != nil {
		log.Printf("⚠️  [Auto] Wayback: %v", err)
		return nil
	}
	for _, t := range targets {
		u, err := url.Parse(t.URL)
		if err != nil || u.Hostname() == "" {
			h.PG.MarkWaybackChecked(ctx, t.ID)
			continue
		}
		snaps, err := wayback.Fetch(u.Hostname(), 6)
		if err != nil {
			// archive.org sedang menolak/lambat: jangan tandai, coba lagi putaran berikutnya
			log.Printf("⚠️  [Auto] Wayback %s: %v", u.Hostname(), err)
			break
		}
		historic := h.PG.SaveWaybackResult(ctx, t.ID, snaps)
		h.PG.Audit(ctx, autoActor, "", "auto.wayback.fetched", fmt.Sprintf("site:%d", t.ID), 200,
			map[string]any{"snapshots": len(snaps), "historic_artifacts": historic})
		if len(snaps) > 0 {
			log.Printf("🗄️  [Auto] Wayback %s: %d snapshot, %d artefak historis", u.Hostname(), len(snaps), historic)
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}

// Reach mengecek keterjangkauan & blokir ISP hingga 150 situs per putaran (8 paralel).
func (h *AutoHandler) Reach(ctx context.Context, _ *asynq.Task) error {
	targets, err := h.PG.ReachTargets(ctx, 150)
	if err != nil {
		log.Printf("⚠️  [Auto] keterjangkauan: %v", err)
		return nil
	}
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	var mu sync.Mutex
	counts := map[string]int{}
	for _, t := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(t store.AutoTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			cctx, cancel := context.WithTimeout(ctx, 40*time.Second)
			r := netutil.CheckReach(cctx, t.URL)
			cancel()
			changed, prev, err := h.PG.SaveReach(ctx, t.ID, r)
			if err != nil {
				return
			}
			mu.Lock()
			counts[r.Status]++
			mu.Unlock()
			if changed && prev != "" {
				h.PG.Audit(ctx, autoActor, "", "auto.reach.changed", fmt.Sprintf("site:%d", t.ID), 200,
					map[string]any{"from": prev, "to": r.Status, "http_status": r.HTTPStatus})
			}
		}(t)
	}
	wg.Wait()
	if len(targets) > 0 {
		log.Printf("📡 [Auto] keterjangkauan %d situs: %v", len(targets), counts)
	}
	return nil
}
