package service

import (
	"testing"

	"jutec-go/internal/telegram"
)

const vflSample = `📳KomInfo Indonesia

Since 2017, Indonesia has required all SIM card owners to register ...

📞Telephone:  6281200000001
📆The date of registration:  2020-01-16
📖Passport number:  3201010101900002
🛰️ Provider:  TELKOMSEL

📞Telephone:  6281200000002
📆The date of registration:  2018-07-19
📖Passport number:  3201010101900001
🛰️ Provider:  TELKOMSEL`

func TestParseVflReplyExactOnly(t *testing.T) {
	msgs := []telegram.BotMessage{{Text: "🔎Request: x\n📁Number of results: 2"}, {Text: vflSample}}
	// kueri awalan (12 digit) tidak boleh mengambil nomor 13 digit orang lain
	if recs, found := ParseVflReply(msgs, "628120000000"); found || len(recs) != 0 {
		t.Fatalf("awalan seharusnya dibuang, dapat %v", recs)
	}
	recs, found := ParseVflReply(msgs, "6281200000002")
	if !found || len(recs) != 1 || recs[0].NIK != "3201010101900001" || recs[0].Provider != "TELKOMSEL" || recs[0].RegDate != "2018-07-19" {
		t.Fatalf("hasil salah: %+v", recs)
	}
	recs, _ = ParseVflReply(msgs, "3201010101900002")
	if len(recs) != 1 || recs[0].Phone != "6281200000001" {
		t.Fatalf("kueri NIK salah: %+v", recs)
	}
	if _, found := ParseVflReply([]telegram.BotMessage{{Text: "🤷‍No results found"}}, "628"); found {
		t.Fatal("no results")
	}
}
