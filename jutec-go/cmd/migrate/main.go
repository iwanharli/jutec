package main

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	dbURL := os.Getenv("POSTGRES_URL")
	if dbURL == "" {
		log.Fatal("POSTGRES_URL environment variable is required")
	}

	// golang-migrate pgx/v5 driver uses pgx5:// scheme
	dbURL = strings.Replace(dbURL, "postgres://", "pgx5://", 1)

	action := "up"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}

	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		log.Fatalf("Failed to initialize migrator: %v", err)
	}
	defer m.Close()

	switch action {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("Migration up failed: %v", err)
		}
		log.Println("Migration up: done")

	case "down":
		if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("Migration down failed: %v", err)
		}
		log.Println("Migration down 1 step: done")

	case "drop":
		log.Println("WARNING: This will drop all tables. Are you sure? (Ctrl+C to cancel, Enter to continue)")
		os.Stdin.Read(make([]byte, 1))
		if err := m.Drop(); err != nil {
			log.Fatalf("Migration drop failed: %v", err)
		}
		log.Println("Migration drop: done")

	case "version":
		v, dirty, err := m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			log.Println("Version: none (no migrations have been run)")
			return
		}
		if err != nil {
			log.Fatalf("Failed to get version: %v", err)
		}
		log.Printf("Version: %d | Dirty: %v", v, dirty)

	default:
		log.Fatalf("Unknown action: %q\nUsage: go run cmd/migrate/main.go [up|down|version|drop]", action)
	}
}
