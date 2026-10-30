package crawler

import (
	"strings"
	"testing"
)

// Kredensial PALSU untuk fixture — disusun dari potongan agar pemindai rahasia (GitHub secret scanning)
// tidak menganggapnya kunci sungguhan. Polanya tetap utuh saat tes berjalan.
var (
	fakeGoogleKey = "AIza" + "SyA1234567890abcdefghijklmnopqrstuv"
	fakeBotToken  = "7123456789:" + "AAFabcdefghijklmnopqrstuvwxyz0123456"
)

func TestExtractOwnerIDs(t *testing.T) {
	html := `
	<div class="g-recaptcha" data-sitekey="6LcAbCdEfGhIjKlMnOpQrStUvWxYz012345678_-"></div>
	<script src="https://www.google.com/recaptcha/api.js?render=6LeIxAcTAAAAAJcZVRqyHh4X1y_-Uq-3n_Nhnwm7"></script>
	<script>const firebaseConfig = { apiKey: "{{GOOGLE_KEY}}", authDomain: "judolku-app.firebaseapp.com", projectId: "judolku-app" };</script>
	<ins class="adsbygoogle" data-ad-client="ca-pub-1234567890123456"></ins>
	<script>fetch("https://api.telegram.org/bot{{BOT_TOKEN}}/sendMessage", {body: JSON.stringify({chat_id: "-1001987654321", text: "deposit"})})</script>
	<script defer src="https://static.cloudflareinsights.com/beacon.min.js" data-cf-beacon='{"token": "0123456789abcdef0123456789abcdef"}'></script>
	<script>(function(c,l,a,r,i,t,y){})(window,document,"clarity","script","abcd123xyz");</script><script src="https://www.clarity.ms/tag/abcd123xyz"></script>
	`
	html = strings.NewReplacer("{{GOOGLE_KEY}}", fakeGoogleKey, "{{BOT_TOKEN}}", fakeBotToken).Replace(html)
	got := map[string]string{}
	for _, x := range ExtractOwnerIDs(html) {
		got[x.Platform+"="+x.Value] = x.Platform
	}
	want := []string{
		"ReCaptcha=6LcAbCdEfGhIjKlMnOpQrStUvWxYz012345678_-",
		"GoogleAPIKey=" + fakeGoogleKey,
		"FirebaseProject=judolku-app",
		"AdSense=pub-1234567890123456",
		"TelegramBot=7123456789",
		"TelegramChat=-1001987654321",
		"CloudflareAnalytics=0123456789abcdef0123456789abcdef",
		"Clarity=abcd123xyz",
	}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Errorf("tidak tertangkap: %s", w)
		}
	}
	// Kunci uji resmi reCAPTCHA & token bot utuh tidak boleh tersimpan
	for k := range got {
		if k == "ReCaptcha=6LeIxAcTAAAAAJcZVRqyHh4X1y_-Uq-3n_Nhnwm7" {
			t.Error("kunci uji reCAPTCHA ikut tersimpan")
		}
		if len(k) > 40 && k[:12] == "TelegramBot=" {
			t.Errorf("token bot tersimpan utuh: %s", k)
		}
	}
	// Tanpa konteks Telegram, chat_id biasa tidak ditangkap
	if ids := ExtractOwnerIDs(`{"chat_id": 123456789}`); len(ids) != 0 {
		t.Errorf("salah tangkap chat_id tanpa konteks: %v", ids)
	}
}

func TestRegistrable(t *testing.T) {
	for in, want := range map[string]string{"cdn.situs.com": "situs.com", "a.b.situs.co.id": "situs.co.id", "situs.com": "situs.com"} {
		if got := registrable(in); got != want {
			t.Errorf("registrable(%q)=%q mau %q", in, got, want)
		}
	}
}
