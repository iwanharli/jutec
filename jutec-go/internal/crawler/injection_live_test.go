package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestInjectionLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	kws := []WeightedKeyword{{"slot gacor", 0.6}, {"togel", 0.5}, {"maxwin", 0.6}, {"situs toto", 0.6}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/cloak" {
			if strings.Contains(r.UserAgent(), "Googlebot") {
				fmt.Fprint(w, `<html><body><h1>Slot gacor maxwin</h1><p>situs toto togel resmi</p></body></html>`)
				return
			}
			fmt.Fprint(w, `<html><body><h1>Dinas Kesehatan</h1><p>Layanan publik</p></body></html>`)
			return
		}
		links := ""
		for i := 0; i < 6; i++ {
			links += fmt.Sprintf(`<a href="https://slotgacor%d.example/">slot gacor %d</a> `, i, i)
		}
		fmt.Fprintf(w, `<html><body><h1>Universitas Contoh</h1><p>Berita kampus</p><div style="position:absolute;left:-9999px">%s</div></body></html>`, links)
	}))
	defer srv.Close()
	c := NewCrawler()
	for _, p := range []string{"/spam", "/cloak"} {
		r, err := c.ProcessURL(srv.URL+p, kws, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s → injection=%+v conf=%.2f kw=%v", p, r.Injection, r.Confidence, r.FoundKeywords)
	}
}
