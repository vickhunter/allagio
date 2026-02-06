package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type TrackingHandler struct {
	DB *database.DB
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
	siteID := c.GetString("site_id")
	userAgent := c.GetHeader("User-Agent")
	ref := c.GetHeader("Referer")
	_, err := h.DB.Pool.Exec(c.Request.Context(), `
INSERT INTO clicks (site_id, article_id, recommendation_id, program_id, user_agent, referrer, ip_hash)
VALUES ($1,$2,$3,$4,$5,$6,$7)`, siteID, nullIfEmpty(req.ArticleID), nullIfEmpty(req.RecommendationID), nullIfEmpty(req.ProgramID), userAgent, ref, hashIP(c.ClientIP()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
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
	c.JSON(http.StatusOK, gin.H{"tuning": rows})
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
