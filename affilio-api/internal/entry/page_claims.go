package entry

import (
	"encoding/json"
	"html"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	// PageReasonUnreachable means the merchant page did not answer.
	PageReasonUnreachable = "page_unreachable"
	// PageReasonCard means a listing-card claim is missing from the page.
	PageReasonCard = "card_claim_not_on_page"
	// PageReasonPrice means the card's pricing summary is missing from the page.
	PageReasonPrice = "price_not_on_page"
	// PageReasonCommission means the offer's commission number is missing from the page.
	PageReasonCommission = "commission_not_on_page"
	// PageReasonTerms means a number in the offer terms is missing from the page.
	PageReasonTerms = "terms_not_on_page"
)

// PageDocument is the merchant page the checker compares against.
// OK is false when the cached HEAD or the GET did not return a page.
type PageDocument struct {
	OK   bool
	Body string
}

// PageClaimInput is what the listing card and the offer assert.
// HasCard is false when the product has no stored listing card.
// Commission and terms are still checked.
type PageClaimInput struct {
	Card            ListingCard
	HasCard         bool
	Terms           string
	CommissionValue float64
}

// PageClaimMessage is the sentence for a page-claim reason.
func PageClaimMessage(reason string) string {
	switch reason {
	case PageReasonCard:
		return "A claim on the listing card is not on the merchant page."
	case PageReasonPrice:
		return "The price on the listing card is not on the merchant page."
	case PageReasonCommission:
		return "The commission is not on the merchant page."
	case PageReasonTerms:
		return "The terms are not on the merchant page."
	case PageReasonUnreachable:
		return "The merchant page did not answer."
	default:
		return "Use only claims from the offer."
	}
}

// CheckPageClaims compares the listing card, the commission number, and the
// numbers in the terms with the merchant page. The same inputs always return
// the same reason. It does not call the network or a paid API.
// The first miss wins, in this order: page, card name, card category, card
// summary, each feature, pricing summary, commission, then terms.
func CheckPageClaims(doc PageDocument, in PageClaimInput) (reason, message string) {
	if !doc.OK {
		return failPage(PageReasonUnreachable)
	}
	page := pageEvidence(doc.Body)
	if in.HasCard {
		card := in.Card
		if !pageHasPhrase(page, card.Name) || !pageHasPhrase(page, card.Category) || !pageHasPhrase(page, card.Summary) {
			return failPage(PageReasonCard)
		}
		for _, feature := range card.Features {
			if !pageHasPhrase(page, feature) {
				return failPage(PageReasonCard)
			}
		}
		if strings.TrimSpace(card.PricingSummary) != "" && !pageHasPhrase(page, card.PricingSummary) {
			return failPage(PageReasonPrice)
		}
	}
	if in.CommissionValue > 0 {
		token := strconv.FormatFloat(in.CommissionValue, 'f', -1, 64)
		if !pageHasNumber(page, token) {
			return failPage(PageReasonCommission)
		}
	}
	for _, token := range claimToken.FindAllString(in.Terms, -1) {
		if !pageHasNumber(page, token) {
			return failPage(PageReasonTerms)
		}
	}
	return "", ""
}

func failPage(reason string) (string, string) {
	log.Printf("claim check failed reason=%s", reason)
	return reason, PageClaimMessage(reason)
}

// RecordedClaimCheck is a page-claim result computed before the database
// transaction. Restore and retry still call CheckClaims inside that
// transaction, so a miss leaves the row unchanged. Real is false, so this
// checker does not turn yolo or auto on.
type RecordedClaimCheck struct {
	Err error
}

func (r RecordedClaimCheck) CheckClaims(string, string) error { return r.Err }
func (RecordedClaimCheck) Real() bool                         { return false }

// PageClaimChecker is the checker used when CLAIMS_CHECK=real. CheckClaims
// still refuses a number the offer text does not contain, and that refusal
// is a reason code. Real is true, so yolo and auto can proceed once that
// copy check passes. Check compares the listing with a merchant document.
// A miss is a reason code. The hosted default stays the stub until the env
// is set to real.
type PageClaimChecker struct{}

func (PageClaimChecker) CheckClaims(copyText, offerData string) error {
	return StubClaimChecker{}.CheckClaims(copyText, offerData)
}

func (PageClaimChecker) Real() bool { return true }

// Check compares the listing card, the commission, and the terms with one
// merchant document. It does not fetch the page.
func (PageClaimChecker) Check(doc PageDocument, in PageClaimInput) (string, string) {
	return CheckPageClaims(doc, in)
}

var (
	pageTag     = regexp.MustCompile(`</?[A-Za-z][^>\x00]*>`)
	pageComment = regexp.MustCompile(`<!--[^\x00]*?-->`)
)

func normalizePage(raw string) string {
	s := strings.ToLower(raw)
	s = pageComment.ReplaceAllString(s, " ")
	s = pageTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

var pageJSONLDType = regexp.MustCompile(`(?i)(?:^|\s)type\s*=\s*(?:"application/ld\+json"|'application/ld\+json'|application/ld\+json(?:[\s>]|$))`)

// evidenceBreak keeps one extracted fragment from joining the next.
// A title "Northwind" must not combine with a description that starts
// "Ledger" into the card name "Northwind Ledger".
const evidenceBreak = "\x00"

// pageEvidence is the text a claim is compared with. Visible text is not
// enough for a client-rendered page, so meta and Open Graph content and
// JSON-LD values are included as their own fragments. Script, style,
// noscript, and template elements are removed first, through their own
// closer or through the end of the document when that closer is missing.
// A broken JSON-LD block is kept as one fragment. Nothing here panics.
func pageEvidence(raw string) string {
	stripped := stripEmbedded(raw)
	parts := []string{visiblePageText(stripped)}
	parts = append(parts, metaParts(stripped)...)
	parts = append(parts, jsonLDParts(raw)...)
	return normalizePage(strings.Join(parts, evidenceBreak))
}

func visiblePageText(stripped string) string {
	stripped = dropElement(stripped, "title")
	return dropVoid(stripped, "meta")
}

func metaParts(stripped string) []string {
	var parts []string
	if title := elementBody(stripped, "title"); title != "" {
		parts = append(parts, title)
	}
	for _, tag := range voidTags(stripped, "meta") {
		content, ok := attrValue(tag, "content")
		if !ok {
			continue
		}
		parts = append(parts, html.UnescapeString(content))
	}
	return parts
}

func elementBody(raw, name string) string {
	lower := asciiLower(raw)
	start, openEnd, bodyEnd, _, ok := findNamed(lower, raw, 0, name)
	if !ok || bodyEnd < openEnd {
		return ""
	}
	_ = start
	return raw[openEnd:bodyEnd]
}

func dropElement(raw, name string) string {
	lower := asciiLower(raw)
	var b strings.Builder
	pos := 0
	for pos < len(raw) {
		start, _, _, next, ok := findNamed(lower, raw, pos, name)
		if !ok {
			b.WriteString(raw[pos:])
			break
		}
		b.WriteString(raw[pos:start])
		if next <= start {
			break
		}
		pos = next
	}
	return b.String()
}

func dropVoid(raw, name string) string {
	lower := asciiLower(raw)
	var b strings.Builder
	pos := 0
	for _, tag := range voidSpans(lower, raw, name) {
		b.WriteString(raw[pos:tag.start])
		pos = tag.next
	}
	b.WriteString(raw[pos:])
	return b.String()
}

func voidTags(raw, name string) []string {
	lower := asciiLower(raw)
	var tags []string
	for _, tag := range voidSpans(lower, raw, name) {
		tags = append(tags, raw[tag.start:tag.next])
	}
	return tags
}

type voidSpan struct {
	start int
	next  int
}

func voidSpans(lower, raw, name string) []voidSpan {
	var spans []voidSpan
	pos := 0
	needle := "<" + name
	for pos < len(lower) {
		rel := strings.Index(lower[pos:], needle)
		if rel < 0 {
			break
		}
		at := pos + rel
		endName := at + len(needle)
		if endName < len(lower) {
			switch lower[endName] {
			case '>', ' ', '\t', '\n', '\r', '/':
			default:
				pos = at + 1
				continue
			}
		}
		next := tagEnd(raw, at)
		if next <= at || raw[next-1] != '>' {
			break
		}
		spans = append(spans, voidSpan{start: at, next: next})
		pos = next
	}
	return spans
}

func findNamed(lower, raw string, from int, name string) (start, openEnd, bodyEnd, next int, ok bool) {
	needle := "<" + name
	pos := from
	for pos < len(lower) {
		rel := strings.Index(lower[pos:], needle)
		if rel < 0 {
			return 0, 0, 0, 0, false
		}
		at := pos + rel
		endName := at + len(needle)
		if endName < len(lower) {
			switch lower[endName] {
			case '>', ' ', '\t', '\n', '\r', '/':
			default:
				pos = at + 1
				continue
			}
		}
		openEnd = tagEnd(raw, at)
		if openEnd <= at || raw[openEnd-1] != '>' {
			return at, 0, 0, len(raw), true
		}
		bodyEnd, next = findCloser(lower, openEnd, name)
		if bodyEnd < 0 {
			return at, openEnd, len(raw), len(raw), true
		}
		return at, openEnd, bodyEnd, next, true
	}
	return 0, 0, 0, 0, false
}

func attrValue(tag, name string) (string, bool) {
	lower := asciiLower(tag)
	pos := 0
	for pos < len(lower) {
		rel := strings.Index(lower[pos:], name)
		if rel < 0 {
			return "", false
		}
		at := pos + rel
		if at > 0 {
			switch lower[at-1] {
			case ' ', '\t', '\n', '\r', '/', '<':
			default:
				pos = at + len(name)
				continue
			}
		}
		i := at + len(name)
		for i < len(tag) && (tag[i] == ' ' || tag[i] == '\t' || tag[i] == '\n' || tag[i] == '\r') {
			i++
		}
		if i >= len(tag) || tag[i] != '=' {
			pos = at + len(name)
			continue
		}
		i++
		for i < len(tag) && (tag[i] == ' ' || tag[i] == '\t' || tag[i] == '\n' || tag[i] == '\r') {
			i++
		}
		if i >= len(tag) {
			return "", false
		}
		if tag[i] == '"' || tag[i] == '\'' {
			quote := tag[i]
			i++
			end := strings.IndexByte(tag[i:], quote)
			if end < 0 {
				return tag[i:], true
			}
			return tag[i : i+end], true
		}
		end := i
		for end < len(tag) && tag[end] != ' ' && tag[end] != '\t' && tag[end] != '>' && tag[end] != '\n' && tag[end] != '\r' {
			end++
		}
		return tag[i:end], true
	}
	return "", false
}

func jsonLDParts(raw string) []string {
	var parts []string
	for _, el := range embeddedElements(raw) {
		if el.name != "script" || !pageJSONLDType.MatchString(el.openTag) {
			continue
		}
		payload := strings.TrimSpace(el.body)
		if payload == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(payload))
		dec.UseNumber()
		var value any
		if err := dec.Decode(&value); err != nil {
			parts = append(parts, html.UnescapeString(payload))
			continue
		}
		parts = append(parts, jsonValueParts(value)...)
	}
	return parts
}

func jsonValueParts(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case json.Number:
		return []string{typed.String()}
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var parts []string
		for _, key := range keys {
			parts = append(parts, jsonValueParts(typed[key])...)
		}
		return parts
	case []any:
		var parts []string
		for _, child := range typed {
			parts = append(parts, jsonValueParts(child)...)
		}
		return parts
	default:
		return nil
	}
}

type embeddedElement struct {
	name    string
	openTag string
	body    string
	start   int
	next    int
}

func embeddedElements(raw string) []embeddedElement {
	lower := asciiLower(raw)
	var found []embeddedElement
	for i := 0; i < len(raw); {
		el, ok := nextEmbedded(raw, lower, i)
		if !ok {
			break
		}
		found = append(found, el)
		if el.next <= i {
			break
		}
		i = el.next
	}
	return found
}

func stripEmbedded(raw string) string {
	lower := asciiLower(raw)
	var b strings.Builder
	for i := 0; i < len(raw); {
		el, ok := nextEmbedded(raw, lower, i)
		if !ok {
			b.WriteString(raw[i:])
			break
		}
		b.WriteString(raw[i:el.start])
		if el.next <= el.start {
			break
		}
		i = el.next
	}
	return b.String()
}

func nextEmbedded(raw, lower string, from int) (embeddedElement, bool) {
	for from < len(raw) {
		start, name := findEmbeddedOpen(lower, from)
		commentAt := strings.Index(lower[from:], "<!--")
		if commentAt >= 0 {
			commentAt += from
		} else {
			commentAt = -1
		}
		if start < 0 && commentAt < 0 {
			return embeddedElement{}, false
		}
		if commentAt >= 0 && (start < 0 || commentAt < start) {
			end := strings.Index(lower[commentAt+4:], "-->")
			if end < 0 {
				return embeddedElement{}, false
			}
			from = commentAt + 4 + end + 3
			continue
		}
		openEnd := tagEnd(raw, start)
		if openEnd <= start || raw[openEnd-1] != '>' {
			return embeddedElement{name: name, openTag: raw[start:], start: start, next: len(raw)}, true
		}
		bodyEnd, next := findCloser(lower, openEnd, name)
		if bodyEnd < 0 {
			return embeddedElement{name: name, openTag: raw[start:openEnd], body: raw[openEnd:], start: start, next: len(raw)}, true
		}
		return embeddedElement{name: name, openTag: raw[start:openEnd], body: raw[openEnd:bodyEnd], start: start, next: next}, true
	}
	return embeddedElement{}, false
}

func tagEnd(raw string, start int) int {
	quote := byte(0)
	for i := start + 1; i < len(raw); i++ {
		c := raw[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '>':
			return i + 1
		}
	}
	return len(raw)
}

// findCloser returns the body end and the index after a real closer.
// The closer's first byte after the name must be >, /, or whitespace.
// </scripting> is not a </script> closer. A missing closer returns -1, -1.
func findCloser(lower string, from int, name string) (int, int) {
	closer := "</" + name
	pos := from
	for pos < len(lower) {
		rel := strings.Index(lower[pos:], closer)
		if rel < 0 {
			return -1, -1
		}
		at := pos + rel
		afterName := at + len(closer)
		if afterName >= len(lower) {
			return -1, -1
		}
		switch lower[afterName] {
		case '>', '/', ' ', '\t', '\n', '\r':
			if end := closerTagEnd(lower, afterName); end > 0 {
				return at, end
			}
		}
		pos = afterName
	}
	return -1, -1
}

func closerTagEnd(lower string, from int) int {
	for i := from; i < len(lower); i++ {
		switch lower[i] {
		case '>':
			return i + 1
		case '<':
			return -1
		}
	}
	return -1
}

func asciiLower(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

func findEmbeddedOpen(lower string, from int) (int, string) {
	best := -1
	bestName := ""
	for _, name := range []string{"script", "style", "noscript", "template"} {
		needle := "<" + name
		pos := from
		for pos < len(lower) {
			rel := strings.Index(lower[pos:], needle)
			if rel < 0 {
				break
			}
			at := pos + rel
			end := at + len(needle)
			if end < len(lower) {
				switch lower[end] {
				case '>', ' ', '\t', '\n', '\r', '/':
				default:
					pos = at + len(needle)
					continue
				}
			}
			if best < 0 || at < best {
				best = at
				bestName = name
			}
			break
		}
	}
	return best, bestName
}

func pageHasPhrase(page, phrase string) bool {
	phrase = normalizePage(phrase)
	if phrase == "" {
		return true
	}
	return strings.Contains(page, phrase)
}

func pageHasNumber(page, token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	token = strings.TrimSuffix(token, "%")
	if token == "" {
		return true
	}
	for i := 0; i+len(token) <= len(page); {
		if page[i:i+len(token)] != token {
			i++
			continue
		}
		if i > 0 && isDigitByte(page[i-1]) {
			i++
			continue
		}
		end := i + len(token)
		if end < len(page) && isDigitByte(page[end]) {
			i++
			continue
		}
		if end < len(page) && (page[end] == '.' || page[end] == ',') && end+1 < len(page) && isDigitByte(page[end+1]) {
			i++
			continue
		}
		return true
	}
	return false
}

func isDigitByte(b byte) bool { return b >= '0' && b <= '9' }
