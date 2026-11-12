package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"affilio-api/internal/config"
	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/handlers"
	"affilio-api/internal/middleware"
	"affilio-api/internal/migrations"
	"affilio-api/internal/services"
	"affilio-api/internal/version"

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

// loopbackMCPResource is the join-token audience for a process that is bound
// only to loopback and has no PUBLIC_BASE_URL. Empty HOST and 0.0.0.0 listen
// on every interface, so they get no audience. Release mode stays on the
// public URL. The token is minted against the request host, so this string
// has to be that same origin.
func loopbackMCPResource(cfg config.Config) (string, bool) {
	if config.Hosted(cfg.GinMode) {
		return "", false
	}
	host := strings.TrimSpace(cfg.Host)
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost", "::1":
	default:
		return "", false
	}
	return "http://" + net.JoinHostPort(host, cfg.Port) + "/mcp", true
}

func healthHandler(native bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":                         "ok",
			"site_installation_registration": true,
			"native_distribution":            native,
			"commit":                         version.Commit(),
		})
	}
}

func readyHandler(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Pool.Ping(ctx); err != nil {
			c.JSON(503, gin.H{"status": "unavailable", "commit": version.Commit()})
			return
		}
		c.JSON(200, gin.H{"status": "ok", "commit": version.Commit()})
	}
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
	if cfg.PublicBaseURL != "" && len(cfg.TrustedProxies) == 0 {
		log.Print("warning: TRUSTED_PROXIES is empty on a hosted service; every request will share the edge IP, collapsing per-IP rate limits into one global bucket")
	}
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	if cfg.POCReportPath != "" {
		if cfg.Host != "127.0.0.1" {
			log.Fatal("POC report requires HOST=127.0.0.1")
		}
		r.GET("/pilot", func(c *gin.Context) { c.File(cfg.POCReportPath) })
	}
	health := healthHandler(cfg.PublicBaseURL != "")
	r.GET("/health", health)
	r.GET("/ready", readyHandler(db))
	preview := &handlers.PreviewHandler{
		DB:      db,
		Limiter: handlers.NewIPRateLimiter(10, time.Minute),
	}
	r.POST("/api/preview", preview.Preview)
	r.GET("/.well-known/dcp.json", handlers.ServeDCPDiscovery(cfg.PublicBaseURL))
	r.GET("/.well-known/allagio.json", handlers.ServeAllagioDiscovery(cfg.PublicBaseURL))
	r.GET("/.well-known/agent-card.json", handlers.ServeAgentCard(cfg.PublicBaseURL))
	r.GET("/.well-known/agent.json", handlers.ServeAgentCard(cfg.PublicBaseURL))

	v1 := r.Group("/v1")
	v1.Use(middleware.MaxRequestBody(8 << 20))

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
		GracePeriod:     cfg.KeyRotationGracePeriod,
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
	claims := entry.ClaimChecker(entry.StubClaimChecker{})
	if cfg.ClaimsCheck == config.ClaimsReal {
		claims = entry.PageClaimChecker{}
	}
	placements := &handlers.PlacementsHandler{DB: db, RequireVerifiedConnection: cfg.PublicBaseURL != "", Claims: claims}
	payouts := &handlers.PayoutsHandler{DB: db}
	redirects := &handlers.RedirectsHandler{
		DB:           db,
		RateLimiter:  handlers.NewIPRateLimiter(120, time.Minute),
		HomeURL:      handlers.OfferRailsHome,
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
		hosted = &handlers.HostedHandler{DB: db, BaseURL: cfg.PublicBaseURL, LoopbackURL: "http://127.0.0.1:" + cfg.Port, Limiter: handlers.NewIPRateLimiter(240, time.Minute), Payments: &handlers.MerchantConnectionsHandler{DB: db, Cipher: connectionCipher, Verifier: &services.StripeAPI{}, BaseURL: cfg.PublicBaseURL}, AssetsPath: cfg.UIAssetsPath, DownloadsPath: cfg.DownloadsPath, OperatorID: cfg.OperatorOwnerID, Assessments: assessments, ClaimsCheck: cfg.ClaimsCheck, TrustedProxies: cfg.TrustedProxies, Render: cfg.Render, TrustedPlatform: cfg.TrustedPlatform}
		if err = hosted.LoadWorkspace(); err != nil {
			log.Fatalf("workspace assets failed: %v", err)
		}
		hosted.Mount(r)
		oauthResources = append(oauthResources, cfg.PublicBaseURL+"/mcp")
	} else if resource, ok := loopbackMCPResource(cfg); ok {
		oauthResources = append(oauthResources, resource)
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
	v1.GET("/m/:code", redirects.Monetization)
	v1.HEAD("/m/:code", redirects.Monetization)
	v1.POST("/webhooks/:provider", webhooks.Receive)
	v1.POST("/webhooks/:provider/:product_id", webhooks.Receive)
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	if hosted != nil {
		issuer, clientID, clientSecret, callback := cfg.OIDCIssuer, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.PublicBaseURL+"/auth/callback"
		go handlers.DiscoverOwnerLogin(workerCtx, hosted, clientSecret, func() (services.OwnerLogin, error) {
			return services.NewOIDCLogin(workerCtx, issuer, clientID, clientSecret, callback)
		})
	}
	go webhooks.RunWorker(workerCtx)
	go handlers.PurgeClickSignals(workerCtx, db)
	mcp := &handlers.MCPHandler{
		LoopbackURL: "http://127.0.0.1:" + cfg.Port,
		RateLimiter: handlers.NewIPRateLimiter(240, time.Minute),
	}
	v1.POST("/mcp", mcp.Receive)
	v1.Match([]string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions}, "/mcp", mcp.MethodNotAllowed)

	openSignup := handlers.NewIPRateLimiter(handlers.OpenSignupPerMinute, time.Minute)
	addressKey, addressWarning := services.ClientAddressHMACKey(cfg.APIKeySecret)
	if addressWarning != "" {
		log.Print(addressWarning)
	}
	agentEntry := &handlers.AgentEntry{
		DB:                 db,
		BaseURL:            strings.TrimRight(cfg.PublicBaseURL, "/"),
		Claims:             claims,
		AccountLimiter:     openSignup,
		TokenLimiter:       openSignup,
		AddressKey:         addressKey,
		ListenHost:         cfg.Host,
		Render:             cfg.Render,
		TrustedPlatform:    cfg.TrustedPlatform,
		DeviceLimiter:      handlers.NewIPRateLimiter(30, time.Minute),
		PluginOfflineAfter: cfg.PluginOfflineAfter,
		StatusLimiter:      handlers.NewPlacementPollLimiter(),
		ClaimsCheck:        cfg.ClaimsCheck,
		AssetsPath:         cfg.UIAssetsPath,
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
	(&handlers.ORPHost{
		Entry:          agentEntry,
		SuggestLimiter: handlers.NewIPRateLimiter(30, time.Minute),
		MessageLimiter: handlers.NewIPRateLimiter(30, time.Minute),
	}).Mount(r)
	(&handlers.A2A{Entry: agentEntry, Engine: r}).Mount(r)
	signals := v1.Group("/agent-signals")
	signals.Use(middleware.AdminKeyAuth(cfg.AdminAPIKey, audit))
	signals.GET("", agentEntry.AgentSignals)

	secured := v1.Group("/")
	secured.Use(middleware.APIKeyAuth(db, audit, oauthResources...))
	secured.GET("/sites/current", sites.Current)
	secured.GET("/sites/:id", sites.Get)
	secured.POST("/sites/:id/verify", sites.Verify)
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
		offerAdmin.POST("/steps/:id/approve", offerReview.ApproveStep)
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

	if hosted == nil {
		handlers.InstallSlashRedirect(r)
	}
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("Affilio API listening on %s", addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
	cancelWorker()
}
