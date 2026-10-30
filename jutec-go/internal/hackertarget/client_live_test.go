package hackertarget

import (
	"os"
	"testing"
)

// Uji ke API sungguhan — hanya jalan bila LIVE=1 (memakai kuota harian).
func TestLive(t *testing.T) {
	if os.Getenv("LIVE") != "1" {
		t.Skip("set LIVE=1")
	}
	c := NewClient("")
	d, err := c.ReverseAnalytics("G-ABCDEFG123")
	t.Log(d, err)
	if err != nil || len(d) == 0 {
		t.Fatal("harus ada hasil")
	}
	d, err = c.ReverseAnalytics("UA-2238316")
	t.Log(d, err)
	if err != nil || len(d) != 0 {
		t.Fatal("harus kosong tanpa galat")
	}
}
