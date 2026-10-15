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

// defaultIgnorable is Unicode Default_Ignorable_Code_Point for assigned
// characters: Other_Default_Ignorable, format characters, and variation
// selectors, excluding annotation and Egyptian format controls.
var defaultIgnorable = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x00AD, 0x00AD, 1},
		{0x034F, 0x034F, 1},
		{0x0600, 0x0605, 1},
		{0x061C, 0x061C, 1},
		{0x06DD, 0x06DD, 1},
		{0x070F, 0x070F, 1},
		{0x0890, 0x0891, 1},
		{0x08E2, 0x08E2, 1},
		{0x115F, 0x1160, 1},
		{0x17B4, 0x17B5, 1},
		{0x180B, 0x180F, 1},
		{0x200B, 0x200F, 1},
		{0x202A, 0x202E, 1},
		{0x2060, 0x206F, 1},
		{0x3164, 0x3164, 1},
		{0xFE00, 0xFE0F, 1},
		{0xFEFF, 0xFEFF, 1},
		{0xFFA0, 0xFFA0, 1},
	},
	R32: []unicode.Range32{
		{0x110BD, 0x110BD, 1},
		{0x110CD, 0x110CD, 1},
		{0x1BCA0, 0x1BCA3, 1},
		{0x1D173, 0x1D17A, 1},
		{0xE0000, 0xE0FFF, 1},
	},
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
	if raw == "" || !readerCopyOK(raw) || !ReaderCopySafe(raw) || sharesInternalReason(raw, internalReason) {
		return "", false
	}
	return raw, true
}

func ContainsInternalCopy(text, internalReason string) bool {
	return sharesInternalReason(text, internalReason)
}

// ReaderCopySafe reports whether text can be shown to a reader. It rejects
// default-ignorable code points, any mark NFC does not compose away, a
// no-break space between letters, odd spaces, markup, mixed scripts, and
// Latin letters outside the blocks used by European copy.
func ReaderCopySafe(text string) bool {
	if hasMarkup(text) {
		return false
	}
	if nbspBetweenLetters(text) {
		return false
	}
	for _, r := range text {
		if _, blank := blankLetters[r]; blank || unicode.Is(unicode.Cf, r) || unicode.IsControl(r) || unicode.Is(defaultIgnorable, r) || badSpace(r) {
			return false
		}
	}
	return !hasStandaloneMark(text) && !mixesScripts(text)
}

func hasMarkup(text string) bool {
	folded := norm.NFKC.String(text)
	if strings.ContainsAny(folded, "<>") || strings.Contains(folded, "&#") {
		return true
	}
	return htmlEntityPattern.MatchString(folded)
}

func badSpace(r rune) bool {
	return unicode.IsSpace(r) && r != ' ' && r != '\u00A0'
}

func nbspBetweenLetters(text string) bool {
	rs := []rune(text)
	for i, r := range rs {
		if r != '\u00A0' || i == 0 || i+1 == len(rs) {
			continue
		}
		if unicode.IsLetter(rs[i-1]) && unicode.IsLetter(rs[i+1]) {
			return true
		}
	}
	return false
}

func hasStandaloneMark(text string) bool {
	for _, r := range norm.NFC.String(foldMicro(text)) {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r) {
			return true
		}
	}
	return false
}

func latinLetterAllowed(r rune) bool {
	if !unicode.IsLetter(r) || r == '\u0130' || r == '\u0131' {
		return false
	}
	return r <= 0x007F || (r >= 0x00A0 && r <= 0x00FF) || (r >= 0x0100 && r <= 0x017F)
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
	compact := strings.ReplaceAll(normalized, " ", "")
	return strings.Contains(normalized, "performance adjusted") || strings.Contains(compact, "performanceadjusted") ||
		strings.Contains(normalized, "contextual fit") || strings.Contains(compact, "contextualfit")
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
	for _, r := range strings.ToLower(norm.NFKD.String(text)) {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if r != ' ' {
			continue
		}
		if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func foldMicro(text string) string {
	return strings.ReplaceAll(text, "\u00B5", "u")
}

func mixesScripts(text string) bool {
	folded := norm.NFKC.String(foldMicro(text))
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
		if _, blank := blankLetters[r]; blank || latinOutOfRange(word) {
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

func latinOutOfRange(word []rune) bool {
	latin := false
	for _, r := range word {
		if scriptName(r) == "Latin" {
			latin = true
			break
		}
	}
	if !latin {
		return false
	}
	for _, r := range word {
		if unicode.IsLetter(r) && !latinLetterAllowed(r) {
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
