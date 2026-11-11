package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

// DirectoryMakers is GET /v1/directory/makers. It lists the parties whose
// standing offers are listed — data an ORP discover already returns — so a
// caller can browse the marketplace without speaking the envelope. The
// makers account table holds emails and keys and is never read here.
// A failed database read is 503. That is not an empty list.
func (h *HostedHandler) DirectoryMakers(c *gin.Context) {
	if err := directoryMakersReady(c.Request.Context(), h.DB); err != nil {
		log.Printf("directory makers unavailable: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "directory unavailable"})
		return
	}
	offers, err := h.DB.ListORPOffers(c.Request.Context(), 200)
	if err != nil {
		log.Printf("directory makers list: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "directory unavailable"})
		return
	}
	now := time.Now()
	type row struct {
		count  int
		titles []any
	}
	byParty := map[string]*row{}
	order := []string{}
	for _, offer := range offers {
		terms, err := decodeObject(offer.Terms)
		if err != nil || orp.OfferExpired(terms, now) {
			continue
		}
		entry, seen := byParty[offer.Party]
		if !seen {
			entry = &row{titles: []any{}}
			byParty[offer.Party] = entry
			order = append(order, offer.Party)
		}
		entry.count++
		if len(entry.titles) < 5 {
			if title := orp.TermsTitle(terms); title != "" {
				entry.titles = append(entry.titles, title)
			}
		}
	}
	items := make([]any, 0, len(order))
	for _, party := range order {
		entry := byParty[party]
		items = append(items, map[string]any{
			"party":       party,
			"offer_count": entry.count,
			"titles":      entry.titles,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"next_cursor": nil,
		"total":       len(items),
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
