// tglogin: login Telegram SEKALI secara interaktif agar engine bisa me-resolve username/channel.
// Kode login dikirim Telegram ke aplikasi pemilik nomor TELEGRAM_PHONE; sesi disimpan di
// ~/.jutec/tg_session/session.json. Server & worker hanya MEMAKAI sesi ini, tak pernah login sendiri.
//
//	go run cmd/tglogin/main.go           # login
//	go run cmd/tglogin/main.go -status   # cek apakah sesi masih aktif (tanpa mengirim kode)
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"jutec-go/internal/config"
	"jutec-go/internal/telegram"

	"github.com/joho/godotenv"
)

func main() {
	status := flag.Bool("status", false, "cek status sesi saja")
	flag.Parse()
	godotenv.Load()
	cfg := config.LoadConfig()
	c := telegram.NewClient(cfg.TelegramAPIID, cfg.TelegramAPIHash)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if *status {
		ok, err := c.Status(ctx)
		if err != nil {
			fmt.Println("Gagal cek status:", err)
			os.Exit(1)
		}
		if ok {
			fmt.Println("✅ Sesi Telegram aktif")
		} else {
			fmt.Println("❌ Sesi Telegram belum login — jalankan tanpa -status untuk login")
		}
		return
	}
	who, err := c.Login(ctx)
	if err != nil {
		fmt.Println("Login gagal:", err)
		os.Exit(1)
	}
	fmt.Println("✅ Login berhasil sebagai", who)
}
