package router

import (
	"log"
	"os"
	"time"

	"jutec-go/internal/api/handlers"
	"jutec-go/internal/worker/tasks"
	"jutec-go/internal/discovery"
	"jutec-go/internal/api/middleware"
	"jutec-go/internal/dashboard"
	"jutec-go/internal/hackertarget"
	"jutec-go/internal/leakosint"
	"jutec-go/internal/tron"
	"jutec-go/internal/service"
	"jutec-go/internal/store"
	"jutec-go/internal/urlscan"
	"jutec-go/internal/virustotal"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

func errorOnlyLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		if status >= 400 {
			log.Printf("[GIN] %d | %s | %s %s | %s",
				status, time.Since(start), c.Request.Method, c.Request.URL.Path, c.Errors.String())
		}
	}
}

func SetupRouter(neo4j *store.Neo4jDB, pg *store.PostgresDB, serperKey string, serperPages int, leakosintToken string, tgApiID int, tgApiHash string, urlscanKey string, vtKey string, serpApiKey string, taskClient *asynq.Client, evidenceDir string) *gin.Engine {
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.Use(gin.Recovery())
	r.Use(errorOnlyLogger())
	r.Use(middleware.CORS())
	r.Use(middleware.Audit(pg, handlers.ActorOf))

	// Services (taskClient nil di router — worker di-setup di main.go)
	leakClient      := leakosint.NewClient(leakosintToken)
	urlscanClient   := urlscan.NewClient(urlscanKey)
	vtClient        := virustotal.NewClient(vtKey)
	// taskClient dipakai untuk mengantrekan enrichment URLScan/VirusTotal setelah tiap scan
	scanSvc         := service.NewScanService(neo4j, pg, taskClient)
	discoverySvc    := service.NewDiscoveryService(serperKey, serperPages, scanSvc, pg)
	discoverySvc.Scraper.SerpApiKey = serpApiKey // fallback saat Serper habis kredit
	tgSvc           := service.NewTelegramService(tgApiID, tgApiHash, pg)
	autoEnricher    := service.NewAutoEnricher(pg, leakClient, tgSvc, urlscanClient, vtClient)

	// Handlers
	healthHandler    := handlers.NewHealthHandler()
	tgHandler        := handlers.NewTelegramHandler(tgSvc, pg)
	settingsHandler  := handlers.NewSettingsHandler(pg)
	graphHandler     := handlers.NewGraphHandler(pg)
	sweepJobHandler  := handlers.NewSweepJobHandler(discoverySvc, pg)
	scanHandler      := handlers.NewScanHandler(scanSvc)
	intelHandler     := handlers.NewIntelligenceHandler(neo4j, pg)
	discoveryHandler := handlers.NewDiscoveryHandler(discoverySvc, pg)
	discoveryHandler.Enricher = autoEnricher
	sweepJobHandler.Enricher  = autoEnricher
	dataHandler      := handlers.NewDataHandler(pg)
	netHandler       := handlers.NewNetworkHandler(pg)
	artHandler       := handlers.NewArtifactHandler(pg)
	kwHandler        := handlers.NewKeywordHandler(pg)
	blHandler        := handlers.NewBlacklistHandler(pg)
	operatorHandler  := handlers.NewOperatorHandler(pg)
	whoisHandler     := handlers.NewWHOISHandler(pg)
	leakHandler      := handlers.NewLeakHandler(pg, leakClient)
	siteHandler      := handlers.NewSiteHandler(pg)
	siteHandler.Scan = scanSvc
	siteHandler.Tasks = taskClient
	if taskClient != nil {
		siteHandler.Mirror = &tasks.MirrorWatchHandler{PG: pg, CT: discovery.NewCertScraper(), Client: taskClient}
	}

	// Streaming (WebSocket + SSE)
	streamHandler := handlers.NewStreamHandler(scanSvc)
	r.GET("/ws/analyze", streamHandler.AnalyzeWS)
	r.GET("/sse/analyze", streamHandler.AnalyzeSSE)

	// Health
	r.GET("/", healthHandler.Check)
	r.GET("/health", healthHandler.Ping)

	// Data
	overviewHandler := &handlers.OverviewHandler{PG: pg}
	if taskClient != nil {
		redisAddr := os.Getenv("REDIS_URL")
		if redisAddr == "" {
			redisAddr = "localhost:6379"
		}
		overviewHandler.Inspector = asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr})
	}
	data := r.Group("/data")
	{
		data.GET("/summary", dataHandler.GetSummary)
		data.GET("/overview", overviewHandler.Get)
		data.GET("/register-forms", (&handlers.RegisterFormsHandler{PG: pg}).List)
		data.GET("/sites", dataHandler.GetSites)
		data.GET("/apps", dataHandler.GetApps)
		data.GET("/artifacts", artHandler.TopShared)
		data.GET("/artifacts/filters", artHandler.GetFilters)
		data.GET("/pending-summary", dataHandler.PendingSummary)
		data.GET("/tracking", dataHandler.GetTracking)
		data.GET("/operators", dataHandler.GetOperators)
		data.GET("/links", dataHandler.GetLinks)
		data.GET("/telegram", dataHandler.GetTelegram)
		data.GET("/kominfo", dataHandler.GetKomInfo)
		data.GET("/dukcapil", dataHandler.GetDukcapil)
	}

	// Scan
	r.GET("/analyze", scanHandler.AnalyzeSite)
	r.POST("/analyze/bulk", scanHandler.AnalyzeBulk)

	// Intelligence
	intel := r.Group("/intelligence")
	{
		intel.GET("/reverse-lookup", intelHandler.ReverseLookup)
		intel.GET("/network-summary", intelHandler.GetNetworkSummary)
		intel.GET("/lookup", intelHandler.Lookup)
	}

	// Discovery
	disc := r.Group("/discovery")
	{
		disc.GET("/google", discoveryHandler.DiscoverGoogle)
		disc.GET("/certs", discoveryHandler.DiscoverCerts)
		disc.POST("/sweep", discoveryHandler.Sweep)
		disc.POST("/deep-sweep", discoveryHandler.DeepSweep)
		disc.GET("/deep-sweep",  discoveryHandler.DeepSweep)
		disc.GET("/cache/stats", discoveryHandler.CacheStats)
		disc.DELETE("/cache",    discoveryHandler.CachePurge)
	}

	// Network Intelligence
	net := r.Group("/network")
	{
		net.GET("/tracking/lookup", netHandler.TrackingLookup)
		net.GET("/tracking/top", netHandler.TrackingTop)
		net.GET("/ip/lookup", netHandler.IPLookup)
		net.GET("/ip/subnet", netHandler.SubnetLookup)
		net.GET("/ip/top", netHandler.SubnetTop)
		net.GET("/template/similar", netHandler.TemplateSimilar)
		net.GET("/ns/lookup", netHandler.NSLookup)
		net.GET("/ns/top", netHandler.NSTop)
		net.GET("/ssl/lookup", netHandler.SSLLookup)
		net.GET("/ssl/top", netHandler.SSLTop)
		net.GET("/txt/lookup", netHandler.TXTLookup)
		net.GET("/txt/top", netHandler.TXTTop)
	}

	// Artifacts
	art := r.Group("/artifacts")
	{
		art.GET("/lookup", artHandler.LookupByValue)
		art.GET("/top", artHandler.TopShared)
	}

	// Keywords
	kw := r.Group("/keywords")
	{
		kw.GET("/detection", kwHandler.ListDetection)
		kw.GET("/detection/stats", kwHandler.DetectionStats)
		kw.POST("/detection", kwHandler.CreateDetection)
		kw.PUT("/detection/:id", kwHandler.UpdateDetection)
		kw.DELETE("/detection/:id", kwHandler.DeleteDetection)

		kw.GET("/search", kwHandler.ListSearch)
		kw.POST("/search", kwHandler.CreateSearch)
		kw.PUT("/search/:id", kwHandler.UpdateSearch)
		kw.DELETE("/search/:id", kwHandler.DeleteSearch)
	}

	// Domain Blacklist
	bl := r.Group("/blacklist")
	{
		bl.GET("", blHandler.List)
		bl.POST("", blHandler.Create)
		bl.PUT("/:id", blHandler.Update)
		bl.DELETE("/:id", blHandler.Delete)
	}

	// Operator Profiling
	ops := r.Group("/operators")
	{
		ops.GET("", operatorHandler.List)
		ops.GET("/:id", operatorHandler.GetByID)
		ops.POST("/discover", operatorHandler.Discover)
	}

	// Settings
	r.GET("/settings", settingsHandler.List)
	r.GET("/settings/api-keys", settingsHandler.APIKeys)
	r.PUT("/settings/api-keys/:env", settingsHandler.SetAPIKey)
	r.POST("/settings/apply", settingsHandler.Apply)
	r.PATCH("/settings/:key", settingsHandler.Update)

	// Network Graph
	r.GET("/network/graph", graphHandler.GetGraph)

	// Detail satu situs + pohon keterkaitan
	verifyHandler := &handlers.VerifyHandler{PG: pg}
	r.GET("/verification/queue", verifyHandler.Queue)
	ipHandler := &handlers.IPHandler{PG: pg, URLScanKey: urlscanKey}
	r.GET("/ips/:ip", ipHandler.Get)
	evidenceHandler := &handlers.EvidenceHandler{PG: pg, Tasks: taskClient, BaseDir: evidenceDir}
	extIntel := &handlers.ExternalIntelHandler{PG: pg, Tasks: taskClient,
		HT: hackertarget.NewClient(os.Getenv("HACKERTARGET_API_KEY")), Tron: tron.NewClient(os.Getenv("TRONGRID_API_KEY"))}
	sitesGrp := r.Group("/sites")
	{
		sitesGrp.GET("/lookup", siteHandler.Lookup)
		sitesGrp.GET("/:id", siteHandler.Detail)
		sitesGrp.GET("/:id/relations", siteHandler.Relations)
		sitesGrp.GET("/:id/identities", siteHandler.Identities)
		sitesGrp.POST("/:id/enrich", siteHandler.Enrich)
		sitesGrp.GET("/:id/changes", siteHandler.Changes)
		sitesGrp.GET("/:id/apk", siteHandler.APK)
		sitesGrp.POST("/:id/deep", siteHandler.Deep)
		sitesGrp.GET("/:id/wayback", siteHandler.Wayback)
		sitesGrp.POST("/:id/wayback", siteHandler.FetchWayback)
		sitesGrp.POST("/:id/mirrors", siteHandler.Mirrors)
		sitesGrp.GET("/:id/telegram", siteHandler.Telegram)
		sitesGrp.POST("/:id/verify", verifyHandler.Verify)
		sitesGrp.GET("/:id/verifications", verifyHandler.History)
		sitesGrp.GET("/:id/reachability", verifyHandler.Reach)
		sitesGrp.GET("/:id/infra", ipHandler.SiteInfra)
		sitesGrp.POST("/:id/origin", ipHandler.FindOrigin)
		sitesGrp.POST("/:id/evidence", evidenceHandler.Request)
		sitesGrp.GET("/:id/evidence", evidenceHandler.List)
		sitesGrp.GET("/:id/reverse-analytics", extIntel.ReverseAnalytics)
		sitesGrp.POST("/:id/reverse-analytics", extIntel.RefreshReverseAnalytics)
		sitesGrp.GET("/:id/crypto", extIntel.Crypto)
	}

	// Bukti forensik & log audit
	ev := r.Group("/evidence")
	{
		ev.GET("/:id/verify", evidenceHandler.Verify)
		ev.GET("/:id/files/:name", evidenceHandler.File)
		ev.GET("/:id/zip", evidenceHandler.Zip)
	}
	auditHandler := &handlers.AuditHandler{PG: pg}
	r.GET("/audit", auditHandler.List)
	r.GET("/audit/verify", auditHandler.Verify)

	// Sweep Jobs (pause/resume)
	maint := &handlers.MaintenanceHandler{PG: pg, Tasks: taskClient}
	mt := r.Group("/maintenance")
	{
		mt.GET("/summary", maint.Summary)
		mt.GET("/artifacts/fake", maint.FakeArtifacts)
		mt.POST("/artifacts/clean", maint.CleanArtifacts)
		mt.POST("/rescan", maint.StartRescan)
		mt.GET("/rescan", maint.RescanStatus)
		mt.POST("/cache/clear", maint.ClearCache)
	}

	sweep := r.Group("/sweep")
	{
		sweep.POST("/start",       sweepJobHandler.Start)
		sweep.GET("/active",       sweepJobHandler.Active)
		sweep.GET("/:id",          sweepJobHandler.Status)
		sweep.POST("/:id/pause",   sweepJobHandler.Pause)
		sweep.POST("/:id/resume",  sweepJobHandler.Resume)
		sweep.POST("/:id/cancel",  sweepJobHandler.Cancel)
	}

	// WHOIS / Reverse WHOIS
	r.GET("/whois/reverse", whoisHandler.Reverse)

	// Telegram Intel
	tg := r.Group("/telegram")
	{
		tg.POST("/resolve",         tgHandler.Resolve)
		tg.POST("/scrape",          tgHandler.Scrape)
		tg.POST("/resolve-pending", tgHandler.ResolvePending)
		tg.GET("/pending-count",   tgHandler.PendingCount)
		tg.GET("/channels",         tgHandler.List)
		tg.GET("/messages",         tgHandler.Messages)
	}

	// LeakOSINT
	leak := r.Group("/leak")
	{
		leak.GET("/phone", leakHandler.LookupPhone)
		leak.GET("/nik", leakHandler.LookupNIK)
		leak.POST("/enrich",        leakHandler.EnrichAll)
		leak.GET("/pending-count", leakHandler.PendingCount)
		vflHandler := &handlers.VflHandler{PG: pg, Tasks: taskClient}
		leak.POST("/vfl",        vflHandler.Check)
		leak.GET("/vfl/chain",   vflHandler.Chain)
		leak.GET("/vfl/status",  vflHandler.Status)
	}

	dashboard.RegisterRoutes(r)

	return r
}
