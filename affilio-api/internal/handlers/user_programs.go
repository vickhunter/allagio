package handlers

import (
	"net/http"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type UserProgramsHandler struct {
	DB *database.DB
}

type userProgramCreateRequest struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Link   string `json:"affiliate_link"`
}

type importRequest struct {
	Detections []struct {
		URL          string `json:"url"`
		Domain       string `json:"domain"`
		IsAffiliate  bool   `json:"is_affiliate"`
		ProgramGuess string `json:"program_guess"`
	} `json:"detections"`
}

func (h *UserProgramsHandler) List(c *gin.Context) {
	siteID := c.GetString("site_id")
	programs, err := h.DB.ListUserPrograms(c.Request.Context(), siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"programs": programs})
}

func (h *UserProgramsHandler) Create(c *gin.Context) {
	var req userProgramCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	siteID := c.GetString("site_id")
	id, err := h.DB.CreateUserProgram(c.Request.Context(), siteID, req.Name, database.NormalizeDomain(req.Domain), req.Link)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *UserProgramsHandler) Delete(c *gin.Context) {
	siteID := c.GetString("site_id")
	if err := h.DB.DeleteUserProgram(c.Request.Context(), siteID, c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *UserProgramsHandler) Import(c *gin.Context) {
	var req importRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	siteID := c.GetString("site_id")
	imported := 0
	for _, det := range req.Detections {
		if !det.IsAffiliate || det.Domain == "" {
			continue
		}
		name := det.ProgramGuess
		if name == "" {
			name = det.Domain
		}
		_, err := h.DB.UpsertUserProgram(c.Request.Context(), siteID, name, det.Domain, det.ProgramGuess, det.URL)
		if err == nil {
			imported += 1
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "imported", "count": imported})
}
