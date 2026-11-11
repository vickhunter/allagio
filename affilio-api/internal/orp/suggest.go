package orp

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	langPattern     = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)
	categoryPattern = regexp.MustCompile(`^[a-z0-9\-]+(\.[a-z0-9\-]+)*$`)
	urlPattern      = regexp.MustCompile(`^https?://`)
	spaceKindCore   = map[string]bool{
		"web.article": true, "web.page": true, "newsletter.issue": true, "newsletter.dedicated": true,
		"social.post": true, "social.reply": true, "social.repost": true, "social.profile": true,
		"video.description": true, "video.segment": true, "video.comment": true,
		"audio.segment": true, "audio.notes": true, "community.post": true, "community.comment": true,
		"community.message": true, "app.slot": true, "data.feed": true,
	}
	dealKinds = map[string]bool{
		"affiliate": true, "sponsored": true, "swap": true, "traffic_swap": true,
		"link_exchange": true, "repost": true, "comment": true, "cross_sell": true,
		"data": true, "partnership": true,
	}
	personalJoinKeys = map[string]bool{
		"email": true, "phone": true, "name": true, "password": true, "domain": true, "website": true,
	}
	spaceExtPattern = regexp.MustCompile(`^[a-z0-9\-]+(\.[a-z0-9\-]+){2,}$`)
)

// DraftSurfaces is the platform and surface set POST /orp/suggest and
// POST /orp/monetize both accept. blog and email are first-class values,
// alongside x, newsletter, and web.
var DraftSurfaces = []string{"x", "blog", "newsletter", "email", "web"}

// NeutralFitReason is the fit line when the draft and the offer share no terms.
const NeutralFitReason = "No terms in the draft match this offer."

// DraftSurfaceAllowed reports whether value is in DraftSurfaces.
func DraftSurfaceAllowed(value string) bool {
	switch value {
	case "x", "blog", "newsletter", "email", "web":
		return true
	default:
		return false
	}
}

// DraftSurfaceProblem is the invalid_message for a platform or surface
// outside DraftSurfaces. The message lists the allowed values.
func DraftSurfaceProblem(field string) Problem {
	return Invalid("Allowed values: x, blog, newsletter, email, web.", field)
}

// Candidate is a listed offer or curated program the suggest layer can pitch.
type Candidate struct {
	ID              string
	Source          string
	Name            string
	Company         string
	Blurb           string
	Website         string
	Keywords        []string
	Category        string
	CommissionUnit  string
	CommissionType  string
	CommissionValue float64
	Currency        string
	CookieDays      int
}

// SuggestRequest is a credential-free suggest body.
type SuggestRequest struct {
	Text     string
	Want     []string
	Kinds    []string
	Limit    int
	HasJoin  bool
	Join     JoinRequest
	Haystack string
}

// JoinRequest is the optional join object. Key registration is refused.
type JoinRequest struct {
	AgentName    string
	AgentVersion string
}

// ParseSuggest checks a suggest body. The draft is kept only in this value.
func ParseSuggest(value any) (SuggestRequest, Problem, bool) {
	if found := DirtyPointer(value); found != "" {
		return SuggestRequest{}, Invalid("The suggest request contains disallowed characters or markup.", found), false
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return SuggestRequest{}, Invalid("The suggest request is not valid.", ""), false
	}
	for key := range obj {
		switch key {
		case "text", "platform", "space_kind", "want", "kinds", "lang", "join", "limit":
		default:
			return SuggestRequest{}, Invalid("The suggest request is not valid.", "/"+escapePointer(key)), false
		}
	}
	text, _ := obj["text"].(string)
	if text == "" || utf8.RuneCountInString(text) > 4000 {
		return SuggestRequest{}, Invalid("The suggest request is not valid.", "/text"), false
	}
	req := SuggestRequest{Text: text, Haystack: text, Limit: 3}
	if platform, present := obj["platform"]; present {
		text, ok := platform.(string)
		if !ok || !DraftSurfaceAllowed(text) {
			return SuggestRequest{}, DraftSurfaceProblem("/platform"), false
		}
	}
	if kind, present := obj["space_kind"]; present {
		text, ok := kind.(string)
		if !ok || !validSpaceKind(text) {
			return SuggestRequest{}, Invalid("The suggest request is not valid.", "/space_kind"), false
		}
	}
	if want, present := obj["want"]; present {
		items, problem, bad := stringEnum(want, "/want", map[string]bool{"earn": true, "grow": true})
		if bad {
			return SuggestRequest{}, problem, false
		}
		req.Want = items
		req.Haystack += " " + strings.Join(items, " ")
	}
	if kinds, present := obj["kinds"]; present {
		items, problem, bad := stringEnum(kinds, "/kinds", dealKinds)
		if bad {
			return SuggestRequest{}, problem, false
		}
		req.Kinds = items
	}
	if lang, present := obj["lang"]; present {
		text, ok := lang.(string)
		if !ok || !langPattern.MatchString(text) {
			return SuggestRequest{}, Invalid("The suggest request is not valid.", "/lang"), false
		}
	}
	if limit, present := obj["limit"]; present {
		n, ok := whole(limit)
		if !ok || n < 1 || n > 5 {
			return SuggestRequest{}, Invalid("The suggest request is not valid.", "/limit"), false
		}
		req.Limit = n
	}
	if join, present := obj["join"]; present {
		parsed, problem, joinOK := ParseJoin(join, "/join")
		if !joinOK {
			return SuggestRequest{}, problem, false
		}
		req.HasJoin = true
		req.Join = parsed
	}
	return req, Problem{}, true
}

// ParseJoin checks a join body. An empty object is complete. Email, name,
// domain, phone, and password are refused. A signing key is refused because
// this layer does not store one.
func ParseJoin(value any, prefix string) (JoinRequest, Problem, bool) {
	if value == nil {
		return JoinRequest{}, Invalid("The join request is not valid.", prefix), false
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return JoinRequest{}, Invalid("The join request is not valid.", prefix), false
	}
	if found := DirtyPointer(value); found != "" {
		field := found
		if prefix != "" && found != "/" {
			field = prefix + found
		} else if prefix != "" {
			field = prefix
		}
		return JoinRequest{}, Invalid("The join request contains disallowed characters or markup.", field), false
	}
	for key := range obj {
		if personalJoinKeys[key] {
			return JoinRequest{}, Invalid("Join asks for nothing personal.", prefix+"/"+escapePointer(key)), false
		}
		if key == "orp" {
			version, _ := obj[key].(string)
			if version == Version04 {
				continue
			}
			return JoinRequest{}, Invalid("Use 0.4.", prefix+"/"+escapePointer(key)), false
		}
		if key != "agent" && key != "key" {
			return JoinRequest{}, Invalid("The join request is not valid.", prefix+"/"+escapePointer(key)), false
		}
	}
	if _, present := obj["key"]; present {
		return JoinRequest{}, Invalid("This testing host does not register signing keys.", prefix+"/key"), false
	}
	var req JoinRequest
	if agent, present := obj["agent"]; present {
		path := prefix + "/agent"
		if prefix == "" {
			path = "/agent"
		}
		if problem, bad := checkAgent(agent, path); bad {
			problem, _ = relabelVersion(Version04, problem, bad)
			return JoinRequest{}, problem, false
		}
		obj := agent.(map[string]any)
		req.AgentName, _ = obj["name"].(string)
		req.AgentVersion, _ = obj["version"].(string)
	}
	return req, Problem{}, true
}

func validSpaceKind(value string) bool {
	return spaceKindCore[value] || (spaceExtPattern.MatchString(value) && len(value) <= 128)
}

func stringEnum(value any, path string, allowed map[string]bool) ([]string, Problem, bool) {
	raw, ok := value.([]any)
	if !ok || len(raw) < 1 {
		return nil, Invalid("The suggest request is not valid.", path), true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for i, item := range raw {
		text, ok := item.(string)
		if !ok || !allowed[text] || seen[text] {
			return nil, Invalid("The suggest request is not valid.", path+"/"+itoa(i)), true
		}
		seen[text] = true
		out = append(out, text)
	}
	return out, Problem{}, false
}

// AllowsAffiliate reports whether the request can take affiliate pitches.
// Earn is the curated catalog. Grow-only, or a kinds list without affiliate,
// is filtered.
func (r SuggestRequest) AllowsAffiliate() bool {
	if len(r.Want) > 0 && !contains(r.Want, "earn") {
		return false
	}
	if len(r.Kinds) > 0 && !contains(r.Kinds, "affiliate") {
		return false
	}
	return true
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

// Match builds suggest items from candidates. It does not write and it does
// not keep the draft. empty is no_match or filtered when items is empty.
func Match(dns string, req SuggestRequest, candidates []Candidate) ([]any, string) {
	if !req.AllowsAffiliate() {
		return []any{}, "filtered"
	}
	words := matchWords(req.Haystack)
	type hit struct {
		item  map[string]any
		score int
		name  string
	}
	var hits []hit
	for _, candidate := range candidates {
		item, score, ok := pitch(dns, candidate, req.Text, words)
		if !ok {
			continue
		}
		hits = append(hits, hit{item: item, score: score, name: candidate.Name})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].name < hits[j].name
	})
	if req.Limit <= 0 {
		req.Limit = 3
	}
	if len(hits) > req.Limit {
		hits = hits[:req.Limit]
	}
	items := make([]any, 0, len(hits))
	for _, one := range hits {
		items = append(items, one.item)
	}
	if len(items) == 0 {
		return items, "no_match"
	}
	return items, ""
}

// FitReason cites draft words that also appear in the offer terms.
// It does not name an offer title the draft never used. An empty overlap
// is NeutralFitReason.
func FitReason(draft string, terms ...string) string {
	words := matchWords(draft)
	blob := strings.ToLower(strings.Join(terms, " "))
	lowerDraft := strings.ToLower(draft)
	cited := make([]string, 0, 4)
	for _, word := range words {
		if !strings.Contains(blob, word) || !strings.Contains(lowerDraft, word) {
			continue
		}
		cited = append(cited, word)
		if len(cited) == 4 {
			break
		}
	}
	if len(cited) == 0 {
		return NeutralFitReason
	}
	reason := "The draft mentions " + joinCited(cited) + "."
	if utf8.RuneCountInString(reason) > 200 {
		return clipRunes(reason, 200)
	}
	return reason
}

func joinCited(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	default:
		return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
	}
}

func matchWords(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	seen := map[string]bool{}
	out := make([]string, 0, len(fields))
	for _, word := range fields {
		if len(word) < 4 || seen[word] {
			continue
		}
		seen[word] = true
		out = append(out, word)
	}
	return out
}

func pitch(dns string, candidate Candidate, draft string, words []string) (map[string]any, int, bool) {
	blob := strings.ToLower(strings.Join([]string{
		candidate.Name, candidate.Company, candidate.Blurb, candidate.Category, strings.Join(candidate.Keywords, " "),
	}, " "))
	hits := 0
	for _, word := range words {
		if strings.Contains(blob, word) {
			hits++
		}
	}
	if hits == 0 {
		return nil, 0, false
	}
	terms, payLabel, ok := termsOf(candidate)
	if !ok {
		return nil, 0, false
	}
	title, _ := terms["title"].(string)
	if DirtyPointer(title) != "" || DirtyPointer(payLabel) != "" {
		return nil, 0, false
	}
	score := 40 + hits*15
	if score > 100 {
		score = 100
	}
	text := "You can monetize this draft: " + title + " pays " + payLabel + ". You would add one link labelled Affiliate link."
	if utf8.RuneCountInString(text) > 280 {
		text = clipRunes(text, 280)
	}
	reason := FitReason(draft, candidate.Name, candidate.Company, candidate.Blurb, candidate.Category, strings.Join(candidate.Keywords, " "))
	item := map[string]any{
		"offer": map[string]any{
			"id":         candidate.ID,
			"terms":      terms,
			"terms_hash": TermsHash(terms),
		},
		"party": CatalogParty(dns, candidate.Source, candidate.ID),
		"fit": map[string]any{
			"score":   score,
			"reasons": []any{reason},
		},
		"pool":   "catalog",
		"checks": map[string]any{"claims": "not_checked"},
		"pitch":  text,
	}
	return item, score, true
}

func termsOf(candidate Candidate) (map[string]any, string, bool) {
	title := strings.TrimSpace(candidate.Name)
	if title == "" || utf8.RuneCountInString(title) > 120 {
		title = clipRunes(title, 120)
	}
	if title == "" || DirtyPointer(title) != "" {
		return nil, "", false
	}
	website := strings.TrimSpace(candidate.Website)
	if !urlPattern.MatchString(website) || len(website) > 2048 || strings.ContainsAny(website, " \t\r\n") {
		return nil, "", false
	}
	pay, label, ok := payObligation(candidate)
	if !ok {
		return nil, "", false
	}
	place := map[string]any{
		"action":      "place",
		"by":          "acceptor",
		"content":     map[string]any{"link": true},
		"destination": website,
		"id":          "place",
		"proof":       []any{"link"},
	}
	terms := map[string]any{
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"kind":       "affiliate",
		"obligations": []any{
			place,
			pay,
		},
		"title": title,
	}
	if category := categoryOf(candidate.Category); category != "" {
		terms["categories"] = []any{category}
	}
	return terms, label, true
}

func payObligation(candidate Candidate) (map[string]any, string, bool) {
	mode := payMode(candidate.CommissionUnit, candidate.CommissionType)
	pay := map[string]any{
		"action": "pay",
		"by":     "offerer",
		"id":     "commission",
		"per":    "conversion",
		"proof":  []any{"provider"},
		"rail":   "manual",
	}
	if candidate.CookieDays > 0 && candidate.CookieDays <= 3650 {
		pay["window"] = "P" + strconv.Itoa(candidate.CookieDays) + "D"
	}
	switch mode {
	case "percent":
		bps := int(math.Round(candidate.CommissionValue * 100))
		if bps < 1 || bps > 10000 {
			return nil, "", false
		}
		pay["rate_bps"] = bps
		return pay, percentLabel(candidate.CommissionValue), true
	case "fixed":
		minor := int(math.Round(candidate.CommissionValue * 100))
		if minor < 0 || minor > 100000000000 {
			return nil, "", false
		}
		currency := strings.ToUpper(strings.TrimSpace(candidate.Currency))
		if !regexp.MustCompile(`^[A-Z]{3}$`).MatchString(currency) {
			currency = "USD"
		}
		pay["amount"] = map[string]any{"amount_minor": minor, "currency": currency}
		return pay, strconv.Itoa(minor/100) + " " + currency, true
	default:
		return nil, "", false
	}
}

func payMode(unit, typ string) string {
	u := strings.ToLower(strings.TrimSpace(unit))
	if u == "" {
		u = strings.ToLower(strings.TrimSpace(typ))
	}
	switch u {
	case "percent", "percentage":
		return "percent"
	case "fixed":
		return "fixed"
	default:
		return ""
	}
}

func percentLabel(value float64) string {
	if value == math.Trunc(value) {
		return strconv.FormatFloat(value, 'f', 0, 64) + "%"
	}
	return strconv.FormatFloat(value, 'g', -1, 64) + "%"
}

func categoryOf(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if categoryPattern.MatchString(raw) && len(raw) <= 64 {
		return raw
	}
	return ""
}

func clipRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return string(runes[:n])
}

// SuggestResponse is the credential-free suggest body.
func SuggestResponse(origin string, items []any, empty string, joined map[string]any) map[string]any {
	if items == nil {
		items = []any{}
	}
	doc := map[string]any{
		"items": items,
		"join":  strings.TrimRight(origin, "/") + "/orp/join",
	}
	if len(items) == 0 && empty != "" {
		doc["empty_reason"] = empty
	}
	if joined != nil {
		doc["joined"] = joined
	}
	return doc
}

// JoinResponse is the bearer for the agent account that was just minted.
func JoinResponse(origin, dns, ownerID, token string) map[string]any {
	return map[string]any{
		"party": MemberParty(dns, ownerID),
		"token": token,
		"claim": strings.TrimRight(origin, "/") + "/owner",
		"steps": map[string]any{
			"list": "ask", "negotiate": "ask", "accept": "ask", "execute": "ask",
			"amend": "ask", "settle": "ask", "share": "ask", "revoke": "auto",
		},
		"limits": map[string]any{
			"open_deals":          5,
			"messages_per_minute": 30,
		},
	}
}

// OfferMatches reports whether a standing offer fits discover text.
// An empty word list matches every offer. A word counts at length 4.
func OfferMatches(terms map[string]any, text string) bool {
	_, _, ok := ScoreOffer(terms, text)
	return ok
}

// ScoreOffer is the one matcher suggest and discover share for standing
// offers. It returns a 0–100 fit and a reason that quotes the matched
// words (FitReason), or ok=false when no draft word overlaps the terms.
// An empty text lists every offer at a neutral 55 with an honest
// "no query" reason.
func ScoreOffer(terms map[string]any, text string) (score int, reason string, ok bool) {
	words := matchWords(text)
	if len(words) == 0 {
		reason = "Standing offer: " + TermsTitle(terms) + "."
		if utf8.RuneCountInString(reason) > 200 {
			reason = clipRunes(reason, 200)
		}
		return 55, reason, true
	}
	blob := strings.ToLower(termsBlob(terms))
	hits := 0
	for _, word := range words {
		if strings.Contains(blob, word) {
			hits++
		}
	}
	if hits == 0 {
		return 0, "", false
	}
	score = 40 + hits*15
	if score > 100 {
		score = 100
	}
	return score, FitReason(text, termsBlob(terms)), true
}

func termsBlob(terms map[string]any) string {
	parts := []string{TermsTitle(terms)}
	raw, _ := terms["obligations"].([]any)
	for _, item := range raw {
		obj, _ := item.(map[string]any)
		if dest, _ := obj["destination"].(string); dest != "" {
			parts = append(parts, dest)
		}
	}
	return strings.Join(parts, " ")
}

// DiscoverHaystack reads the caller's own text from a discover body.
func DiscoverHaystack(body map[string]any) (string, int) {
	limit := 5
	if n, ok := whole(body["limit"]); ok && n >= 1 && n <= 50 {
		limit = n
	}
	parts := []string{}
	if text, _ := body["query"].(string); text != "" {
		parts = append(parts, text)
	}
	if ctx, ok := body["context"].(map[string]any); ok {
		if text, _ := ctx["text"].(string); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " "), limit
}
