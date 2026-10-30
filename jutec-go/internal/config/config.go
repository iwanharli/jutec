package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Config struct {
	Host         string // HOST: alamat bind, default 127.0.0.1 (hanya lokal)
	Port         string
	GinMode      string
	Neo4jURI     string
	Neo4jUsr     string
	Neo4jPwd     string
	PostgresURL  string
	RedisURL        string
	SerperAPIKey    string
	SerpApiKey      string // serpapi.com — fallback saat Serper limit
	SerperPages     int
	LeakOSINTToken  string
	URLScanAPIKey    string
	VirusTotalAPIKey string
	TelegramAPIID    int
	TelegramAPIHash  string
	EvidenceDir      string // folder bukti forensik (tulis-sekali), default ~/.jutec/evidence
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func LoadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mode := os.Getenv("GIN_MODE")
	if mode == "" {
		mode = gin.DebugMode
	}

	serperPages, _ := strconv.Atoi(getEnv("SERPER_PAGES", "5"))
	tgAPIID, _     := strconv.Atoi(getEnv("TELEGRAM_API_ID", "0"))
	if serperPages <= 0 {
		serperPages = 5
	}

	return &Config{
		Host:           getEnv("HOST", "127.0.0.1"),
		Port:           port,
		GinMode:        mode,
		Neo4jURI:       os.Getenv("NEO4J_URI"),
		Neo4jUsr:       os.Getenv("NEO4J_USER"),
		Neo4jPwd:       os.Getenv("NEO4J_PASSWORD"),
		PostgresURL:    os.Getenv("POSTGRES_URL"),
		RedisURL:       getEnv("REDIS_URL", "localhost:6379"),
		SerperAPIKey:   getEnv("SERPER_API_KEY", ""),
		SerpApiKey:     getEnv("SERPAPI_API_KEY", ""),
		SerperPages:    serperPages,
		LeakOSINTToken:   getEnv("LEAKOSINT_TOKEN", ""),
		URLScanAPIKey:    getEnv("URLSCAN_API_KEY", ""),
		VirusTotalAPIKey: getEnv("VIRUSTOTAL_API_KEY", ""),
		TelegramAPIID:    tgAPIID,
		TelegramAPIHash:  getEnv("TELEGRAM_API_HASH", ""),
		EvidenceDir:      getEnv("EVIDENCE_DIR", defaultEvidenceDir()),
	}
}

func defaultEvidenceDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".jutec", "evidence")
	}
	return "evidence"
}
