package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type TrackingHandler struct {
	DB              *database.DB
	ClickRateLimit  int
	ClickRateWindow time.Duration
	DuplicateWindow time.Duration
}

type tuningInsightItem struct {
	ProgramID   string  `json:"program_id"`
	ProgramName string  `json:"program_name"`
	ProgramType string  `json:"program_type"`
	Multiplier  float64 `json:"multiplier"`
	SampleSize  int     `json:"sample_size"`
	Reason      string  `json:"reason"`
}

type tuningInsights struct {
	TotalProfiles  int                 `json:"total_profiles"`
	BoostedCount   int                 `json:"boosted_count"`
	PenalizedCount int                 `json:"penalized_count"`
	NeutralCount   int                 `json:"neutral_count"`
	HighConfidence int                 `json:"high_confidence_profiles"`
	TopBoosted     []tuningInsightItem `json:"top_boosted"`
	TopPenalized   []tuningInsightItem `json:"top_penalized"`
	Notes          []string            `json:"notes"`
}

type trackRequest struct {
	RecommendationID string `json:"recommendation_id"`
	ArticleID        string `json:"article_id"`
	ProgramID        string `json:"program_id"`
}

func (h *TrackingHandler) TrackClick(c *gin.Context) {
	var req trackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	req.RecommendationID = strings.TrimSpace(req.RecommendationID)
	req.ArticleID = strings.TrimSpace(req.ArticleID)
	req.ProgramID = strings.TrimSpace(req.ProgramID)
	if req.RecommendationID == "" && req.ArticleID == "" && req.ProgramID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one tracking identifier is required"})
		return
	}

	siteID := c.GetString("site_id")
	userAgent := c.GetHeader("User-Agent")
	ref := c.GetHeader("Referer")
	ipHash := hashIP(c.ClientIP())
	if ipHash != "" {
		limited, err := h.exceedsClickRate(c.Request.Context(), siteID, ipHash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record"})
			return
		}
		if limited {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		duplicate, err := h.isDuplicateClick(c.Request.Context(), siteID, req, ipHash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record"})
			return
		}
		if duplicate {
			c.JSON(http.StatusOK, gin.H{"status": "duplicate_suppressed"})
			return
		}
	}

	clickClass := ClassifyClick(c.Request.Method, userAgent, c.Request.Header)
	_, err := h.DB.Pool.Exec(c.Request.Context(), `
INSERT INTO clicks (site_id, article_id, recommendation_id, program_id, user_agent, referrer, ip_hash, click_class)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, siteID, nullIfEmpty(req.ArticleID), nullIfEmpty(req.RecommendationID), nullIfEmpty(req.ProgramID), userAgent, ref, ipHash, clickClass)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

func (h *TrackingHandler) duplicateWindowSeconds() int {
	window := h.DuplicateWindow
	if window <= 0 {
		window = 45 * time.Second
	}
	return int(window.Seconds())
}

func (h *TrackingHandler) clickRateWindowSeconds() int {
	window := h.ClickRateWindow
	if window <= 0 {
		window = time.Minute
	}
	return int(window.Seconds())
}

func (h *TrackingHandler) clickRateLimit() int {
	limit := h.ClickRateLimit
	if limit <= 0 {
		limit = 25
	}
	return limit
}

func (h *TrackingHandler) isDuplicateClick(ctx context.Context, siteID string, req trackRequest, ipHash string) (bool, error) {
	windowSeconds := h.duplicateWindowSeconds()
	if windowSeconds <= 0 {
		return false, nil
	}
	var exists bool
	err := h.DB.Pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM clicks
    WHERE site_id=$1
      AND ip_hash=$2
      AND COALESCE(recommendation_id::text,'') = COALESCE($3,'')
      AND COALESCE(article_id::text,'') = COALESCE($4,'')
      AND COALESCE(program_id::text,'') = COALESCE($5,'')
      AND clicked_at >= NOW() - ($6 * INTERVAL '1 second')
)`, siteID, ipHash, req.RecommendationID, req.ArticleID, req.ProgramID, windowSeconds).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (h *TrackingHandler) exceedsClickRate(ctx context.Context, siteID, ipHash string) (bool, error) {
	windowSeconds := h.clickRateWindowSeconds()
	limit := h.clickRateLimit()
	if windowSeconds <= 0 || limit <= 0 {
		return false, nil
	}
	var count int
	err := h.DB.Pool.QueryRow(ctx, `
SELECT COUNT(*)::int
FROM clicks
WHERE site_id=$1
  AND ip_hash=$2
  AND clicked_at >= NOW() - ($3 * INTERVAL '1 second')`, siteID, ipHash, windowSeconds).Scan(&count)
	if err != nil {
		return false, err
	}
	return count >= limit, nil
}

func (h *TrackingHandler) ArticleClickCounts(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	rows, err := h.DB.ListArticleClickCounts(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"articles": rows})
}

func (h *TrackingHandler) RecommendationClickCounts(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	rows, err := h.DB.ListRecommendationClickCounts(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recommendations": rows})
}

func (h *TrackingHandler) Summary(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	summary, err := h.DB.GetClickSummary(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

func (h *TrackingHandler) Trends(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	rows, err := h.DB.ListDailyClickCounts(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"trends": rows})
}

func (h *TrackingHandler) Attribution(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	modelID := parseAttributionModel(c.Query("model"))
	rows, totalRevenue, model, err := h.buildAttribution(c, siteID, days, modelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"programs": rows,
		"meta": gin.H{
			"model_id":                    modelID,
			"estimated_total_revenue_usd": round2(totalRevenue),
			"model":                       model,
		},
	})
}

func (h *TrackingHandler) Export(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	modelID := parseAttributionModel(c.Query("model"))
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "json")))
	if format != "json" && format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be json or csv"})
		return
	}

	summary, err := h.DB.GetClickSummary(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	funnel, err := h.DB.GetFunnelMetrics(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	funnel.InsertToClickRate = round2(funnel.InsertToClickRate)
	funnel.DismissRate = round2(funnel.DismissRate)
	trends, err := h.DB.ListDailyClickCounts(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	articles, err := h.DB.ListArticleClickCounts(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	recommendations, err := h.DB.ListRecommendationClickCounts(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	attribution, totalRevenue, model, err := h.buildAttribution(c, siteID, days, modelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}

	filename := fmt.Sprintf("affilio-analytics-%s.%s", time.Now().UTC().Format("20060102-150405"), format)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	if format == "json" {
		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusOK, gin.H{
			"generated_at":    time.Now().UTC().Format(time.RFC3339),
			"days":            days,
			"summary":         summary,
			"funnel":          funnel,
			"trends":          trends,
			"articles":        articles,
			"recommendations": recommendations,
			"attribution": gin.H{
				"programs": attribution,
				"meta": gin.H{
					"model_id":                    modelID,
					"estimated_total_revenue_usd": round2(totalRevenue),
					"model":                       model,
				},
			},
		})
		return
	}

	csvBytes, err := buildAnalyticsCSV(summary, funnel, trends, articles, recommendations, attribution, totalRevenue, model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate csv"})
		return
	}
	c.Data(http.StatusOK, "text/csv; charset=utf-8", csvBytes)
}

func (h *TrackingHandler) Funnel(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	funnel, err := h.DB.GetFunnelMetrics(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	funnel.InsertToClickRate = round2(funnel.InsertToClickRate)
	funnel.DismissRate = round2(funnel.DismissRate)
	c.JSON(http.StatusOK, gin.H{"funnel": funnel})
}

func (h *TrackingHandler) Cohorts(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	rows, err := h.DB.ListCohortMetrics(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	for i := range rows {
		rows[i].ClicksPerArticle = round2(rows[i].ClicksPerArticle)
	}
	c.JSON(http.StatusOK, gin.H{"cohorts": rows})
}

func (h *TrackingHandler) RecommendationQuality(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	raw, err := h.DB.ListRecommendationQualityRaw(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	rows := make([]database.RecommendationQuality, 0, len(raw))
	summary := database.RecommendationQuality{ProgramName: "All"}
	for _, r := range raw {
		row := database.RecommendationQuality{
			ProgramName: r.ProgramName,
			ProgramType: r.ProgramType,
			Total:       r.Total,
			Approved:    r.Approved,
			Dismissed:   r.Dismissed,
			Inserted:    r.Inserted,
			Pending:     r.Pending,
			Clicks:      r.Clicks,
		}
		if row.Total > 0 {
			row.ApprovalRate = round2(float64(row.Approved) / float64(row.Total) * 100)
			row.DismissRate = round2(float64(row.Dismissed) / float64(row.Total) * 100)
			row.InsertRate = round2(float64(row.Inserted) / float64(row.Total) * 100)
			row.ClickRate = round2(float64(row.Clicks) / float64(row.Total) * 100)
		}
		rows = append(rows, row)
		summary.Total += row.Total
		summary.Approved += row.Approved
		summary.Dismissed += row.Dismissed
		summary.Inserted += row.Inserted
		summary.Pending += row.Pending
		summary.Clicks += row.Clicks
	}
	if summary.Total > 0 {
		summary.ApprovalRate = round2(float64(summary.Approved) / float64(summary.Total) * 100)
		summary.DismissRate = round2(float64(summary.Dismissed) / float64(summary.Total) * 100)
		summary.InsertRate = round2(float64(summary.Inserted) / float64(summary.Total) * 100)
		summary.ClickRate = round2(float64(summary.Clicks) / float64(summary.Total) * 100)
	}
	c.JSON(http.StatusOK, gin.H{
		"summary":  summary,
		"programs": rows,
	})
}

func (h *TrackingHandler) Retention(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	rows, err := h.DB.ListRetentionCohorts(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	for i := range rows {
		if rows[i].Articles > 0 {
			rows[i].Week1Rate = round2(float64(rows[i].Week1Active) / float64(rows[i].Articles) * 100)
			rows[i].Week2Rate = round2(float64(rows[i].Week2Active) / float64(rows[i].Articles) * 100)
			rows[i].Week3Rate = round2(float64(rows[i].Week3Active) / float64(rows[i].Articles) * 100)
		}
		rows[i].Week1Churn = round2(100 - rows[i].Week1Rate)
		rows[i].Week2Churn = round2(100 - rows[i].Week2Rate)
		rows[i].Week3Churn = round2(100 - rows[i].Week3Rate)
	}
	c.JSON(http.StatusOK, gin.H{"retention": rows})
}

func (h *TrackingHandler) RecommendationTuning(c *gin.Context) {
	siteID := c.GetString("site_id")
	days := parseDays(c.Query("days"))
	rows, err := h.DB.ListRecommendationTuneProfiles(c.Request.Context(), siteID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analytics"})
		return
	}
	for i := range rows {
		rows[i].ApprovalRate = round2(rows[i].ApprovalRate * 100)
		rows[i].InsertRate = round2(rows[i].InsertRate * 100)
		rows[i].ClickRate = round2(rows[i].ClickRate * 100)
		rows[i].Multiplier = round2(rows[i].Multiplier)
	}
	c.JSON(http.StatusOK, gin.H{
		"tuning":   rows,
		"insights": buildTuningInsights(rows),
	})
}

func buildTuningInsights(rows []database.RecommendationTuneProfile) tuningInsights {
	out := tuningInsights{
		TopBoosted:   make([]tuningInsightItem, 0, 3),
		TopPenalized: make([]tuningInsightItem, 0, 3),
		Notes:        make([]string, 0, 3),
	}
	if len(rows) == 0 {
		out.Notes = append(out.Notes, "No profile reached minimum sample size yet.")
		return out
	}
	out.TotalProfiles = len(rows)
	boosted := make([]database.RecommendationTuneProfile, 0, len(rows))
	penalized := make([]database.RecommendationTuneProfile, 0, len(rows))
	for _, row := range rows {
		switch {
		case row.Multiplier > 1.02:
			out.BoostedCount++
			boosted = append(boosted, row)
		case row.Multiplier < 0.98:
			out.PenalizedCount++
			penalized = append(penalized, row)
		default:
			out.NeutralCount++
		}
		if row.SampleSize >= 10 {
			out.HighConfidence++
		}
	}
	if out.BoostedCount == 0 && out.PenalizedCount == 0 {
		out.Notes = append(out.Notes, "Profiles are currently neutral; system has not found strong winners or weak performers yet.")
	}
	if out.HighConfidence == 0 {
		out.Notes = append(out.Notes, "Collect more outcomes per program to increase tuning confidence.")
	}
	sort.Slice(boosted, func(i, j int) bool { return boosted[i].Multiplier > boosted[j].Multiplier })
	sort.Slice(penalized, func(i, j int) bool { return penalized[i].Multiplier < penalized[j].Multiplier })
	appendTop := func(src []database.RecommendationTuneProfile, target *[]tuningInsightItem, isBoost bool) {
		limit := 3
		if len(src) < limit {
			limit = len(src)
		}
		for i := 0; i < limit; i++ {
			row := src[i]
			item := tuningInsightItem{
				ProgramID:   row.ProgramID,
				ProgramName: row.ProgramName,
				ProgramType: "curated",
				Multiplier:  row.Multiplier,
				SampleSize:  row.SampleSize,
				Reason:      tuningReason(row, isBoost),
			}
			if row.IsUserProgram {
				item.ProgramType = "user"
			}
			*target = append(*target, item)
		}
	}
	appendTop(boosted, &out.TopBoosted, true)
	appendTop(penalized, &out.TopPenalized, false)
	return out
}

func tuningReason(row database.RecommendationTuneProfile, isBoost bool) string {
	if isBoost {
		if row.ClickRate >= 25 {
			return "Strong click-through outcomes."
		}
		if row.InsertRate >= 50 {
			return "High insert conversion after recommendation."
		}
		return "Consistently positive acceptance outcomes."
	}
	if row.ApprovalRate <= 25 {
		return "Low approval rate across outcomes."
	}
	if row.ClickRate <= 5 {
		return "Low click-through outcomes after insertion."
	}
	return "Weak conversion outcomes vs other programs."
}

func parseDays(raw string) int {
	if raw == "" {
		return 0
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val < 0 {
		return 0
	}
	return val
}

func parseAttributionModel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "first_click", "linear_session":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "last_click"
	}
}

func attributionModelLabel(modelID string) string {
	switch modelID {
	case "first_click":
		return "first_click: first session click gets full credit"
	case "linear_session":
		return "linear_session: equal credit per touched recommendation in session"
	default:
		return "last_click: each click gets full credit"
	}
}

func (h *TrackingHandler) buildAttribution(c *gin.Context, siteID string, days int, modelID string) ([]database.AttributionProgram, float64, string, error) {
	raw, err := h.DB.ListAttributionProgramRaw(c.Request.Context(), siteID, days, modelID)
	if err != nil {
		return nil, 0, "", err
	}
	total := 0.0
	for _, r := range raw {
		total += r.Credits
	}
	rows := make([]database.AttributionProgram, 0, len(raw))
	totalRevenue := 0.0
	for _, r := range raw {
		share := 0.0
		if total > 0 {
			share = round2((r.Credits / total) * 100)
		}
		estimated := estimateRevenue(r)
		totalRevenue += estimated
		rows = append(rows, database.AttributionProgram{
			ProgramName:         r.ProgramName,
			ProgramType:         r.ProgramType,
			AttributedClicks:    round2(r.Credits),
			ClickSharePercent:   share,
			EstimatedRevenueUSD: round2(estimated),
			CommissionType:      r.CommissionType,
			CommissionUnit:      r.CommissionUnit,
		})
	}
	return rows, totalRevenue, attributionModelLabel(modelID), nil
}

func buildAnalyticsCSV(
	summary *database.ClickSummary,
	funnel *database.FunnelMetrics,
	trends []database.DailyClickCount,
	articles []database.ArticleClickCount,
	recommendations []database.RecommendationClickCount,
	attribution []database.AttributionProgram,
	totalRevenue float64,
	model string,
) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	write := func(row ...string) error { return w.Write(row) }

	if err := write("section", "metric", "value"); err != nil {
		return nil, err
	}
	if summary != nil {
		_ = write("summary", "total_clicks", strconv.Itoa(summary.TotalClicks))
		_ = write("summary", "unique_articles", strconv.Itoa(summary.UniqueArticles))
		_ = write("summary", "unique_recommendations", strconv.Itoa(summary.UniqueRecommendations))
	}
	if funnel != nil {
		_ = write("funnel", "inserted", strconv.Itoa(funnel.Inserted))
		_ = write("funnel", "clicks", strconv.Itoa(funnel.Clicks))
		_ = write("funnel", "dismissed", strconv.Itoa(funnel.Dismissed))
		_ = write("funnel", "approved", strconv.Itoa(funnel.Approved))
		_ = write("funnel", "pending", strconv.Itoa(funnel.Pending))
		_ = write("funnel", "insert_to_click_rate_percent", fmt.Sprintf("%.2f", funnel.InsertToClickRate))
		_ = write("funnel", "dismiss_rate_percent", fmt.Sprintf("%.2f", funnel.DismissRate))
	}

	_ = write("", "", "")
	_ = write("trends", "day", "clicks")
	for _, row := range trends {
		_ = write("trends", row.Day, strconv.Itoa(row.Clicks))
	}

	_ = write("", "", "")
	_ = write("articles", "title", "clicks")
	for _, row := range articles {
		_ = write("articles", row.Title, strconv.Itoa(row.Clicks))
	}

	_ = write("", "", "")
	_ = write("recommendations", "program_name", "clicks")
	for _, row := range recommendations {
		_ = write("recommendations", row.ProgramName, strconv.Itoa(row.Clicks))
	}

	_ = write("", "", "")
	_ = write("attribution", "program_name", "attributed_clicks", "share_percent", "estimated_revenue_usd")
	for _, row := range attribution {
		_ = write("attribution", row.ProgramName, fmt.Sprintf("%.2f", row.AttributedClicks), fmt.Sprintf("%.2f", row.ClickSharePercent), fmt.Sprintf("%.2f", row.EstimatedRevenueUSD))
	}
	_ = write("attribution_meta", "estimated_total_revenue_usd", fmt.Sprintf("%.2f", totalRevenue))
	_ = write("attribution_meta", "model", model)

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func estimateRevenue(in database.AttributionProgramRaw) float64 {
	if in.ProgramType == "user" || in.Commission <= 0 {
		return 0
	}
	switch in.CommissionUnit {
	case "fixed":
		return in.Credits * in.Commission
	case "percent":
		return in.Credits * in.Commission // assumes $100 order value
	default:
		return 0
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func hashIP(ip string) string {
	if ip == "" {
		return ""
	}
	h := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(h[:])
}

func nullIfEmpty(val string) *string {
	if val == "" {
		return nil
	}
	return &val
}
