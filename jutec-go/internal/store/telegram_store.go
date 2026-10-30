package store

import (
	"context"
	"time"

	"jutec-go/internal/telegram"
)

type TelegramChannel struct {
	ID          int       `json:"id"`
	Username    string    `json:"username"`
	TelegramID  int64     `json:"telegram_id"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	MemberCount int       `json:"member_count"`
	ChannelType string    `json:"channel_type"`
	IsActive    bool      `json:"is_active"`
	LinkedURL   string    `json:"linked_url"`
	ResolvedAt  time.Time `json:"resolved_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SaveTelegramChannel upsert data channel — update jika username sudah ada.
func (db *PostgresDB) SaveTelegramChannel(ctx context.Context, intel *telegram.ChannelIntel) (int, error) {
	linkedURL := ""
	if len(intel.LinkedURLs) > 0 {
		linkedURL = intel.LinkedURLs[0]
	}

	var id int
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO telegram_channels
			(username, telegram_id, display_name, bio, member_count, channel_type, is_active, linked_url, resolved_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())
		ON CONFLICT (username) DO UPDATE SET
			telegram_id  = EXCLUDED.telegram_id,
			display_name = EXCLUDED.display_name,
			bio          = EXCLUDED.bio,
			member_count = EXCLUDED.member_count,
			channel_type = EXCLUDED.channel_type,
			is_active    = EXCLUDED.is_active,
			linked_url   = EXCLUDED.linked_url,
			resolved_at  = EXCLUDED.resolved_at,
			updated_at   = NOW()
		RETURNING id
	`, intel.Username, intel.TelegramID, intel.DisplayName, intel.Bio,
		intel.MemberCount, intel.ChannelType, intel.IsActive, linkedURL, intel.ResolvedAt,
	).Scan(&id)
	return id, err
}

// SaveTelegramResolveFailure marks a username as checked even when Telegram rejects it.
// SaveTelegramNotFound: Telegram menyatakan username tidak ada (USERNAME_NOT_OCCUPIED/INVALID).
func (db *PostgresDB) SaveTelegramNotFound(ctx context.Context, username string) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO telegram_channels
			(username, telegram_id, display_name, bio, member_count, channel_type, is_active, linked_url, resolved_at, updated_at, last_error)
		VALUES ($1, 0, $1, '', 0, 'not_found', false, '', NOW(), NOW(), 'username tidak terdaftar di Telegram')
		ON CONFLICT (username) DO UPDATE SET
			is_active = false, channel_type = 'not_found', resolved_at = NOW(), updated_at = NOW(),
			last_error = 'username tidak terdaftar di Telegram'
	`, username)
	return err
}

// SaveTelegramResolveFailure dipertahankan untuk kompatibilitas pemanggil lama → kini berarti "tidak ditemukan".
func (db *PostgresDB) SaveTelegramResolveFailure(ctx context.Context, username string) error {
	return db.SaveTelegramNotFound(ctx, username)
}

// MarkTelegramScraped mencatat waktu & hasil pembacaan pesan terakhir.
func (db *PostgresDB) MarkTelegramScraped(ctx context.Context, channelID int, errMsg string) {
	db.Pool.Exec(ctx, `UPDATE telegram_channels SET last_scraped_at = NOW(), last_error = NULLIF($2, '') WHERE id = $1`, channelID, errMsg)
}

// TelegramChannelsToScrape: channel/grup publik valid yang paling lama tidak dibaca.
func (db *PostgresDB) TelegramChannelsToScrape(ctx context.Context, limit int) ([]string, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT username FROM telegram_channels
		WHERE is_active AND channel_type IN ('channel','group')
		  AND (last_scraped_at IS NULL OR last_scraped_at < NOW() - INTERVAL '6 hours')
		ORDER BY last_scraped_at NULLS FIRST, member_count DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var u string
		if rows.Scan(&u) == nil {
			out = append(out, u)
		}
	}
	return out, nil
}

// TelegramStatusForSite: status setiap kontak Telegram situs + ringkasan isi channel.
func (db *PostgresDB) TelegramStatusForSite(ctx context.Context, siteID int) ([]map[string]any, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT LTRIM(a.value, '@'), tc.id, COALESCE(tc.channel_type, ''), COALESCE(tc.is_active, false),
		       COALESCE(tc.display_name, ''), COALESCE(tc.bio, ''), COALESCE(tc.member_count, 0), COALESCE(tc.linked_url, ''),
		       tc.resolved_at, tc.last_scraped_at, COALESCE(tc.last_error, ''),
		       (SELECT COUNT(*) FROM telegram_messages m WHERE m.channel_id = tc.id),
		       (SELECT MAX(date) FROM telegram_messages m WHERE m.channel_id = tc.id),
		       (SELECT COUNT(*) FROM telegram_artifacts ta WHERE ta.channel_id = tc.id),
		       (SELECT COUNT(DISTINCT sa2.site_id) FROM artifacts a2 JOIN site_artifacts sa2 ON sa2.artifact_id = a2.id
		        WHERE a2.type = 'Contact' AND a2.name = 'Telegram' AND LOWER(LTRIM(a2.value,'@')) = LOWER(LTRIM(a.value,'@')))
		FROM site_artifacts sa JOIN artifacts a ON a.id = sa.artifact_id
		LEFT JOIN telegram_channels tc ON LOWER(tc.username) = LOWER(LTRIM(a.value, '@'))
		WHERE sa.site_id = $1 AND a.type = 'Contact' AND a.name = 'Telegram'
		ORDER BY tc.member_count DESC NULLS LAST`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var username, typ, name, bio, link, lastErr string
		var chID *int
		var active bool
		var members, msgCount, artCount, siteCount int
		var resolved, scraped, lastMsg *time.Time
		if err := rows.Scan(&username, &chID, &typ, &active, &name, &bio, &members, &link, &resolved, &scraped, &lastErr,
			&msgCount, &lastMsg, &artCount, &siteCount); err != nil {
			return nil, err
		}
		status := "pending"
		switch {
		case chID != nil && active:
			status = "valid"
		case typ == "not_found":
			status = "not_found"
		}
		out = append(out, map[string]any{
			"username": username, "status": status, "type": typ, "name": name, "bio": bio, "members": members,
			"linked_url": link, "resolved_at": resolved, "last_scraped_at": scraped, "last_error": lastErr,
			"message_count": msgCount, "last_message_at": lastMsg, "artifact_count": artCount, "site_count": siteCount,
		})
	}
	return out, nil
}

// TelegramRecentLinks: link dari pesan terbaru channel (untuk ditampilkan & dianalisis).
func (db *PostgresDB) TelegramRecentMessages(ctx context.Context, username string, limit int) ([]map[string]any, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT m.message_id, m.text, m.date FROM telegram_messages m JOIN telegram_channels tc ON tc.id = m.channel_id
		WHERE LOWER(tc.username) = LOWER($1) ORDER BY m.date DESC LIMIT $2`, username, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id int64
		var text string
		var date time.Time
		if rows.Scan(&id, &text, &date) == nil {
			out = append(out, map[string]any{"message_id": id, "text": text, "date": date})
		}
	}
	return out, nil
}

// SaveTelegramMessages menyimpan pesan dari channel.
func (db *PostgresDB) SaveTelegramMessages(ctx context.Context, channelID int, msgs []telegram.MessageIntel) error {
	for _, m := range msgs {
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO telegram_messages (channel_id, message_id, text, date)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (channel_id, message_id) DO NOTHING
		`, channelID, m.MessageID, m.Text, m.Date)
		if err != nil {
			return err
		}
	}
	return nil
}

// LinkTelegramArtifact menghubungkan channel ke artifact yang ditemukan dari pesannya.
func (db *PostgresDB) LinkTelegramArtifact(ctx context.Context, channelID, artifactID int) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO telegram_artifacts (channel_id, artifact_id)
		VALUES ($1,$2) ON CONFLICT DO NOTHING
	`, channelID, artifactID)
	return err
}

// GetTelegramChannel mengambil data channel berdasarkan username.
func (db *PostgresDB) GetTelegramChannel(ctx context.Context, username string) (*TelegramChannel, error) {
	var ch TelegramChannel
	err := db.Pool.QueryRow(ctx, `
		SELECT id, username, telegram_id, display_name, bio, member_count,
		       channel_type, is_active, COALESCE(linked_url,''), resolved_at, created_at, updated_at
		FROM telegram_channels WHERE username = $1
	`, username).Scan(
		&ch.ID, &ch.Username, &ch.TelegramID, &ch.DisplayName, &ch.Bio,
		&ch.MemberCount, &ch.ChannelType, &ch.IsActive, &ch.LinkedURL,
		&ch.ResolvedAt, &ch.CreatedAt, &ch.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

// ListTelegramChannels mengembalikan semua channel yang sudah di-resolve, terbaru dulu.
func (db *PostgresDB) ListTelegramChannels(ctx context.Context, limit, offset int) ([]TelegramChannel, int, error) {
	var total int
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM telegram_channels WHERE channel_type <> 'unresolved'`).Scan(&total)

	rows, err := db.Pool.Query(ctx, `
		SELECT id, username, telegram_id, display_name, bio, member_count,
		       channel_type, is_active, COALESCE(linked_url,''), resolved_at, created_at, updated_at
		FROM telegram_channels
		WHERE channel_type <> 'unresolved'
		ORDER BY member_count DESC NULLS LAST, resolved_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var channels []TelegramChannel
	for rows.Next() {
		var ch TelegramChannel
		if err := rows.Scan(
			&ch.ID, &ch.Username, &ch.TelegramID, &ch.DisplayName, &ch.Bio,
			&ch.MemberCount, &ch.ChannelType, &ch.IsActive, &ch.LinkedURL,
			&ch.ResolvedAt, &ch.CreatedAt, &ch.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		channels = append(channels, ch)
	}
	return channels, total, nil
}

// GetUnresolvedTelegramUsernames mengambil username @Telegram dari artifacts yang belum di-resolve.
func (db *PostgresDB) GetUnresolvedTelegramUsernames(ctx context.Context, limit int) ([]string, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT LTRIM(a.value, '@') AS username
		FROM artifacts a
		WHERE a.type = 'Contact' AND a.name = 'Telegram'
		  AND NOT EXISTS (
		    SELECT 1 FROM telegram_channels tc
		    WHERE LOWER(tc.username) = LOWER(LTRIM(a.value, '@'))
		      -- "tidak ditemukan" dicek ulang setelah 30 hari (username bisa didaftarkan kembali)
		      AND NOT (tc.channel_type = 'not_found' AND tc.resolved_at < NOW() - INTERVAL '30 days')
		  )
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var u string
		if rows.Scan(&u) == nil {
			usernames = append(usernames, u)
		}
	}
	return usernames, nil
}
