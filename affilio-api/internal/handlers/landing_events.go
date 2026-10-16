package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const landingEventMaxBytes = 1024

var landingEventNames = map[string]struct{}{
	"landing_view": {},
	"signin_click": {},
	"zip_download": {},
}

func (h *HostedHandler) RecordLandingEvent(c *gin.Context) {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		ip = "unknown"
	}
	if h.EventLimiter != nil && !h.EventLimiter.Allow(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, landingEventMaxBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || strings.Contains(err.Error(), "request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(body))
	if err = dec.Decode(&raw); err != nil || dec.More() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if len(raw) == 0 || len(raw) > 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	for key := range raw {
		if key != "event" && key != "source" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
	}
	eventRaw, ok := raw["event"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	var event string
	if err = json.Unmarshal(eventRaw, &event); err != nil || !landingEventAllowed(event) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event"})
		return
	}
	source := ""
	if sourceRaw, exists := raw["source"]; exists {
		var given string
		if err = json.Unmarshal(sourceRaw, &given); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		given = strings.TrimSpace(given)
		if given != "" {
			source = landingSourceTag(given)
			if source == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event"})
				return
			}
		}
	}
	if source == "" {
		source = landingSourceFrom(c)
	}
	if h.DB == nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if err = h.DB.InsertLandingEvent(c.Request.Context(), event, source); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

func landingEventAllowed(name string) bool {
	_, ok := landingEventNames[name]
	return ok
}
