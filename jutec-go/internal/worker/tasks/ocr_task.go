package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"jutec-go/internal/crawler"
	"jutec-go/internal/service"
	"jutec-go/internal/store"

	"github.com/hibiken/asynq"
)

const TypeOCRImage = "ocr:image"

type OCRImagePayload struct {
	SiteID   int    `json:"site_id"`
	ImageURL string `json:"image_url"`
}

func NewOCRImageTask(siteID int, imageURL string) (*asynq.Task, error) {
	payload, err := json.Marshal(OCRImagePayload{
		SiteID:   siteID,
		ImageURL: imageURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}
	return asynq.NewTask(TypeOCRImage, payload, asynq.MaxRetry(3)), nil
}

type OCRImageHandler struct {
	OCRSvc  *service.OCRService
	PG      *store.PostgresDB
}

func NewOCRImageHandler(ocrSvc *service.OCRService, pg *store.PostgresDB) *OCRImageHandler {
	return &OCRImageHandler{
		OCRSvc: ocrSvc,
		PG:     pg,
	}
}

func (h *OCRImageHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload OCRImagePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %v", err)
	}

	log.Printf("[OCR] Downloading and scanning image: %s", payload.ImageURL)

	text, err := h.OCRSvc.ExtractText(payload.ImageURL)
	if err != nil {
		log.Printf("[OCR] Failed to extract text from %s: %v", payload.ImageURL, err)
		return nil // Jangan retry jika gambar tidak bisa di-download/bukan gambar
	}

	if text == "" {
		return nil
	}

	// Ambil detection keywords
	var dbKeywords []crawler.WeightedKeyword
	rows, err := h.PG.Pool.Query(ctx, "SELECT keyword, weight FROM detection_keywords WHERE is_active = true")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var kw crawler.WeightedKeyword
			if err := rows.Scan(&kw.Keyword, &kw.Weight); err == nil {
				dbKeywords = append(dbKeywords, kw)
			}
		}
	}

	// Analisis
	analyzer := crawler.NewAnalyzer()
	result := analyzer.AnalyzeOCRText(text, dbKeywords)

	if len(result.Artifacts) > 0 || len(result.FoundKeywords) > 0 {
		log.Printf("  📸 [OCR Hit] Found %d keywords and %d artifacts in image from Site %d", 
			len(result.FoundKeywords), len(result.Artifacts), payload.SiteID)

		// Simpan artifacts
		if len(result.Artifacts) > 0 {
			var artMaps []map[string]string
			for _, a := range result.Artifacts {
				artMaps = append(artMaps, map[string]string{
					"type": a.Type, "value": a.Value, "name": a.Name,
				})
			}
			if err := h.PG.SaveArtifacts(ctx, payload.SiteID, artMaps); err != nil {
				log.Printf("  ⚠️  [Postgres] Failed to save OCR artifacts: %v", err)
			}
		}

		// Jika is_gambling, update prioritas site
		if result.IsGambling {
			// Kita ambil detail situs saat ini, dan jika skor OCR lebih tinggi, kita naikkan
			_, err = h.PG.Pool.Exec(ctx, `
				UPDATE sites 
				SET is_gambling = true, 
				    priority = CASE WHEN priority = 'Low' THEN $1 ELSE priority END
				WHERE id = $2 AND verified_status IS DISTINCT FROM 'bukan_judol'
			`, result.Priority, payload.SiteID)
			if err != nil {
				log.Printf("  ⚠️  [Postgres] Failed to update site priority from OCR: %v", err)
			}
		}
	}

	return nil
}
