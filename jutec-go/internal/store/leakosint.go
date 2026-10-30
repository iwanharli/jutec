package store

import (
	"context"
	"time"
)

// --- KomInfo ---

type LeakKomInfo struct {
	ID         int       `json:"id"`
	Phone      string    `json:"phone"`
	NIK        string    `json:"nik"`
	Provider   string    `json:"provider"`
	RegDate    string    `json:"reg_date"`
	ArtifactID *int      `json:"artifact_id,omitempty"`
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"created_at"`
}

func (db *PostgresDB) SaveLeakKomInfo(ctx context.Context, phone, nik, provider, regDate string, artifactID *int) error {
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO leak_kominfo (phone, nik, provider, reg_date, artifact_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (phone, nik, reg_date) DO NOTHING`,
		phone, nik, provider, regDate, artifactID,
	)
	return err
}

func (db *PostgresDB) SaveLeakEnrichmentAttempt(ctx context.Context, artifactID int, phone, status, errorMessage string) error {
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO leak_enrichment_attempts (artifact_id, phone, status, error_message, attempted_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (artifact_id) DO UPDATE SET
		   phone = EXCLUDED.phone,
		   status = EXCLUDED.status,
		   error_message = EXCLUDED.error_message,
		   attempted_at = NOW()`,
		artifactID, phone, status, errorMessage,
	)
	return err
}

func (db *PostgresDB) GetLeakKomInfoByPhone(ctx context.Context, phone string) ([]LeakKomInfo, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, phone, nik, provider, reg_date, artifact_id, COALESCE(source, 'LeakOSINT'), created_at
		 FROM leak_kominfo WHERE phone = $1 ORDER BY reg_date`, phone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LeakKomInfo
	for rows.Next() {
		var r LeakKomInfo
		if err := rows.Scan(&r.ID, &r.Phone, &r.NIK, &r.Provider, &r.RegDate, &r.ArtifactID, &r.Source, &r.CreatedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

func (db *PostgresDB) GetLeakKomInfoByNIK(ctx context.Context, nik string) ([]LeakKomInfo, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, phone, nik, provider, reg_date, artifact_id, COALESCE(source, 'LeakOSINT'), created_at
		 FROM leak_kominfo WHERE nik = $1 ORDER BY reg_date`, nik)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LeakKomInfo
	for rows.Next() {
		var r LeakKomInfo
		if err := rows.Scan(&r.ID, &r.Phone, &r.NIK, &r.Provider, &r.RegDate, &r.ArtifactID, &r.Source, &r.CreatedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

// --- Dukcapil ---

type LeakDukcapil struct {
	ID         int       `json:"id"`
	Phone      string    `json:"phone"`
	NIK        string    `json:"nik"`
	FullName   string    `json:"full_name"`
	Gender     string    `json:"gender"`
	BirthDate  string    `json:"birth_date"`
	Address    string    `json:"address"`
	District   string    `json:"district"`
	Location   string    `json:"location"`
	Region     string    `json:"region"`
	ArtifactID *int      `json:"artifact_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func (db *PostgresDB) SaveLeakDukcapil(ctx context.Context, phone, nik, fullName, gender, birthDate, address, district, location, region string, artifactID *int) error {
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO leak_dukcapil (phone, nik, full_name, gender, birth_date, address, district, location, region, artifact_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (phone, nik) DO UPDATE SET
		   full_name = COALESCE(NULLIF($3, ''), leak_dukcapil.full_name),
		   gender = COALESCE(NULLIF($4, ''), leak_dukcapil.gender),
		   birth_date = COALESCE(NULLIF($5, ''), leak_dukcapil.birth_date),
		   address = COALESCE(NULLIF($6, ''), leak_dukcapil.address),
		   district = COALESCE(NULLIF($7, ''), leak_dukcapil.district),
		   location = COALESCE(NULLIF($8, ''), leak_dukcapil.location),
		   region = COALESCE(NULLIF($9, ''), leak_dukcapil.region)`,
		phone, nik, fullName, gender, birthDate, address, district, location, region, artifactID,
	)
	return err
}

func (db *PostgresDB) GetLeakDukcapilByPhone(ctx context.Context, phone string) ([]LeakDukcapil, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, phone, nik, full_name, gender, birth_date, address, district, location, region, artifact_id, created_at
		 FROM leak_dukcapil WHERE phone = $1`, phone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LeakDukcapil
	for rows.Next() {
		var r LeakDukcapil
		if err := rows.Scan(&r.ID, &r.Phone, &r.NIK, &r.FullName, &r.Gender, &r.BirthDate, &r.Address, &r.District, &r.Location, &r.Region, &r.ArtifactID, &r.CreatedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

func (db *PostgresDB) GetLeakDukcapilByNIK(ctx context.Context, nik string) ([]LeakDukcapil, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, phone, nik, full_name, gender, birth_date, address, district, location, region, artifact_id, created_at
		 FROM leak_dukcapil WHERE nik = $1`, nik)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LeakDukcapil
	for rows.Next() {
		var r LeakDukcapil
		if err := rows.Scan(&r.ID, &r.Phone, &r.NIK, &r.FullName, &r.Gender, &r.BirthDate, &r.Address, &r.District, &r.Location, &r.Region, &r.ArtifactID, &r.CreatedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

func (db *PostgresDB) ListLeakKomInfo(ctx context.Context, q string, limit, offset int) ([]LeakKomInfo, int, error) {
	var total int
	if q != "" {
		db.Pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM leak_kominfo WHERE phone ILIKE $1 OR nik ILIKE $1`,
			"%"+q+"%").Scan(&total)
	} else {
		db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM leak_kominfo`).Scan(&total)
	}

	var rows interface {
		Next() bool
		Scan(...any) error
		Close()
	}
	var err error
	if q != "" {
		rows, err = db.Pool.Query(ctx,
			`SELECT id, phone, nik, provider, reg_date, artifact_id, COALESCE(source, 'LeakOSINT'), created_at
			 FROM leak_kominfo WHERE phone ILIKE $1 OR nik ILIKE $1
			 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			"%"+q+"%", limit, offset)
	} else {
		rows, err = db.Pool.Query(ctx,
			`SELECT id, phone, nik, provider, reg_date, artifact_id, COALESCE(source, 'LeakOSINT'), created_at
			 FROM leak_kominfo ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []LeakKomInfo
	for rows.Next() {
		var r LeakKomInfo
		if err := rows.Scan(&r.ID, &r.Phone, &r.NIK, &r.Provider, &r.RegDate, &r.ArtifactID, &r.Source, &r.CreatedAt); err == nil {
			results = append(results, r)
		}
	}
	return results, total, nil
}

func (db *PostgresDB) ListLeakDukcapil(ctx context.Context, q string, limit, offset int) ([]LeakDukcapil, int, error) {
	var total int
	if q != "" {
		db.Pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM leak_dukcapil WHERE phone ILIKE $1 OR nik ILIKE $1 OR full_name ILIKE $1`,
			"%"+q+"%").Scan(&total)
	} else {
		db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM leak_dukcapil`).Scan(&total)
	}

	var rows interface {
		Next() bool
		Scan(...any) error
		Close()
	}
	var err error
	if q != "" {
		rows, err = db.Pool.Query(ctx,
			`SELECT id, phone, nik, full_name, gender, birth_date, address, district, location, region, artifact_id, created_at
			 FROM leak_dukcapil WHERE phone ILIKE $1 OR nik ILIKE $1 OR full_name ILIKE $1
			 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			"%"+q+"%", limit, offset)
	} else {
		rows, err = db.Pool.Query(ctx,
			`SELECT id, phone, nik, full_name, gender, birth_date, address, district, location, region, artifact_id, created_at
			 FROM leak_dukcapil ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []LeakDukcapil
	for rows.Next() {
		var r LeakDukcapil
		if err := rows.Scan(&r.ID, &r.Phone, &r.NIK, &r.FullName, &r.Gender, &r.BirthDate,
			&r.Address, &r.District, &r.Location, &r.Region, &r.ArtifactID, &r.CreatedAt); err == nil {
			results = append(results, r)
		}
	}
	return results, total, nil
}

// --- Utilities ---

// GetPhoneArtifactsNotEnriched mengambil semua artifact nomor HP yang belum pernah di-query ke LeakOSINT.
func (db *PostgresDB) GetPhoneArtifactsNotEnriched(ctx context.Context) ([]struct {
	ID    int
	Phone string
}, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT a.id, a.value FROM artifacts a
		 WHERE a.type IN ('Contact', 'E-Wallet')
		   AND a.value ~ '^\d{10,15}$'
		   AND NOT EXISTS (SELECT 1 FROM leak_kominfo lk WHERE lk.artifact_id = a.id)
		   AND NOT EXISTS (SELECT 1 FROM leak_enrichment_attempts la WHERE la.artifact_id = a.id)
		 ORDER BY a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		ID    int
		Phone string
	}
	for rows.Next() {
		var r struct {
			ID    int
			Phone string
		}
		if err := rows.Scan(&r.ID, &r.Phone); err == nil {
			results = append(results, r)
		}
	}
	return results, nil
}
