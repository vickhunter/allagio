package services

import (
	"strings"
	"testing"
)

func TestSanitizeReaderCopyRejectsScoreMatchReasonBeforeStripping(t *testing.T) {
	t.Parallel()
	reason := scoreMatchReason(t)
	const want = "The article discusses \u201cemail marketing\u201d, making it a strong contextual fit for this recommendation."
	if reason != want {
		t.Fatalf("scoreMatch reason = %q", reason)
	}

	chopped := "The article discusses \u201cemail marketing\u201d, making it a for this recommendation."
	assertRejectedCopy(t, reason, reason)
	assertRejectedCopy(t, chopped, reason)

	variant := "The   ARTICLE discusses \"email marketing\", making it a Strong Contextual Fit for this recommendation."
	assertRejectedCopy(t, variant, reason)

	partial := "The article discusses email marketing making it a useful comparison for readers who want a tool."
	assertRejectedCopy(t, partial, reason)

	contextual := "There is a contextual fit worth mentioning to readers comparing tools."
	assertRejectedCopy(t, contextual, reason)

	tuned := tunedScoreReason(t, reason)
	if !strings.Contains(tuned, reason) || !strings.Contains(tuned, "Performance-adjusted (") {
		t.Fatalf("tuning did not append the score clause: %q", tuned)
	}
	assertRejectedCopy(t, tuned, tuned)
	assertRejectedCopy(t, "Teams can compare tools here. Performance-adjusted.", tuned)
	assertRejectedCopy(t, "Teams can compare tools here. performance adjusted for the quarter.", tuned)

	clean := "Mailchimp helps teams schedule campaigns and see which emails get replies."
	got, ok := SanitizeReaderCopy(clean, tuned)
	if !ok || got != clean {
		t.Fatalf("clean copy rejected: %q ok=%v", got, ok)
	}
}

func TestSanitizeReaderCopyRejectsCategoryScoreMatchSentence(t *testing.T) {
	t.Parallel()
	matches := MatchCuratedPrograms("seo email marketing", []CuratedProgramInput{{
		ID:       "prog-seo",
		Name:     "email marketing",
		Category: "seo",
	}}, nil)
	if len(matches) != 1 {
		t.Fatalf("matches: %#v", matches)
	}
	reason := matches[0].Reason
	if !strings.Contains(reason, "within the seo space") || !strings.Contains(reason, "contextual fit") {
		t.Fatalf("category scoreMatch reason = %q", reason)
	}
	assertRejectedCopy(t, reason, reason)
}

func scoreMatchReason(t *testing.T) string {
	t.Helper()
	matches := MatchCuratedPrograms("This article covers email marketing for small teams.", []CuratedProgramInput{{
		ID:   "prog-email",
		Name: "email marketing",
	}}, nil)
	if len(matches) != 1 {
		t.Fatalf("matches: %#v", matches)
	}
	return matches[0].Reason
}

func tunedScoreReason(t *testing.T, reason string) string {
	t.Helper()
	tuned := ApplyRecommendationTuning([]ProgramMatch{{
		ProgramID: "prog-email",
		Reason:    reason,
		Relevance: 80,
	}}, []RecommendationTuneProfile{{
		ProgramID:  "prog-email",
		Multiplier: 1.05,
		SampleSize: 9,
	}})
	if len(tuned) != 1 {
		t.Fatalf("tuned: %#v", tuned)
	}
	return tuned[0].Reason
}

func assertRejectedCopy(t *testing.T, blurb, reason string) {
	t.Helper()
	got, ok := SanitizeReaderCopy(blurb, reason)
	if ok || got != "" {
		t.Fatalf("accepted internal copy %q from blurb %q", got, blurb)
	}
	if strings.Contains(got, "making it a for") {
		t.Fatalf("published a stripped reason: %q", got)
	}
}
