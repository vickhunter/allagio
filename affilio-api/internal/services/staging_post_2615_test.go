package services

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// stagingPost2615FixtureSHA is the hosted post body used with the hosted Demo offer.
const stagingPost2615FixtureSHA = "4a2fff3f8e223bd0f4493b4da115e3df8532f613118fbbbf0e598e7c37ecf1d5"

// hostedDemoOfferTerms is the hosted Demo offer as /v1/revenue/preview scores it:
// product name, tagline, primary category, and keywords. Tags are empty.
// product_type "other" and the offer terms text "Commission paid directly by merchant."
// are stored on the rows and are not passed into OfferTerms.
func hostedDemoOfferTerms() OfferTerms {
	return OfferTerms{
		Name:     "Offer Rails Demo",
		Tagline:  "Demo landing page builder: launch a landing page without code",
		Category: "landing page builder",
		Keywords: []string{"landing page", "landing page builder", "page builder", "no-code", "templates", "conversion", "lead generation", "email marketing"},
	}
}

func TestStagingPost2615ScoresTheHostedDemoOffer(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "staging_post_2615.txt"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != stagingPost2615FixtureSHA {
		t.Fatalf("fixture sha256 %s, want %s", got, stagingPost2615FixtureSHA)
	}
	score := ScoreOfferForProfile(BuildSiteProfile([]string{string(raw)}, nil), hostedDemoOfferTerms())
	if score < 32 {
		t.Fatalf("hosted Demo offer relevance %d, want >= 32", score)
	}
}
