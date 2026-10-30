package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"

	"jutec-go/internal/hackertarget"
	"jutec-go/internal/tron"

	"github.com/hibiken/asynq"
)

// Pengayaan dari sumber eksternal yang sebelumnya hanya lewat tombol di halaman detail.
const (
	TypeAutoReverse = "auto:reverse" // reverse lookup ID analytics (HackerTarget) + antrekan domain baru
	TypeAutoCrypto  = "auto:crypto"  // profil transaksi USDT-TRC20 (TronGrid)
)

// Reverse: maks. 8 ID per jam — kuota gratis HackerTarget kecil, sisanya untuk penggunaan manual.
func (h *AutoHandler) Reverse(ctx context.Context, _ *asynq.Task) error {
	ids, err := h.PG.AutoReverseTargets(ctx, 8)
	if err != nil || len(ids) == 0 {
		return nil
	}
	ht := hackertarget.NewClient(os.Getenv("HACKERTARGET_API_KEY"))
	queued := 0
	for _, id := range ids {
		domains, err := ht.ReverseAnalytics(id)
		if errors.Is(err, hackertarget.ErrQuota) {
			log.Printf("⏸️  [Auto] reverse analytics: kuota HackerTarget habis — lanjut putaran berikutnya")
			break
		}
		h.PG.SaveReverseAnalytics(ctx, id, domains, err)
		// Domain baru dengan ID yang sama dengan situs judol → analisis (maks. 20 per putaran)
		for _, d := range h.PG.UnknownDomains(ctx, domains) {
			if queued >= 20 || h.Client == nil {
				break
			}
			payload, _ := json.Marshal(ScanURLPayload{URL: "https://" + d})
			if _, err := h.Client.Enqueue(asynq.NewTask(TypeScanURL, payload), asynq.Queue("default"), asynq.MaxRetry(1), asynq.Unique(24*time.Hour)); err == nil {
				queued++
			}
		}
		time.Sleep(2 * time.Second)
	}
	if queued > 0 || len(ids) > 0 {
		log.Printf("🔁 [Auto] reverse analytics: %d ID dicek, %d domain baru diantrekan", len(ids), queued)
	}
	return nil
}

func (h *AutoHandler) Crypto(ctx context.Context, _ *asynq.Task) error {
	addrs, err := h.PG.AutoCryptoTargets(ctx, 10)
	if err != nil || len(addrs) == 0 {
		return nil
	}
	tc := tron.NewClient(os.Getenv("TRONGRID_API_KEY"))
	for _, a := range addrs {
		p, err := tc.AnyProfile(a, 5)
		if errors.Is(err, tron.ErrRateLimit) {
			break
		}
		if err == nil {
			h.PG.SaveCryptoProfile(ctx, a, p)
		}
		time.Sleep(time.Second)
	}
	log.Printf("💰 [Auto] profil USDT: %d alamat diperbarui", len(addrs))
	return nil
}
