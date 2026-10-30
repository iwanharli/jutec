package crawler

import (
	"regexp"
	"strings"
)

var reHTMLTag = regexp.MustCompile(`<[^>]+>`)
var reHTMLEntity = regexp.MustCompile(`&[a-zA-Z0-9#]+;`)
var reMultiSpace = regexp.MustCompile(`[ \t]{2,}`)

// stripHTML menghilangkan tag HTML dan entity, mengganti dengan spasi agar
// teks seperti "<td>BCA</td><td>1234567890</td>" tetap bisa diproses regex.
func stripHTML(html string) string {
	s := reHTMLTag.ReplaceAllString(html, " ")
	s = reHTMLEntity.ReplaceAllString(s, " ")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

type TransactionArtifact struct {
	Type  string `json:"type"`  // Bank, E-Wallet, VirtualAccount, Crypto, QRIS, Contact, Phone
	Value string `json:"value"` // Nomor rekening, address, username, dll
	Name  string `json:"name"`  // Nama bank/platform/pemilik
}

// bankList mencakup semua bank umum Indonesia yang sering dipakai di situs judol.
var bankList = []string{
	"BCA", "MANDIRI", "BNI", "BRI", "CIMB",
	"DANAMON", "BSI", "BTN", "PERMATA", "MEGA",
	"MAYBANK", "BUKOPIN", "SINARMAS", "OCBC", "PANIN",
	"NEO", "JAGO",
}

// walletList mencakup e-wallet Indonesia yang umum dipakai untuk deposit judol.
var walletList = []string{
	"DANA", "OVO", "GOPAY", "LINKAJA",
	"SHOPEEPAY", "JENIUS", "DOKU", "ISAKU", "ASTRAPAY",
}

var (
	// Virtual Account — konteks "virtual account" / "VA" diikuti 13-17 digit
	reVAContext = regexp.MustCompile(`(?i)(?:virtual\s*account|no\.?\s*va|nomor\s*va|kode\s*va)\s*[:\-]?\s*(\d{13,17})`)
	// VA dengan nama bank: "VA BCA : 1234567890123"
	reVABank = regexp.MustCompile(`(?i)(?:va|virtual\s*account)\s+(?:BCA|BNI|MANDIRI|BRI|CIMB|PERMATA|DANAMON|BSI|BTN|MEGA|MAYBANK)\s*[:\-]?\s*(\d{10,17})`)

	// Crypto
	reETH      = regexp.MustCompile(`\b(0x[a-fA-F0-9]{40})\b`)
	reBTC      = regexp.MustCompile(`\b([13][a-km-zA-HJ-NP-Z1-9]{25,34})\b`)
	reBTCBech  = regexp.MustCompile(`\b(bc1[a-z0-9]{39,59})\b`)
	reTRON     = regexp.MustCompile(`\b(T[A-HJ-NP-Za-km-z1-9]{33})\b`) // USDT TRC20

	// QRIS
	reQRIS = regexp.MustCompile(`\bID[0-9]{13,15}\b`)

	// Contact — WhatsApp
	reWA = regexp.MustCompile(`(?:wa\.me|api\.whatsapp\.com/send[^"']*phone=)\s*[/=]?\s*(?:\+|%2B)?(\d{10,15})`) // wa.me/+62… juga umum
	// Telegram
	reTG = regexp.MustCompile(`t\.me/(?:@|%40)?([a-zA-Z0-9_]{5,32})`) // t.me/@user juga umum
	// Line
	reLine = regexp.MustCompile(`(?i)line\.me/ti/p/~([a-zA-Z0-9_.]{3,30})`)
	// Discord
	reDiscord = regexp.MustCompile(`(?i)discord(?:\.gg|\.com/invite)/([a-zA-Z0-9]{5,20})`)

	// Nomor HP Indonesia standalone — hanya jika ada konteks pembayaran/kontak di sekitarnya
	rePhoneContext = regexp.MustCompile(`(?i)(?:nomor|no\.?|hp|telp(?:on)?|hubungi|transfer\s+ke|kirim\s+ke|konfirmasi)\s*[:\-]?\s*(08\d{8,11}|\+628\d{8,11})`)
)

func ExtractArtifacts(html string) []TransactionArtifact {
	var artifacts []TransactionArtifact

	// Plain text: tag HTML dihapus agar nomor rekening yang terpisah tag tetap terdeteksi.
	// Contoh: "<td>BCA</td><td>1234567890</td>" → "BCA 1234567890"
	text := stripHTML(html)

	// 1. Bank Indonesia — nama bank diikuti nomor rekening 10-16 digit
	// Pattern diperluas: izinkan sampai 40 karakter non-digit antara nama bank & nomor
	for _, bank := range bankList {
		re := regexp.MustCompile(`(?i)` + bank + `[^0-9]{0,40}?(\d{10,16})`)
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			artifacts = append(artifacts, TransactionArtifact{Type: "Bank", Value: m[1], Name: bank})
		}
	}

	// 2. E-Wallet — nama wallet diikuti nomor HP
	for _, w := range walletList {
		re := regexp.MustCompile(`(?i)` + w + `[^0-9]{0,30}?(08\d{8,11}|\+628\d{8,11})`)
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			artifacts = append(artifacts, TransactionArtifact{Type: "E-Wallet", Value: m[1], Name: w})
		}
	}

	// 3. Virtual Account (pada text stripped)
	for _, m := range reVAContext.FindAllStringSubmatch(text, -1) {
		artifacts = append(artifacts, TransactionArtifact{Type: "VirtualAccount", Value: m[1], Name: "VA"})
	}
	for _, m := range reVABank.FindAllStringSubmatch(text, -1) {
		artifacts = append(artifacts, TransactionArtifact{Type: "VirtualAccount", Value: m[1], Name: "VA"})
	}

	// 4. Crypto — dijalankan pada HTML ASLI (bukan lowercase) karena address case-sensitive
	for _, m := range reETH.FindAllStringSubmatch(html, -1) {
		artifacts = append(artifacts, TransactionArtifact{Type: "Crypto", Value: m[1], Name: "ETH/ERC20"})
	}
	for _, m := range reBTC.FindAllStringSubmatch(html, -1) {
		if validBTC(m[1]) { // checksum: tolak hash hex yang kebetulan berawalan 1/3
			artifacts = append(artifacts, TransactionArtifact{Type: "Crypto", Value: m[1], Name: "BTC"})
		}
	}
	for _, m := range reBTCBech.FindAllStringSubmatch(html, -1) {
		artifacts = append(artifacts, TransactionArtifact{Type: "Crypto", Value: m[1], Name: "BTC-SegWit"})
	}
	for _, m := range reTRON.FindAllStringSubmatch(html, -1) {
		if validTRON(m[1]) {
			artifacts = append(artifacts, TransactionArtifact{Type: "Crypto", Value: m[1], Name: "USDT-TRC20"})
		}
	}

	// 5. QRIS Merchant ID
	for _, val := range reQRIS.FindAllString(text, -1) {
		artifacts = append(artifacts, TransactionArtifact{Type: "QRIS", Value: val, Name: "QRIS_ID"})
	}

	// 6. Contact
	for _, m := range reWA.FindAllStringSubmatch(html, -1) {
		artifacts = append(artifacts, TransactionArtifact{Type: "Contact", Value: m[1], Name: "WhatsApp"})
	}
	for _, m := range reTG.FindAllStringSubmatch(html, -1) {
		if validTelegramUser(m[1]) {
			artifacts = append(artifacts, TransactionArtifact{Type: "Contact", Value: m[1], Name: "Telegram"})
		}
	}
	for _, m := range reLine.FindAllStringSubmatch(html, -1) {
		artifacts = append(artifacts, TransactionArtifact{Type: "Contact", Value: m[1], Name: "Line"})
	}
	for _, m := range reDiscord.FindAllStringSubmatch(html, -1) {
		artifacts = append(artifacts, TransactionArtifact{Type: "Contact", Value: m[1], Name: "Discord"})
	}

	// 7. Nomor HP standalone dalam konteks pembayaran/kontak
	for _, m := range rePhoneContext.FindAllStringSubmatch(text, -1) {
		artifacts = append(artifacts, TransactionArtifact{Type: "Phone", Value: m[1], Name: "Phone"})
	}

	return uniqueArtifacts(artifacts)
}

func uniqueArtifacts(input []TransactionArtifact) []TransactionArtifact {
	// Buang tangkapan palsu (placeholder rekening, hash yang mirip alamat crypto, dsb.)
	valid := input[:0:0]
	for _, a := range input {
		if IsValidArtifact(a.Type, a.Name, a.Value) {
			valid = append(valid, a)
		}
	}
	input = valid
	keys := make(map[string]bool)
	var list []TransactionArtifact
	for _, entry := range input {
		key := entry.Type + entry.Value
		if !keys[key] {
			keys[key] = true
			list = append(list, entry)
		}
	}
	return list
}
