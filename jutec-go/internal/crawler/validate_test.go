package crawler

import "testing"

func TestValidators(t *testing.T) {
	if !validBTC("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa") { // alamat genesis Bitcoin (valid)
		t.Error("alamat BTC valid ditolak")
	}
	if validBTC("1a4636e98aef84ef35d9bdf38dab9ca3") { // hash MD5 (sampah dari extractor lama)
		t.Error("hash hex harus ditolak sebagai alamat BTC")
	}
	if !validTRON("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t") { // kontrak USDT-TRC20 (valid)
		t.Error("alamat TRON valid ditolak")
	}
	for _, bad := range [][2]string{{"GoogleTag", "g-recaptcha"}, {"GoogleTagManager", "gtm-script"}, {"GoogleTag", "G-ABCDEFG"}} {
		if validTrackingID(bad[0], bad[1]) {
			t.Errorf("%s %q harus ditolak", bad[0], bad[1])
		}
	}
	if !validTrackingID("GoogleTag", "G-DW5XRK7GYT") || !validTrackingID("GoogleTagManager", "GTM-P24PF4B") {
		t.Error("ID GA4/GTM valid ditolak")
	}
	if validTelegramUser("share") || !validTelegramUser("lazadatoto") {
		t.Error("validasi username Telegram salah")
	}
}
