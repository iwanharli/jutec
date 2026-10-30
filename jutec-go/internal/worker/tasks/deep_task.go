package tasks

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"jutec-go/internal/apkscan"
	"jutec-go/internal/qris"
	"jutec-go/internal/store"

	"github.com/hibiken/asynq"
)

// TypeDeepSite: analisis lanjutan situs judol di background — dekode QRIS, OCR banner, analisis APK.
const TypeDeepSite = "site:deep"

type DeepPayload struct {
	SiteID int `json:"site_id"`
}

func NewDeepSiteTask(siteID int) *asynq.Task {
	payload, _ := json.Marshal(DeepPayload{SiteID: siteID})
	// Unique 6 jam: scan ulang beruntun tidak mengulang pekerjaan berat yang sama
	return asynq.NewTask(TypeDeepSite, payload, asynq.MaxRetry(1), asynq.Queue("low"),
		asynq.Unique(6*time.Hour), asynq.Timeout(15*time.Minute))
}

type DeepSiteHandler struct {
	PG     *store.PostgresDB
	Client *asynq.Client
}

// Banner yang kemungkinan memuat rekening/kontak/promo → layak di-OCR
var reOCRWorthy = regexp.MustCompile(`(?i)(qr|qris|deposit|depo|bayar|payment|bank|rek|wa|whatsapp|contact|kontak|cs|promo|bonus|banner|popup|info)`)

func (h *DeepSiteHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p DeepPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil
	}
	siteURL, images, apps, err := h.PG.DeepTargets(ctx, p.SiteID)
	if err != nil {
		return nil
	}
	log.Printf("🔬 [Deep] site=%d %s · %d gambar · %d APK", p.SiteID, siteURL, len(images), len(apps))

	h.decodeQRIS(ctx, p.SiteID, images)
	h.queueOCR(p.SiteID, images)
	for _, a := range apps {
		h.analyzeAPK(ctx, a)
	}
	return nil
}

// decodeQRIS mencoba membaca QR di gambar situs; merchant QRIS disimpan sebagai artefak.
func (h *DeepSiteHandler) decodeQRIS(ctx context.Context, siteID int, images []string) {
	if len(images) > 25 {
		images = images[:25]
	}
	client := &http.Client{Timeout: 10 * time.Second}
	var mu sync.Mutex
	var found []map[string]string
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, img := range images {
		wg.Add(1)
		sem <- struct{}{}
		go func(img string) {
			defer wg.Done()
			defer func() { <-sem }()
			m, err := qris.DecodeURL(client, img)
			if err != nil || m == nil || m.Key() == "" {
				return
			}
			name := m.MerchantName
			if m.Acquirer != "" {
				name += " · " + m.Acquirer
			}
			mu.Lock()
			found = append(found, map[string]string{"type": "QRIS", "value": m.Key(), "name": strings.TrimSpace(name)})
			mu.Unlock()
			log.Printf("  🔳 [QRIS] %s → merchant=%q nmid=%s pjsp=%s kota=%s", img, m.MerchantName, m.NMID, m.Acquirer, m.City)
		}(img)
	}
	wg.Wait()
	if len(found) > 0 {
		if err := h.PG.SaveArtifacts(ctx, siteID, found); err != nil {
			log.Printf("  ⚠️  [QRIS] Gagal simpan: %v", err)
		}
	}
}

// queueOCR mengantrekan OCR untuk banner yang relevan (antrean "ocr", handler sudah ada).
func (h *DeepSiteHandler) queueOCR(siteID int, images []string) {
	if h.Client == nil {
		return
	}
	n := 0
	for _, img := range images {
		if n == 8 {
			break
		}
		if !reOCRWorthy.MatchString(img) {
			continue
		}
		payload, _ := json.Marshal(map[string]any{"site_id": siteID, "image_url": img})
		if _, err := h.Client.Enqueue(asynq.NewTask(TypeOCRImage, payload, asynq.MaxRetry(1)), asynq.Queue("ocr"),
			asynq.Unique(24*time.Hour)); err == nil {
			n++
		}
	}
	if n > 0 {
		log.Printf("  📸 [Deep] %d banner diantrekan untuk OCR", n)
	}
}

// analyzeAPK membongkar APK (tanpa menjalankan) dan menyebarkan sidik jarinya ke semua situs pemakai.
func (h *DeepSiteHandler) analyzeAPK(ctx context.Context, app store.DeepApp) {
	res, err := apkscan.Analyze(app.URL)
	rec := store.APKRecord{}
	if res != nil {
		rec = store.APKRecord{SHA256: res.SHA256, SizeBytes: res.SizeBytes, PackageName: res.PackageName,
			VersionName: res.VersionName, AppLabel: res.AppLabel, CertSHA256: res.CertSHA256, CertSubject: res.CertSubject,
			CertIssuer: res.CertIssuer, Permissions: res.Permissions, URLs: res.URLs, Domains: res.Domains}
		if !res.CertNotBefore.IsZero() {
			nb := res.CertNotBefore
			rec.CertNotBefore = &nb
		}
	}
	if err != nil {
		rec.Error = err.Error()
	}
	if err := h.PG.SaveAPKAnalysis(ctx, app.ID, rec); err != nil {
		log.Printf("  ⚠️  [APK] Gagal simpan analisis: %v", err)
		return
	}
	if res == nil {
		log.Printf("  📦 [APK] %s gagal: %s", app.URL, rec.Error)
		return
	}
	log.Printf("  📦 [APK] %s → pkg=%s cert=%s… (%s)", app.URL, res.PackageName, short(res.CertSHA256), res.CertSubject)

	var fps []map[string]string
	if res.CertSHA256 != "" {
		fps = append(fps, map[string]string{"kind": "apk_cert", "value": res.CertSHA256[:32]})
	}
	if res.PackageName != "" {
		fps = append(fps, map[string]string{"kind": "apk_pkg", "value": res.PackageName})
	}
	for i, d := range res.Domains {
		if i == 10 {
			break
		}
		fps = append(fps, map[string]string{"kind": "api", "value": d})
	}
	for _, siteID := range h.PG.SitesUsingApp(ctx, app.ID) {
		if err := h.PG.SaveFingerprints(ctx, siteID, fps); err != nil {
			log.Printf("  ⚠️  [APK] Gagal simpan sidik jari site %d: %v", siteID, err)
		}
	}
}

func short(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}
