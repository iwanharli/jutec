package telegram

import (
	"context"
	"fmt"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

type ChannelIntel struct {
	Username    string    `json:"username"`
	TelegramID  int64     `json:"telegram_id"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	MemberCount int       `json:"member_count"`
	ChannelType string    `json:"channel_type"` // user, bot, channel, group
	IsActive    bool      `json:"is_active"`
	LinkedURLs  []string  `json:"linked_urls"`
	ResolvedAt  time.Time `json:"resolved_at"`
}

type MessageIntel struct {
	MessageID int64     `json:"message_id"`
	Text      string    `json:"text"`
	Date      time.Time `json:"date"`
	Links     []string  `json:"links"` // link tersembunyi di balik teks & tombol inline (mis. "DAFTAR")
}

// IsFlood: Telegram membatasi laju (FLOOD_WAIT, kode 420) — hentikan putaran, coba lagi nanti.
func IsFlood(err error) bool {
	e, ok := tgerr.As(err)
	return ok && e.Code == 420
}

// IsNotFound: Telegram menyatakan username memang tidak ada (bukan gangguan sesi/jaringan).
func IsNotFound(err error) bool {
	return tgerr.Is(err, "USERNAME_NOT_OCCUPIED", "USERNAME_INVALID")
}

// messageLinks mengambil URL dari entitas teks (link tersembunyi) & tombol inline pesan.
func messageLinks(msg *tg.Message) []string {
	var out []string
	for _, e := range msg.Entities {
		if t, ok := e.(*tg.MessageEntityTextURL); ok {
			out = append(out, t.URL)
		}
	}
	if rm, ok := msg.ReplyMarkup.(*tg.ReplyInlineMarkup); ok {
		for _, row := range rm.Rows {
			for _, b := range row.Buttons {
				if u, ok := b.(*tg.KeyboardButtonURL); ok {
					out = append(out, u.URL)
				}
			}
		}
	}
	return append(out, reURL.FindAllString(msg.Message, -1)...)
}

var reURL = regexp.MustCompile(`https?://[^\s<>"']+`)

// ErrNotChannel: username adalah akun pribadi/bot — riwayat pesannya tidak dibaca.
var ErrNotChannel = errors.New("bukan channel/grup publik")

// ResolveUsername mengambil metadata dari username Telegram (tanpa @).
func (c *Client) ResolveUsername(ctx context.Context, username string) (*ChannelIntel, error) {
	username = strings.TrimPrefix(username, "@")
	var intel *ChannelIntel

	err := c.withClient(ctx, func(ctx context.Context, api *tg.Client) error {
		res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
			Username: username,
		})
		if err != nil {
			return err
		}

		intel = &ChannelIntel{
			Username:   username,
			IsActive:   true,
			ResolvedAt: time.Now(),
		}

		for _, peer := range res.Users {
			u, ok := peer.(*tg.User)
			if !ok {
				continue
			}
			intel.TelegramID = int64(u.ID)
			fn, _ := u.GetFirstName()
			ln, _ := u.GetLastName()
			intel.DisplayName = strings.TrimSpace(fn + " " + ln)
			intel.ChannelType = "user"
			if u.GetBot() {
				intel.ChannelType = "bot"
			}

			// Bio perlu call UsersGetFullUser
			full, err := api.UsersGetFullUser(ctx, &tg.InputUser{
				UserID:     u.ID,
				AccessHash: u.AccessHash,
			})
			if err == nil {
				about, _ := full.FullUser.GetAbout()
				intel.Bio = about
				intel.LinkedURLs = reURL.FindAllString(about, -1)
			}
		}

		for _, peer := range res.Chats {
			switch ch := peer.(type) {
			case *tg.Channel:
				intel.TelegramID = int64(ch.ID)
				intel.DisplayName = ch.Title
				if ch.Megagroup {
					intel.ChannelType = "group"
				} else {
					intel.ChannelType = "channel"
				}

				// Bio + member count dari FullChannel
				full, err := api.ChannelsGetFullChannel(ctx, &tg.InputChannel{
					ChannelID:  ch.ID,
					AccessHash: ch.AccessHash,
				})
				if err == nil {
					if fc, ok := full.FullChat.(*tg.ChannelFull); ok {
						intel.Bio = fc.About
						if cnt, ok := fc.GetParticipantsCount(); ok {
							intel.MemberCount = cnt
						}
						intel.LinkedURLs = reURL.FindAllString(fc.About, -1)
					}
				}

			case *tg.Chat:
				intel.TelegramID = int64(ch.ID)
				intel.DisplayName = ch.Title
				intel.ChannelType = "group"
				intel.MemberCount = ch.ParticipantsCount
			}
		}

		return nil
	})

	return intel, err
}

// GetRecentMessages mengambil N pesan terbaru dari channel publik.
func (c *Client) GetRecentMessages(ctx context.Context, username string, limit int) ([]MessageIntel, error) {
	username = strings.TrimPrefix(username, "@")
	var messages []MessageIntel

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	err := c.withClient(ctx, func(ctx context.Context, api *tg.Client) error {
		res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
			Username: username,
		})
		if err != nil {
			return err
		}

		var inputPeer tg.InputPeerClass
		for _, peer := range res.Chats {
			if ch, ok := peer.(*tg.Channel); ok {
				inputPeer = &tg.InputPeerChannel{
					ChannelID:  ch.ID,
					AccessHash: ch.AccessHash,
				}
				break
			}
		}
		// Hanya channel/grup publik — akun pribadi & bot tidak dibaca (itu membuka percakapan)
		if inputPeer == nil {
			return ErrNotChannel
		}

		history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  inputPeer,
			Limit: limit,
		})
		if err != nil {
			return err
		}

		switch h := history.(type) {
		case *tg.MessagesChannelMessages:
			for _, m := range h.Messages {
				msg, ok := m.(*tg.Message)
				if !ok || (msg.Message == "" && msg.ReplyMarkup == nil) {
					continue
				}
				messages = append(messages, MessageIntel{
					MessageID: int64(msg.ID),
					Text:      msg.Message,
					Date:      time.Unix(int64(msg.Date), 0),
					Links:     messageLinks(msg),
				})
			}
		case *tg.MessagesMessages:
			for _, m := range h.Messages {
				msg, ok := m.(*tg.Message)
				if !ok || (msg.Message == "" && msg.ReplyMarkup == nil) {
					continue
				}
				messages = append(messages, MessageIntel{
					MessageID: int64(msg.ID),
					Text:      msg.Message,
					Date:      time.Unix(int64(msg.Date), 0),
					Links:     messageLinks(msg),
				})
			}
		}

		return nil
	})

	return messages, err
}

// BotMessage: satu pesan dalam percakapan dengan bot (Out = dikirim oleh akun kita).
type BotMessage struct {
	ID      int       `json:"id"`
	Out     bool      `json:"out"`
	Text    string    `json:"text"`
	Date    time.Time `json:"date"`
	Buttons []string  `json:"buttons,omitempty"`
}

// botPeer me-resolve username bot menjadi InputPeer.
func botPeer(ctx context.Context, api *tg.Client, username string) (*tg.InputPeerUser, error) {
	res, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: strings.TrimPrefix(username, "@")})
	if err != nil {
		return nil, err
	}
	for _, p := range res.Users {
		if u, ok := p.(*tg.User); ok {
			return &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash}, nil
		}
	}
	return nil, ErrNotChannel
}

func toBotMessages(h tg.MessagesMessagesClass) []BotMessage {
	var raw []tg.MessageClass
	switch v := h.(type) {
	case *tg.MessagesMessages:
		raw = v.Messages
	case *tg.MessagesMessagesSlice:
		raw = v.Messages
	}
	var out []BotMessage
	for _, m := range raw {
		msg, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		bm := BotMessage{ID: msg.ID, Out: msg.Out, Text: msg.Message, Date: time.Unix(int64(msg.Date), 0)}
		if rm, ok := msg.ReplyMarkup.(*tg.ReplyInlineMarkup); ok {
			for _, row := range rm.Rows {
				for _, b := range row.Buttons {
					bm.Buttons = append(bm.Buttons, fmt.Sprintf("%T:%s", b, buttonText(b)))
				}
			}
		}
		out = append(out, bm)
	}
	return out
}

func buttonText(b tg.KeyboardButtonClass) string {
	if t, ok := b.(interface{ GetText() string }); ok {
		return t.GetText()
	}
	return ""
}

// BotHistory membaca riwayat percakapan akun ini dengan bot (read-only, tidak mengirim apa pun).
func (c *Client) BotHistory(ctx context.Context, bot string, limit int) ([]BotMessage, error) {
	var out []BotMessage
	err := c.withClient(ctx, func(ctx context.Context, api *tg.Client) error {
		peer, err := botPeer(ctx, api, bot)
		if err != nil {
			return err
		}
		h, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{Peer: peer, Limit: limit})
		if err != nil {
			return err
		}
		out = toBotMessages(h)
		return nil
	})
	return out, err
}

// AskBot mengirim satu pesan ke bot lalu menunggu balasan (maks. timeout). Mengembalikan pesan bot
// yang datang setelah pesan kita. Balasan yang masih "loading" ditunggu hingga teksnya stabil.
func (c *Client) AskBot(ctx context.Context, bot, text string, timeout time.Duration) ([]BotMessage, error) {
	var out []BotMessage
	err := c.withClient(ctx, func(ctx context.Context, api *tg.Client) error {
		peer, err := botPeer(ctx, api, bot)
		if err != nil {
			return err
		}
		upd, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
			Peer: peer, Message: text, RandomID: time.Now().UnixNano(),
		})
		if err != nil {
			return err
		}
		sentID := sentMessageID(upd)
		deadline := time.Now().Add(timeout)
		last, stable := "", 0
		for time.Now().Before(deadline) {
			time.Sleep(3 * time.Second)
			h, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{Peer: peer, Limit: 10})
			if err != nil {
				return err
			}
			var replies []BotMessage
			for _, m := range toBotMessages(h) {
				if !m.Out && m.ID > sentID {
					replies = append([]BotMessage{m}, replies...)
				}
			}
			if len(replies) == 0 {
				continue
			}
			sig := ""
			for _, r := range replies {
				sig += r.Text
			}
			if sig == last {
				stable++
			} else {
				last, stable = sig, 0
			}
			out = replies
			if stable >= 1 { // teks tak berubah dua kali polling berturut-turut
				return nil
			}
		}
		if len(out) == 0 {
			return fmt.Errorf("bot tidak membalas dalam %s", timeout)
		}
		return nil
	})
	return out, err
}

func sentMessageID(u tg.UpdatesClass) int {
	switch v := u.(type) {
	case *tg.UpdateShortSentMessage:
		return v.ID
	case *tg.Updates:
		for _, x := range v.Updates {
			if m, ok := x.(*tg.UpdateMessageID); ok {
				return m.ID
			}
		}
	}
	return 0
}
