package services

import (
	"regexp"
	"strings"
	"unicode"
)

var scoringCopyPattern = regexp.MustCompile(`(?i)performance-adjusted\s*\(\s*[+-]?\d+\s+from\s+\d+\s+historical\s+outcomes\s*\)`)
var contextualFitPattern = regexp.MustCompile(`(?i)strong\s+contextual\s+fit`)

func NormalizeSiteLanguage(language string) string {
	language = strings.TrimSpace(strings.ReplaceAll(language, "-", "_"))
	if language == "" || strings.EqualFold(language, "auto") {
		return "en"
	}
	return language
}

func SanitizeReaderCopy(text, internalReason string) (string, bool) {
	cleaned := stripInternalCopy(text, internalReason)
	return cleaned, readerCopyOK(cleaned) && !ContainsInternalCopy(cleaned, internalReason)
}

func ContainsInternalCopy(text, internalReason string) bool {
	if text == "" {
		return false
	}
	if scoringCopyPattern.MatchString(text) || contextualFitPattern.MatchString(text) {
		return true
	}
	reason := strings.TrimSpace(internalReason)
	return len(reason) >= 8 && containsFold(text, reason)
}

func stripInternalCopy(text, internalReason string) string {
	text = scoringCopyPattern.ReplaceAllString(text, " ")
	text = contextualFitPattern.ReplaceAllString(text, " ")
	reason := strings.TrimSpace(internalReason)
	if len(reason) >= 8 {
		text = replaceFold(text, reason, " ")
	}
	text = strings.Join(strings.Fields(text), " ")
	text = strings.Trim(text, " \t-—")
	text = strings.TrimLeft(text, ".,;:")
	return strings.TrimSpace(text)
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

func containsFold(text, needle string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(needle))
}

func replaceFold(text, needle, repl string) string {
	if needle == "" {
		return text
	}
	lowerNeedle := strings.ToLower(needle)
	var b strings.Builder
	rest := text
	for {
		lower := strings.ToLower(rest)
		i := strings.Index(lower, lowerNeedle)
		if i < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:i])
		b.WriteString(repl)
		rest = rest[i+len(lowerNeedle):]
	}
}
