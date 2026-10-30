package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"jutec-go/internal/service"
	"jutec-go/internal/store"

	"github.com/hibiken/asynq"
)

const TypeRescanStale = "rescan:stale"

// RescanStaleHandler memproses task rescan untuk semua situs yang sudah lama tidak di-scan.
type RescanStaleHandler struct {
	ScanSvc *service.ScanService
	PG      *store.PostgresDB
}

func NewRescanStaleHandler(svc *service.ScanService, pg *store.PostgresDB) *RescanStaleHandler {
	return &RescanStaleHandler{ScanSvc: svc, PG: pg}
}

func (h *RescanStaleHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	if h.PG == nil {
		return nil
	}

	staleDays := 7 // re-scan situs yang belum di-scan lebih dari 7 hari
	cutoff := time.Now().AddDate(0, 0, -staleDays)

	rows, err := h.PG.Pool.Query(ctx, `
		SELECT id, url FROM sites
		WHERE (last_scanned_at < $1 OR last_scanned_at IS NULL)
		  AND status NOT IN ('unreachable')
		ORDER BY last_scanned_at ASC NULLS FIRST
		LIMIT 100
	`, cutoff)
	if err != nil {
		return fmt.Errorf("rescan stale: query failed: %w", err)
	}
	defer rows.Close()

	type staleEntry struct {
		ID  int
		URL string
	}
	var entries []staleEntry
	for rows.Next() {
		var e staleEntry
		if err := rows.Scan(&e.ID, &e.URL); err == nil {
			entries = append(entries, e)
		}
	}

	log.Printf("🔄 [RescanStale] Found %d stale sites (older than %d days)", len(entries), staleDays)

	enqueued := 0
	for _, e := range entries {
		payload, _ := json.Marshal(ScanURLPayload{URL: e.URL})
		task := asynq.NewTask(TypeScanURL, payload, asynq.MaxRetry(1), asynq.Queue("default"))
		if _, err := h.ScanSvc.TaskClient.Enqueue(task); err == nil {
			enqueued++
		}

		// Update next_scan_at
		h.PG.Pool.Exec(ctx, `
			UPDATE sites SET next_scan_at = $1, rescan_count = COALESCE(rescan_count, 0) + 1
			WHERE id = $2
		`, time.Now().AddDate(0, 0, staleDays), e.ID)
	}

	log.Printf("✅ [RescanStale] Enqueued %d rescan tasks", enqueued)
	return nil
}
