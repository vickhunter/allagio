package services

import "testing"

func TestApplyRecommendationTuning_AdjustsAndResorts(t *testing.T) {
	t.Parallel()
	matches := []ProgramMatch{
		{ProgramID: "c-high", Relevance: 80, Reason: "match", IsUserProgram: false},
		{ProgramID: "c-mid", Relevance: 75, Reason: "match", IsUserProgram: false},
		{ProgramID: "u-low", Relevance: 70, Reason: "match", IsUserProgram: true},
	}
	profiles := []RecommendationTuneProfile{
		{ProgramID: "c-mid", IsUserProgram: false, Multiplier: 1.2, SampleSize: 12},
		{ProgramID: "c-high", IsUserProgram: false, Multiplier: 0.8, SampleSize: 8},
	}
	out := ApplyRecommendationTuning(matches, profiles)
	if len(out) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(out))
	}
	if out[0].ProgramID != "c-mid" {
		t.Fatalf("expected c-mid to become top result, got %s", out[0].ProgramID)
	}
	if out[0].Relevance <= 75 {
		t.Fatalf("expected c-mid relevance to increase, got %d", out[0].Relevance)
	}
	if out[2].ProgramID != "c-high" {
		t.Fatalf("expected c-high to fall in rank, got %s", out[2].ProgramID)
	}
}

func TestApplyRecommendationTuning_UsesProgramTypeBoundary(t *testing.T) {
	t.Parallel()
	matches := []ProgramMatch{
		{ProgramID: "shared", Relevance: 80, Reason: "match", IsUserProgram: true},
		{ProgramID: "shared", Relevance: 80, Reason: "match", IsUserProgram: false},
	}
	profiles := []RecommendationTuneProfile{
		{ProgramID: "shared", IsUserProgram: true, Multiplier: 1.1, SampleSize: 5},
		{ProgramID: "shared", IsUserProgram: false, Multiplier: 0.9, SampleSize: 5},
	}
	out := ApplyRecommendationTuning(matches, profiles)
	if out[0].IsUserProgram == out[1].IsUserProgram {
		t.Fatalf("expected both user and curated entries")
	}
	userScore := 0
	curatedScore := 0
	for _, m := range out {
		if m.IsUserProgram {
			userScore = m.Relevance
		} else {
			curatedScore = m.Relevance
		}
	}
	if userScore <= curatedScore {
		t.Fatalf("expected user score > curated score, got %d <= %d", userScore, curatedScore)
	}
}
