// Package qris membaca gambar QR dan mengurai payload QRIS (standar EMVCo MPM) untuk
// mendapatkan merchant penampung dana: nama, kota, NMID, dan PJSP/bank penerbit.
package qris

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	_ "golang.org/x/image/webp"
)

type Merchant struct {
	Raw          string `json:"raw"`
	MerchantName string `json:"merchant_name"`
	City         string `json:"city"`
	NMID         string `json:"nmid"`     // National Merchant ID (ID…)
	Acquirer     string `json:"acquirer"` // GUID PJSP, mis. ID.CO.BRI.WWW / COM.GO-JEK.WWW
	MerchantPAN  string `json:"merchant_pan"`
	MerchantID   string `json:"merchant_id"`
	Postal       string `json:"postal"`
}

// Key: nilai paling unik untuk menghubungkan situs (NMID > PAN > merchant ID > nama).
func (m *Merchant) Key() string {
	for _, v := range []string{m.NMID, m.MerchantPAN, m.MerchantID, m.MerchantName} {
		if v != "" {
			return v
		}
	}
	return ""
}

const maxImageBytes = 4 << 20

// DecodeURL mengunduh gambar lalu mencoba membaca QR di dalamnya. nil bila bukan QRIS.
func DecodeURL(client *http.Client, imageURL string) (*Merchant, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Get(imageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil || len(data) > maxImageBytes {
		return nil, fmt.Errorf("gambar terlalu besar atau gagal dibaca")
	}
	return DecodeImage(data)
}

func DecodeImage(data []byte) (*Merchant, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, err
	}
	hints := map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, hints)
	if err != nil {
		return nil, nil // tidak ada QR di gambar ini
	}
	return Parse(res.GetText()), nil
}

// Parse mengurai payload QRIS. Mengembalikan nil bila teks bukan payload EMVCo.
func Parse(payload string) *Merchant {
	payload = strings.TrimSpace(payload)
	if !strings.HasPrefix(payload, "000201") {
		return nil
	}
	m := &Merchant{Raw: payload}
	top := tlv(payload)
	m.MerchantName = strings.TrimSpace(top["59"])
	m.City = strings.TrimSpace(top["60"])
	m.Postal = top["61"]
	// Template informasi akun merchant: tag 26–51 (51 = template QRIS nasional berisi NMID)
	for tag := 26; tag <= 51; tag++ {
		v, ok := top[fmt.Sprintf("%02d", tag)]
		if !ok {
			continue
		}
		sub := tlv(v)
		guid := sub["00"]
		if tag == 51 || strings.Contains(strings.ToUpper(guid), "QRIS") {
			if id := sub["02"]; strings.HasPrefix(id, "ID") {
				m.NMID = id
			}
			continue
		}
		if m.Acquirer == "" {
			m.Acquirer = guid
			m.MerchantPAN = sub["01"]
			m.MerchantID = sub["02"]
		}
	}
	return m
}

// tlv mengurai format Tag(2)-Length(2)-Value EMVCo.
func tlv(s string) map[string]string {
	out := map[string]string{}
	for i := 0; i+4 <= len(s); {
		tag := s[i : i+2]
		var n int
		if _, err := fmt.Sscanf(s[i+2:i+4], "%d", &n); err != nil || i+4+n > len(s) {
			break
		}
		out[tag] = s[i+4 : i+4+n]
		i += 4 + n
	}
	return out
}
