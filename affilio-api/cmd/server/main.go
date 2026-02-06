package main

import (
	"context"
	"log"

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

	sites := &handlers.SitesHandler{DB: db}
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

	v1.POST("/sites/register", sites.Register)
	v1.DELETE("/sites", sites.Delete)

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
	}

	admin := v1.Group("/program-discovery")
	admin.Use(middleware.AdminKeyAuth(cfg.AdminAPIKey))
	{
		admin.GET("/queue", programDiscovery.List)
		admin.POST("/queue/:id/review", programDiscovery.Review)
		admin.POST("/queue/:id/promote", programDiscovery.Promote)
		admin.POST("/sites/rotate-key", siteKeys.Rotate)
	}

	log.Printf("Affilio API listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
