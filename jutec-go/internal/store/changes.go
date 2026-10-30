package store

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

// ScanChange: apa yang muncul/hilang dibanding scan sebelumnya (per kategori).
type ScanChange struct {
	ScanID  int                 `json:"scan_id"`
	At      time.Time           `json:"at"`
	Added   map[string][]string `json:"added"`
	Removed map[string][]string `json:"removed"`
}

// snapshot minimal dari payload scan_logs.result.result (AnalysisResult)
type scanSnapshot struct {
	Artifacts []struct {
		Type, Value, Name string
	} `json:"artifacts"`
	Links *struct {
		Apps     []string `json:"apps"`
		Register []string `json:"register"`
	} `json:"links"`
	TrackingIDs []struct {
		Platform, Value string
	} `json:"tracking_ids"`
	Fingerprints []struct {
		Kind, Value string
	} `json:"fingerprints"`
	WidgetLinks    []string `json:"widget_links"`
	NetworkSignals *struct {
		GameProviders []string `json:"game_providers"`
	} `json:"network_signals"`
}

func (s *scanSnapshot) sets() map[string]map[string]bool {
	out := map[string]map[string]bool{
		"Artefak": {}, "Aplikasi": {}, "Link pendaftaran": {}, "Tracking": {}, "Sidik jari": {}, "Link widget": {}, "Provider game": {},
	}
	for _, a := range s.Artifacts {
		label := a.Type
		if a.Name != "" {
			label += " " + a.Name
		}
		out["Artefak"][label+": "+a.Value] = true
	}
	if s.Links != nil {
		for _, u := range s.Links.Apps {
			out["Aplikasi"][u] = true
		}
		for _, u := range s.Links.Register {
			out["Link pendaftaran"][u] = true
		}
	}
	for _, t := range s.TrackingIDs {
		out["Tracking"][t.Platform+": "+t.Value] = true
	}
	for _, f := range s.Fingerprints {
		out["Sidik jari"][f.Kind+": "+f.Value] = true
	}
	for _, u := range s.WidgetLinks {
		out["Link widget"][u] = true
	}
	if s.NetworkSignals != nil {
		for _, p := range s.NetworkSignals.GameProviders {
			out["Provider game"][p] = true
		}
	}
	return out
}

// SiteChanges membandingkan scan berhasil berurutan (maks. 12 terakhir) dan mengembalikan perubahannya,
// terbaru dulu. Scan pertama tidak punya pembanding sehingga tidak dimasukkan.
func (db *PostgresDB) SiteChanges(ctx context.Context, siteID int) ([]ScanChange, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, created_at, result->'result' FROM scan_logs
		WHERE site_id = $1 AND NOT (result ? 'error') AND result ? 'result'
		  -- hanya scan lengkap (dengan pass HP) agar beda kedalaman scan tak terbaca sebagai "perubahan"
		  AND (result->'result'->>'mobile_crawled')::boolean IS TRUE
		ORDER BY created_at DESC LIMIT 12`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		id   int
		at   time.Time
		sets map[string]map[string]bool
	}
	var list []row
	for rows.Next() {
		var r row
		var raw []byte
		if err := rows.Scan(&r.id, &r.at, &raw); err != nil {
			return nil, err
		}
		var snap scanSnapshot
		if json.Unmarshal(raw, &snap) != nil {
			continue
		}
		r.sets = snap.sets()
		list = append(list, r)
	}
	// list terurut terbaru → lama; bandingkan tiap scan dengan scan sebelumnya
	changes := []ScanChange{}
	for i := 0; i+1 < len(list); i++ {
		cur, prev := list[i], list[i+1]
		ch := ScanChange{ScanID: cur.id, At: cur.at, Added: map[string][]string{}, Removed: map[string][]string{}}
		for cat, set := range cur.sets {
			for v := range set {
				if !prev.sets[cat][v] {
					ch.Added[cat] = append(ch.Added[cat], v)
				}
			}
			for v := range prev.sets[cat] {
				if !set[v] {
					ch.Removed[cat] = append(ch.Removed[cat], v)
				}
			}
			sort.Strings(ch.Added[cat])
			sort.Strings(ch.Removed[cat])
		}
		for k, v := range ch.Added {
			if len(v) == 0 {
				delete(ch.Added, k)
			}
		}
		for k, v := range ch.Removed {
			if len(v) == 0 {
				delete(ch.Removed, k)
			}
		}
		if len(ch.Added) > 0 || len(ch.Removed) > 0 {
			changes = append(changes, ch)
		}
	}
	return changes, nil
}

// ── Wayback ──────────────────────────────────────────────────────────

type WaybackSnapshot struct {
	Timestamp string              `json:"timestamp"`
	URL       string              `json:"url"`
	Artifacts []map[string]string `json:"artifacts"`
	Register  []string            `json:"register"`
	Tracking  []map[string]string `json:"tracking"`
}

func (db *PostgresDB) SaveWaybackSnapshot(ctx context.Context, siteID int, s WaybackSnapshot) error {
	art, _ := json.Marshal(nonNilMaps(s.Artifacts))
	trk, _ := json.Marshal(nonNilMaps(s.Tracking))
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO wayback_snapshots (site_id, snapshot_ts, url, artifacts, register, tracking)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (site_id, snapshot_ts) DO UPDATE SET artifacts=$4, register=$5, tracking=$6, fetched_at=NOW()`,
		siteID, s.Timestamp, s.URL, art, nz(s.Register), trk)
	return err
}

func (db *PostgresDB) WaybackSnapshots(ctx context.Context, siteID int) ([]WaybackSnapshot, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT snapshot_ts, url, artifacts, register, tracking FROM wayback_snapshots
		WHERE site_id = $1 ORDER BY snapshot_ts DESC`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WaybackSnapshot{}
	for rows.Next() {
		var s WaybackSnapshot
		var art, trk []byte
		if err := rows.Scan(&s.Timestamp, &s.URL, &art, &s.Register, &trk); err != nil {
			return nil, err
		}
		json.Unmarshal(art, &s.Artifacts)
		json.Unmarshal(trk, &s.Tracking)
		out = append(out, s)
	}
	return out, nil
}

func nonNilMaps(m []map[string]string) []map[string]string {
	if m == nil {
		return []map[string]string{}
	}
	return m
}

// SaveWaybackResult menyimpan snapshot Wayback sekaligus memasukkan artefak lamanya ke relasi
// (rekening yang sudah dihapus bisa masih dipakai situs lain). Mengembalikan jumlah artefak historis.
func (db *PostgresDB) SaveWaybackResult(ctx context.Context, siteID int, snaps []WaybackSnapshot) int {
	historic := 0
	for _, s := range snaps {
		db.SaveWaybackSnapshot(ctx, siteID, s)
		if len(s.Artifacts) > 0 && db.SaveArtifacts(ctx, siteID, s.Artifacts) == nil {
			historic += len(s.Artifacts)
		}
	}
	db.MarkWaybackChecked(ctx, siteID)
	return historic
}
