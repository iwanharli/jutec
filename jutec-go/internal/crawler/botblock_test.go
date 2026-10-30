package crawler

import (
	"strings"
	"testing"
)

func TestBotBlockReason(t *testing.T) {
	cf := `<!DOCTYPE html><html><head><title>Attention Required! | Cloudflare</title></head>
	<body><h1>Sorry, you have been blocked</h1><div id="cf-error-details">...</div></body></html>`
	if botBlockReason(cf) == "" {
		t.Error("halaman blokir Cloudflare harus dikenali")
	}
	if botBlockReason(`<html><head><title>Just a moment...</title></head><body>…</body></html>`) == "" {
		t.Error("tantangan Cloudflare harus dikenali")
	}
	// Situs asli (besar) yang kebetulan memuat kata-kata itu di artikel tidak boleh ditolak
	big := "<html><body>" + strings.Repeat("<p>slot gacor hari ini</p>", 3000) + "Sorry, you have been blocked</body></html>"
	if botBlockReason(big) != "" {
		t.Error("halaman besar tidak boleh dianggap halaman blokir")
	}
	if botBlockReason(`<html><body><h1>Daftar Slot Gacor</h1></body></html>`) != "" {
		t.Error("halaman biasa tidak boleh dianggap blokir")
	}
}
