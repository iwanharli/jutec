package crawler

import "testing"

func TestDetectAppPromo(t *testing.T) {
	cases := []struct {
		name string
		html string
		want bool
	}{
		{
			name: "template white-label dengan tombol unduh tanpa link",
			html: `<div>Unduh <strong>INDO303 App</strong></div>
				<div class="download-apk-container"><picture><img alt="Download Android APK" src="//cdn/x/android-logo.webp"></picture>
				<a href="#">Unduh</a></div>
				<a class="download-apk-guide" href="#" data-target="#apk_install_guide_modal">klik disini!</a>`,
			want: true,
		},
		{
			name: "situs biasa yang hanya menyebut app di script",
			html: `<html><script src="/static/app.js"></script><p>Our web app is fast.</p></html>`,
			want: false,
		},
		{
			name: "satu penanda saja tidak cukup",
			html: `<img alt="Android" src="robot.png"><p>Tentang kami</p>`,
			want: false,
		},
	}
	for _, c := range cases {
		got := DetectAppPromo(c.html)
		if got.Detected != c.want {
			t.Errorf("%s: Detected=%v, want %v (evidence %v)", c.name, got.Detected, c.want, got.Evidence)
		}
	}
}

func TestContactRegexFormats(t *testing.T) {
	arts := ExtractArtifacts(`<a href="https://wa.me/+6281200000003">WA</a> <a href="https://t.me/@Indo_303">TG</a>`)
	want := map[string]bool{"6281200000003": false, "Indo_303": false}
	for _, a := range arts {
		if _, ok := want[a.Value]; ok {
			want[a.Value] = true
		}
	}
	for v, found := range want {
		if !found {
			t.Errorf("kontak %q tidak terdeteksi (artefak: %+v)", v, arts)
		}
	}
}
