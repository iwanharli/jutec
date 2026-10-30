package qris

import "testing"

// Payload QRIS contoh (data fiktif, struktur sesuai standar EMVCo MPM)
const sample = "00020101021126570011ID.DANA.WWW011893600915302259148102090225914810303UMI51440014ID.CO.QRIS.WWW0215ID10200211817450303UMI5204729853033605802ID5914TOKO CONTOH JD6015JAKARTA SELATAN61051231063049C2B"

func TestParseQRIS(t *testing.T) {
	m := Parse(sample)
	if m == nil {
		t.Fatal("payload QRIS harus terbaca")
	}
	if m.MerchantName != "TOKO CONTOH JD" || m.City != "JAKARTA SELATAN" {
		t.Errorf("nama/kota salah: %+v", m)
	}
	if m.NMID != "ID1020021181745" || m.Acquirer != "ID.DANA.WWW" {
		t.Errorf("NMID/acquirer salah: nmid=%q acquirer=%q", m.NMID, m.Acquirer)
	}
	if Parse("https://bukan-qris.com") != nil {
		t.Error("teks bukan QRIS harus nil")
	}
}
