package services

import (
	"strings"
	"testing"
)

func TestDryRunMatchTakesTextAndReturnsTopOffer(t *testing.T) {
	text := "InboxPilot email inbox productivity for founders who live in their inbox."
	offers := []OfferTerms{
		{Name: "PixelQuest", Tagline: "email inbox tool", Category: "productivity", Keywords: []string{"game", "retro", "platformer"}},
		{Name: "InboxPilot", Tagline: "email inbox tool", Category: "productivity", Keywords: []string{"email", "inbox", "productivity"}},
	}
	hit, ok := DryRunMatch(text, offers)
	if !ok {
		t.Fatal("expected a match")
	}
	if hit.Index != 1 || hit.Relevance < RelevanceFloor {
		t.Fatalf("hit = %+v", hit)
	}
	if !strings.Contains(hit.Reason, "lexical fit") || hit.Slot != "end_of_article" {
		t.Fatalf("hit = %+v", hit)
	}

	miss, ok := DryRunMatch("Gardening tips for tomatoes in small gardens and watering herbs.", offers)
	if ok || miss.Slot != "" {
		t.Fatalf("unrelated text matched: %+v", miss)
	}
}

func TestDryRunMatchSlotFollowsTheMatchingParagraph(t *testing.T) {
	text := strings.Join([]string{
		"InboxPilot email inbox productivity is the topic of this opening.",
		"Tomatoes need steady water in a small garden.",
		"Herbs prefer a lighter soil than tomatoes.",
		"A watering schedule keeps the garden even.",
	}, "\n\n")
	hit, ok := DryRunMatch(text, []OfferTerms{{
		Name: "InboxPilot", Tagline: "email inbox tool", Category: "productivity", Keywords: []string{"email", "inbox", "productivity"},
	}})
	if !ok || hit.Slot != "after_intro" {
		t.Fatalf("hit = %+v", hit)
	}
}

func TestRankPageOffersKeepsThreeClosestBelowTheFloor(t *testing.T) {
	text := "Gardening tips for tomatoes in small gardens and watering herbs."
	noise := []string{"quilt", "marble", "cipher", "velvet", "orbit", "lantern", "cinder", "plume", "sable", "ivory"}
	offers := []OfferTerms{
		{Name: "QuiltNone", Keywords: append([]string{"quilt"}, noise...)},
		{Name: "GardenLow", Keywords: append([]string{"garden"}, noise...)},
		{Name: "HerbMid", Keywords: append([]string{"garden", "herb"}, noise...)},
		{Name: "WaterMost", Keywords: append([]string{"garden", "herb", "water"}, noise...)},
	}
	ranked := RankPageOffers(text, offers)
	if len(ranked) != 4 {
		t.Fatalf("ranked %d", len(ranked))
	}
	if ranked[0].Index != 3 || ranked[1].Index != 2 || ranked[2].Index != 1 || ranked[3].Index != 0 {
		t.Fatalf("order = %+v", ranked)
	}
	if ranked[0].Relevance >= RelevanceFloor || ranked[0].Relevance <= ranked[1].Relevance {
		t.Fatalf("closest = %+v next %+v", ranked[0], ranked[1])
	}
	if !strings.Contains(ranked[0].Reason, "Closest topic overlap") || strings.Contains(strings.ToLower(ranked[0].Reason), "no match") {
		t.Fatalf("reason = %q", ranked[0].Reason)
	}
	if _, ok := DryRunMatch(text, offers); ok {
		t.Fatal("below-floor page matched")
	}
}
