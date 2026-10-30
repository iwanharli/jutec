// cleanup menghapus artefak & tracking ID yang tidak valid (tersimpan sebelum validasi checksum/format
// ditambahkan), lalu membangun ulang klaster operator. Aman dijalankan berulang.
//
//	go run cmd/cleanup/main.go          # hapus & bangun ulang klaster
//	go run cmd/cleanup/main.go -dry-run # hanya hitung
package main

import (
	"context"
	"flag"
	"log"

	"jutec-go/internal/config"
	"jutec-go/internal/crawler"
	"jutec-go/internal/store"

	"github.com/joho/godotenv"
)

func main() {
	dry := flag.Bool("dry-run", false, "hanya hitung, jangan hapus")
	flag.Parse()
	godotenv.Load()
	pg, err := store.NewPostgresConnection(config.LoadConfig().PostgresURL)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	var badArt []int
	rows, _ := pg.Pool.Query(ctx, `SELECT id, type, COALESCE(name,''), value FROM artifacts WHERE type IN ('Crypto','Contact')`)
	for rows.Next() {
		var id int
		var t, n, v string
		if rows.Scan(&id, &t, &n, &v) == nil && !crawler.IsValidArtifact(t, n, v) {
			badArt = append(badArt, id)
		}
	}
	rows.Close()

	var badTrk []int
	rows, _ = pg.Pool.Query(ctx, `SELECT id, platform, value FROM tracking_ids`)
	for rows.Next() {
		var id int
		var p, v string
		if rows.Scan(&id, &p, &v) == nil && !crawler.IsValidTrackingID(p, v) {
			badTrk = append(badTrk, id)
		}
	}
	rows.Close()
	log.Printf("Artefak tidak valid: %d · Tracking ID tidak valid: %d", len(badArt), len(badTrk))
	if *dry {
		return
	}

	if _, err := pg.Pool.Exec(ctx, `DELETE FROM site_artifacts WHERE artifact_id = ANY($1)`, badArt); err != nil {
		log.Fatal(err)
	}
	pg.Pool.Exec(ctx, `DELETE FROM artifacts WHERE id = ANY($1)`, badArt)
	if _, err := pg.Pool.Exec(ctx, `DELETE FROM site_tracking WHERE tracking_id = ANY($1)`, badTrk); err != nil {
		log.Fatal(err)
	}
	pg.Pool.Exec(ctx, `DELETE FROM tracking_ids WHERE id = ANY($1)`, badTrk)
	log.Printf("Dihapus. Membangun ulang klaster operator...")

	n, err := pg.ClusterOperators(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Selesai: %d klaster operator", n)
}
