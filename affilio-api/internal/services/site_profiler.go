package services

import "sort"

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
