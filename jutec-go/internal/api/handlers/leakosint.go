package handlers

import (
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"jutec-go/internal/leakosint"
	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

// Validasi sebelum memanggil LeakOSINT agar input sampah tidak menghabiskan kuota API.
var (
	phonePattern = regexp.MustCompile(`^(\+?62|0)8\d{7,12}$`)
	nikPattern   = regexp.MustCompile(`^\d{16}$`)
)

type LeakHandler struct {
	PG     *store.PostgresDB
	Client *leakosint.Client
}

func NewLeakHandler(pg *store.PostgresDB, client *leakosint.Client) *LeakHandler {
	return &LeakHandler{PG: pg, Client: client}
}

func (h *LeakHandler) dbRequired(c *gin.Context) bool {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return false
	}
	return true
}

// GET /leak/phone?q=6281200000004
// Lookup nomor HP: query LeakOSINT → simpan → return.
func (h *LeakHandler) LookupPhone(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}

	phone := strings.TrimSpace(c.Query("q"))
	if phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' (phone number) is required"})
		return
	}
	phone = strings.NewReplacer(" ", "", "-", "").Replace(phone)
	if !phonePattern.MatchString(phone) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format nomor HP tidak valid (contoh: 08123456789)"})
		return
	}

	ctx := c.Request.Context()
	norm := strings.TrimPrefix(phone, "+")
	if strings.HasPrefix(norm, "0") {
		norm = "62" + norm[1:]
	}

	// Query LeakOSINT API. Bila gagal (mis. kuota habis), tetap tampilkan data yang sudah tersimpan
	// (termasuk hasil bot VflOSINT).
	result, err := h.Client.Lookup(phone)
	if err != nil || (len(result.KomInfo) == 0 && len(result.Dukcapil) == 0) {
		apiErr := ""
		if err != nil {
			apiErr = err.Error()
		}
		kom, _ := h.PG.GetLeakKomInfoByPhone(ctx, norm)
		entries := []gin.H{}
		related := []gin.H{}
		seenNIK := map[string]bool{}
		for _, k := range kom {
			entries = append(entries, gin.H{"Phone": k.Phone, "Passport": k.NIK, "Provider": k.Provider, "RegDate": k.RegDate, "Source": k.Source})
			if seenNIK[k.NIK] {
				continue
			}
			seenNIK[k.NIK] = true
			others, _ := h.PG.GetLeakKomInfoByNIK(ctx, k.NIK)
			for _, o := range others {
				if o.Phone != norm {
					related = append(related, gin.H{"Phone": o.Phone, "Passport": o.NIK, "Provider": o.Provider, "RegDate": o.RegDate, "Source": o.Source})
				}
			}
		}
		duk, _ := h.PG.GetLeakDukcapilByPhone(ctx, norm)
		dukE := []gin.H{}
		for _, d := range duk {
			dukE = append(dukE, gin.H{"Phone": d.Phone, "Passport": d.NIK, "FullName": d.FullName, "Gender": d.Gender, "BDay": d.BirthDate,
				"Address": d.Address, "District": d.District, "Location": d.Location, "Region": d.Region})
		}
		c.JSON(http.StatusOK, gin.H{
			"phone": norm, "from_cache": true, "api_error": apiErr,
			"kominfo_count": len(entries), "dukcapil_count": len(dukE),
			"kominfo": entries, "dukcapil": dukE, "related": related,
			"vfl_checked": h.PG.VflQueriedWithin(ctx, norm, 30*24*time.Hour),
		})
		return
	}

	// Simpan ke DB
	savedKomInfo := 0
	savedDukcapil := 0

	for _, entry := range result.KomInfo {
		if err := h.PG.SaveLeakKomInfo(ctx, entry.Phone, entry.NIK, entry.Provider, entry.RegDate, nil); err == nil {
			savedKomInfo++
		}
	}

	for _, entry := range result.Dukcapil {
		if err := h.PG.SaveLeakDukcapil(ctx, entry.Phone, entry.NIK, entry.FullName, entry.Gender, entry.BirthDate, entry.Address, entry.District, entry.Location, entry.Region, nil); err == nil {
			savedDukcapil++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"phone":          phone,
		"kominfo_count":  len(result.KomInfo),
		"dukcapil_count": len(result.Dukcapil),
		"saved_kominfo":  savedKomInfo,
		"saved_dukcapil": savedDukcapil,
		"kominfo":        result.KomInfo,
		"dukcapil":       result.Dukcapil,
	})
}

// GET /leak/nik?q=3201010101900003
// Cari semua data (KomInfo + Dukcapil) berdasarkan NIK.
func (h *LeakHandler) LookupNIK(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}

	nik := strings.TrimSpace(c.Query("q"))
	if nik == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' (NIK) is required"})
		return
	}
	if !nikPattern.MatchString(nik) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "NIK harus 16 digit angka"})
		return
	}

	ctx := c.Request.Context()

	kominfo, _ := h.PG.GetLeakKomInfoByNIK(ctx, nik)
	dukcapil, _ := h.PG.GetLeakDukcapilByNIK(ctx, nik)

	c.JSON(http.StatusOK, gin.H{
		"nik":            nik,
		"kominfo_count":  len(kominfo),
		"dukcapil_count": len(dukcapil),
		"kominfo":        kominfo,
		"dukcapil":       dukcapil,
	})
}

// POST /leak/enrich
// Enrich semua artifact nomor HP yang belum pernah di-query.
func (h *LeakHandler) EnrichAll(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}

	ctx := c.Request.Context()

	// Ambil semua artifact HP yang belum di-enrich
	artifacts, err := h.PG.GetPhoneArtifactsNotEnriched(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(artifacts) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":         "no new phone artifacts to enrich",
			"total":           0,
			"total_artifacts": 0,
			"processed":       0,
			"errors":          0,
			"saved_kominfo":   0,
			"saved_dukcapil":  0,
		})
		return
	}

	totalKomInfo := 0
	totalDukcapil := 0
	processed := 0
	errors := 0

	for _, art := range artifacts {
		phone := art.Phone
		log.Printf("[LeakOSINT] Enriching artifact #%d: %s", art.ID, phone)

		result, err := h.Client.Lookup(phone)
		if err != nil {
			log.Printf("[LeakOSINT] Error for %s: %v", phone, err)
			if markErr := h.PG.SaveLeakEnrichmentAttempt(ctx, art.ID, phone, "error", err.Error()); markErr != nil {
				log.Printf("[LeakOSINT] Failed to mark artifact #%d: %v", art.ID, markErr)
			}
			errors++
			continue
		}

		artifactID := art.ID
		for _, entry := range result.KomInfo {
			if err := h.PG.SaveLeakKomInfo(ctx, entry.Phone, entry.NIK, entry.Provider, entry.RegDate, &artifactID); err == nil {
				totalKomInfo++
			}
		}

		for _, entry := range result.Dukcapil {
			if err := h.PG.SaveLeakDukcapil(ctx, entry.Phone, entry.NIK, entry.FullName, entry.Gender, entry.BirthDate, entry.Address, entry.District, entry.Location, entry.Region, &artifactID); err == nil {
				totalDukcapil++
			}
		}

		status := "empty"
		if len(result.KomInfo) > 0 || len(result.Dukcapil) > 0 {
			status = "success"
		}
		if err := h.PG.SaveLeakEnrichmentAttempt(ctx, art.ID, phone, status, ""); err != nil {
			log.Printf("[LeakOSINT] Failed to mark artifact #%d: %v", art.ID, err)
		}
		processed++
	}

	c.JSON(http.StatusOK, gin.H{
		"total_artifacts": len(artifacts),
		"processed":       processed,
		"errors":          errors,
		"saved_kominfo":   totalKomInfo,
		"saved_dukcapil":  totalDukcapil,
	})
}

// GET /leak/pending-count — jumlah artifact HP yang belum di-enrich
func (h *LeakHandler) PendingCount(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	artifacts, err := h.PG.GetPhoneArtifactsNotEnriched(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(artifacts)})
}
