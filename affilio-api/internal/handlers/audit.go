package handlers

import (
	"net/http"
	"strconv"

	"affilio-api/internal/database"
	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	DB *database.DB
}

func (h *AuditHandler) List(c *gin.Context) {
	limit := intQueryDefault(c, "limit", 50)
	offset := intQueryDefault(c, "offset", 0)
	eventType := c.Query("event_type")
	severity := c.Query("severity")

	events, total, err := h.DB.ListAuditEvents(c.Request.Context(), eventType, severity, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit events"})
		return
	}
	if events == nil {
		events = []database.AuditEvent{}
	}
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  total,
		"pagination": gin.H{
			"limit":  limit,
			"offset": offset,
		},
	})
}

func (h *AuditHandler) Alerts(c *gin.Context) {
	// Recent high-severity events
	warnings, _, err := h.DB.ListAuditEvents(c.Request.Context(), "", "warn", 20, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load alerts"})
		return
	}
	errors, _, err2 := h.DB.ListAuditEvents(c.Request.Context(), "", "error", 20, 0)
	if err2 != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load alerts"})
		return
	}

	if warnings == nil {
		warnings = []database.AuditEvent{}
	}
	if errors == nil {
		errors = []database.AuditEvent{}
	}

	// Check for IPs with repeated auth failures (>5 in last 15 min)
	suspiciousIPs := []map[string]interface{}{}
	seen := map[string]bool{}
	for _, ev := range warnings {
		if ev.IPAddress == "" || ev.IPAddress == "unknown" || seen[ev.IPAddress] {
			continue
		}
		seen[ev.IPAddress] = true
		count, cerr := h.DB.CountRecentAuthFailures(c.Request.Context(), ev.IPAddress, 15)
		if cerr == nil && count >= 5 {
			suspiciousIPs = append(suspiciousIPs, map[string]interface{}{
				"ip":              ev.IPAddress,
				"failures_15_min": count,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"recent_warnings":  warnings,
		"recent_errors":    errors,
		"suspicious_ips":   suspiciousIPs,
	})
}

func intQueryDefault(c *gin.Context, key string, def int) int {
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
