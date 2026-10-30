package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"jutec-go/internal/crawler"
	"jutec-go/internal/store"
	"jutec-go/internal/telegram"
)

type TelegramService struct {
	client *telegram.Client
	pg     *store.PostgresDB
}

func NewTelegramService(apiID int, apiHash string, pg *store.PostgresDB) *TelegramService {
	return &TelegramService{
		client: telegram.NewClient(apiID, apiHash),
		pg:     pg,
	}
}

// ResolveAndSave resolve username → simpan ke DB, return intel.
func (s *TelegramService) ResolveAndSave(ctx context.Context, username string) (*store.TelegramChannel, error) {
	username = strings.TrimPrefix(username, "@")

	intel, err := s.client.ResolveUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	channelID, err := s.pg.SaveTelegramChannel(ctx, intel)
	if err != nil {
		return nil, err
	}

	log.Printf("📱 [Telegram] Resolved @%s → %s (%d members)", username, intel.DisplayName, intel.MemberCount)

	// Jika ada linked URL di bio, enqueue untuk scan
	for _, u := range intel.LinkedURLs {
		log.Printf("📱 [Telegram] Found linked URL from @%s: %s", username, u)
		_ = u // caller bisa gunakan intel.LinkedURLs untuk trigger scan
	}

	_ = channelID
	return s.pg.GetTelegramChannel(ctx, username)
}

// ScrapeMessages membaca N pesan terbaru channel/grup PUBLIK (tanpa bergabung), menyimpan pesannya,
// mengekstrak artefak, dan mengembalikan semua link di pesan (termasuk link tersembunyi & tombol).
func (s *TelegramService) ScrapeMessages(ctx context.Context, username string, limit int) (int, []string, error) {
	username = strings.TrimPrefix(username, "@")

	ch, err := s.pg.GetTelegramChannel(ctx, username)
	if err != nil {
		if _, err2 := s.ResolveAndSave(ctx, username); err2 != nil {
			return 0, nil, err2
		}
		if ch, err = s.pg.GetTelegramChannel(ctx, username); err != nil {
			return 0, nil, err
		}
	}

	msgs, err := s.client.GetRecentMessages(ctx, username, limit)
	if err != nil {
		if !errors.Is(err, telegram.ErrNotLoggedIn) {
			s.pg.MarkTelegramScraped(ctx, ch.ID, err.Error())
		}
		return 0, nil, err
	}
	if err := s.pg.SaveTelegramMessages(ctx, ch.ID, msgs); err != nil {
		return 0, nil, err
	}

	artifactCount := 0
	seen := map[string]bool{}
	var links []string
	for _, m := range msgs {
		for _, l := range m.Links {
			if !seen[l] {
				seen[l] = true
				links = append(links, l)
			}
		}
		// Artefak dari teks + link (mis. wa.me/62… di balik tombol)
		arts := crawler.ExtractArtifacts(m.Text + "\n" + strings.Join(m.Links, "\n"))
		if len(arts) == 0 {
			continue
		}
		var artMaps []map[string]string
		for _, a := range arts {
			artMaps = append(artMaps, map[string]string{"type": a.Type, "value": a.Value, "name": a.Name})
		}
		savedIDs, err := s.pg.SaveArtifactsGetIDs(ctx, artMaps)
		if err != nil {
			log.Printf("⚠️  [Telegram] artifact save error: %v", err)
			continue
		}
		for _, aid := range savedIDs {
			s.pg.LinkTelegramArtifact(ctx, ch.ID, aid)
		}
		artifactCount += len(arts)
	}
	s.pg.MarkTelegramScraped(ctx, ch.ID, "")
	log.Printf("📱 [Telegram] @%s: %d pesan, %d artefak, %d link", username, len(msgs), artifactCount, len(links))
	return artifactCount, links, nil
}

// ResolveAllPending resolve semua Telegram username dari artifacts yang belum di-resolve.
// Dipanggil secara berkala atau on-demand.
func (s *TelegramService) ResolveAllPending(ctx context.Context) (int, error) {
	usernames, err := s.pg.GetUnresolvedTelegramUsernames(ctx, 50)
	if err != nil {
		return 0, err
	}

	resolved := 0
	for _, u := range usernames {
		if _, err := s.ResolveAndSave(ctx, u); err != nil {
			// Sesi belum login / dibatasi Telegram: hentikan putaran, JANGAN tandai username
			if errors.Is(err, telegram.ErrNotLoggedIn) || telegram.IsFlood(err) {
				return resolved, err
			}
			// Hanya jawaban pasti "username tidak ada" yang dicatat; gangguan lain dicoba lagi nanti
			if telegram.IsNotFound(err) {
				s.pg.SaveTelegramNotFound(ctx, u)
				log.Printf("🚫 [Telegram] @%s tidak terdaftar di Telegram", u)
			} else {
				log.Printf("⚠️  [Telegram] @%s gagal (dicoba lagi nanti): %v", u, err)
			}
			continue
		}
		resolved++
		// Rate limit: 1 request per 2 detik untuk hindari flood wait
		select {
		case <-ctx.Done():
			return resolved, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return resolved, nil
}
