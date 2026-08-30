package main

import (
	"context"
	"log"
	"time"

	"affilio-api/internal/config"
	"affilio-api/internal/database"
	"affilio-api/internal/handlers"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	r := gin.Default()
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Fatalf("failed to set trusted proxies: %v", err)
	}
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	v1 := r.Group("/v1")

	sites := &handlers.SitesHandler{
		DB:              db,
		AdminAPIKey:     cfg.AdminAPIKey,
		RegisterLimiter: handlers.NewIPRateLimiter(8, time.Minute),
	}
	siteKeys := &handlers.SiteKeysHandler{DB: db}
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
	programDiscovery := &handlers.ProgramDiscoveryHandler{DB: db}
	makers := &handlers.MakersHandler{
		DB:              db,
		RegisterLimiter: handlers.NewIPRateLimiter(8, time.Minute),
	}
	products := &handlers.ProductsHandler{DB: db}
	offers := &handlers.OffersHandler{DB: db}
	offerReview := &handlers.OfferReviewHandler{DB: db}
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
	payouts := &handlers.PayoutsHandler{DB: db}
	redirects := &handlers.RedirectsHandler{
		DB:          db,
		RateLimiter: handlers.NewIPRateLimiter(120, time.Minute),
	}
	webhooks := &handlers.WebhooksHandler{
		DB: db,
		Secrets: map[string]string{
			"stripe":       cfg.StripeWebhookSecret,
			"lemonsqueezy": cfg.LemonSqueezyWebhookSecret,
			"polar":        cfg.PolarWebhookSecret,
		},
	}

	v1.POST("/sites/register", sites.Register)
	v1.DELETE("/sites", sites.Delete)
	v1.POST("/makers/register", makers.Register)
	v1.GET("/go/:code", redirects.Go)
	v1.POST("/webhooks/:provider", webhooks.Receive)

	secured := v1.Group("/")
	secured.Use(middleware.APIKeyAuth(db))
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

		secured.GET("/marketplace/offers", marketplace.ListOffers)
		secured.POST("/marketplace/offers/:id/join", marketplace.JoinOffer)

		secured.GET("/payouts/balance", payouts.Balance)
		secured.GET("/payouts/history", payouts.History)
	}

	maker := v1.Group("/maker")
	maker.Use(middleware.MakerKeyAuth(db))
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

		maker.GET("/earnings", payouts.MakerEarnings)
	}

	admin := v1.Group("/program-discovery")
	admin.Use(middleware.AdminKeyAuth(cfg.AdminAPIKey))
	{
		admin.GET("/queue", programDiscovery.List)
		admin.POST("/queue/:id/review", programDiscovery.Review)
		admin.POST("/queue/:id/promote", programDiscovery.Promote)
		admin.POST("/sites/rotate-key", siteKeys.Rotate)
	}

	offerAdmin := v1.Group("/offer-review")
	offerAdmin.Use(middleware.AdminKeyAuth(cfg.AdminAPIKey))
	{
		offerAdmin.GET("/queue", offerReview.Queue)
		offerAdmin.POST("/:id/approve", offerReview.Approve)
		offerAdmin.POST("/:id/reject", offerReview.Reject)
		offerAdmin.POST("/makers/rotate-key", makers.RotateKey)
	}

	payoutAdmin := v1.Group("/payout-batches")
	payoutAdmin.Use(middleware.AdminKeyAuth(cfg.AdminAPIKey))
	{
		payoutAdmin.POST("/run", payouts.RunBatch)
		payoutAdmin.POST("/:id/mark-paid", payouts.MarkPaid)
	}

	log.Printf("Affilio API listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
