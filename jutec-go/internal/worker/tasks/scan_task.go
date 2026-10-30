package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"jutec-go/internal/service"

	"github.com/hibiken/asynq"
)

const TypeScanURL = "scan:url"

type ScanURLPayload struct {
	URL string `json:"url"`
}

func NewScanURLTask(url string) (*asynq.Task, error) {
	payload, err := json.Marshal(ScanURLPayload{URL: url})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}
	return asynq.NewTask(TypeScanURL, payload), nil
}

type ScanURLHandler struct {
	ScanSvc *service.ScanService
}

func NewScanURLHandler(svc *service.ScanService) *ScanURLHandler {
	return &ScanURLHandler{ScanSvc: svc}
}

func (h *ScanURLHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload ScanURLPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %v", err)
	}

	log.Printf("[Worker] Scanning: %s", payload.URL)

	result, err := h.ScanSvc.AnalyzeURL(ctx, payload.URL, 0)
	if err != nil {
		return fmt.Errorf("scan failed for %s: %v", payload.URL, err)
	}

	if result.Result != nil {
		log.Printf("[Worker] Done: %s | gambling=%v | priority=%s | confidence=%.2f",
			payload.URL, result.Result.IsGambling, result.Result.Priority, result.Result.Confidence)
	}

	return nil
}
