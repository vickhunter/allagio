package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type AssessmentsHandler struct {
	DB     *database.DB
	Engine *services.ReadinessEngine
}

type assessRequest struct {
	RefParam string `json:"ref_param"`
}

// Assess runs the readiness check catalog against the product's live pages,
// stores the assessment, and updates the product's readiness gate. Offers can
// only be submitted for listing while the product is 'ready'.
func (h *AssessmentsHandler) Assess(c *gin.Context) {
	product, err := h.DB.GetProduct(c.Request.Context(), makerID(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product"})
		return
	}
	var req assessRequest
	_ = c.ShouldBindJSON(&req)

	result := h.Engine.Assess(c.Request.Context(), services.ReadinessProduct{
		Name:            product.Name,
		Tagline:         product.Tagline,
		WebsiteURL:      product.WebsiteURL,
		DocsURL:         product.DocsURL,
		PricingURL:      product.PricingURL,
		PaymentProvider: product.PaymentProvider,
		RefParam:        strings.TrimSpace(req.RefParam),
	})

	checksJSON, err := json.Marshal(result.Checks)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode assessment"})
		return
	}
	assessmentID, err := h.DB.InsertReadinessAssessment(c.Request.Context(), product.ID, result.Score, checksJSON, result.AISummary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store assessment"})
		return
	}
	if err := h.DB.UpdateProductReadiness(c.Request.Context(), product.ID, result.Score, result.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update product readiness"})
		return
	}
	if result.Category != "" || len(result.Keywords) > 0 || len(result.Tags) > 0 {
		_ = h.DB.FillProductTaxonomy(c.Request.Context(), product.ID, result.Category, result.Keywords, result.Tags)
	}
	c.JSON(http.StatusOK, gin.H{
		"assessment_id":    assessmentID,
		"product_id":       product.ID,
		"readiness_status": result.Status,
		"readiness_score":  result.Score,
		"checks":           result.Checks,
		"ai_summary":       result.AISummary,
	})
}

func (h *AssessmentsHandler) List(c *gin.Context) {
	// Ownership is enforced by the maker join inside the query.
	assessments, err := h.DB.ListReadinessAssessments(c.Request.Context(), makerID(c), c.Param("id"), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load assessments"})
		return
	}
	out := make([]gin.H, 0, len(assessments))
	for _, a := range assessments {
		checks := []services.ReadinessCheck{}
		_ = json.Unmarshal(a.ChecksJSON, &checks)
		item := gin.H{
			"id":         a.ID,
			"product_id": a.ProductID,
			"status":     a.Status,
			"score":      a.Score,
			"checks":     checks,
			"ai_summary": a.AISummary,
			"created_at": a.CreatedAt.Format(time.RFC3339),
		}
		if a.FinishedAt != nil {
			item["finished_at"] = a.FinishedAt.Format(time.RFC3339)
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"assessments": out})
}
