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
