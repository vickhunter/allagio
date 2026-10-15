package services

import (
	"strings"
	"unicode"
)

const overlapNGram = 5

func NormalizeSiteLanguage(language string) string {
	language = strings.TrimSpace(strings.ReplaceAll(language, "-", "_"))
	if language == "" || strings.EqualFold(language, "auto") {
		return "en"
	}
	return language
}

// SanitizeReaderCopy accepts reader-facing text only when it does not share
// the internal reason. Scoring phrases are rejected whole. Nothing is stripped
// out of a blurb and then published.
func SanitizeReaderCopy(text, internalReason string) (string, bool) {
	raw := strings.TrimSpace(text)
	if raw == "" || !readerCopyOK(raw) || sharesInternalReason(raw, internalReason) {
		return "", false
	}
	return raw, true
}

func ContainsInternalCopy(text, internalReason string) bool {
	return sharesInternalReason(text, internalReason)
}

func sharesInternalReason(text, reason string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	normText := normalizeForOverlap(text)
	if normText == "" {
		return false
	}
	if hasScoringMarker(normText) {
		return true
	}
	normReason := normalizeForOverlap(reason)
	if normReason == "" {
		return false
	}
	if containsPhrase(normText, normReason) {
		return true
	}
	for _, sentence := range reasonSentences(reason) {
		if containsPhrase(normText, sentence) {
			return true
		}
	}
	return hasLongNGram(strings.Fields(normText), strings.Fields(normReason), overlapNGram)
}

func hasScoringMarker(normalized string) bool {
	return strings.Contains(normalized, "performance adjusted") || strings.Contains(normalized, "contextual fit")
}

func reasonSentences(reason string) []string {
	parts := strings.FieldsFunc(reason, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized := normalizeForOverlap(part)
		if len(strings.Fields(normalized)) < 3 {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func containsPhrase(text, phrase string) bool {
	if phrase == "" {
		return false
	}
	return strings.Contains(" "+text+" ", " "+phrase+" ")
}

func hasLongNGram(textTokens, reasonTokens []string, n int) bool {
	if n < 1 || len(reasonTokens) < n || len(textTokens) < n {
		return false
	}
	grams := make(map[string]struct{}, len(reasonTokens)-n+1)
	for i := 0; i+n <= len(reasonTokens); i++ {
		grams[strings.Join(reasonTokens[i:i+n], " ")] = struct{}{}
	}
	for i := 0; i+n <= len(textTokens); i++ {
		if _, ok := grams[strings.Join(textTokens[i:i+n], " ")]; ok {
			return true
		}
	}
	return false
}

func normalizeForOverlap(text string) string {
	var b strings.Builder
	prevSpace := true
	for _, r := range strings.ToLower(text) {
		switch r {
		case '"', '\'', '`', '\u2018', '\u2019', '\u201c', '\u201d', '\u00ab', '\u00bb', '\u201a', '\u201e':
			continue
		case '-', '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2212':
			r = ' '
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func readerCopyOK(text string) bool {
	words := 0
	for _, field := range strings.Fields(text) {
		letters := 0
		for _, r := range field {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters >= 3 {
			words++
		}
	}
	return words >= 3
}
