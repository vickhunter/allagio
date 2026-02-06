package handlers

import (
	"net/http"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type ProgramsHandler struct {
	DB *database.DB
}

func (h *ProgramsHandler) List(c *gin.Context) {
	rows, err := h.DB.Pool.Query(c.Request.Context(), `SELECT id, name, company, slug, description, website_url FROM affiliate_programs WHERE status='active' ORDER BY program_quality_score DESC LIMIT 100`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load programs"})
		return
	}
	defer rows.Close()
	programs := []map[string]interface{}{}
	for rows.Next() {
		var id, name, company, slug, desc, url string
		if err := rows.Scan(&id, &name, &company, &slug, &desc, &url); err != nil {
			continue
		}
		programs = append(programs, gin.H{
			"id":          id,
			"name":        name,
			"company":     company,
			"slug":        slug,
			"description": desc,
			"website_url": url,
		})
	}
	c.JSON(http.StatusOK, gin.H{"programs": programs})
}

func (h *ProgramsHandler) Search(c *gin.Context) {
	q := c.Query("q")
	rows, err := h.DB.Pool.Query(c.Request.Context(), `SELECT id, name, company, slug FROM affiliate_programs WHERE status='active' AND (name ILIKE $1 OR company ILIKE $1) LIMIT 50`, "%"+q+"%")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}
	defer rows.Close()
	programs := []map[string]interface{}{}
	for rows.Next() {
		var id, name, company, slug string
		if err := rows.Scan(&id, &name, &company, &slug); err != nil {
			continue
		}
		programs = append(programs, gin.H{"id": id, "name": name, "company": company, "slug": slug})
	}
	c.JSON(http.StatusOK, gin.H{"programs": programs})
}
