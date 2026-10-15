package services

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const overlapNGram = 5

var htmlEntityPattern = regexp.MustCompile(`&(?:#(?:x[0-9A-Fa-f]+|[0-9]+)|[A-Za-z][A-Za-z0-9]+);`)

var blankLetters = map[rune]struct{}{
	'\u115F': {},
	'\u1160': {},
	'\u2800': {},
	'\u3164': {},
	'\uFFA0': {},
}

var letterScripts = []struct {
	name  string
	table *unicode.RangeTable
}{
	{"Latin", unicode.Latin},
	{"Cyrillic", unicode.Cyrillic},
	{"Greek", unicode.Greek},
	{"Han", unicode.Han},
	{"Hangul", unicode.Hangul},
	{"Hiragana", unicode.Hiragana},
	{"Katakana", unicode.Katakana},
	{"Arabic", unicode.Arabic},
	{"Hebrew", unicode.Hebrew},
	{"Devanagari", unicode.Devanagari},
	{"Thai", unicode.Thai},
	{"Georgian", unicode.Georgian},
	{"Armenian", unicode.Armenian},
	{"Bengali", unicode.Bengali},
	{"Gujarati", unicode.Gujarati},
	{"Gurmukhi", unicode.Gurmukhi},
	{"Kannada", unicode.Kannada},
	{"Malayalam", unicode.Malayalam},
	{"Oriya", unicode.Oriya},
	{"Sinhala", unicode.Sinhala},
	{"Tamil", unicode.Tamil},
	{"Telugu", unicode.Telugu},
	{"Myanmar", unicode.Myanmar},
	{"Khmer", unicode.Khmer},
	{"Lao", unicode.Lao},
	{"Tibetan", unicode.Tibetan},
	{"Mongolian", unicode.Mongolian},
	{"Ethiopic", unicode.Ethiopic},
	{"Bopomofo", unicode.Bopomofo},
	{"Canadian_Aboriginal", unicode.Canadian_Aboriginal},
	{"Cherokee", unicode.Cherokee},
	{"Coptic", unicode.Coptic},
	{"Syriac", unicode.Syriac},
	{"Thaana", unicode.Thaana},
	{"Ogham", unicode.Ogham},
	{"Runic", unicode.Runic},
	{"Tagalog", unicode.Tagalog},
	{"Yi", unicode.Yi},
}

func NormalizeSiteLanguage(language string) string {
	language = strings.TrimSpace(strings.ReplaceAll(language, "-", "_"))
	if language == "" || strings.EqualFold(language, "auto") {
		return "en"
	}
	return language
}

// SanitizeReaderCopy accepts reader-facing text only when it does not share
// the internal reason. Scoring phrases, hidden characters, mixed scripts, and
// raw HTML are rejected whole. Nothing is stripped out of a blurb and then published.
func SanitizeReaderCopy(text, internalReason string) (string, bool) {
	raw := strings.TrimSpace(text)
	if raw == "" || !readerCopyOK(raw) || !readerCopySafe(raw) || sharesInternalReason(raw, internalReason) {
		return "", false
	}
	return raw, true
}

func ContainsInternalCopy(text, internalReason string) bool {
	return sharesInternalReason(text, internalReason)
}

func readerCopySafe(text string) bool {
	if strings.ContainsAny(text, "<>") || htmlEntityPattern.MatchString(text) {
		return false
	}
	for _, r := range text {
		if _, blank := blankLetters[r]; blank || unicode.Is(unicode.Cf, r) || unicode.IsControl(r) {
			return false
		}
	}
	return !mixesScripts(text)
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
	long, short := classifyReasonSentences(reason)
	for _, sentence := range long {
		if containsPhrase(normText, sentence) {
			return true
		}
	}
	for _, sentence := range short {
		if normText == sentence {
			return true
		}
	}
	return hasLongNGram(strings.Fields(normText), strings.Fields(normReason), overlapNGram)
}

func hasScoringMarker(normalized string) bool {
	return strings.Contains(normalized, "performance adjusted") || strings.Contains(normalized, "contextual fit")
}

func classifyReasonSentences(reason string) (long, short []string) {
	parts := strings.FieldsFunc(reason, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})
	for _, part := range parts {
		normalized := normalizeForOverlap(part)
		switch len(strings.Fields(normalized)) {
		case 0, 1:
		case 2:
			short = append(short, normalized)
		default:
			long = append(long, normalized)
		}
	}
	return long, short
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
	for _, r := range strings.ToLower(norm.NFKC.String(text)) {
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

func mixesScripts(text string) bool {
	folded := norm.NFKC.String(text)
	var word []rune
	flush := func() bool {
		if len(word) == 0 {
			return false
		}
		mixed := wordMixesScripts(word)
		word = word[:0]
		return mixed
	}
	for _, r := range folded {
		if unicode.IsLetter(r) {
			word = append(word, r)
			continue
		}
		if flush() {
			return true
		}
	}
	return flush()
}

func wordMixesScripts(word []rune) bool {
	seen := map[string]struct{}{}
	for _, r := range word {
		if _, blank := blankLetters[r]; blank {
			return true
		}
		name := scriptName(r)
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
		if len(seen) > 1 {
			return true
		}
	}
	return false
}

func scriptName(r rune) string {
	if !unicode.IsLetter(r) {
		return ""
	}
	for _, script := range letterScripts {
		if unicode.Is(script.table, r) {
			return script.name
		}
	}
	return "Other"
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
