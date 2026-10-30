package store

import (
	"context"
	"math"
)

// KeywordStat: seberapa tepat sebuah kata kunci deteksi menurut situs yang sudah diverifikasi analis.
// Dihitung dari scan lengkap terakhir tiap situs (found_keywords di scan_logs).
type KeywordStat struct {
	ID          int      `json:"id"`
	Keyword     string   `json:"keyword"`
	Weight      float64  `json:"weight"`
	VerJudol    int      `json:"verified_judol"`     // muncul di situs terverifikasi judol
	VerBukan    int      `json:"verified_bukan"`     // muncul di situs terverifikasi bukan judol (false positive)
	AutoJudol   int      `json:"auto_judol"`         // belum diverifikasi, dinilai judol otomatis
	AutoBukan   int      `json:"auto_bukan"`         // belum diverifikasi, dinilai bukan judol
	Precision   *float64 `json:"precision"`          // VerJudol / (VerJudol+VerBukan); null bila data belum cukup
	Suggested   *float64 `json:"suggested_weight"`   // saran bobot dari ketepatan
}

type KeywordStatsResult struct {
	VerifiedJudol int           `json:"verified_judol"`
	VerifiedBukan int           `json:"verified_bukan"`
	MinSample     int           `json:"min_sample"` // situs terverifikasi minimal per kata kunci sebelum ada saran
	Keywords      []KeywordStat `json:"keywords"`
}

const keywordMinSample = 5

func (db *PostgresDB) KeywordStats(ctx context.Context) (*KeywordStatsResult, error) {
	out := &KeywordStatsResult{MinSample: keywordMinSample, Keywords: []KeywordStat{}}
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE verified_status='judol'), COUNT(*) FILTER (WHERE verified_status='bukan_judol') FROM sites`).
		Scan(&out.VerifiedJudol, &out.VerifiedBukan)
	rows, err := db.Pool.Query(ctx, `
		WITH last AS (
			SELECT DISTINCT ON (l.site_id) l.site_id, l.result->'result'->'found_keywords' AS kw
			FROM scan_logs l
			WHERE NOT (l.result ? 'error') AND jsonb_typeof(l.result->'result'->'found_keywords') = 'array'
			ORDER BY l.site_id, l.created_at DESC)
		SELECT k.id, k.keyword, COALESCE(k.weight, 0),
		       COUNT(s.id) FILTER (WHERE s.verified_status = 'judol'),
		       COUNT(s.id) FILTER (WHERE s.verified_status = 'bukan_judol'),
		       COUNT(s.id) FILTER (WHERE s.verified_status IS NULL AND s.is_gambling),
		       COUNT(s.id) FILTER (WHERE s.verified_status IS NULL AND NOT s.is_gambling)
		FROM detection_keywords k
		LEFT JOIN last ON last.kw ? k.keyword
		LEFT JOIN sites s ON s.id = last.site_id
		WHERE k.is_active
		GROUP BY k.id ORDER BY k.keyword`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k KeywordStat
		if err := rows.Scan(&k.ID, &k.Keyword, &k.Weight, &k.VerJudol, &k.VerBukan, &k.AutoJudol, &k.AutoBukan); err != nil {
			return nil, err
		}
		if n := k.VerJudol + k.VerBukan; n >= keywordMinSample {
			p := float64(k.VerJudol) / float64(n)
			// Kata yang selalu benar → bobot tinggi; yang sering muncul di situs bersih → pendukung saja
			sug := math.Round((0.1+0.8*p)*10) / 10
			k.Precision, k.Suggested = &p, &sug
		}
		out.Keywords = append(out.Keywords, k)
	}
	return out, rows.Err()
}
