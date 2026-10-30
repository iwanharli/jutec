// rescore menghitung ulang skor situs yang ditandai judol dari scan terakhirnya memakai scorer terbaru,
// tanpa crawl ulang. Default dry-run (hanya menampilkan); -apply menyimpan perubahan.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	"jutec-go/internal/config"
	"jutec-go/internal/crawler"
	"jutec-go/internal/intelligence"
	"jutec-go/internal/store"

	"github.com/joho/godotenv"
)

func main() {
	apply := flag.Bool("apply", false, "simpan perubahan ke tabel sites")
	idList := flag.String("ids", "", "batasi ke id situs tertentu (pisah koma)")
	flag.Parse()
	only := map[int]bool{}
	for _, x := range strings.Split(*idList, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(x)); err == nil {
			only[n] = true
		}
	}
	godotenv.Load()
	cfg := config.LoadConfig()
	pg, err := store.NewPostgresConnection(cfg.PostgresURL)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	rows, err := pg.Pool.Query(ctx, `
		SELECT DISTINCT ON (s.id) s.id, s.url, COALESCE(s.confidence, 0), l.result->'result', l.result->'score'->'breakdown'
		FROM sites s JOIN scan_logs l ON l.site_id = s.id
		WHERE s.is_gambling AND s.verified_status IS NULL AND l.result ? 'result' AND l.result ? 'score'
		ORDER BY s.id, l.created_at DESC`)
	if err != nil {
		log.Fatal(err)
	}
	type change struct {
		id         int
		url        string
		old, new   float64
		prio       string
		gambling   bool
		providers  int
		artifacts  int
	}
	var changes []change
	total := 0
	for rows.Next() {
		var id int
		var u string
		var old float64
		var rawRes, rawBd []byte
		if rows.Scan(&id, &u, &old, &rawRes, &rawBd) != nil || len(only) > 0 && !only[id] {
			continue
		}
		var a crawler.AnalysisResult
		var bd intelligence.Breakdown
		if json.Unmarshal(rawRes, &a) != nil || json.Unmarshal(rawBd, &bd) != nil {
			continue
		}
		total++
		// Buang artefak yang kini ditolak validator (mis. hash MD5 yang dulu terbaca sebagai BTC)
		valid := a.Artifacts[:0]
		for _, x := range a.Artifacts {
			if crawler.IsValidArtifact(x.Type, x.Name, x.Value) {
				valid = append(valid, x)
			}
		}
		a.Artifacts = valid
		sc := intelligence.Compute(&a, bd.DomainScore, u, bd.SimHashBoost, bd.EnrichmentBoost)
		if !sc.IsGambling || sc.Priority != "High" && old >= 0.75 {
			np := 0
			if a.NetworkSignals != nil {
				np = len(a.NetworkSignals.GameProviders)
			}
			changes = append(changes, change{id, u, old, sc.Final, sc.Priority, sc.IsGambling, np, len(valid)})
		}
	}
	rows.Close()

	flips := 0
	for _, c := range changes {
		mark := "turun"
		if !c.gambling {
			mark = "BUKAN JUDOL"
			flips++
		}
		fmt.Printf("%5d  %-45s %.2f → %.2f  %-6s %-11s provider=%d artefak=%d\n", c.id, c.url, c.old, c.new, c.prio, mark, c.providers, c.artifacts)
	}
	fmt.Printf("\n%d situs judol diperiksa · %d berubah · %d tidak lagi judol\n", total, len(changes), flips)

	if !*apply {
		fmt.Println("(dry-run — jalankan dengan -apply untuk menyimpan)")
		return
	}
	for _, c := range changes {
		if _, err := pg.Pool.Exec(ctx, `UPDATE sites SET is_gambling = $2, confidence = $3, priority = $4 WHERE id = $1`,
			c.id, c.gambling, c.new, c.prio); err != nil {
			log.Printf("update %d: %v", c.id, err)
		}
	}
	fmt.Printf("%d situs diperbarui\n", len(changes))
}
