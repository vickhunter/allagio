package entry

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

const listingBodyLimit = 8192

// ListingCard is the only scan payload allowed to leave the agent's machine.
type ListingCard struct {
	Name           string   `json:"name"`
	Summary        string   `json:"summary"`
	Category       string   `json:"category"`
	Features       []string `json:"features"`
	PricingSummary string   `json:"pricing_summary"`
}

// ListingSignal is one readiness check.
type ListingSignal struct {
	Name string `json:"name"`
	Pass bool   `json:"pass"`
}

// ListingUpload is the strict body of POST /v1/listing-cards.
type ListingUpload struct {
	ProductID       string          `json:"product_id"`
	Card            ListingCard     `json:"card"`
	Signals         []ListingSignal `json:"signals"`
	Score           int             `json:"score"`
	PaymentsCheck   string          `json:"payments_check"`
	OfferLinkHealth string          `json:"offer_link_health"`
}

var (
	secretPattern = regexp.MustCompile(`(?i)(sk_live_|sk_test_|rk_live_|pk_live_|whsec_|api[_-]?key\s*[:=]|secret\s*[:=]|password\s*[:=]|-----BEGIN |ghp_|github_pat_|xox[baprs]-|AKIA[0-9A-Z]{16}|eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,})`)
	sourcePattern = regexp.MustCompile(`(?i)(\bfunc\s+\w+\s*\(|\bpackage\s+[a-z0-9_]+|<\?php|<\s*script\b|\bfunction\s+\w+\s*\(|\bclass\s+\w+\s*\{|#include\s*<|\bdef\s+\w+\s*\(|\bimport\s+[a-z0-9_.{])`)
)

var errListingRejected = errors.New("listing card rejected")

// ParseListingUpload checks the schema, size caps, and the secret/source denylist.
// The error is generic so a rejected secret is not echoed back.
func ParseListingUpload(body []byte) (ListingUpload, error) {
	if len(body) == 0 || len(body) > listingBodyLimit {
		return ListingUpload{}, errListingRejected
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	var upload ListingUpload
	if err := dec.Decode(&upload); err != nil {
		return ListingUpload{}, errListingRejected
	}
	var extra any
	if err := dec.Decode(&extra); err != nil && err.Error() != "EOF" {
		return ListingUpload{}, errListingRejected
	}
	if err := validateListing(upload); err != nil {
		return ListingUpload{}, err
	}
	if looksSensitive(string(body)) {
		return ListingUpload{}, errListingRejected
	}
	return upload, nil
}

func validateListing(upload ListingUpload) error {
	if upload.ProductID != "" && !uuidLike(upload.ProductID) {
		return errListingRejected
	}
	card := upload.Card
	if card.Name == "" || utf8.RuneCountInString(card.Name) > 80 {
		return errListingRejected
	}
	if card.Summary == "" || utf8.RuneCountInString(card.Summary) > 280 {
		return errListingRejected
	}
	if card.Category == "" || utf8.RuneCountInString(card.Category) > 64 {
		return errListingRejected
	}
	if utf8.RuneCountInString(card.PricingSummary) > 80 {
		return errListingRejected
	}
	if len(card.Features) > 8 {
		return errListingRejected
	}
	for _, feature := range card.Features {
		if feature == "" || utf8.RuneCountInString(feature) > 80 {
			return errListingRejected
		}
	}
	if len(upload.Signals) < 1 || len(upload.Signals) > 32 {
		return errListingRejected
	}
	seen := map[string]bool{}
	for _, signal := range upload.Signals {
		name := strings.TrimSpace(signal.Name)
		if name == "" || utf8.RuneCountInString(name) > 32 || seen[strings.ToLower(name)] {
			return errListingRejected
		}
		seen[strings.ToLower(name)] = true
	}
	if upload.Score < 0 || upload.Score > 100 {
		return errListingRejected
	}
	switch upload.PaymentsCheck {
	case "pass", "fail", "unknown":
	default:
		return errListingRejected
	}
	switch upload.OfferLinkHealth {
	case "ok", "broken", "unknown":
	default:
		return errListingRejected
	}
	return nil
}

func looksSensitive(raw string) bool {
	return secretPattern.MatchString(raw) || sourcePattern.MatchString(raw)
}

func uuidLike(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				return false
			}
		}
	}
	return true
}
