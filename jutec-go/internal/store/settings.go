package store

import (
	"context"
	"strconv"
	"time"
)

type Setting struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (db *PostgresDB) GetAllSettings(ctx context.Context) ([]Setting, error) {
	rows, err := db.Pool.Query(ctx, `SELECT key, value, description, updated_at FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var settings []Setting
	for rows.Next() {
		var s Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.Description, &s.UpdatedAt); err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, nil
}

func (db *PostgresDB) GetSetting(ctx context.Context, key string) string {
	var value string
	db.Pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&value)
	return value
}

func (db *PostgresDB) SetSetting(ctx context.Context, key, value string) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, key, value)
	return err
}

// GetSettingInt returns setting as int, falls back to defaultVal if missing or invalid.
func (db *PostgresDB) GetSettingInt(ctx context.Context, key string, defaultVal int) int {
	v := db.GetSetting(ctx, key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// GetSettingBool returns setting as bool, falls back to defaultVal.
func (db *PostgresDB) GetSettingBool(ctx context.Context, key string, defaultVal bool) bool {
	v := db.GetSetting(ctx, key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}
