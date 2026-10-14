package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/services"
)

// merchantPage is one fetched merchant page. Tests install a fixture fetcher
// so CI never dials. Err is set only by that test fetcher.
type merchantPage struct {
	OK   bool
	Body string
	Err  string
}

type merchantFetcher func(context.Context, string) merchantPage

var fetchMerchantPage merchantFetcher = liveMerchantPage

func init() {
	if testing.Testing() {
		fetchMerchantPage = func(context.Context, string) merchantPage {
			return merchantPage{Err: "fixture required"}
		}
	}
}

type merchantCacheEntry struct {
	page merchantPage
	at   time.Time
}

var merchantCache sync.Map

// liveMerchantPage uses the cached HEAD probe (10 minutes, 1.5s timeout) and
// the readiness GET (10s timeout, 512 KiB cap). A HEAD miss does not GET.
func liveMerchantPage(ctx context.Context, rawURL string) merchantPage {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return merchantPage{}
	}
	if cached, ok := merchantCache.Load(rawURL); ok {
		saved := cached.(merchantCacheEntry)
		if time.Since(saved.at) < 10*time.Minute {
			return saved.page
		}
	}
	page := merchantPage{}
	if probeLanding(rawURL) {
		body, _, ok := (&services.ReadinessEngine{}).FetchPublicPage(ctx, rawURL)
		page = merchantPage{OK: ok, Body: body}
	}
	merchantCache.Store(rawURL, merchantCacheEntry{page: page, at: time.Now()})
	return page
}

func compareOfferClaims(ctx context.Context, db *database.DB, makerID, productID string, offer database.Offer) (string, string) {
	in := entry.PageClaimInput{Terms: offer.Terms, CommissionValue: offer.CommissionValue}
	if db != nil && makerID != "" && productID != "" {
		raw, err := db.LatestListingCardJSON(ctx, makerID, productID)
		if err == nil && len(raw) > 0 {
			var card entry.ListingCard
			if json.Unmarshal(raw, &card) == nil {
				in.Card = card
				in.HasCard = true
			}
		}
	}
	page := fetchMerchantPage(ctx, offer.LandingURL)
	return entry.CheckPageClaims(entry.PageDocument{OK: page.OK, Body: page.Body}, in)
}
