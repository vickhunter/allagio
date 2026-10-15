package main

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"affilio-api/internal/config"
	"affilio-api/internal/database"
	"affilio-api/internal/dcp"
	"affilio-api/internal/entry"
	"affilio-api/internal/handlers"
	"affilio-api/internal/middleware"
	"affilio-api/internal/migrations"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

// localSession wires cookie sessions without OIDC so a localhost test can
// approve a device code. It refuses release mode and any non-loopback host.
func localSession(cfg config.Config) bool {
	if os.Getenv("OFFER_RAILS_CI_SESSION") != "1" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(cfg.GinMode), "release") {
		log.Fatal("OFFER_RAILS_CI_SESSION cannot be used when GIN_MODE=release")
	}
	host := strings.TrimSpace(cfg.Host)
	switch host {
	case "", "127.0.0.1", "localhost", "::1":
		return true
	default:
		log.Fatal("OFFER_RAILS_CI_SESSION only listens on localhost")
	}
	return false
}

func main() {
	cfg := config.Load()
	if err := cfg.ValidateSecrets(); err != nil {
		log.Fatalf("refusing to start: %v", err)
	}
	if err := cfg.ValidateRuntime(); err != nil {
		log.Fatalf("refusing to start: %v", err)
	}
	if err := cfg.ValidateClaims(); err != nil {
		log.Fatalf("refusing to start: %v", err)
	}
	ctx := context.Background()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()
	if cfg.AutoMigrate {
		// 038 reads stored faces as UTC. Do not SET the session to UTC here.
		if err := migrations.RequireUTCSession(ctx, db.Pool); err != nil {
			log.Fatalf("%v", err)
		}
		if err := migrations.Apply(ctx, db.Pool); err != nil {
			log.Fatalf("database migration failed: %v", err)
		}
	}

	audit := services.NewAuditLogger(db)
	defer audit.Close()

	r := gin.New()
	r.Use(gin.LoggerWithFormatter(middleware.AccessLogFormatter), gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		log.Print(middleware.ScrubLoggedIPs("request panic recovered"))
		c.AbortWithStatus(500)
	}))
	if err := middleware.ConfigureTrustedProxies(r, cfg.TrustedProxies); err != nil {
		log.Fatalf("failed to set trusted proxies: %v", err)
	}
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	if cfg.POCReportPath != "" {
		if cfg.Host != "127.0.0.1" {
			log.Fatal("POC report requires HOST=127.0.0.1")
		}
		r.GET("/pilot", func(c *gin.Context) { c.File(cfg.POCReportPath) })
	}
	health := func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "site_installation_registration": true, "native_distribution": cfg.PublicBaseURL != ""})
	}
	r.GET("/health", health)
	r.GET("/.well-known/dcp.json", func(c *gin.Context) {
		mcpURL, mcpTransport := "/v1/mcp", "legacy-http-jsonrpc"
		if cfg.PublicBaseURL != "" {
			mcpURL, mcpTransport = "/mcp", "streamable-http"
		}
		makerVerbs := make([]string, 0, len(dcp.Tools))
		for _, tool := range dcp.Tools {
			makerVerbs = append(makerVerbs, tool.Name)
		}
		publisherVerbs := make([]string, 0, len(dcp.PublisherTools))
		for _, tool := range append(dcp.PublisherTools, dcp.RevenueTools...) {
			publisherVerbs = append(publisherVerbs, tool.Name)
		}
		c.JSON(200, gin.H{
			"dcp_version": "0.1",
			"name":        "distributoor",
			"description": "Agent-native distribution network: makers publish offers, publishers earn commissions, agents do the operational work.",
			"http_base":   "/v1",
			"mcp": gin.H{
				"transport": mcpTransport,
				"url":       mcpURL,
				"stdio": gin.H{
					"command": "distributoor-mcp",
					"env":     []string{"DISTRIBUTOOR_API_URL", "DISTRIBUTOOR_API_KEY", "DISTRIBUTOOR_SITE_KEY"},
				},
			},
			"verbs": gin.H{
				"public":    []string{"preview_revenue"},
				"maker":     makerVerbs,
				"publisher": publisherVerbs,
			},
			"payout_methods": []string{"manual"},
		})
	})

	v1 := r.Group("/v1")

	sites := &handlers.SitesHandler{
		DB:              db,
		AdminAPIKey:     cfg.AdminAPIKey,
		RegisterLimiter: handlers.NewIPRateLimiter(8, time.Minute),
		Audit:           audit,
	}
	siteKeys := &handlers.SiteKeysHandler{DB: db, Audit: audit, GracePeriod: cfg.KeyRotationGracePeriod}
	classifier := &services.AIClassifier{
		APIKey:     cfg.ClaudeAPIKey,
		Model:      cfg.ClaudeModel,
		APIURL:     cfg.ClaudeAPIURL,
		APIVersion: cfg.ClaudeAPIVersion,
	}
	scan := &handlers.ScanHandler{DB: db, Classifier: classifier}
	articles := &handlers.ArticlesHandler{DB: db}
	recommendations := &handlers.RecommendationsHandler{DB: db}
	programs := &handlers.ProgramsHandler{DB: db}
	generator := &services.AIContentGenerator{
		APIKey:     cfg.ClaudeAPIKey,
		Model:      cfg.ClaudeModel,
		APIURL:     cfg.ClaudeAPIURL,
		APIVersion: cfg.ClaudeAPIVersion,
	}
	generate := &handlers.GenerateHandler{DB: db, Generator: generator}
	tracking := &handlers.TrackingHandler{DB: db}
	userPrograms := &handlers.UserProgramsHandler{DB: db}
	detect := &handlers.DetectHandler{Classifier: classifier}
	programDiscovery := &handlers.ProgramDiscoveryHandler{DB: db, Audit: audit}
	auditHandler := &handlers.AuditHandler{DB: db}
	makers := &handlers.MakersHandler{
		DB:              db,
		RegisterLimiter: handlers.NewIPRateLimiter(8, time.Minute),
	}
	products := &handlers.ProductsHandler{DB: db}
	offers := &handlers.OffersHandler{DB: db, ClaimsCheck: cfg.ClaimsCheck}
	offerReview := &handlers.OfferReviewHandler{DB: db, ClaimsCheck: cfg.ClaimsCheck}
	assessments := &handlers.AssessmentsHandler{
		DB: db,
		Engine: &services.ReadinessEngine{
			APIKey:     cfg.ClaudeAPIKey,
			Model:      cfg.ClaudeModel,
			APIURL:     cfg.ClaudeAPIURL,
			APIVersion: cfg.ClaudeAPIVersion,
		},
	}

	marketplace := &handlers.MarketplaceHandler{DB: db}
	placements := &handlers.PlacementsHandler{DB: db, RequireVerifiedConnection: cfg.PublicBaseURL != "", Claims: entry.StubClaimChecker{}}
	payouts := &handlers.PayoutsHandler{DB: db}
	redirects := &handlers.RedirectsHandler{
		DB:           db,
		RateLimiter:  handlers.NewIPRateLimiter(120, time.Minute),
		HomeURL:      "https://offerrails.com/",
		IPSaltSecret: cfg.APIKeySecret,
	}
	var connectionCipher *services.ConnectionCipher
	if cfg.ConnectionEncryptionKey != "" {
		connectionCipher, err = services.NewConnectionCipher(cfg.ConnectionEncryptionKey)
		if err != nil {
			log.Fatal(err)
		}
	}
	connections := &handlers.PaymentConnectionsHandler{DB: db, Cipher: connectionCipher}
	var oauthResources []string
	var hosted *handlers.HostedHandler
	if cfg.PublicBaseURL != "" {
		if err = handlers.ValidatePublicURL(cfg.PublicBaseURL); err != nil {
			log.Fatal(err)
		}
		cfg.PublicBaseURL = strings.TrimRight(cfg.PublicBaseURL, "/")
		redirects.HomeURL = cfg.PublicBaseURL + "/"
		assessments.Engine.PublicBaseURL = cfg.PublicBaseURL
		login, loginErr := services.NewOIDCLogin(ctx, cfg.OIDCIssuer, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.PublicBaseURL+"/auth/callback")
		if loginErr != nil {
			log.Fatalf("owner sign-in configuration failed: %v", loginErr)
		}
		hosted = &handlers.HostedHandler{DB: db, BaseURL: cfg.PublicBaseURL, LoopbackURL: "http://127.0.0.1:" + cfg.Port, Login: login, Limiter: handlers.NewIPRateLimiter(240, time.Minute), Payments: &handlers.MerchantConnectionsHandler{DB: db, Cipher: connectionCipher, Verifier: &services.StripeAPI{}, BaseURL: cfg.PublicBaseURL}, AssetsPath: cfg.UIAssetsPath, DownloadsPath: cfg.DownloadsPath, OperatorID: cfg.OperatorOwnerID, Assessments: assessments, ClaimsCheck: cfg.ClaimsCheck}
		if err = hosted.LoadWorkspace(); err != nil {
			log.Fatalf("workspace assets failed: %v", err)
		}
		hosted.Mount(r)
		oauthResources = append(oauthResources, cfg.PublicBaseURL+"/mcp")
	}
	webhooks := &handlers.WebhooksHandler{
		Cipher: connectionCipher,
		DB:     db,
		Secrets: map[string]string{
			"stripe":       cfg.StripeWebhookSecret,
			"lemonsqueezy": cfg.LemonSqueezyWebhookSecret,
			"polar":        cfg.PolarWebhookSecret,
		},
	}

	previewLimiter := handlers.NewIPRateLimiter(30, time.Minute)
	v1.POST("/revenue/preview", func(c *gin.Context) {
		if !previewLimiter.Allow(c.ClientIP()) {
			c.JSON(429, gin.H{"error": "preview rate limit exceeded"})
			return
		}
		placements.Find(c)
	})
	v1.POST("/sites/register", sites.Register)
	v1.GET("/health", health)
	v1.DELETE("/sites", sites.Delete)
	v1.POST("/makers/register", makers.Register)
	v1.GET("/go/:code", redirects.Go)
	v1.HEAD("/go/:code", redirects.Go)
	v1.POST("/webhooks/:provider", webhooks.Receive)
	v1.POST("/webhooks/:provider/:product_id", webhooks.Receive)
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	go webhooks.RunWorker(workerCtx)
	go handlers.PurgeClickSignals(workerCtx, db)
	mcp := &handlers.MCPHandler{
		LoopbackURL: "http://127.0.0.1:" + cfg.Port,
		RateLimiter: handlers.NewIPRateLimiter(240, time.Minute),
	}
	v1.POST("/mcp", mcp.Receive)

	agentEntry := &handlers.AgentEntry{
		DB:                 db,
		BaseURL:            strings.TrimRight(cfg.PublicBaseURL, "/"),
		Claims:             entry.StubClaimChecker{},
		AccountLimiter:     handlers.NewIPRateLimiter(8, time.Minute),
		DeviceLimiter:      handlers.NewIPRateLimiter(30, time.Minute),
		PluginOfflineAfter: cfg.PluginOfflineAfter,
		StatusLimiter:      handlers.NewPlacementPollLimiter(),
		ClaimsCheck:        cfg.ClaimsCheck,
	}
	if hosted != nil {
		agentEntry.Owner = hosted.RequireOwnerSession
	} else if localSession(cfg) {
		if cfg.Host == "" {
			cfg.Host = "127.0.0.1"
		}
		base := "http://" + net.JoinHostPort(cfg.Host, cfg.Port)
		agentEntry.BaseURL = base
		agentEntry.Owner = (&handlers.HostedHandler{DB: db, BaseURL: base}).RequireOwnerSession
		log.Printf("local session auth enabled for %s", base)
	}
	agentEntry.Mount(r)
	signals := v1.Group("/agent-signals")
	signals.Use(middleware.AdminKeyAuth(cfg.AdminAPIKey, audit))
	signals.GET("", agentEntry.AgentSignals)

	secured := v1.Group("/")
	secured.Use(middleware.APIKeyAuth(db, audit, oauthResources...))
	if hosted != nil {
		secured.GET("/sites/owner-connection", hosted.SiteConnectionStatus)
		secured.POST("/sites/owner-connection", hosted.StartSiteConnection)
	}
	{
		secured.POST("/scan/full", scan.ScanFull)
		secured.POST("/scan/single", scan.ScanSingle)
		secured.GET("/scan/status", scan.ScanStatus)

		secured.GET("/articles", articles.List)
		secured.GET("/articles/:id", articles.Get)

		secured.GET("/recommendations", recommendations.List)
		secured.GET("/recommendations/:id", recommendations.Get)
		secured.PATCH("/recommendations/:id", recommendations.Patch)

		secured.POST("/generate/content", generate.GenerateContent)
		secured.POST("/insert/confirm", generate.ConfirmInsert)

		secured.GET("/programs", programs.List)
		secured.GET("/programs/search", programs.Search)

		secured.POST("/detect/links", detect.DetectLinks)
		secured.POST("/user-programs/import", userPrograms.Import)
		secured.GET("/user-programs", userPrograms.List)
		secured.POST("/user-programs", userPrograms.Create)
		secured.DELETE("/user-programs/:id", userPrograms.Delete)

		secured.POST("/track/click", tracking.TrackClick)
		secured.GET("/track/articles", tracking.ArticleClickCounts)
		secured.GET("/track/recommendations", tracking.RecommendationClickCounts)
		secured.GET("/track/summary", tracking.Summary)
		secured.GET("/track/trends", tracking.Trends)
		secured.GET("/track/attribution", tracking.Attribution)
		secured.GET("/track/funnel", tracking.Funnel)
		secured.GET("/track/export", tracking.Export)
		secured.GET("/track/cohorts", tracking.Cohorts)
		secured.GET("/track/recommendation-quality", tracking.RecommendationQuality)
		secured.GET("/track/retention", tracking.Retention)
		secured.GET("/track/recommendation-tuning", tracking.RecommendationTuning)

		secured.POST("/revenue/opportunities", placements.Find)
		secured.POST("/placements", placements.Prepare)
		secured.GET("/placements", placements.List)
		secured.POST("/placements/:id/commit", placements.Commit)
		secured.POST("/placements/:id/receipt", placements.RecordReceipt)
		secured.POST("/placements/:id/refuse", placements.Refuse)
		secured.POST("/placements/:id/removed", placements.MarkRemoved)
		// Revoke lives on AgentEntry so a site owner or the offer's maker can call it.
		secured.GET("/marketplace/offers", marketplace.ListOffers)
		secured.POST("/marketplace/offers/:id/join", marketplace.JoinOffer)

		secured.GET("/payouts/balance", payouts.Balance)
		secured.GET("/payouts/history", payouts.History)
		secured.POST("/payouts/:id/confirm-received", payouts.ConfirmReceived)
	}

	maker := v1.Group("/maker")
	maker.Use(middleware.MakerKeyAuth(db, oauthResources...))
	maker.Use(middleware.LimitOfferScope())
	{
		maker.POST("/products", products.Create)
		maker.GET("/products", products.List)
		maker.GET("/products/:id", products.Get)
		maker.PATCH("/products/:id", products.Patch)
		maker.POST("/products/:id/assess", assessments.Assess)
		maker.GET("/products/:id/assessments", assessments.List)

		maker.POST("/offers", offers.Create)
		maker.GET("/offers", offers.List)
		maker.GET("/offers/:id", offers.Get)
		maker.PATCH("/offers/:id", offers.Patch)
		maker.POST("/offers/:id/submit", offers.Submit)
		maker.GET("/offers/:id/matches", offers.Matches)

		maker.GET("/earnings", payouts.MakerEarnings)
		reports := &handlers.RefundReportHandler{DB: db}
		maker.POST("/sales", reports.ReportSale)
		maker.POST("/refunds", reports.ReportRefund)
	}
	listings := v1.Group("/listings")
	listings.Use(middleware.MakerKeyAuth(db, oauthResources...))
	listings.Use(middleware.LimitOfferScope())
	listings.GET("", offers.Listings)

	admin := v1.Group("/program-discovery")
	admin.Use(middleware.AdminKeyAuth(cfg.AdminAPIKey, audit))
	{
		admin.GET("/queue", programDiscovery.List)
		admin.POST("/queue/:id/review", programDiscovery.Review)
		admin.POST("/queue/:id/promote", programDiscovery.Promote)
		admin.POST("/sites/rotate-key", siteKeys.Rotate)
		admin.GET("/audit/events", auditHandler.List)
		admin.GET("/audit/alerts", auditHandler.Alerts)
	}

	offerAdmin := v1.Group("/offer-review")
	offerAdmin.Use(middleware.AdminKeyAuth(cfg.AdminAPIKey, audit))
	{
		offerAdmin.GET("/queue", offerReview.Queue)
		offerAdmin.POST("/placements/:id/approve", placements.Approve)
		offerAdmin.POST("/agent-access/sites/:site_id", placements.CreateAgentAccess)
		offerAdmin.POST("/agent-access/:id/revoke", placements.RevokeAgentAccess)
		offerAdmin.POST("/products/:id/connection", connections.Create)
		offerAdmin.POST("/:id/approve", offerReview.Approve)
		offerAdmin.POST("/:id/reject", offerReview.Reject)
		offerAdmin.POST("/makers/rotate-key", makers.RotateKey)
		offerAdmin.POST("/makers/:id/trust", offerReview.SetMakerTrust)
		offerAdmin.GET("/flagged-conversions", offerReview.FlaggedConversions)
		offerAdmin.POST("/conversions/:id/clear-flag", offerReview.ClearConversionFlag)
	}

	payoutAdmin := v1.Group("/payout-batches")
	payoutAdmin.Use(middleware.AdminKeyAuth(cfg.AdminAPIKey, audit))
	{
		payoutAdmin.POST("/run", payouts.RunBatch)
		payoutAdmin.POST("/:id/mark-paid", payouts.MarkPaid)
	}

	log.Printf("Affilio API listening on %s", net.JoinHostPort(cfg.Host, cfg.Port))
	if err := r.Run(net.JoinHostPort(cfg.Host, cfg.Port)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
