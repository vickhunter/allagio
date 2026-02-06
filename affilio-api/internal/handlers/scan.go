package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

type ScanHandler struct {
	DB         *database.DB
	Classifier *services.AIClassifier
}

type scanFullRequest struct {
	Articles []scanArticle `json:"articles"`
}

type scanArticle struct {
	WPPostID int64  `json:"wp_post_id"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	URL      string `json:"url"`
}

func (h *ScanHandler) ScanFull(c *gin.Context) {
	var req scanFullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	siteID := c.GetString("site_id")
	jobID, err := h.DB.CreateScanJob(c.Request.Context(), siteID, len(req.Articles))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create scan job"})
		return
	}
	detected := 0
	for idx, article := range req.Articles {
		articleID, detCount, err := h.upsertAndAnalyze(c, siteID, article)
		if err != nil {
			_ = h.DB.UpdateScanJobProgress(c.Request.Context(), jobID, idx, "failed", strPtr(err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "scan failed"})
			return
		}
		_ = articleID
		detected += detCount
		_ = h.DB.UpdateScanJobProgress(c.Request.Context(), jobID, idx+1, "scanning", nil)
	}
	_ = h.DB.UpdateScanJobProgress(c.Request.Context(), jobID, len(req.Articles), "complete", nil)
	c.JSON(http.StatusAccepted, gin.H{"status": "queued", "job_id": jobID, "articles": len(req.Articles), "detections": detected})
}

func (h *ScanHandler) ScanSingle(c *gin.Context) {
	var req scanArticle
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	siteID := c.GetString("site_id")
	articleID, _, err := h.upsertAndAnalyze(c, siteID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis failed"})
		return
	}
	recs, _ := h.DB.ListRecommendations(c.Request.Context(), siteID, &articleID)
	article, _ := h.DB.GetArticle(c.Request.Context(), siteID, articleID)
	c.JSON(http.StatusOK, gin.H{
		"monetization_score": article.MonetizationScore,
		"potential":          extractPotential(article.AnalysisJSON),
		"recommendations":    recs,
		"detections":         extractDetections(article.AnalysisJSON),
	})
}

func (h *ScanHandler) ScanStatus(c *gin.Context) {
	siteID := c.GetString("site_id")
	job, err := h.DB.LatestScanJob(c.Request.Context(), siteID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "pending"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *ScanHandler) upsertAndAnalyze(c *gin.Context, siteID string, article scanArticle) (string, int, error) {
	hash := sha256.Sum256([]byte(article.Content))
	contentHash := hex.EncodeToString(hash[:])
	articleID, err := h.DB.UpsertArticle(c.Request.Context(), siteID, article.WPPostID, article.Title, article.URL, contentHash)
	if err != nil {
		return "", 0, err
	}
	detections := services.DetectAffiliateLinks(article.Content)
	if h.Classifier != nil {
		if classified, err := h.Classifier.ClassifyAffiliateLinks(c.Request.Context(), detections); err == nil {
			detections = classified
		}
	}
	_, _ = h.DB.Pool.Exec(c.Request.Context(), `DELETE FROM link_detections WHERE article_id=$1`, articleID)
	for _, det := range detections {
		_ = h.DB.InsertLinkDetection(c.Request.Context(), articleID, det.URL, det.Domain, det.IsAffiliate, det.Confidence, det.Method)
		if det.IsAffiliate && det.Domain != "" {
			name := det.ProgramGuess
			if name == "" {
				name = det.Domain
			}
			_, _ = h.DB.UpsertUserProgram(c.Request.Context(), siteID, name, det.Domain, det.ProgramGuess, det.URL)
			_ = h.DB.EnqueueDiscovery(c.Request.Context(), det.Domain, det.URL)
		}
	}
	programs, _ := h.DB.ListUserPrograms(c.Request.Context(), siteID)
	userInputs := make([]services.UserProgramInput, 0, len(programs))
	for _, p := range programs {
		userInputs = append(userInputs, services.UserProgramInput{ID: p.ID, Name: p.Name, Domain: p.Domain})
	}
	matches := services.MatchUserPrograms(article.Content, userInputs, detections)
	curated, _ := h.DB.ListCuratedProgramsForMatching(c.Request.Context(), 200)
	curatedInputs := make([]services.CuratedProgramInput, 0, len(curated))
	for _, p := range curated {
		curatedInputs = append(curatedInputs, services.CuratedProgramInput{ID: p.ID, Name: p.Name, Company: p.Company, Slug: p.Slug, Keywords: p.Keywords, Category: p.Category})
	}
	matches = append(matches, services.MatchCuratedPrograms(article.Content, curatedInputs, detections)...)
	matches = services.DedupeMatches(matches)
	if profiles, err := h.DB.ListRecommendationTuneProfiles(c.Request.Context(), siteID, 90); err == nil && len(profiles) > 0 {
		tuning := make([]services.RecommendationTuneProfile, 0, len(profiles))
		for _, p := range profiles {
			tuning = append(tuning, services.RecommendationTuneProfile{
				ProgramID:     p.ProgramID,
				IsUserProgram: p.IsUserProgram,
				Multiplier:    p.Multiplier,
				SampleSize:    p.SampleSize,
			})
		}
		matches = services.ApplyRecommendationTuning(matches, tuning)
	}
	_, _ = h.DB.Pool.Exec(c.Request.Context(), `DELETE FROM recommendations WHERE article_id=$1`, articleID)
	limit := 5
	for _, m := range matches {
		if limit == 0 {
			break
		}
		if m.IsUserProgram {
			userProgramID := m.ProgramID
			_ = h.DB.CreateRecommendation(c.Request.Context(), articleID, nil, &userProgramID, m.Relevance, m.Reason, m.Insertion, m.Position)
		} else {
			programID := m.ProgramID
			_ = h.DB.CreateRecommendation(c.Request.Context(), articleID, &programID, nil, m.Relevance, m.Reason, m.Insertion, m.Position)
		}
		limit--
	}
	wordCount := services.WordCount(article.Content)
	score := services.ComputeMonetizationScore(wordCount, len(matches)+len(detections), len(detections), topOpportunity(matches), len(matches))
	analysis := map[string]interface{}{
		"word_count": wordCount,
		"potential":  score.Potential,
		"detections": detections,
	}
	existingLinks := services.ExtractLinks(article.Content)
	analysisJSON := database.ToJSON(analysis)
	linksJSON := database.ToJSON(existingLinks)
	if err := h.DB.UpdateArticleAnalysis(c.Request.Context(), articleID, analysisJSON, score.Score, linksJSON); err != nil {
		return "", 0, err
	}
	return articleID, len(detections), nil
}

func topOpportunity(matches []services.ProgramMatch) int {
	max := 0
	for _, m := range matches {
		if m.Relevance > max {
			max = m.Relevance
		}
	}
	return max
}

func extractPotential(analysis []byte) string {
	if analysis == nil || len(analysis) == 0 {
		return "low"
	}
	var payload map[string]interface{}
	_ = json.Unmarshal(analysis, &payload)
	if v, ok := payload["potential"].(string); ok {
		return v
	}
	return "low"
}

func extractDetections(analysis []byte) interface{} {
	if analysis == nil || len(analysis) == 0 {
		return []interface{}{}
	}
	var payload map[string]interface{}
	_ = json.Unmarshal(analysis, &payload)
	if v, ok := payload["detections"]; ok {
		return v
	}
	return []interface{}{}
}

func strPtr(s string) *string { return &s }
