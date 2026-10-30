package main

import (
	"jutec-go/internal/browser"
	"context"
	"log"

	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jutec-go/internal/api/router"
	"jutec-go/internal/config"
	"jutec-go/internal/discovery"
	"jutec-go/internal/service"
	"jutec-go/internal/store"
	"jutec-go/internal/urlscan"
	"jutec-go/internal/virustotal"
	"jutec-go/internal/worker"
	"jutec-go/internal/worker/tasks"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := config.LoadConfig()
	gin.SetMode(cfg.GinMode)

	// Neo4j (optional)
	var neo4jDB *store.Neo4jDB
	if cfg.Neo4jURI != "" {
		var err error
		neo4jDB, err = store.NewNeo4jConnection(cfg.Neo4jURI, cfg.Neo4jUsr, cfg.Neo4jPwd)
		if err != nil {
			log.Printf("Warning: Failed to connect to Neo4j: %v", err)
		} else {
			defer neo4jDB.Close(context.Background())
		}
	}

	// Postgres (optional)
	var postgresDB *store.PostgresDB
	if cfg.PostgresURL != "" {
		var err error
		postgresDB, err = store.NewPostgresConnection(cfg.PostgresURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to Postgres: %v", err)
		} else {
			defer postgresDB.Close()
		}
	} else {
		log.Println("Postgres not configured, skipping connection")
	}

	// Sisa Chrome headless & folder profil dari proses sebelumnya (restart/crash) — cegah disk penuh
	browser.CleanupStale()

	// Kunci API yang diatur dari halaman Pengaturan menimpa .env
	if postgresDB != nil {
		if n := postgresDB.ApplyAPIKeys(context.Background()); n > 0 {
			log.Printf("🔑 %d kunci API dimuat dari Pengaturan", n)
			cfg = config.LoadConfig()
		}
	}

	// Background worker (Redis-backed)
	var taskClient *asynq.Client
	if cfg.RedisURL != "" {
		w := worker.NewWorker(cfg.RedisURL)
		taskClient = w.Client
		scanSvc := service.NewScanService(neo4jDB, postgresDB, w.Client)
		ocrSvc := service.NewOCRService()

		mux := asynq.NewServeMux()
		usCli := urlscan.NewClient(cfg.URLScanAPIKey)
		vtCli := virustotal.NewClient(cfg.VirusTotalAPIKey)

		mux.HandleFunc(tasks.TypeScanURL,     tasks.NewScanURLHandler(scanSvc).ProcessTask)
		mux.HandleFunc(tasks.TypeOCRImage,    tasks.NewOCRImageHandler(ocrSvc, postgresDB).ProcessTask)
		mux.HandleFunc(tasks.TypeEnrichSite,  tasks.NewEnrichHandler(usCli, vtCli, postgresDB).ProcessTask)
		mux.HandleFunc(tasks.TypeRescanStale, tasks.NewRescanStaleHandler(scanSvc, postgresDB).ProcessTask)
		mirrorWatch := &tasks.MirrorWatchHandler{PG: postgresDB, CT: discovery.NewCertScraper(), Client: w.Client}
		mux.HandleFunc(tasks.TypeMirrorWatch, mirrorWatch.ProcessTask)
		tgSvc := service.NewTelegramService(cfg.TelegramAPIID, cfg.TelegramAPIHash, postgresDB)
		mux.HandleFunc(tasks.TypeTelegramResolve, (&tasks.TelegramResolveHandler{Svc: tgSvc}).ProcessTask)
		mux.HandleFunc(tasks.TypeTelegramScrape, (&tasks.TelegramScrapeHandler{Svc: tgSvc, PG: postgresDB, Client: w.Client}).ProcessTask)
		vfl := &tasks.VflHandler{Svc: service.NewVflService(cfg.TelegramAPIID, cfg.TelegramAPIHash, postgresDB)}
		mux.HandleFunc(tasks.TypeVflChain, vfl.ProcessChain)
		mux.HandleFunc(tasks.TypeVflPending, vfl.ProcessPending)
		auto := &tasks.AutoHandler{PG: postgresDB, Client: w.Client, URLScanKey: cfg.URLScanAPIKey}
		mux.HandleFunc(tasks.TypeAutoEvidence, auto.Evidence)
		mux.HandleFunc(tasks.TypeAutoReverse, auto.Reverse)
		mux.HandleFunc(tasks.TypeAutoCrypto, auto.Crypto)
		mux.HandleFunc(tasks.TypeAutoTrustPositif, auto.TrustPositif)
		mux.HandleFunc(tasks.TypeAutoCommonCrawl, auto.CommonCrawl)
		mux.HandleFunc(tasks.TypeAutoCluster, auto.Cluster)
		mux.HandleFunc(tasks.TypeAutoOrigin, auto.Origin)
		mux.HandleFunc(tasks.TypeAutoWayback, auto.Wayback)
		mux.HandleFunc(tasks.TypeAutoReach, auto.Reach)
		mux.HandleFunc(tasks.TypeEvidenceCapture, (&tasks.EvidenceHandler{PG: postgresDB, BaseDir: cfg.EvidenceDir}).ProcessTask)
		mux.HandleFunc(tasks.TypeDeepSite,    (&tasks.DeepSiteHandler{PG: postgresDB, Client: w.Client}).ProcessTask)

		go w.Start(mux)
		log.Printf("Background worker started (Redis: %s)", cfg.RedisURL)

		// Scheduled jobs — re-scan stale sites setiap 6 jam
		redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisURL}
		scheduler := asynq.NewScheduler(redisOpt, nil)
		staleTask := asynq.NewTask(tasks.TypeRescanStale, nil)
		if _, err := scheduler.Register("@every 6h", staleTask); err != nil {
			log.Printf("Warning: Failed to register rescan scheduler: %v", err)
		}
		// Pantau domain mirror brand judol (CT Logs) & resolve Telegram tertunda
		if _, err := scheduler.Register("@every 12h", asynq.NewTask(tasks.TypeMirrorWatch, nil), asynq.Queue("low")); err != nil {
			log.Printf("Warning: Failed to register mirror watch: %v", err)
		}
		if _, err := scheduler.Register("@every 2h", asynq.NewTask(tasks.TypeTelegramResolve, nil), asynq.Queue("low")); err != nil {
			log.Printf("Warning: Failed to register telegram resolve: %v", err)
		}
		if _, err := scheduler.Register("@every 1h", asynq.NewTask(tasks.TypeVflPending, nil), asynq.Queue("enrichment")); err != nil {
			log.Printf("Warning: Failed to register vfl pending: %v", err)
		}
		for _, j := range []struct{ spec, typ string }{
			{"@every 30m", tasks.TypeAutoEvidence}, {"@every 1h", tasks.TypeAutoCluster},
			{"@every 1h", tasks.TypeAutoOrigin}, {"@every 1h", tasks.TypeAutoWayback}, {"@every 2h", tasks.TypeAutoReach},
			{"@every 1h", tasks.TypeAutoReverse}, {"@every 3h", tasks.TypeAutoCrypto},
			{"@every 12h", tasks.TypeAutoTrustPositif}, {"@every 2h", tasks.TypeAutoCommonCrawl},
		} {
			if _, err := scheduler.Register(j.spec, asynq.NewTask(j.typ, nil), asynq.Queue("low"), asynq.Unique(25*time.Minute)); err != nil {
				log.Printf("Warning: Failed to register %s: %v", j.typ, err)
			}
		}
		if _, err := scheduler.Register("@every 6h", asynq.NewTask(tasks.TypeTelegramScrape, nil), asynq.Queue("low")); err != nil {
			log.Printf("Warning: Failed to register telegram scrape: %v", err)
		}
		go func() {
			if err := scheduler.Run(); err != nil {
				log.Printf("Warning: Scheduler stopped: %v", err)
			}
		}()
		log.Println("Scheduler started — rescan 6h · mirror 12h · telegram resolve 2h · telegram baca 6h · VflOSINT 1h · bukti 30m · klaster/IP asli/Wayback 1h")
	}

	// HTTP server
	r := router.SetupRouter(neo4jDB, postgresDB, cfg.SerperAPIKey, cfg.SerperPages, cfg.LeakOSINTToken, cfg.TelegramAPIID, cfg.TelegramAPIHash, cfg.URLScanAPIKey, cfg.VirusTotalAPIKey, cfg.SerpApiKey, taskClient, cfg.EvidenceDir)

	srv := &http.Server{
		Addr:    cfg.Host + ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
