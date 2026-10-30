package tron

import (
	"os"
	"testing"
)

// Uji ke TronGrid sungguhan — hanya jalan bila LIVE=1.
func TestLive(t *testing.T) {
	if os.Getenv("LIVE") != "1" {
		t.Skip("set LIVE=1")
	}
	p, err := NewClient("").USDTProfile("TNXoiAJ3dct8Fjg4M9fkLFh9S2v9TXc32G", 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("transfers=%d in=%.2f out=%.2f complete=%v first=%d last=%d top_recv=%+v", p.Transfers, p.TotalIn, p.TotalOut, p.Complete, p.FirstAt, p.LastAt, p.TopReceivers[:2])
	if p.Transfers == 0 || p.TotalOut == 0 {
		t.Fatal("profil kosong")
	}
}
