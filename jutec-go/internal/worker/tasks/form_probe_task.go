package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"jutec-go/internal/crawler"
	"jutec-go/internal/probeio"
	"jutec-go/internal/store"

	"github.com/hibiken/asynq"
)

const TypeFormProbe = "form:probe"

type FormProbeHandler struct {
	PG      *store.PostgresDB
	BaseDir string
}

// ProcessTask: daftar → login → buka deposit dengan data dummy. Semua respons & screenshot disimpan
// sebagai berkas ber-hash; rekening/kontak yang muncul setelah daftar/login ditautkan ke situs.
func (h *FormProbeHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p struct {
		ProbeID int    `json:"probe_id"`
		Action  string `json:"action"`
		Login   string `json:"login_url"`
	}
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil
	}
	pr, err := h.PG.GetFormProbe(ctx, p.ProbeID)
	if err != nil {
		return nil
	}
	if h.PG.BatchStopped(ctx, h.PG.FormProbeBatchID(ctx, pr.ID)) {
		h.PG.FinishFormProbe(ctx, pr.ID, "skipped", "", "", "uji massal dihentikan", "", map[string]any{}, []any{})
		return nil
	}
	pp, err := h.PG.GetProbeProfile(ctx, h.PG.FormProbeProfileID(ctx, pr.ID))
	if err != nil {
		h.PG.FinishFormProbe(ctx, pr.ID, "error", "", "", "identitas dummy tidak ditemukan", "", map[string]any{}, []any{})
		return nil
	}
	prof := crawler.ProbeProfile{Username: pr.Username, Password: pp.Password, FullName: pp.FullName, Email: pp.Email, Phone: pp.Phone}
	// Rekening uji bank = daftar bersama (settings probe_accounts) untuk semua identitas
	json.Unmarshal([]byte(h.PG.GetSetting(ctx, "probe_accounts")), &prof.Accounts)
	var own []crawler.ProbeAccount
	if json.Unmarshal(pp.Accounts, &own) == nil {
		prof.Accounts = append(prof.Accounts, own...) // sisa data lama per identitas, bila ada
	}
	log.Printf("🧪 [Uji Form] #%d %s (user %s)", pr.ID, pr.PageURL, pr.Username)
	res := crawler.NewCrawler().ProbeForm(pr.PageURL, p.Action, p.Login, prof)

	// Berkas bukti uji: screenshot & HTML sebelum/sesudah, dengan SHA-256
	probeio.Save(ctx, h.PG, h.BaseDir, pr.ID, pr.SiteID, res)
	loginStatus, arts := "", len(res.Artifacts)
	if res.Login != nil {
		loginStatus = res.Login.Status
		arts += len(res.Login.Artifacts)
	}
	h.PG.Audit(ctx, pr.RequestedBy, "", "form.probe."+res.Status, fmt.Sprintf("site:%d", pr.SiteID), 200,
		map[string]any{"probe_id": pr.ID, "outcome": res.Outcome, "login": loginStatus, "artifacts": arts})
	log.Printf("🧪 [Uji Form] #%d selesai: %s/%s login=%s artefak=%d", pr.ID, res.Status, res.Outcome, loginStatus, arts)
	return nil
}
