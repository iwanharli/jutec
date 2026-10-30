package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const auditGenesis = "GENESIS"

type AuditEntry struct {
	ID       int64           `json:"id"`
	At       time.Time       `json:"at"`
	Actor    string          `json:"actor"`
	IP       string          `json:"ip"`
	Action   string          `json:"action"`
	Target   string          `json:"target"`
	Detail   json.RawMessage `json:"detail"`
	Status   int             `json:"status"`
	PrevHash string          `json:"prev_hash"`
	Hash     string          `json:"hash"`
}

// auditHash: isi baris yang di-hash. Urutan & format tidak boleh diubah (memutus rantai lama).
func auditHash(prev string, at time.Time, actor, ip, action, target string, status int, detail []byte) string {
	s := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%s", prev, at.UTC().Format(time.RFC3339Nano), actor, ip, action, target, status, detail)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Audit menambah satu baris ke log audit berantai hash. Kunci advisory menjaga rantai tetap lurus
// walau banyak request menulis bersamaan.
func (db *PostgresDB) Audit(ctx context.Context, actor, ip, action, target string, status int, detail any) error {
	raw, _ := json.Marshal(detail)
	dj := canonicalJSON(raw)
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(727274)`); err != nil {
		return err
	}
	prev := auditGenesis
	tx.QueryRow(ctx, `SELECT hash FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&prev)
	at := time.Now().UTC().Truncate(time.Microsecond) // presisi timestamptz Postgres
	h := auditHash(prev, at, actor, ip, action, target, status, dj)
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_log (at, actor, ip, action, target, detail, status, prev_hash, hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, at, actor, ip, action, target, dj, status, prev, h); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *PostgresDB) ListAudit(ctx context.Context, q, action, target string, limit, offset int) ([]AuditEntry, int, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, at, actor, COALESCE(ip,''), action, COALESCE(target,''), COALESCE(detail,'null'::jsonb), COALESCE(status,0),
		       prev_hash, hash, COUNT(*) OVER()
		FROM audit_log
		WHERE ($1 = '' OR action ILIKE '%'||$1||'%' OR actor ILIKE '%'||$1||'%' OR target ILIKE '%'||$1||'%')
		  AND ($2 = '' OR target = $2)
		  AND ($5 = '' OR action ILIKE '%'||$5||'%')
		ORDER BY id DESC LIMIT $3 OFFSET $4`, q, target, limit, offset, action)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	total := 0
	for rows.Next() {
		var e AuditEntry
		var d []byte
		if err := rows.Scan(&e.ID, &e.At, &e.Actor, &e.IP, &e.Action, &e.Target, &d, &e.Status, &e.PrevHash, &e.Hash, &total); err != nil {
			return nil, 0, err
		}
		e.Detail = d
		out = append(out, e)
	}
	return out, total, nil
}

// VerifyAuditChain menghitung ulang seluruh rantai. Mengembalikan id baris pertama yang rusak (0 = utuh).
func (db *PostgresDB) VerifyAuditChain(ctx context.Context) (checked int, brokenAt int64, err error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, at, actor, COALESCE(ip,''), action, COALESCE(target,''), COALESCE(detail::text,'null'), COALESCE(status,0), prev_hash, hash
		FROM audit_log ORDER BY id`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	prev := auditGenesis
	for rows.Next() {
		var id int64
		var at time.Time
		var actor, ip, action, target, detail, ph, h string
		var status int
		if err := rows.Scan(&id, &at, &actor, &ip, &action, &target, &detail, &status, &ph, &h); err != nil {
			return checked, 0, err
		}
		checked++
		if ph != prev || auditHash(prev, at, actor, ip, action, target, status, canonicalJSON([]byte(detail))) != h {
			return checked, id, nil
		}
		prev = h
	}
	return checked, 0, nil
}

// canonicalJSON: bentuk tunggal yang sama sebelum disimpan & sesudah dibaca dari JSONB
// (kunci terurut, tanpa spasi) agar hash bisa dihitung ulang dengan tepat.
func canonicalJSON(b []byte) []byte {
	var v any
	if len(b) == 0 || json.Unmarshal(b, &v) != nil {
		return []byte("null")
	}
	out, _ := json.Marshal(v)
	return out
}
