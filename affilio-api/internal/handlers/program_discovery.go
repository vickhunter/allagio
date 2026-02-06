package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
)

type ProgramDiscoveryHandler struct {
	DB         *database.DB
	HTTPClient *http.Client
}

type reviewPayload struct {
	Status string `json:"status"`
	Note   string `json:"note"`
	Actor  string `json:"actor"`
}

type promotePayload struct {
	Name            string `json:"name"`
	Company         string `json:"company"`
	Slug            string `json:"slug"`
	WebsiteURL      string `json:"website_url"`
	SignupURL       string `json:"signup_url"`
	PrimaryCategory string `json:"primary_category"`
	Description     string `json:"description"`
	Actor           string `json:"actor"`
	Note            string `json:"note"`
}

func (h *ProgramDiscoveryHandler) List(c *gin.Context) {
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	rows, err := h.DB.Pool.Query(c.Request.Context(), `
SELECT id, domain, sample_url, sites_count, status, created_at
FROM program_discovery_queue
ORDER BY created_at DESC
LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load queue"})
		return
	}
	defer rows.Close()
	queue := []map[string]interface{}{}
	for rows.Next() {
		var id, domain, sample, status string
		var sitesCount int
		var createdAt time.Time
		if err := rows.Scan(&id, &domain, &sample, &sitesCount, &status, &createdAt); err != nil {
			continue
		}
		enriched := services.EnrichDiscovery(domain, sample)
		reviews := listDiscoveryReviews(c, h.DB, id)
		queue = append(queue, gin.H{
			"id":                    id,
			"domain":                domain,
			"sample_url":            sample,
			"sites_count":           sitesCount,
			"status":                status,
			"created_at":            createdAt.Format(time.RFC3339),
			"suggested_name":        enriched.SuggestedName,
			"suggested_company":     enriched.SuggestedCompany,
			"suggested_slug":        enriched.SuggestedSlug,
			"suggested_website_url": enriched.SuggestedWebsiteURL,
			"suggested_signup_url":  enriched.SuggestedSignupURL,
			"suggested_category":    enriched.SuggestedCategory,
			"suggested_description": enriched.SuggestedDescription,
			"suggested_confidence":  enriched.Confidence,
			"logo_url":              logoURL(domain),
			"reviews":               reviews,
		})
	}
	total := countDiscoveryQueue(c, h.DB)
	pagination := gin.H{
		"page":        (offset/limit + 1),
		"prev_offset": prevOffset(offset, limit),
		"next_offset": nextOffset(offset, limit, total),
	}
	c.JSON(http.StatusOK, gin.H{"queue": queue, "pagination": pagination})
}

func (h *ProgramDiscoveryHandler) Review(c *gin.Context) {
	var payload reviewPayload
	_ = c.ShouldBindJSON(&payload)
	status := payload.Status
	if status == "" {
		status = c.Query("status")
	}
	if status == "" {
		status = "reviewed"
	}
	if status != "reviewed" && status != "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	_, err := h.DB.Pool.Exec(c.Request.Context(), `UPDATE program_discovery_queue SET status=$1 WHERE id=$2`, status, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	_, _ = h.DB.Pool.Exec(c.Request.Context(), `
INSERT INTO program_discovery_reviews (discovery_id, status, note, reviewed_by)
VALUES ($1,$2,$3,$4)`, c.Param("id"), status, payload.Note, payload.Actor)
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (h *ProgramDiscoveryHandler) Promote(c *gin.Context) {
	var payload promotePayload
	_ = c.ShouldBindJSON(&payload)
	row := h.DB.Pool.QueryRow(c.Request.Context(), `SELECT domain, COALESCE(sample_url,'') FROM program_discovery_queue WHERE id=$1`, c.Param("id"))
	var domain, sampleURL string
	if err := row.Scan(&domain, &sampleURL); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	enriched := services.EnrichDiscoveryWithMetadata(c.Request.Context(), domain, sampleURL, h.HTTPClient)
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = enriched.SuggestedName
	}
	company := strings.TrimSpace(payload.Company)
	if company == "" {
		company = enriched.SuggestedCompany
	}
	slug := strings.TrimSpace(payload.Slug)
	if slug == "" {
		slug = enriched.SuggestedSlug
	}
	website := strings.TrimSpace(payload.WebsiteURL)
	if website == "" {
		website = enriched.SuggestedWebsiteURL
	}
	signup := strings.TrimSpace(payload.SignupURL)
	if signup == "" {
		signup = enriched.SuggestedSignupURL
	}
	category := strings.TrimSpace(payload.PrimaryCategory)
	if category == "" {
		category = enriched.SuggestedCategory
	}
	description := strings.TrimSpace(payload.Description)
	if description == "" {
		description = enriched.SuggestedDescription
	}
	var programID string
	err := h.DB.Pool.QueryRow(c.Request.Context(), `
INSERT INTO affiliate_programs (name, company, slug, description, website_url, commission_type, primary_category, signup_url, status, is_verified)
VALUES ($1,$2,$3,$4,$5,'unknown',$6,$7,'active',false)
ON CONFLICT (slug) DO UPDATE SET
    name=EXCLUDED.name,
    company=EXCLUDED.company,
    description=EXCLUDED.description,
    website_url=EXCLUDED.website_url,
    primary_category=EXCLUDED.primary_category,
    signup_url=EXCLUDED.signup_url,
    updated_at=NOW()
RETURNING id`, name, company, slug, description, website, category, signup).Scan(&programID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to promote"})
		return
	}
	_, _ = h.DB.Pool.Exec(c.Request.Context(), `UPDATE program_discovery_queue SET status='promoted' WHERE id=$1`, c.Param("id"))
	actor := payload.Actor
	if actor == "" {
		actor = "admin"
	}
	note := payload.Note
	if note == "" {
		note = "Promoted to curated program."
	}
	_, _ = h.DB.Pool.Exec(c.Request.Context(), `
INSERT INTO program_discovery_reviews (discovery_id, status, note, reviewed_by)
VALUES ($1,'promoted',$2,$3)`, c.Param("id"), note, actor)
	c.JSON(http.StatusOK, gin.H{"status": "promoted", "program_id": programID})
}

func listDiscoveryReviews(c *gin.Context, db *database.DB, id string) []map[string]interface{} {
	rows, err := db.Pool.Query(c.Request.Context(), `
SELECT status, COALESCE(note,''), COALESCE(reviewed_by,''), reviewed_at
FROM program_discovery_reviews
WHERE discovery_id=$1
ORDER BY reviewed_at DESC
LIMIT 5`, id)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var status, note, actor string
		var reviewedAt time.Time
		if err := rows.Scan(&status, &note, &actor, &reviewedAt); err != nil {
			continue
		}
		out = append(out, gin.H{
			"status":      status,
			"note":        note,
			"actor":       actor,
			"reviewed_at": reviewedAt.Format(time.RFC3339),
		})
	}
	return out
}

func logoURL(domain string) string {
	clean := strings.TrimSpace(strings.ToLower(domain))
	clean = strings.TrimPrefix(clean, "www.")
	if clean == "" {
		return ""
	}
	return "https://logo.clearbit.com/" + clean
}

func intQuery(c *gin.Context, key string, def int) int {
	val := c.Query(key)
	if val == "" {
		return def
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return parsed
}

func countDiscoveryQueue(c *gin.Context, db *database.DB) int {
	row := db.Pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM program_discovery_queue`)
	var total int
	if err := row.Scan(&total); err != nil {
		return 0
	}
	return total
}

func prevOffset(offset, limit int) *int {
	if offset <= 0 {
		return nil
	}
	val := offset - limit
	if val < 0 {
		val = 0
	}
	return &val
}

func nextOffset(offset, limit, total int) *int {
	next := offset + limit
	if next >= total {
		return nil
	}
	return &next
}
