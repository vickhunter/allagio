package services

import "testing"

func TestDedupeMatches_PrioritizesUserPrograms(t *testing.T) {
	t.Parallel()
	matches := []ProgramMatch{
		{
			ProgramID:     "u1",
			Name:          "Example",
			Domain:        "example.com",
			Relevance:     80,
			IsUserProgram: true,
		},
		{
			ProgramID:     "c1",
			Name:          "Example",
			Domain:        "example",
			Relevance:     90,
			IsUserProgram: false,
		},
	}
	out := DedupeMatches(matches)
	if len(out) != 1 {
		t.Fatalf("expected 1 match, got %d", len(out))
	}
	if !out[0].IsUserProgram {
		t.Fatalf("expected user program to win, got %#v", out[0])
	}
}

func TestDedupeMatches_KeepsHighestRelevance(t *testing.T) {
	t.Parallel()
	matches := []ProgramMatch{
		{ProgramID: "c1", Name: "Alpha", Relevance: 60},
		{ProgramID: "c1", Name: "Alpha", Relevance: 85},
	}
	out := DedupeMatches(matches)
	if len(out) != 1 {
		t.Fatalf("expected 1 match, got %d", len(out))
	}
	if out[0].Relevance != 85 {
		t.Fatalf("expected higher relevance, got %d", out[0].Relevance)
	}
}

func TestSemanticMatchCuratedPrograms(t *testing.T) {
	t.Parallel()
	content := "We focus on marketing emails and newsletter growth."
	programs := []CuratedProgramInput{{
		ID:       "c1",
		Name:     "Email Marketing Suite",
		Company:  "MailTools",
		Slug:     "mailtools",
		Keywords: []string{"email marketing"},
		Category: "email",
	}}
	matches := MatchCuratedPrograms(content, programs, nil)
	if len(matches) == 0 {
		t.Fatalf("expected semantic match for curated program")
	}
}

func TestSemanticMatchUserPrograms(t *testing.T) {
	t.Parallel()
	content := "We focus on marketing emails and newsletter growth."
	programs := []UserProgramInput{{
		ID:   "u1",
		Name: "email marketing",
	}}
	matches := MatchUserPrograms(content, programs, nil)
	if len(matches) == 0 {
		t.Fatalf("expected semantic match for user program")
	}
}
