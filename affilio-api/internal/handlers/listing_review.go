package handlers

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"affilio-api/internal/config"
	"affilio-api/internal/database"
	"affilio-api/internal/entry"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

const (
	listingMinRelevance = 25
	listingCap          = 3

	reviewReasonClaims    = "claims"
	reviewReasonHead      = "head"
	reviewReasonRelevance = "relevance"
	reviewReasonCaps      = "caps"

	reviewClaimsMessage    = "Use only claims from the offer."
	reviewClaimsNotLive    = "Claims are not checked yet."
	reviewHeadMessage      = "The offer page did not answer."
	reviewRelevanceMessage = "Relevance is below the minimum."
	reviewCapsMessage      = "The placement cap is full."
	readinessFailMessage   = "This product has not passed readiness assessment. Fix the failed checks, then submit again."
)

// failedListingCheck is the server gate for an untrusted maker. The order is
// claims, the cached HEAD check, minimum relevance, then the cap. A trusted
// maker does not use this gate. real replaces the numeric stub with the page
// checker. stub and off-local keep the numeric stub.
func failedListingCheck(ctx context.Context, db *database.DB, mode string, offer database.Offer, product database.Product, listed int) (string, string, bool) {
	if claimsReal(mode) {
		reason, message := compareOfferClaims(ctx, db, product.MakerID, product.ID, offer)
		if reason == entry.PageReasonUnreachable {
			return reviewReasonHead, reviewHeadMessage, true
		}
		if reason != "" {
			return reviewReasonClaims, message, true
		}
	} else {
		claims := entry.StubClaimChecker{}
		if claims.CheckClaims(offer.Terms, offerFacts(offer, product)) != nil {
			return reviewReasonClaims, reviewClaimsMessage, true
		}
	}
	if !landingReachable(offer.LandingURL) {
		return reviewReasonHead, reviewHeadMessage, true
	}
	score := services.ScorePlacement(offer.Terms, product.Name, product.PrimaryCategory, offer.LandingURL)
	if score < listingMinRelevance {
		return reviewReasonRelevance, reviewRelevanceMessage, true
	}
	if listed >= listingCap {
		return reviewReasonCaps, reviewCapsMessage, true
	}
	return "", "", false
}

// claimsHeld is true only for stub. Empty and off-local run the four checks.
// real runs the page checker instead of the not-live hold.
func claimsHeld(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), config.ClaimsStub)
}

func claimsReal(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), config.ClaimsReal)
}

// claimsCheckValue is the offer field. stub stays not_checked. real and
// off-local report passed or failed once a draft has been submitted.
func claimsCheckValue(mode, status, reviewReason string) string {
	if claimsHeld(mode) {
		return "not_checked"
	}
	switch strings.TrimSpace(status) {
	case "", "draft":
		return "not_checked"
	}
	if reviewReason == reviewReasonClaims {
		return "failed"
	}
	return "passed"
}

func attachClaimsCheck(item gin.H, mode, status, reviewReason string) {
	item["claims_check"] = claimsCheckValue(mode, status, reviewReason)
}

// listingReview is recomputed on read. It is not stored. Under stub, every
// submitted offer reports claims and the not-live message. That includes an
// offer that failed a different check the last time this process ran off-local.
// real reports the page checker's reason.
func listingReview(ctx context.Context, db *database.DB, mode string, offer database.Offer, product database.Product, listed int) (string, string) {
	if offer.Status != "submitted" {
		return "", ""
	}
	if claimsHeld(mode) {
		return reviewReasonClaims, reviewClaimsNotLive
	}
	reason, message, failed := failedListingCheck(ctx, db, mode, offer, product, listed)
	if !failed {
		return "", ""
	}
	return reason, message
}

func offerFacts(offer database.Offer, product database.Product) string {
	parts := []string{
		product.Name,
		product.Tagline,
		product.PrimaryCategory,
		product.WebsiteURL,
		offer.LandingURL,
		offer.CommissionType,
		offer.CommissionCurrency,
		strconv.FormatFloat(offer.CommissionValue, 'f', -1, 64),
		offer.RefParam,
	}
	parts = append(parts, product.Keywords...)
	parts = append(parts, product.Tags...)
	return strings.Join(parts, " ")
}

func (h *OffersHandler) readinessReason(ctx context.Context, product *database.Product) (string, string) {
	assessments, err := h.DB.ListReadinessAssessments(ctx, product.MakerID, product.ID, 1)
	if err != nil || len(assessments) == 0 {
		return reviewReasonHead, readinessFailMessage
	}
	var checks []services.ReadinessCheck
	if err := json.Unmarshal(assessments[0].ChecksJSON, &checks); err != nil || len(checks) == 0 {
		return reviewReasonHead, readinessFailMessage
	}
	if reason, message, ok := mapReadinessChecks(checks); ok {
		return reason, message
	}
	return reviewReasonHead, readinessFailMessage
}

func mapReadinessChecks(checks []services.ReadinessCheck) (string, string, bool) {
	failed := map[string]services.ReadinessCheck{}
	for _, check := range checks {
		if check.Gate && check.Status == "fail" {
			failed[check.Key] = check
		}
	}
	order := []struct {
		reason string
		keys   []string
	}{
		{reviewReasonClaims, []string{"terms", "privacy_policy", "checkout_present"}},
		{reviewReasonHead, []string{"site_reachable", "ref_param_survives"}},
		{reviewReasonRelevance, []string{"pricing_page", "docs_present", "onboarding_clarity", "docs_quality", "value_prop_clarity"}},
	}
	for _, bucket := range order {
		for _, key := range bucket.keys {
			check, ok := failed[key]
			if !ok {
				continue
			}
			message := strings.TrimSpace(check.Remediation)
			if message == "" {
				message = reviewMessage(bucket.reason)
			}
			return bucket.reason, message, true
		}
	}
	for _, check := range failed {
		message := strings.TrimSpace(check.Remediation)
		if message == "" {
			message = readinessFailMessage
		}
		return reviewReasonHead, message, true
	}
	return "", "", false
}

func reviewMessage(reason string) string {
	switch reason {
	case reviewReasonClaims:
		return reviewClaimsMessage
	case reviewReasonHead:
		return reviewHeadMessage
	case reviewReasonRelevance:
		return reviewRelevanceMessage
	case reviewReasonCaps:
		return reviewCapsMessage
	default:
		return readinessFailMessage
	}
}

func nullReview(reason, message string) (any, any) {
	if reason == "" {
		return nil, nil
	}
	return reason, message
}

func reviewForMakerProduct(ctx context.Context, db *database.DB, claimsCheck, makerID, productID string) (string, string) {
	if makerID == "" || productID == "" {
		return "", ""
	}
	offers, err := db.ListOffers(ctx, makerID)
	if err != nil {
		return "", ""
	}
	listed, err := db.CountListedOffers(ctx, makerID)
	if err != nil {
		return "", ""
	}
	var product *database.Product
	for i := range offers {
		if offers[i].ProductID != productID || offers[i].Status != "submitted" {
			continue
		}
		if claimsHeld(claimsCheck) {
			return reviewReasonClaims, reviewClaimsNotLive
		}
		if product == nil {
			product, err = db.GetProduct(ctx, makerID, productID)
			if err != nil {
				return "", ""
			}
		}
		return listingReview(ctx, db, claimsCheck, offers[i], *product, listed)
	}
	return "", ""
}
