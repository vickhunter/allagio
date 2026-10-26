package orp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// Version04 is the many-party version this host speaks beside 0.3.
	Version04   = "0.4"
	freshWindow = 90 * 24 * time.Hour
)

var (
	partyRoles = map[string]bool{
		"offerer": true, "acceptor": true, "curator": true, "facilitator": true,
		"service": true, "signer": true, "member": true,
	}
	piiKeys = map[string]bool{
		"email": true, "name": true, "phone": true, "address": true,
		"given_name": true, "family_name": true, "tax_id": true, "ssn": true, "dob": true,
	}
	reservedHolders = map[string]bool{
		"host": true, "offerrails": true, "hostedapi": true,
	}
	offerV4Keys = map[string]bool{
		"id": true, "terms": true, "terms_hash": true, "limit": true, "note": true,
		"expires": true, "parties": true, "confirm": true,
	}
	termsV4Keys = map[string]bool{
		"title": true, "kind": true, "obligations": true, "categories": true,
		"disclosure": true, "starts": true, "ends": true,
		"checked_at": true, "expires_at": true, "claims": true,
	}
	// clientPrefixes are product-token prefixes. A hit is class bot.
	// Longer names come first. A device name inside parentheses is not a product token.
	clientPrefixes = []string{
		"python-requests", "python-urllib", "go-http-client", "facebookexternalhit",
		"headlesschrome", "node-fetch", "headless", "libwww", "scrapy", "okhttp",
		"axios", "postman", "whatsapp", "wget", "curl",
	}
)

// HolderKey compares a deposit holder after lowercasing and stripping
// spaces, hyphens, underscores, and dots.
func HolderKey(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if r == ' ' || r == '-' || r == '_' || r == '.' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Seat is one party on a stored 0.4 deal. Owner and Fills stay off the wire.
type Seat struct {
	ID       string         `json:"id"`
	Owner    string         `json:"owner,omitempty"`
	Role     string         `json:"role"`
	Required bool           `json:"required"`
	Signed   bool           `json:"signed"`
	Terms    map[string]any `json:"terms,omitempty"`
	Identity map[string]any `json:"identity,omitempty"`
	Fills    string         `json:"fills,omitempty"`
}

// Vacancy is a required seat that left. It clears only when the replacement
// seated for this id signs.
type Vacancy struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

// Track is one minted link. The token stays in storage. The wire link is built
// from it, so a rolled-back transaction cannot leave an orphan redirect.
type Track struct {
	Obligation  string `json:"obligation"`
	Token       string `json:"token"`
	Destination string `json:"destination"`
	Status      string `json:"status"`
	Param       string `json:"param"`
	Alias       string `json:"alias,omitempty"`
}

// Click is one redirect. Shares name parties. No address, user agent, or referrer.
type Click struct {
	ID         string `json:"id"`
	Obligation string `json:"obligation"`
	Class      string `json:"class"`
	Shares     []any  `json:"shares"`
}

// ClickReadout is the counted view of one obligation. clicks is the number of
// human clicks. Bot, preview, and test clicks are left out. click_share sums
// each party's percent or amount across the counted clicks, in seat order.
// A percent is stored as an integer from 0 to 100, and the sum is that
// integer divided by 100, so 50 twice is 1.0. An amount_minor stays an integer
// sum. The result has no address, user agent, or referrer.
func ClickReadout(doc Many, obligation string) (clicks int, share []any) {
	type acc struct {
		percent    int
		amount     int
		currency   string
		hasPercent bool
		hasAmount  bool
	}
	byParty := map[string]*acc{}
	order := make([]string, 0, len(doc.Seats))
	remember := func(party string) *acc {
		if party == "" {
			return nil
		}
		if row := byParty[party]; row != nil {
			return row
		}
		row := &acc{}
		byParty[party] = row
		order = append(order, party)
		return row
	}
	for _, seat := range doc.Seats {
		row := &acc{}
		if _, ok := wholeNumber(seat.Terms["percent"]); ok {
			row.hasPercent = true
		}
		if amount, ok := seat.Terms["amount"].(map[string]any); ok {
			if _, ok := wholeNumber(amount["amount_minor"]); ok {
				row.hasAmount = true
			}
			if currency, _ := amount["currency"].(string); currency != "" {
				row.currency = currency
			}
		}
		if !row.hasPercent && !row.hasAmount {
			continue
		}
		byParty[seat.ID] = row
		order = append(order, seat.ID)
	}
	for _, click := range doc.Clicks {
		if click.Class != "human" || click.Obligation != obligation {
			continue
		}
		clicks++
		for _, item := range click.Shares {
			one, _ := item.(map[string]any)
			row := remember(oneString(one, "party"))
			if row == nil {
				continue
			}
			if n, ok := wholeNumber(one["percent"]); ok {
				row.percent += n
				row.hasPercent = true
			}
			if n, ok := wholeNumber(one["amount_minor"]); ok {
				row.amount += n
				row.hasAmount = true
			}
			if currency := oneString(one, "currency"); currency != "" {
				row.currency = currency
			}
		}
	}
	share = make([]any, 0, len(order))
	for _, party := range order {
		row := byParty[party]
		item := map[string]any{"party": party}
		if row.hasPercent {
			item["percent"] = fixedPercent(row.percent)
		}
		if row.hasAmount {
			item["amount_minor"] = row.amount
			if row.currency != "" {
				item["currency"] = row.currency
			}
		}
		share = append(share, item)
	}
	return clicks, share
}

func oneString(row map[string]any, key string) string {
	if row == nil {
		return ""
	}
	text, _ := row[key].(string)
	return text
}

func wholeNumber(value any) (int, bool) {
	n, ok := asFloat(value)
	if !ok || n != float64(int(n)) || n < 0 || n > 1e9 {
		return 0, false
	}
	return int(n), true
}

func fixedPercent(percentPoints int) json.Number {
	whole := percentPoints / 100
	frac := percentPoints % 100
	if frac%10 == 0 {
		return json.Number(fmt.Sprintf("%d.%d", whole, frac/10))
	}
	return json.Number(fmt.Sprintf("%d.%02d", whole, frac))
}

// Many is the nullable parties document. A 0.3 row leaves the column NULL.
type Many struct {
	Seats     []Seat    `json:"seats"`
	Vacancies []Vacancy `json:"vacancies,omitempty"`
	Tracking  []Track   `json:"tracking,omitempty"`
	Clicks    []Click   `json:"clicks,omitempty"`
	Live      bool      `json:"live"`
	Round     int       `json:"round"`
}

// DecodeMany reads a stored parties document.
func DecodeMany(raw json.RawMessage) (Many, error) {
	var doc Many
	if len(raw) == 0 {
		return doc, nil
	}
	err := json.Unmarshal(raw, &doc)
	return doc, err
}

// Canonical returns the party object that enters the deal hash.
// signed, owner, and fills are not part of it.
func (s Seat) Canonical() map[string]any {
	item := map[string]any{"id": s.ID, "role": s.Role, "required": s.Required}
	if len(s.Terms) > 0 {
		item["terms"] = s.Terms
	}
	if len(s.Identity) > 0 {
		item["identity"] = s.Identity
	}
	return item
}

// DealHash is the JCS SHA-256 of {terms, parties} in offer order.
func DealHash(terms map[string]any, seats []Seat) string {
	parties := make([]any, len(seats))
	for i, seat := range seats {
		parties[i] = seat.Canonical()
	}
	return ContentHash(map[string]any{"terms": terms, "parties": parties})
}

// ValidateParties applies SPEC §20.1. per, hours, and deposit are not amounts.
func ValidateParties(parties []any, hostID string) (Problem, bool) {
	if len(parties) < 2 || len(parties) > 16 {
		return Invalid("A deal has 2 to 16 parties.", "/body/parties"), true
	}
	ids := map[string]bool{}
	percentSum := 0
	percentCount := 0
	fixedCount := 0
	offerers := 0
	acceptors := 0
	required := 0
	hostKey := HolderKey(hostID)
	for index, item := range parties {
		field := "/body/parties/" + itoa(index)
		party, ok := item.(map[string]any)
		if !ok {
			return Invalid("A party is an object.", field), true
		}
		id, _ := party["id"].(string)
		if !partyPattern.MatchString(id) || ids[id] {
			return Invalid("Party ids are unique.", field+"/id"), true
		}
		ids[id] = true
		role, _ := party["role"].(string)
		if !partyRoles[role] {
			return Invalid("The message is not valid ORP 0.4.", field+"/role"), true
		}
		req, ok := party["required"].(bool)
		if !ok {
			return Invalid("The message is not valid ORP 0.4.", field+"/required"), true
		}
		if req {
			required++
		}
		if role == "offerer" {
			offerers++
		}
		if role == "acceptor" {
			acceptors++
		}
		if offerers > 1 || acceptors > 1 {
			return Invalid("A deal has at most one offerer and one acceptor.", field), true
		}
		for key := range party {
			if piiKeys[key] {
				return Invalid("A deal stores no personal data.", field+"/"+escapePointer(key)), true
			}
			switch key {
			case "id", "role", "required", "terms", "identity":
			default:
				return Invalid("The message is not valid ORP 0.4.", field+"/"+escapePointer(key)), true
			}
		}
		if identity, present := party["identity"]; present {
			obj, ok := identity.(map[string]any)
			if !ok {
				return Invalid("Identity is a hash and an issuer name only.", field+"/identity"), true
			}
			for key := range obj {
				if key != "hash" && key != "issuer" {
					return Invalid("Identity is a hash and an issuer name only.", field+"/identity"), true
				}
			}
			hash, _ := obj["hash"].(string)
			issuer, _ := obj["issuer"].(string)
			if !hashPattern.MatchString(hash) || issuer == "" || utf8.RuneCountInString(issuer) > 64 {
				return Invalid("Identity is a hash and an issuer name only.", field+"/identity"), true
			}
		}
		terms, _ := party["terms"].(map[string]any)
		if raw, present := party["terms"]; present {
			if terms == nil {
				return Invalid("The message is not valid ORP 0.4.", field+"/terms"), true
			}
			_ = raw
		}
		for key := range terms {
			if piiKeys[key] {
				return Invalid("A deal stores no personal data.", field+"/terms/"+escapePointer(key)), true
			}
		}
		if deposit, present := terms["deposit"]; present {
			obj, _ := deposit.(map[string]any)
			holder, _ := obj["holder"].(string)
			key := HolderKey(holder)
			if obj == nil || key == "" || reservedHolders[key] || (hostKey != "" && key == hostKey) {
				return Invalid("Offer Rails does not hold money. A deposit term names an outside holder.", field+"/terms/deposit/holder"), true
			}
		}
		_, percent := terms["percent"]
		_, fixed := terms["amount"]
		if percent {
			n, ok := whole(terms["percent"])
			if !ok || n < 0 || n > 100 {
				return Invalid("A percent split is an integer from 0 to 100.", field+"/terms/percent"), true
			}
			percentCount++
			percentSum += n
		}
		if fixed {
			fixedCount++
		}
		if percent && fixed {
			if _, cap := terms["cap"]; !cap {
				return Invalid("A mixed percent and fixed split needs a cap on each percent term.", field+"/terms/cap"), true
			}
		}
	}
	if required == 0 {
		return Invalid("A deal needs a required party.", "/body/parties"), true
	}
	if percentCount > 0 && fixedCount > 0 {
		for index, item := range parties {
			party := item.(map[string]any)
			terms, _ := party["terms"].(map[string]any)
			if _, percent := terms["percent"]; percent {
				if _, cap := terms["cap"]; !cap {
					return Invalid("A mixed percent and fixed split needs a cap on each percent term.", "/body/parties/"+itoa(index)+"/terms/cap"), true
				}
			}
		}
		if percentSum > 100 {
			return Invalid("Percent splits over 100 are rejected.", "/body/parties"), true
		}
		return Problem{}, false
	}
	if percentCount > 0 && percentSum != 100 {
		return Invalid("Percent splits must sum to 100.", "/body/parties"), true
	}
	return Problem{}, false
}

// SignatureState is active only when every required seat is signed and no
// vacancy is open. An optional seat does not block active.
func SignatureState(seats []Seat, vacancies []Vacancy) string {
	if len(vacancies) > 0 {
		return "pending"
	}
	required := 0
	signed := 0
	for _, seat := range seats {
		if !seat.Required {
			continue
		}
		required++
		if seat.Signed {
			signed++
		}
	}
	if required > 0 && signed == required {
		return "active"
	}
	return "pending"
}

// NormalizeClaims writes checked_at and expires_at. With an empty checked set
// every existing claim stays the string unknown. Absent claims are not invented.
func NormalizeClaims(terms map[string]any, now time.Time) map[string]any {
	next := cloneMap(terms)
	checked := Stamp(now)
	expires := Stamp(now.Add(freshWindow))
	next["checked_at"] = checked
	next["expires_at"] = expires
	raw, ok := next["claims"].(map[string]any)
	if !ok {
		delete(next, "claims")
		return next
	}
	claims := map[string]any{}
	for key := range raw {
		claims[key] = "unknown"
	}
	next["claims"] = claims
	return next
}

// OfferExpired reports a 0.4 offer whose expires_at is at or before now.
// A 0.3 offer has no expires_at and stays listed.
func OfferExpired(terms map[string]any, now time.Time) bool {
	raw, _ := terms["expires_at"].(string)
	if raw == "" {
		return false
	}
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	return !at.After(now)
}

// ClickShares attributes one click by percent and amount only.
func ClickShares(seats []Seat) []any {
	out := make([]any, 0)
	for _, seat := range seats {
		if len(seat.Terms) == 0 {
			continue
		}
		share := map[string]any{"party": seat.ID}
		kept := false
		if _, ok := seat.Terms["percent"]; ok {
			if n, ok := whole(seat.Terms["percent"]); ok {
				share["percent"] = n
				kept = true
			}
		}
		if amount, ok := seat.Terms["amount"].(map[string]any); ok {
			if n, ok := whole(amount["amount_minor"]); ok {
				share["amount_minor"] = n
				kept = true
			}
			if currency, _ := amount["currency"].(string); currency != "" {
				share["currency"] = currency
			}
		}
		if kept {
			out = append(out, share)
		}
	}
	return out
}

// ClickClass is the one class for a redirect. The user agent string is not stored.
func ClickClass(method, ua, purpose, rawQuery string) string {
	values, _ := url.ParseQuery(rawQuery)
	if values.Get("orp_test") == "1" {
		return "test"
	}
	if strings.EqualFold(method, "HEAD") {
		return "preview"
	}
	low := strings.ToLower(purpose)
	if strings.Contains(low, "prefetch") || strings.Contains(low, "prerender") {
		return "preview"
	}
	if strings.TrimSpace(ua) == "" {
		return "bot"
	}
	if automationClient(ua) {
		return "bot"
	}
	return "human"
}

// automationClient is true for a known automation product token.
// A token is the name before "/", or the name after "compatible;".
// Text inside parentheses is a comment, so CUBOT NOTE 20 stays human.
// bot matches when that token ends in bot or bot-..., or is bot.html.
// crawler, spider, and slurp match anywhere. preview, discord, whatsapp,
// telegram, and bot are not bare substrings.
func automationClient(ua string) bool {
	low := strings.ToLower(ua)
	if strings.Contains(low, "crawler") || strings.Contains(low, "spider") || strings.Contains(low, "slurp") {
		return true
	}
	if strings.Contains(low, "/bot.html") || strings.HasPrefix(low, "bot.html") {
		return true
	}
	for _, token := range productTokens(low) {
		if botProduct(token) || namedClient(token) {
			return true
		}
	}
	return false
}

func botProduct(token string) bool {
	name := strings.TrimSuffix(token, ".html")
	return strings.HasSuffix(name, "bot") || strings.Contains(name, "bot-")
}

func namedClient(token string) bool {
	for _, name := range clientPrefixes {
		if token == name || strings.HasPrefix(token, name) {
			return true
		}
	}
	return false
}

func productTokens(ua string) []string {
	var out []string
	i := 0
	for i < len(ua) {
		if ua[i] == '(' {
			end, inner := commentSpan(ua, i)
			out = append(out, compatibleNames(inner)...)
			i = end
			continue
		}
		if !isProductChar(ua[i]) {
			i++
			continue
		}
		j := i + 1
		for j < len(ua) && isProductChar(ua[j]) {
			j++
		}
		out = append(out, ua[i:j])
		i = j
	}
	return out
}

func compatibleNames(comment string) []string {
	var out []string
	rest := comment
	for {
		i := strings.Index(rest, "compatible;")
		if i < 0 {
			return out
		}
		rest = strings.TrimLeft(rest[i+len("compatible;"):], " \t")
		j := 0
		for j < len(rest) && isProductChar(rest[j]) {
			j++
		}
		if j > 0 {
			out = append(out, rest[:j])
		}
		rest = rest[j:]
	}
}

func commentSpan(ua string, open int) (int, string) {
	depth := 1
	i := open + 1
	for i < len(ua) && depth > 0 {
		switch ua[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		i++
	}
	if depth == 0 {
		return i, ua[open+1 : i-1]
	}
	return len(ua), ua[open+1:]
}

func isProductChar(c byte) bool {
	if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	default:
		return false
	}
}

// MintTracking mints place links that carry content.link or content.code.
// When none exist it mints one link on the first obligation so a click can
// still be attributed. A later return to active does not mint again.
func MintTracking(origin, dealID string, terms map[string]any, existing []Track) []Track {
	if len(existing) > 0 {
		return existing
	}
	raw, _ := terms["obligations"].([]any)
	out := make([]Track, 0)
	for _, item := range raw {
		obj, _ := item.(map[string]any)
		if obj["action"] != "place" {
			continue
		}
		content, _ := obj["content"].(map[string]any)
		link, _ := content["link"].(bool)
		code, _ := content["code"].(bool)
		if !link && !code {
			continue
		}
		dest, _ := obj["destination"].(string)
		id, _ := obj["id"].(string)
		if dest == "" || id == "" {
			continue
		}
		out = append(out, Track{
			Obligation: id, Token: LocalID("tk"), Destination: dest,
			Status: "live", Param: "via", Alias: "ref",
		})
	}
	if len(out) == 0 && len(raw) > 0 {
		obj, _ := raw[0].(map[string]any)
		id, _ := obj["id"].(string)
		if id == "" {
			id = "share"
		}
		dest, _ := obj["destination"].(string)
		if dest == "" {
			dest = strings.TrimRight(origin, "/") + "/orp/proof/" + dealID
		}
		out = append(out, Track{
			Obligation: id, Token: LocalID("tk"), Destination: dest,
			Status: "live", Param: "via", Alias: "ref",
		})
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	raw, err := json.Marshal(in)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil {
		return map[string]any{}
	}
	return out
}

func checkOfferV4(body map[string]any) (Problem, bool) {
	if problem, bad := unknownKey(body, offerV4Keys, "/body"); bad {
		return relabel04(problem), true
	}
	if confirm, present := body["confirm"]; present {
		flag, ok := confirm.(bool)
		if !ok {
			return Invalid("The message is not valid ORP 0.4.", "/body/confirm"), true
		}
		if flag {
			if problem, bad := optionalLocal(body, "id", "/body/id"); bad {
				return relabel04(problem), true
			}
			if _, ok := body["id"].(string); !ok || body["id"] == "" {
				return Invalid("The message is not valid ORP 0.4.", "/body/id"), true
			}
			return Problem{}, false
		}
	}
	terms, ok := body["terms"].(map[string]any)
	if !ok {
		return Invalid("The message is not valid ORP 0.4.", "/body/terms"), true
	}
	if problem, bad := CheckTermsV4(terms, "/body/terms"); bad {
		return problem, true
	}
	if problem, bad := optionalHash(body, "terms_hash", "/body/terms_hash"); bad {
		return relabel04(problem), true
	}
	if problem, bad := optionalLocal(body, "id", "/body/id"); bad {
		return relabel04(problem), true
	}
	if limit, present := body["limit"]; present {
		n, ok := whole(limit)
		if !ok || n < 1 || n > 100000 {
			return Invalid("The message is not valid ORP 0.4.", "/body/limit"), true
		}
	}
	if problem, bad := optionalText(body, "note", "/body/note"); bad {
		return relabel04(problem), true
	}
	if problem, bad := optionalTimestamp(body, "expires", "/body/expires"); bad {
		return relabel04(problem), true
	}
	if parties, present := body["parties"]; present {
		raw, ok := parties.([]any)
		if !ok {
			return Invalid("The message is not valid ORP 0.4.", "/body/parties"), true
		}
		if problem, bad := ValidateParties(raw, ""); bad {
			return problem, true
		}
	}
	if problem, bad := personalIn(body, "/body"); bad {
		return relabel04(problem), true
	}
	return Problem{}, false
}

// CheckTermsV4 accepts 0.4 terms. Share obligations and party-id by values
// stay in the stored terms. 0.3 checkTerms is unchanged.
func CheckTermsV4(terms map[string]any, path string) (Problem, bool) {
	if problem, bad := unknownKey(terms, termsV4Keys, path); bad {
		return relabel04(problem), true
	}
	if problem, bad := checkTermsFrame(terms, path); bad {
		return problem, true
	}
	raw, ok := terms["obligations"].([]any)
	if !ok || len(raw) < 1 || len(raw) > 16 {
		return Invalid("The message is not valid ORP 0.4.", path+"/obligations"), true
	}
	seen := map[string]bool{}
	rest := make([]any, 0)
	for i, item := range raw {
		obj, ok := item.(map[string]any)
		if !ok {
			return Invalid("The message is not valid ORP 0.4.", path+"/obligations/"+itoa(i)), true
		}
		id, _ := obj["id"].(string)
		if !obligationPattern.MatchString(id) || seen[id] {
			return Invalid("The message is not valid ORP 0.4.", path+"/obligations/"+itoa(i)+"/id"), true
		}
		seen[id] = true
		action, _ := obj["action"].(string)
		if action == "share" {
			if problem, bad := checkShare(obj, path+"/obligations/"+itoa(i)); bad {
				return problem, true
			}
			continue
		}
		clone := cloneMap(obj)
		by, _ := clone["by"].(string)
		if !v4By(by) {
			return Invalid("The message is not valid ORP 0.4.", path+"/obligations/"+itoa(i)+"/by"), true
		}
		if by != "offerer" && by != "acceptor" {
			clone["by"] = "acceptor"
		}
		rest = append(rest, clone)
	}
	if len(rest) == 0 {
		return Problem{}, false
	}
	if len(rest) > 10 {
		for _, ob := range rest {
			one := cloneMap(terms)
			delete(one, "checked_at")
			delete(one, "expires_at")
			delete(one, "claims")
			one["obligations"] = []any{ob}
			if problem, bad := checkTerms(one, path); bad {
				return relabel04(problem), true
			}
		}
		return Problem{}, false
	}
	clone := cloneMap(terms)
	delete(clone, "checked_at")
	delete(clone, "expires_at")
	delete(clone, "claims")
	clone["obligations"] = rest
	problem, bad := checkTerms(clone, path)
	return relabel04(problem), bad
}

func checkTermsFrame(terms map[string]any, path string) (Problem, bool) {
	title, _ := terms["title"].(string)
	if title == "" || utf8.RuneCountInString(title) > 120 {
		return Invalid("The message is not valid ORP 0.4.", path+"/title"), true
	}
	if kind, present := terms["kind"]; present {
		text, ok := kind.(string)
		if !ok || !dealKinds[text] {
			return Invalid("The message is not valid ORP 0.4.", path+"/kind"), true
		}
	}
	if categories, present := terms["categories"]; present {
		if problem, bad := checkCategories(categories, path+"/categories"); bad {
			return relabel04(problem), true
		}
	}
	if disclosure, present := terms["disclosure"]; present {
		obj, ok := disclosure.(map[string]any)
		if !ok {
			return Invalid("The message is not valid ORP 0.4.", path+"/disclosure"), true
		}
		if problem, bad := checkDisclosure(obj, path+"/disclosure"); bad {
			return relabel04(problem), true
		}
	}
	for _, key := range []string{"starts", "ends", "checked_at", "expires_at"} {
		if problem, bad := optionalTimestamp(terms, key, path+"/"+key); bad {
			return relabel04(problem), true
		}
	}
	if problem, bad := personalIn(terms, path); bad {
		return relabel04(problem), true
	}
	if raw, present := terms["claims"]; present {
		if problem, bad := checkClaims(raw, path+"/claims"); bad {
			return problem, true
		}
	}
	return Problem{}, false
}

func checkClaims(value any, path string) (Problem, bool) {
	obj, ok := value.(map[string]any)
	if !ok || len(obj) > 16 {
		return Invalid("The message is not valid ORP 0.4.", path), true
	}
	for key := range obj {
		if key == "" || key[0] < 'a' || key[0] > 'z' || len(key) > 41 {
			return Invalid("The message is not valid ORP 0.4.", path+"/"+escapePointer(key)), true
		}
	}
	return Problem{}, false
}

func checkShare(obj map[string]any, path string) (Problem, bool) {
	for key := range obj {
		if key != "id" && key != "by" && key != "action" {
			return Invalid("The message is not valid ORP 0.4.", path+"/"+escapePointer(key)), true
		}
	}
	by, _ := obj["by"].(string)
	if !v4By(by) {
		return Invalid("The message is not valid ORP 0.4.", path+"/by"), true
	}
	return Problem{}, false
}

func v4By(by string) bool {
	return partyRoles[by] || partyPattern.MatchString(by)
}

func relabel04(problem Problem) Problem {
	problem.Message = strings.ReplaceAll(problem.Message, "ORP 0.3.", "ORP 0.4.")
	return problem
}

// HasCheckedAt reports a standing offer the host normalized as 0.4.
func HasCheckedAt(terms map[string]any) bool {
	_, ok := terms["checked_at"].(string)
	return ok
}
