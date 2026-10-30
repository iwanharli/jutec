package telegram

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

type Client struct {
	apiID   int
	apiHash string
	phone   string
}

func NewClient(apiID int, apiHash string) *Client {
	return &Client{
		apiID:   apiID,
		apiHash: apiHash,
		phone:   os.Getenv("TELEGRAM_PHONE"),
	}
}

func sessionPath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".jutec", "tg_session")
	os.MkdirAll(dir, 0700)
	return dir
}

// ErrNotLoggedIn: sesi Telegram belum/tidak lagi login. Server & worker TIDAK PERNAH memulai login
// sendiri — dulu hal ini mengirim kode login ke HP pemilik nomor setiap kali dipanggil.
var ErrNotLoggedIn = errors.New("sesi Telegram belum login — jalankan: go run cmd/tglogin/main.go")

func (c *Client) newTG() *telegram.Client {
	return telegram.NewClient(c.apiID, c.apiHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: filepath.Join(sessionPath(), "session.json")},
	})
}

// withClient menjalankan fn hanya bila sesi tersimpan masih login. Tidak pernah meminta kode login.
func (c *Client) withClient(ctx context.Context, fn func(ctx context.Context, api *tg.Client) error) error {
	client := c.newTG()
	return client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("telegram status: %w", err)
		}
		if !status.Authorized {
			return ErrNotLoggedIn
		}
		return fn(ctx, client.API())
	})
}

// Login menjalankan alur login interaktif (kode dikirim ke aplikasi Telegram pemilik nomor).
// Hanya dipanggil dari perintah terminal cmd/tglogin — bukan dari server atau worker.
func (c *Client) Login(ctx context.Context) (string, error) {
	phone := c.phone
	if phone == "" {
		phone = readLine("Nomor Telegram (mis. +628123456789): ")
	}
	userAuth := auth.Constant(phone, os.Getenv("TELEGRAM_PASSWORD"),
		auth.CodeAuthenticatorFunc(func(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
			return readLine("Kode login (cek aplikasi Telegram): "), nil
		}),
	)
	client := c.newTG()
	var who string
	err := client.Run(ctx, func(ctx context.Context) error {
		if err := client.Auth().IfNecessary(ctx, auth.NewFlow(userAuth, auth.SendCodeOptions{})); err != nil {
			return err
		}
		self, err := client.Self(ctx)
		if err == nil {
			who = strings.TrimSpace(self.FirstName + " " + self.LastName + " @" + self.Username)
		}
		return err
	})
	return who, err
}

// Status melaporkan apakah sesi tersimpan masih login (tanpa memicu login).
func (c *Client) Status(ctx context.Context) (bool, error) {
	client := c.newTG()
	var ok bool
	err := client.Run(ctx, func(ctx context.Context) error {
		st, err := client.Auth().Status(ctx)
		ok = err == nil && st.Authorized
		return err
	})
	return ok, err
}

func readLine(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}
