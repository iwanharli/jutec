// mergesites menggabungkan baris situs duplikat (mis. URL yang hanya beda "/" di akhir) ke satu situs.
// Semua tabel yang mereferensikan sites(id) dibaca dari katalog database, baris yang kembar dibuang,
// sisanya dipindah. Default dry-run; -apply menjalankan dalam satu transaksi setelah membuat backup CSV.
//
//	go run ./cmd/mergesites -keep 5714 -dup 5729            # rencana saja
//	go run ./cmd/mergesites -keep 5714 -dup 5729 -apply     # jalankan
//	go run ./cmd/mergesites -list                           # daftar pasangan duplikat (beda "/" akhir)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jutec-go/internal/config"
	"jutec-go/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type refTable struct {
	name    string
	uniques [][]string // kolom lain (selain site_id) pada setiap constraint unik/PK yang memuat site_id
}

func main() {
	keep := flag.Int("keep", 0, "id situs yang dipertahankan")
	dup := flag.Int("dup", 0, "id situs duplikat yang digabung lalu dihapus")
	apply := flag.Bool("apply", false, "jalankan (tanpa ini hanya rencana)")
	list := flag.Bool("list", false, "tampilkan pasangan duplikat URL")
	flag.Parse()
	godotenv.Load()
	pg, err := store.NewPostgresConnection(config.LoadConfig().PostgresURL)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	if *list {
		rows, _ := pg.Pool.Query(ctx, `
			SELECT a.id, a.url, b.id, b.url FROM sites a JOIN sites b
			  ON lower(rtrim(a.url, '/')) = lower(rtrim(b.url, '/')) AND a.id < b.id ORDER BY a.id`)
		n := 0
		for rows.Next() {
			var ai, bi int
			var au, bu string
			rows.Scan(&ai, &au, &bi, &bu)
			fmt.Printf("%6d %-50s ⇄ %6d %s\n", ai, au, bi, bu)
			n++
		}
		fmt.Printf("%d pasangan duplikat\n", n)
		return
	}
	if *keep == 0 || *dup == 0 || *keep == *dup {
		log.Fatal("isi -keep dan -dup (berbeda)")
	}

	tables, err := referencingTables(ctx, pg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Gabung situs #%d → #%d (%d tabel mereferensikan sites)\n\n", *dup, *keep, len(tables))

	var backupDir string
	if *apply {
		backupDir = filepath.Join("..", fmt.Sprintf("backup-merge-%d-into-%d-%s", *dup, *keep, time.Now().Format("20060102-1504")))
		os.MkdirAll(backupDir, 0o750)
	}

	tx, err := pg.Pool.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// Riwayat verifikasi bersifat tambah-saja; pemindahan baris saat penggabungan dibuka hanya di transaksi ini
	if _, err := tx.Exec(ctx, `SET LOCAL jutec.allow_cascade = 'on'`); err != nil {
		log.Fatal(err)
	}

	for _, t := range tables {
		var total int
		tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE site_id = $1`, t.name), *dup).Scan(&total)
		if total == 0 {
			continue
		}
		// Baris kembar: sudah ada di situs yang dipertahankan untuk salah satu constraint unik
		var conflict string
		for _, cols := range t.uniques {
			if len(cols) == 0 { // constraint unik hanya site_id (mis. site_origin)
				conflict = joinOr(conflict, fmt.Sprintf(`EXISTS (SELECT 1 FROM %s k WHERE k.site_id = $2)`, t.name))
				continue
			}
			var eq []string
			for _, c := range cols {
				eq = append(eq, fmt.Sprintf(`k.%[1]s IS NOT DISTINCT FROM d.%[1]s`, pgx.Identifier{c}.Sanitize()))
			}
			conflict = joinOr(conflict, fmt.Sprintf(`EXISTS (SELECT 1 FROM %s k WHERE k.site_id = $2 AND %s)`, t.name, strings.Join(eq, " AND ")))
		}
		dupCount := 0
		if conflict != "" {
			tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s d WHERE d.site_id = $1 AND (%s)`, t.name, conflict), *dup, *keep).Scan(&dupCount)
		}
		fmt.Printf("  %-22s %4d baris: %4d kembar (dibuang), %4d dipindah\n", t.name, total, dupCount, total-dupCount)
		if !*apply {
			continue
		}
		if err := backupTable(ctx, tx, backupDir, t.name, *dup); err != nil {
			log.Fatalf("backup %s: %v", t.name, err)
		}
		if dupCount > 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s d WHERE d.site_id = $1 AND (%s)`, t.name, conflict), *dup, *keep); err != nil {
				log.Fatalf("hapus kembar %s: %v", t.name, err)
			}
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET site_id = $2 WHERE site_id = $1`, t.name), *dup, *keep); err != nil {
			log.Fatalf("pindah %s: %v", t.name, err)
		}
	}

	if !*apply {
		fmt.Println("\n(rencana saja — tambahkan -apply untuk menjalankan)")
		return
	}
	if err := backupTable(ctx, tx, backupDir, "sites", *dup); err != nil {
		log.Fatal(err)
	}
	// Pertahankan riwayat terlama & scan terbaru dari keduanya
	if _, err := tx.Exec(ctx, `
		UPDATE sites k SET created_at = LEAST(k.created_at, d.created_at),
		       last_scanned_at = GREATEST(k.last_scanned_at, d.last_scanned_at),
		       rescan_count = k.rescan_count + d.rescan_count
		FROM sites d WHERE k.id = $1 AND d.id = $2`, *keep, *dup); err != nil {
		log.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sites WHERE id = $1`, *dup); err != nil {
		log.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}
	pg.Audit(ctx, "system (mergesites)", "", "sites.merged", fmt.Sprintf("site:%d", *keep), 200,
		map[string]any{"merged_from": *dup, "backup": backupDir})
	fmt.Printf("\nSelesai. Situs #%d digabung ke #%d. Backup: %s\n", *dup, *keep, backupDir)
}

func joinOr(a, b string) string {
	if a == "" {
		return b
	}
	return a + " OR " + b
}

// referencingTables membaca semua tabel ber-FK ke sites(id) via kolom site_id, beserta constraint unik/PK-nya.
func referencingTables(ctx context.Context, pg *store.PostgresDB) ([]refTable, error) {
	rows, err := pg.Pool.Query(ctx, `
		SELECT DISTINCT c.conrelid::regclass::text FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		WHERE c.contype = 'f' AND c.confrelid = 'sites'::regclass AND a.attname = 'site_id' ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		names = append(names, n)
	}
	rows.Close()
	var out []refTable
	for _, n := range names {
		t := refTable{name: n}
		ur, err := pg.Pool.Query(ctx, `
			SELECT array_agg(a.attname::text ORDER BY a.attnum) FILTER (WHERE a.attname <> 'site_id')
			FROM pg_constraint c JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
			WHERE c.conrelid = $1::regclass AND c.contype IN ('u','p')
			GROUP BY c.oid HAVING bool_or(a.attname = 'site_id')`, n)
		if err != nil {
			return nil, err
		}
		for ur.Next() {
			var cols []string
			ur.Scan(&cols)
			t.uniques = append(t.uniques, cols)
		}
		ur.Close()
		out = append(out, t)
	}
	return out, nil
}

func backupTable(ctx context.Context, tx pgx.Tx, dir, table string, siteID int) error {
	col := "site_id"
	if table == "sites" {
		col = "id"
	}
	f, err := os.Create(filepath.Join(dir, table+".csv"))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = tx.Conn().PgConn().CopyTo(ctx, f,
		fmt.Sprintf(`COPY (SELECT * FROM %s WHERE %s = %d) TO STDOUT WITH CSV HEADER`, table, col, siteID))
	return err
}
