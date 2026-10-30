package crawler

import (
	"crypto/sha256"
	"math/big"
	"regexp"
	"strings"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// base58CheckValid memverifikasi checksum Base58Check (alamat BTC legacy/P2SH & TRON).
// Menolak string yang hanya "mirip" alamat, mis. hash heksadesimal MD5 yang diawali 1/3.
func base58CheckValid(s string, wantLen int) bool {
	n := new(big.Int)
	for _, ch := range s {
		i := strings.IndexRune(base58Alphabet, ch)
		if i < 0 {
			return false
		}
		n.Mul(n, big.NewInt(58))
		n.Add(n, big.NewInt(int64(i)))
	}
	b := n.Bytes()
	for _, ch := range s { // nol di depan = karakter '1'
		if ch != '1' {
			break
		}
		b = append([]byte{0}, b...)
	}
	if len(b) != wantLen {
		return false
	}
	payload, sum := b[:len(b)-4], b[len(b)-4:]
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	return string(h2[:4]) == string(sum)
}

func validBTC(addr string) bool  { return base58CheckValid(addr, 25) }
func validTRON(addr string) bool { return base58CheckValid(addr, 25) }

// Username Telegram milik sistem (t.me/share/url, t.me/joinchat/…) — bukan akun operator
var tgReserved = map[string]bool{
	"share": true, "joinchat": true, "addstickers": true, "addemoji": true, "proxy": true, "socks": true,
	"setlanguage": true, "addtheme": true, "login": true, "iv": true, "boost": true, "invoice": true,
	"telegram": true, "durov": true, "contact": true, "confirmphone": true,
}

func validTelegramUser(u string) bool { return !tgReserved[strings.ToLower(u)] }

var trackingFormat = map[string]*regexp.Regexp{
	"GoogleTag":        regexp.MustCompile(`^G-[A-Z0-9]{6,12}$`),
	"GoogleTagManager": regexp.MustCompile(`^GTM-[A-Z0-9]{4,10}$`),
	"GoogleAnalytics":  regexp.MustCompile(`^UA-\d{4,12}-\d{1,4}$`),
	"FacebookPixel":    regexp.MustCompile(`^\d{10,20}$`),
}

// validTrackingID menolak tangkapan palsu seperti "g-recaptcha" / "gtm-script" (dulu lolos karena (?i)).
// ID GA4/GTM asli selalu huruf besar dan memuat angka.
func validTrackingID(platform, value string) bool {
	re, ok := trackingFormat[platform]
	if !ok {
		return true
	}
	if !re.MatchString(value) {
		return false
	}
	if platform == "GoogleTag" || platform == "GoogleTagManager" {
		return strings.ContainsAny(value, "0123456789")
	}
	return true
}

// IsValidArtifact dipakai juga untuk membersihkan data lama yang tersimpan sebelum validasi ada.
func IsValidArtifact(typ, name, value string) bool {
	switch {
	case typ == "Bank" || typ == "VirtualAccount" || typ == "Phone":
		return !placeholderNumber(value)
	case typ == "Crypto" && name == "BTC":
		return validBTC(value)
	case typ == "Crypto" && name == "USDT-TRC20":
		return validTRON(value)
	case typ == "Contact" && name == "Telegram":
		return validTelegramUser(value)
	}
	return true
}

// IsValidTrackingID versi ekspor dari validTrackingID.
func IsValidTrackingID(platform, value string) bool { return validTrackingID(platform, value) }

var rePlaceholder = regexp.MustCompile(`^(\d)0+$|^0+\d?$`)

// placeholderNumber: nomor contoh/placeholder (1000000000, 1111111111, 1234567890) bukan rekening asli.
func placeholderNumber(v string) bool {
	if rePlaceholder.MatchString(v) || strings.Contains("12345678901234567890", v) || strings.Contains("09876543210987654321", v) {
		return true
	}
	return strings.Count(v, v[:1]) == len(v) // satu digit diulang
}
