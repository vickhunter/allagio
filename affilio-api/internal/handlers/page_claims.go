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

// liveMerchantPage reads the merchant page through the readiness fetcher
// (10s timeout, 512 KiB cap, public addresses only). A private, loopback,
// or link-local host is not dialed. A miss is an unreachable page, not a 500.
// Results are reused for 10 minutes.
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
		merchantCache.Delete(rawURL)
	}
	body, _, ok := (&services.ReadinessEngine{}).FetchPublicPage(ctx, rawURL)
	page := merchantPage{OK: ok, Body: body}
	merchantCache.Store(rawURL, merchantCacheEntry{page: page, at: time.Now()})
	return page
}

func compareOfferClaims(ctx context.Context, db *database.DB, makerID, productID string, offer database.Offer) (string, string, error) {
	in := entry.PageClaimInput{Terms: offer.Terms, CommissionValue: offer.CommissionValue}
	if db != nil && makerID != "" && productID != "" {
		raw, err := db.LatestListingCardJSON(ctx, makerID, productID)
		if err != nil {
			return "", "", err
		}
		if len(raw) > 0 {
			var card entry.ListingCard
			if err := json.Unmarshal(raw, &card); err != nil {
				return "", "", err
			}
			in.Card = card
			in.HasCard = true
		}
	}
	page := fetchMerchantPage(ctx, offer.LandingURL)
	reason, message := (entry.PageClaimChecker{}).Check(entry.PageDocument{OK: page.OK, Body: page.Body}, in)
	return reason, message, nil
}
