// Package browser mengelola proses Chrome headless (chromedp) agar tidak meninggalkan sampah:
// setiap instance memakai folder profil sendiri di bawah $TMPDIR/jutec-chrome yang dihapus saat selesai,
// dan sisa dari proses backend sebelumnya (restart/crash) dibersihkan saat backend dinyalakan.
// Tanpa ini, profil ±130 MB per Chrome menumpuk sampai disk penuh.
package browser

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// BaseDir adalah induk semua folder profil Chrome milik Jutec.
func BaseDir() string { return filepath.Join(os.TempDir(), "jutec-chrome") }

// NewAllocator seperti chromedp.NewExecAllocator, dengan folder profil yang dihapus saat cancel dipanggil.
func NewAllocator(ctx context.Context, opts ...chromedp.ExecAllocatorOption) (context.Context, context.CancelFunc) {
	_ = os.MkdirAll(BaseDir(), 0o700)
	dir, err := os.MkdirTemp(BaseDir(), "run-")
	if err != nil {
		return chromedp.NewExecAllocator(ctx, opts...) // fallback: perilaku bawaan chromedp
	}
	opts = append(opts, chromedp.UserDataDir(dir))
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	return allocCtx, func() {
		cancel() // menunggu proses Chrome berhenti
		_ = os.RemoveAll(dir)
	}
}

// CleanupStale dipanggil sekali saat backend mulai — belum ada Chrome milik proses ini,
// jadi semua Chrome yang memakai BaseDir adalah sisa proses lama (yatim) dan aman dihentikan.
func CleanupStale() {
	base := BaseDir()
	killed := 0
	if out, err := exec.Command("pgrep", "-f", base).Output(); err == nil {
		killed = len(strings.Fields(string(out)))
		_ = exec.Command("pkill", "-f", base).Run()
		time.Sleep(time.Second)
	}
	removed := 0
	if entries, err := os.ReadDir(base); err == nil {
		for _, e := range entries {
			if os.RemoveAll(filepath.Join(base, e.Name())) == nil {
				removed++
			}
		}
	}
	// Folder bawaan chromedp dari versi sebelum paket ini (atau kode lain) yang sudah lama tak tersentuh
	if matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "chromedp-runner*")); len(matches) > 0 {
		for _, m := range matches {
			if fi, err := os.Stat(m); err == nil && time.Since(fi.ModTime()) > 30*time.Minute {
				if os.RemoveAll(m) == nil {
					removed++
				}
			}
		}
	}
	if killed > 0 || removed > 0 {
		log.Printf("🧹 [Browser] Bersihkan sisa Chrome: %d proses dihentikan, %d folder profil dihapus", killed, removed)
	}
}
