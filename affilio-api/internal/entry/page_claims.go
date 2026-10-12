package entry

import (
	"log"
	"regexp"
	"strconv"
	"strings"
)

const (
	// PageReasonUnreachable means the merchant page did not answer.
	PageReasonUnreachable = "page_unreachable"
	// PageReasonCard means a listing-card claim is missing from the page.
	PageReasonCard = "card_claim_not_on_page"
	// PageReasonPrice means the card's pricing summary is missing from the page.
	PageReasonPrice = "price_not_on_page"
	// PageReasonCommission means the offer's commission number is missing from the page.
	PageReasonCommission = "commission_not_on_page"
	// PageReasonTerms means a number in the offer terms is missing from the page.
	PageReasonTerms = "terms_not_on_page"
)

// PageDocument is the merchant page the checker compares against.
// OK is false when the cached HEAD or the GET did not return a page.
type PageDocument struct {
	OK   bool
	Body string
}

// PageClaimInput is what the listing card and the offer assert.
// HasCard is false when the product has no stored listing card.
// Commission and terms are still checked.
type PageClaimInput struct {
	Card            ListingCard
	HasCard         bool
	Terms           string
	CommissionValue float64
}

// PageClaimMessage is the sentence for a page-claim reason.
func PageClaimMessage(reason string) string {
	switch reason {
	case PageReasonCard:
		return "A claim on the listing card is not on the merchant page."
	case PageReasonPrice:
		return "The price on the listing card is not on the merchant page."
	case PageReasonCommission:
		return "The commission is not on the merchant page."
	case PageReasonTerms:
		return "The terms are not on the merchant page."
	case PageReasonUnreachable:
		return "The merchant page did not answer."
	default:
		return "Use only claims from the offer."
	}
}

// CheckPageClaims compares the listing card, the commission number, and the
// numbers in the terms with the merchant page. The same inputs always return
// the same reason. It does not call the network or a paid API.
// The first miss wins, in this order: page, card name, card category, card
// summary, each feature, pricing summary, commission, then terms.
func CheckPageClaims(doc PageDocument, in PageClaimInput) (reason, message string) {
	if !doc.OK {
		return failPage(PageReasonUnreachable)
	}
	page := normalizePage(doc.Body)
	if in.HasCard {
		card := in.Card
		if !pageHasPhrase(page, card.Name) || !pageHasPhrase(page, card.Category) || !pageHasPhrase(page, card.Summary) {
			return failPage(PageReasonCard)
		}
		for _, feature := range card.Features {
			if !pageHasPhrase(page, feature) {
				return failPage(PageReasonCard)
			}
		}
		if strings.TrimSpace(card.PricingSummary) != "" && !pageHasPhrase(page, card.PricingSummary) {
			return failPage(PageReasonPrice)
		}
	}
	if in.CommissionValue > 0 {
		token := strconv.FormatFloat(in.CommissionValue, 'f', -1, 64)
		if !pageHasNumber(page, token) {
			return failPage(PageReasonCommission)
		}
	}
	for _, token := range claimToken.FindAllString(in.Terms, -1) {
		if !pageHasNumber(page, token) {
			return failPage(PageReasonTerms)
		}
	}
	return "", ""
}

func failPage(reason string) (string, string) {
	log.Printf("claim check failed reason=%s", reason)
	return reason, PageClaimMessage(reason)
}

// RecordedClaimCheck is a page-claim result computed before the database
// transaction. Restore and retry still call CheckClaims inside that
// transaction, so a miss leaves the row unchanged. Real is false, so this
// checker does not turn yolo or auto on.
type RecordedClaimCheck struct {
	Err error
}

func (r RecordedClaimCheck) CheckClaims(string, string) error { return r.Err }
func (RecordedClaimCheck) Real() bool                         { return false }

var pageTag = regexp.MustCompile(`<[^>]*>`)

func normalizePage(raw string) string {
	s := strings.ToLower(raw)
	s = pageTag.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	return strings.Join(strings.Fields(s), " ")
}

func pageHasPhrase(page, phrase string) bool {
	phrase = normalizePage(phrase)
	if phrase == "" {
		return true
	}
	return strings.Contains(page, phrase)
}

func pageHasNumber(page, token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	token = strings.TrimSuffix(token, "%")
	if token == "" {
		return true
	}
	for i := 0; i+len(token) <= len(page); {
		if page[i:i+len(token)] != token {
			i++
			continue
		}
		if i > 0 && isDigitByte(page[i-1]) {
			i++
			continue
		}
		end := i + len(token)
		if end < len(page) && isDigitByte(page[end]) {
			i++
			continue
		}
		if end < len(page) && (page[end] == '.' || page[end] == ',') && end+1 < len(page) && isDigitByte(page[end+1]) {
			i++
			continue
		}
		return true
	}
	return false
}

func isDigitByte(b byte) bool { return b >= '0' && b <= '9' }
