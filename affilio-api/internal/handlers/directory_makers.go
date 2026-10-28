package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"

	"affilio-api/internal/database"

	"github.com/gin-gonic/gin"
)

// DirectoryMakers is GET /v1/directory/makers.
// A reachable database with no public directory rows is an empty list.
// The makers account table holds emails and keys, so it is not this list.
// A failed database read is 503. That is not an empty list.
func (h *HostedHandler) DirectoryMakers(c *gin.Context) {
	if err := directoryMakersReady(c.Request.Context(), h.DB); err != nil {
		log.Printf("directory makers unavailable")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "directory unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":       []any{},
		"next_cursor": nil,
		"total":       0,
	})
}

func directoryMakersReady(ctx context.Context, db *database.DB) error {
	if db == nil || db.Pool == nil {
		return errors.New("directory unavailable")
	}
	var one int
	if err := db.Pool.QueryRow(ctx, `SELECT 1`).Scan(&one); err != nil {
		return err
	}
	if one != 1 {
		return errors.New("directory unavailable")
	}
	return nil
}
