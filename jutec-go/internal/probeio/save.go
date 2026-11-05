// Package probeio menyimpan hasil Uji Form (berkas bukti ber-hash + artefak situs) — dipakai bersama
// oleh uji otomatis (worker) dan Mode Pantau (manual).
package probeio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"jutec-go/internal/crawler"
	"jutec-go/internal/store"
)

type File struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}

// Save menulis berkas bukti, menautkan artefak ke situs, dan menutup catatan uji (FinishFormProbe).
func Save(ctx context.Context, pg *store.PostgresDB, baseDir string, probeID, siteID int, res *crawler.ProbeResult) {
	dir := filepath.Join(baseDir, fmt.Sprintf("probe-%d", probeID))
	var files []File
	write := func(name string, b []byte) {
		if len(b) == 0 {
			return
		}
		if os.MkdirAll(dir, 0o750) != nil {
			return
		}
		if os.WriteFile(filepath.Join(dir, name), b, 0o440) == nil {
			sum := sha256.Sum256(b)
			files = append(files, File{Name: name, SHA256: hex.EncodeToString(sum[:]), Size: len(b)})
		}
	}
	write("1-form-terisi.png", res.ShotBefore)
	write("2-setelah-daftar.png", res.ShotAfter)
	write("2-setelah-daftar.html", []byte(res.HTMLAfter))
	loginStatus := ""
	arts := append([]crawler.TransactionArtifact{}, res.Artifacts...)
	if lg := res.Login; lg != nil {
		loginStatus = lg.Status
		write("3-setelah-login.png", lg.ShotAfter)
		write("4-deposit.png", lg.ShotDeposit)
		write("4-deposit.html", []byte(lg.HTMLDeposit))
		arts = append(arts, lg.Artifacts...)
	}
	if len(files) == 0 {
		dir = ""
	}
	if len(arts) > 0 {
		var m []map[string]string
		seen := map[string]bool{}
		for _, a := range arts {
			if a.Type == "" || seen[a.Type+a.Value] {
				continue
			}
			seen[a.Type+a.Value] = true
			m = append(m, map[string]string{"type": a.Type, "value": a.Value, "name": a.Name})
		}
		if err := pg.SaveArtifacts(ctx, siteID, m); err != nil {
			log.Printf("⚠️  [Uji Form] simpan artefak: %v", err)
		}
	}
	pg.FinishFormProbe(ctx, probeID, res.Status, res.Outcome, loginStatus, res.Reason, dir, res, files)
}
