package services

import (
	"strings"
	"testing"
)

func TestStripDraftPIIRemovesEmailHandleAndPhone(t *testing.T) {
	in := "This AI writing tool drafts posts. Reach ada@secret.example or @ada_private at +1 415 555 0199."
	out := StripDraftPII(in)
	for _, secret := range []string{"ada@secret.example", "secret.example", "@ada_private", "ada_private", "415", "555", "0199"} {
		if strings.Contains(out, secret) {
			t.Fatalf("stripped text still has %q: %q", secret, out)
		}
	}
	if !strings.Contains(out, "AI writing tool") {
		t.Fatalf("topic was removed: %q", out)
	}
}

func TestStripDraftPIILeavesATopicOnlyDraft(t *testing.T) {
	in := "zqmonetizeai zqmonetizeai"
	if StripDraftPII(in) != in {
		t.Fatalf("got %q", StripDraftPII(in))
	}
}

func TestAIWritingDraftClearsTheFloor(t *testing.T) {
	text := StripDraftPII("I have been testing an AI writing tool that drafts posts. Mail ada@secret.example.")
	hit, ok := DryRunMatch(text, []OfferTerms{{
		Name:     "Jasper",
		Tagline:  "AI writing assistant.",
		Category: "ai",
		Keywords: []string{"jasper", "ai writing"},
	}})
	if !ok || hit.Relevance < RelevanceFloor {
		t.Fatalf("hit %+v ok %v", hit, ok)
	}
}

func TestStopwordsDoNotScore(t *testing.T) {
	offers := []OfferTerms{{Name: "Notebook", Keywords: []string{"notebook"}}}
	if _, ok := DryRunMatch("the and for its with your it", offers); ok {
		t.Fatal("stopwords matched a notebook")
	}
	// "its" is on the stopword list. "it" is not. A term that is only "it"
	// can still match that token. That handling stays with the shared scorer.
	hit, ok := DryRunMatch("it", []OfferTerms{{Name: "it", Keywords: []string{"it"}}})
	if !ok || hit.Relevance < RelevanceFloor {
		t.Fatalf("it token handling changed: %+v ok %v", hit, ok)
	}
	if _, ok = DryRunMatch("its", []OfferTerms{{Name: "its", Keywords: []string{"its"}}}); ok {
		t.Fatal("stopword its matched")
	}
}
