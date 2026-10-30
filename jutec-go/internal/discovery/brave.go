package discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Brave Search: indeks web independen dari Google — sebagian situs judol yang tidak muncul di
// Google tampil di sini. Aktif bila BRAVE_API_KEY diisi (kuota gratis ±2.000 kueri/bulan, 1 kueri/detik).
func braveSearch(keyword string, pages int) []string {
	key := os.Getenv("BRAVE_API_KEY")
	if key == "" {
		return nil
	}
	if pages > 2 {
		pages = 2 // hemat kuota: 2 × 20 hasil
	}
	var out []string
	for p := 0; p < pages; p++ {
		q := url.Values{"q": {keyword}, "count": {"20"}, "offset": {fmt.Sprint(p)}, "country": {"ID"}, "search_lang": {"id"}}
		req, _ := http.NewRequest("GET", "https://api.search.brave.com/res/v1/web/search?"+q.Encode(), nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Subscription-Token", key)
		res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
		if err != nil {
			log.Printf("❌ [Brave] '%s': %v", keyword, err)
			break
		}
		var body struct {
			Web struct {
				Results []struct {
					URL string `json:"url"`
				} `json:"results"`
			} `json:"web"`
		}
		err = json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		if res.StatusCode != 200 || err != nil {
			log.Printf("❌ [Brave] '%s': HTTP %d", keyword, res.StatusCode)
			break
		}
		for _, r := range body.Web.Results {
			out = append(out, r.URL)
		}
		if len(body.Web.Results) < 20 {
			break
		}
		time.Sleep(1100 * time.Millisecond) // batas 1 kueri/detik
	}
	return out
}
