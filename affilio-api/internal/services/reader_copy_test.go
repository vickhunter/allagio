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

func TestSanitizeReaderCopyRejectsHiddenAndMarkedText(t *testing.T) {
	t.Parallel()
	reason := scoreMatchReason(t)
	base := "Mailchimp helps teams schedule campaigns and see which emails get replies."

	for _, r := range []rune{'\u200B', '\u200C', '\u200D', '\u2060', '\uFEFF', '\u00AD', '\u0007'} {
		hidden := "Readers can see a strong contex" + string(r) + "tual fit for this comparison of tools."
		assertRejectedCopy(t, hidden, reason)
		assertRejectedCopy(t, strings.Replace(base, "helps", "helps"+string(r), 1), reason)
	}

	assertRejectedCopy(t, "Readers can see a strong contextual \ufb01t for this comparison of tools.", reason)
	assertRejectedCopy(t, "Readers can see a strong \uff43\uff4f\uff4e\uff54\uff45\uff58\uff54\uff55\uff41\uff4c \uff46\uff49\uff54 for this comparison of tools.", reason)
	assertRejectedCopy(t, "Mailchimp helps teams schedule \u0441ampaigns and see which emails get replies.", reason)
	assertRejectedCopy(t, "Mailchimp helps teams schedule c\u03bfmpaigns and see which emails get replies.", reason)
	for _, r := range []rune{'\u3164', '\u115F', '\u1160', '\uFFA0', '\u2800'} {
		assertRejectedCopy(t, strings.Replace(base, "campaigns", "campa"+string(r)+"igns", 1), reason)
	}

	assertRejectedCopy(t, "Readers can see a <em>contextual</em> fit for this comparison of email tools.", reason)
	assertRejectedCopy(t, "Readers can see a contex&#116;ual fit for this comparison of email tools.", reason)
	assertRejectedCopy(t, "Readers can see a contex&shy;tual fit for this comparison of email tools.", reason)
	assertRejectedCopy(t, "Mailchimp helps teams schedule <campaigns> and see which emails get replies.", reason)

	kept, ok := SanitizeReaderCopy("Mailchimp & Semrush help teams schedule campaigns and see replies.", reason)
	if !ok || !strings.Contains(kept, "Mailchimp & Semrush") {
		t.Fatalf("plain ampersand rejected: %q ok=%v", kept, ok)
	}
}

func TestSanitizeReaderCopyRejectsIgnorableMarksSpacesAndLookalikes(t *testing.T) {
	t.Parallel()
	reason := scoreMatchReason(t)
	base := "Mailchimp helps teams schedule campaigns and see which emails get replies."
	hide := func(r rune) string {
		return "Readers can see a strong contex" + string(r) + "tual fit for this comparison of tools."
	}

	for _, r := range []rune{'\u034F', '\uFE00', '\uFE0F', '\U000E0100', '\U000E01EF', '\u180B', '\u180C', '\u180D', '\u180E', '\u180F', '\u17B4', '\u17B5'} {
		assertRejectedCopy(t, hide(r), reason)
	}
	for r := rune('\u2000'); r <= '\u200A'; r++ {
		assertRejectedCopy(t, hide(r), reason)
	}
	assertRejectedCopy(t, hide('\u3000'), reason)
	assertRejectedCopy(t, hide('\u202F'), reason)

	assertRejectedCopy(t, strings.Replace(base, "campaigns", "campa\u0131gns", 1), reason)
	assertRejectedCopy(t, strings.Replace(base, "campaigns", "c\u0251mpaigns", 1), reason)
	assertRejectedCopy(t, strings.Replace(base, "campaigns", "c\u1D00mpaigns", 1), reason)

	assertRejectedCopy(t, "Readers can see a \uFF1Cem\uFF1Econtextual fit\uFF1C/em\uFF1E in this comparison of tools.", reason)
	assertRejectedCopy(t, "Readers can see a contex\uFF06#116ual fit for this comparison of tools.", reason)
	assertRejectedCopy(t, "Readers can see a contex&#116ual fit for this comparison of tools.", reason)
	assertRejectedCopy(t, "Readers can see a contex&#x74ual fit for this comparison of tools.", reason)

	kept, ok := SanitizeReaderCopy("Mailchimp tracks 50\u00B5s samples and shows which emails get replies.", reason)
	if !ok || !strings.Contains(kept, "50\u00B5s") {
		t.Fatalf("micro sign rejected: %q ok=%v", kept, ok)
	}
}

func TestSanitizeReaderCopyRejectsDotAboveAndSplitTokens(t *testing.T) {
	t.Parallel()
	reason := scoreMatchReason(t)
	dotted := insertDotAboveIJ(reason)
	if dotted == reason || !strings.Contains(dotted, "i\u0307") {
		t.Fatalf("dot was not inserted: %q", dotted)
	}
	assertRejectedCopy(t, dotted, reason)
	assertRejectedCopy(t, strings.ReplaceAll(reason, "fit", "f\u00ECt"), reason)
	assertRejectedCopy(t, strings.ReplaceAll(reason, "fit", "fi\u0300t"), reason)
	assertRejectedCopy(t, "Readers can see a strong contex.tual fit for this comparison of tools.", reason)
	assertRejectedCopy(t, "Readers can see a strong contex\u00A0tual fit for this comparison of tools.", reason)
	if ReaderCopySafe("j\u0307") || ReaderCopySafe("i\u0307") {
		t.Fatal("combining dot above on i or j was allowed")
	}
}

func insertDotAboveIJ(text string) string {
	var b strings.Builder
	for _, r := range text {
		b.WriteRune(r)
		if r == 'i' || r == 'I' || r == 'j' || r == 'J' {
			b.WriteRune('\u0307')
		}
	}
	return b.String()
}

func TestSanitizeReaderCopyKeepsAccentedEuropeanCopy(t *testing.T) {
	t.Parallel()
	reason := scoreMatchReason(t)
	samples := []string{
		"Questa città è più adatta ai lettori che confrontano gli strumenti.",
		"Questa citta\u0300 e\u0300 piu\u0300 adatta ai lettori che confrontano gli strumenti.",
		"Für größere Häuser braucht man oft eine bessere Lösung mit Weiß.",
		"Fu\u0308r gro\u0308ßere Ha\u0308user braucht man oft eine bessere Lo\u0308sung mit Weiß.",
		"Le français préfère un cœur déjà prêt pour les lecteurs curieux.",
		"Le franc\u0327ais pre\u0301fe\u0300re un cœur de\u0301ja\u0300 pre\u0302t pour les lecteurs curieux.",
		"Mailchimp aide les équipes françaises à choisir un cœur déjà prêt.",
	}
	for _, sample := range samples {
		got, ok := SanitizeReaderCopy(sample, reason)
		if !ok || got != sample {
			t.Fatalf("European copy rejected: %q ok=%v got %q", sample, ok, got)
		}
	}
}

func TestSanitizeReaderCopyRejectsShortSentenceWhenBlurbIsOnlyThat(t *testing.T) {
	t.Parallel()
	reason := "Good fit. Teams should compare a few tools before they buy anything."
	if !ContainsInternalCopy("Good fit.", reason) {
		t.Fatal("accepted a blurb that is only the short reason sentence")
	}
	if ContainsInternalCopy("Readers can find a good fit among these tools.", reason) {
		t.Fatal("rejected a longer blurb that merely contains the short sentence")
	}
	kept, ok := SanitizeReaderCopy("Readers can find a good fit among these tools.", reason)
	if !ok || kept == "" {
		t.Fatalf("longer blurb rejected: %q ok=%v", kept, ok)
	}
}

func TestSanitizeReaderCopyRejectsDigitStandInsForLetters(t *testing.T) {
	t.Parallel()
	reason := scoreMatchReason(t)
	assertRejectedCopy(t, "There is a c0ntextual fit worth mentioning to readers comparing tools.", reason)
	assertRejectedCopy(t, "There is a contextua1 fit worth mentioning to readers comparing tools.", reason)
	assertRejectedCopy(t, "There is a contextual f1t worth mentioning to readers comparing tools.", reason)
	assertRejectedCopy(t, "Readers notice th3 article discusses email marketing in this comparison.", reason)
	assertRejectedCopy(t, "A note about making it a 5trong c0ntextual choice for the team.", reason)
	if !ContainsInternalCopy("There is a strong contex&#116;ual fit for readers comparing tools.", reason) {
		t.Fatal("an entity hid the internal reason from the overlap check")
	}
	clean := "Mailchimp helps 30 teams schedule campaigns and see which emails get replies."
	got, ok := SanitizeReaderCopy(clean, reason)
	if !ok || got != clean {
		t.Fatalf("digits in unrelated copy were rejected: %q ok=%v", got, ok)
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
