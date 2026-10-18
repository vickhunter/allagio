package services

import (
	"fmt"
	"sort"
	"strings"
)

// SiteProfile is a publisher's topical fingerprint, aggregated from data the
// demand side already produces: scanned article titles and the categories of
// the site's recommendations. It powers offer→publisher ranking — the reverse
// direction of the article→program matcher, reusing the same tokenizer.
type SiteProfile struct {
	TokenWeights map[string]float64 // token -> weight in (0,1], relative to the most frequent token
	Categories   map[string]int     // program category -> recommendation count
}

func BuildSiteProfile(titles []string, categories []string) SiteProfile {
	counts := map[string]int{}
	maxCount := 0
	for _, title := range titles {
		for _, token := range tokenize(title) {
			counts[token]++
			if counts[token] > maxCount {
				maxCount = counts[token]
			}
		}
	}
	weights := make(map[string]float64, len(counts))
	for token, count := range counts {
		weights[token] = float64(count) / float64(maxCount)
	}
	categoryCounts := map[string]int{}
	for _, category := range categories {
		if category != "" {
			categoryCounts[category]++
		}
	}
	return SiteProfile{TokenWeights: weights, Categories: categoryCounts}
}

func (p SiteProfile) Empty() bool {
	return len(p.TokenWeights) == 0
}

// OfferTerms is what an offer looks like to the profiler: the product's
// matcher taxonomy plus its copy.
type OfferTerms struct {
	Name     string
	Tagline  string
	Category string
	Keywords []string
	Tags     []string
}

// ScoreOfferForProfile ranks how well an offer fits a site, 0-100. Coverage of
// the offer's terms in the site's token weights carries most of the score; a
// category the site already recommends adds a bonus. An empty profile scores 0
// — no signal, not a bad fit.
func ScoreOfferForProfile(profile SiteProfile, offer OfferTerms) int {
	if profile.Empty() {
		return 0
	}
	terms := append([]string{offer.Name, offer.Tagline, offer.Category}, offer.Keywords...)
	terms = append(terms, offer.Tags...)
	seen := map[string]struct{}{}
	total := 0
	matched := 0.0
	for _, term := range terms {
		for _, token := range tokenize(term) {
			if _, dup := seen[token]; dup {
				continue
			}
			seen[token] = struct{}{}
			total++
			if weight, ok := profile.TokenWeights[token]; ok {
				matched += 0.5 + 0.5*weight // presence matters more than frequency
			}
		}
	}
	if total == 0 {
		return 0
	}
	score := 90 * matched / float64(total)
	if offer.Category != "" && profile.Categories[offer.Category] > 0 {
		score += 10
	}
	if score > 100 {
		score = 100
	}
	return int(score + 0.5)
}

// RankOffersForProfile orders offer indexes by descending profile fit,
// breaking ties by original order (which the caller keeps as recency).
func RankOffersForProfile(profile SiteProfile, offers []OfferTerms) ([]int, []int) {
	scores := make([]int, len(offers))
	order := make([]int, len(offers))
	for i, offer := range offers {
		scores[i] = ScoreOfferForProfile(profile, offer)
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return scores[order[a]] > scores[order[b]] })
	return order, scores
}

// RelevanceFloor is the lowest ScoreOfferForProfile that counts as a match.
// Placement search uses the same bar.
const RelevanceFloor = 25

// DryRunHit is the top offer for a page, and the slot that offer would use.
type DryRunHit struct {
	Index     int
	Relevance int
	Reason    string
	Slot      string
}

// DryRunMatch scores offers against page text and returns the top offer that
// clears RelevanceFloor, plus the slot that offer would use. It does not
// write anything. text is the extracted page text.
func DryRunMatch(text string, offers []OfferTerms) (DryRunHit, bool) {
	ranked := RankPageOffers(text, offers)
	if len(ranked) == 0 || ranked[0].Relevance < RelevanceFloor {
		return DryRunHit{}, false
	}
	return ranked[0], true
}

// RankPageOffers scores every offer against text, highest first. Equal scores
// keep the original order. Scores below RelevanceFloor stay in the list so a
// preview can show the closest topics instead of an empty result.
func RankPageOffers(text string, offers []OfferTerms) []DryRunHit {
	text = strings.TrimSpace(text)
	if text == "" || len(offers) == 0 {
		return nil
	}
	profile := BuildSiteProfile([]string{text}, nil)
	order := make([]int, len(offers))
	scores := make([]int, len(offers))
	for i, offer := range offers {
		order[i] = i
		scores[i] = ScoreOfferForProfile(profile, offer)
	}
	sort.SliceStable(order, func(a, b int) bool { return scores[order[a]] > scores[order[b]] })
	out := make([]DryRunHit, len(order))
	for n, idx := range order {
		score := scores[idx]
		reason := fmt.Sprintf("Product topics overlap this content (%d/100 lexical fit); review the product before recommending it.", score)
		if score < RelevanceFloor {
			reason = fmt.Sprintf("Closest topic overlap is %d/100. Review the product before recommending it.", score)
		}
		out[n] = DryRunHit{
			Index:     idx,
			Relevance: score,
			Reason:    reason,
			Slot:      matchSlot(text, offers[idx]),
		}
	}
	return out
}

func matchSlot(text string, offer OfferTerms) string {
	paragraphs := splitPreviewParagraphs(text)
	if len(paragraphs) < 2 {
		return slotByLength(text)
	}
	focus := map[string]struct{}{}
	terms := append([]string{offer.Name, offer.Tagline, offer.Category}, offer.Keywords...)
	terms = append(terms, offer.Tags...)
	for _, term := range terms {
		for _, token := range tokenize(term) {
			focus[token] = struct{}{}
		}
	}
	if len(focus) == 0 {
		return slotByLength(text)
	}
	bestIdx := 0
	best := -1
	for i, paragraph := range paragraphs {
		score := 0
		for _, token := range tokenize(paragraph) {
			if _, ok := focus[token]; ok {
				score++
			}
		}
		if score > best {
			best = score
			bestIdx = i
		}
	}
	ratio := float64(bestIdx+1) / float64(len(paragraphs))
	switch {
	case ratio <= 0.25:
		return "after_intro"
	case ratio <= 0.75:
		return "mid_article"
	default:
		return "before_conclusion"
	}
}

func splitPreviewParagraphs(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	out := make([]string, 0, len(raw))
	for _, paragraph := range raw {
		trimmed := strings.TrimSpace(paragraph)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func slotByLength(text string) string {
	words := len(strings.Fields(strings.TrimSpace(text)))
	switch {
	case words > 1400:
		return "mid_article"
	case words > 700:
		return "before_conclusion"
	case words > 250:
		return "after_intro"
	default:
		return "end_of_article"
	}
}
