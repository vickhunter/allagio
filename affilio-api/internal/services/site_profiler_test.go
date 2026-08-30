package services

import "testing"

func TestSiteProfileRanksTopicalOffersHigher(t *testing.T) {
	profile := BuildSiteProfile([]string{
		"How I organize my email inbox for productivity",
		"The best email tools for busy founders",
		"Inbox zero: a practical productivity guide",
	}, []string{"software", "software", "marketing"})

	emailTool := OfferTerms{
		Name: "InboxPilot", Tagline: "Email productivity for founders",
		Category: "software", Keywords: []string{"email", "inbox", "productivity"},
	}
	gameTool := OfferTerms{
		Name: "PixelQuest", Tagline: "A retro platformer adventure",
		Category: "games", Keywords: []string{"game", "retro", "platformer"},
	}
	emailScore := ScoreOfferForProfile(profile, emailTool)
	gameScore := ScoreOfferForProfile(profile, gameTool)
	if emailScore <= gameScore {
		t.Fatalf("expected topical offer to outrank unrelated one: email=%d game=%d", emailScore, gameScore)
	}
	if emailScore < 50 {
		t.Fatalf("expected strong fit score, got %d", emailScore)
	}

	order, scores := RankOffersForProfile(profile, []OfferTerms{gameTool, emailTool})
	if order[0] != 1 {
		t.Fatalf("expected email offer ranked first, got order %v scores %v", order, scores)
	}
}

func TestSiteProfileCategoryBonus(t *testing.T) {
	titles := []string{"Email productivity tips", "Inbox management for founders"}
	withCategory := BuildSiteProfile(titles, []string{"software"})
	withoutCategory := BuildSiteProfile(titles, nil)

	offer := OfferTerms{Name: "InboxPilot", Category: "software", Keywords: []string{"email", "inbox"}}
	if ScoreOfferForProfile(withCategory, offer) <= ScoreOfferForProfile(withoutCategory, offer) {
		t.Fatalf("expected category bonus to raise the score")
	}
}

func TestSiteProfileEmptyScoresZero(t *testing.T) {
	profile := BuildSiteProfile(nil, nil)
	if !profile.Empty() {
		t.Fatalf("expected empty profile")
	}
	if score := ScoreOfferForProfile(profile, OfferTerms{Name: "Anything"}); score != 0 {
		t.Fatalf("expected 0 for empty profile, got %d", score)
	}
}
