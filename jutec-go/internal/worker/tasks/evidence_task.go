package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"jutec-go/internal/evidence"
	"jutec-go/internal/store"

	"github.com/hibiken/asynq"
)

const TypeEvidenceCapture = "evidence:capture"

type EvidencePayload struct {
	CaptureID   int    `json:"capture_id"`
	SiteID      int    `json:"site_id"`
	URL         string `json:"url"`
	RequestedBy string `json:"requested_by"`
}

type EvidenceHandler struct {
	PG      *store.PostgresDB
	BaseDir string
}

// EvidenceDirFor: <base>/site-<id>/<capture>-<UTC> — nama unik, tidak pernah ditimpa.
func EvidenceDirFor(base string, siteID, captureID int) string {
	return filepath.Join(base, fmt.Sprintf("site-%d", siteID), fmt.Sprintf("%06d-%s", captureID, time.Now().UTC().Format("20060102T150405Z")))
}

func (h *EvidenceHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p EvidencePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil
	}
	dir := EvidenceDirFor(h.BaseDir, p.SiteID, p.CaptureID)
	h.PG.SetEvidenceDir(ctx, p.CaptureID, dir)
	target := fmt.Sprintf("evidence:%d", p.CaptureID)

	res, err := evidence.Capture(ctx, dir, p.CaptureID, p.SiteID, p.URL, p.RequestedBy)
	if err != nil {
		log.Printf("⚠️  [Bukti] #%d %s: %v", p.CaptureID, p.URL, err)
		h.PG.FailEvidenceCapture(ctx, p.CaptureID, err.Error())
		h.PG.Audit(ctx, "system", "", "evidence.failed", target, 500, map[string]any{"site_id": p.SiteID, "url": p.URL, "error": err.Error()})
		return nil
	}
	files := make([]store.EvidenceFile, 0, len(res.Files))
	for _, f := range res.Files {
		files = append(files, store.EvidenceFile{Name: f.Name, Kind: f.Kind, SHA256: f.SHA256, Size: f.Size})
	}
	m := res.Manifest
	if err := h.PG.CompleteEvidenceCapture(ctx, p.CaptureID, m.FinalURL, int(m.HTTPStatus), m.ServerIP, m.Redirects,
		m.UserAgent, res.ManifestSHA256, m.FinishedAt, files); err != nil {
		log.Printf("⚠️  [Bukti] #%d simpan: %v", p.CaptureID, err)
		return nil
	}
	h.PG.Audit(ctx, "system", "", "evidence.captured", target, 200, map[string]any{
		"site_id": p.SiteID, "url": p.URL, "final_url": m.FinalURL, "manifest_sha256": res.ManifestSHA256, "files": len(files),
	})
	log.Printf("🧾 [Bukti] #%d %s → %d file, manifest %s…", p.CaptureID, p.URL, len(files), res.ManifestSHA256[:12])
	return nil
}
