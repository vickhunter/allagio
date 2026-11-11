package orp

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	obligationPattern = regexp.MustCompile(`^[a-z0-9_\-]{1,32}$`)
	currencyPattern   = regexp.MustCompile(`^[A-Z]{3}$`)
	railPattern       = regexp.MustCompile(`^[a-z0-9_.\-]{2,40}$`)
	emailPattern      = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	durationPattern   = regexp.MustCompile(`^P(?:\d+Y)?(?:\d+M)?(?:\d+W)?(?:\d+D)?(?:T(?:\d+H)?(?:\d+M)?(?:\d+S)?)?$`)
	proofMethods      = map[string]bool{
		"link": true, "proof_url": true, "platform_check": true,
		"attestation": true, "witness": true, "provider": true,
	}
	personalKeys = map[string]bool{
		"email": true, "phone": true, "password": true, "ip": true,
		"user_agent": true, "referrer": true, "address": true,
	}
	offerBodyKeys = map[string]bool{
		"id": true, "terms": true, "terms_hash": true, "limit": true, "note": true, "expires": true,
	}
	acceptBodyKeys = map[string]bool{
		"terms_hash": true, "offer": true, "spaces": true, "note": true,
	}
	executeBodyKeys = map[string]bool{
		"obligation": true, "proof_url": true, "space": true, "at": true, "note": true,
	}
)

func checkOfferBody(body map[string]any) (Problem, bool) {
	if problem, bad := unknownKey(body, offerBodyKeys, "/body"); bad {
		return problem, true
	}
	terms, ok := body["terms"].(map[string]any)
	if !ok {
		return Invalid("The message is not valid ORP 0.3.", "/body/terms"), true
	}
	if problem, bad := checkTerms(terms, "/body/terms"); bad {
		return problem, true
	}
	if problem, bad := optionalHash(body, "terms_hash", "/body/terms_hash"); bad {
		return problem, true
	}
	if problem, bad := optionalLocal(body, "id", "/body/id"); bad {
		return problem, true
	}
	if limit, present := body["limit"]; present {
		n, ok := whole(limit)
		if !ok || n < 1 || n > 100000 {
			return Invalid("The message is not valid ORP 0.3.", "/body/limit"), true
		}
	}
	if problem, bad := optionalText(body, "note", "/body/note"); bad {
		return problem, true
	}
	if problem, bad := optionalTimestamp(body, "expires", "/body/expires"); bad {
		return problem, true
	}
	if problem, bad := personalIn(body, "/body"); bad {
		return problem, true
	}
	return Problem{}, false
}

func checkAcceptBody(body map[string]any) (Problem, bool) {
	if problem, bad := unknownKey(body, acceptBodyKeys, "/body"); bad {
		return problem, true
	}
	rawHash, present := body["terms_hash"]
	hash, hashOK := rawHash.(string)
	if !present || !hashOK || hash == "" {
		return Invalid("terms_hash must be sha256-<b64url43>.", "/body/terms_hash"), true
	}
	// A present string that is not this offer's hash is terms_mismatch,
	// including a string that is not sha256-<b64url43>. The host compares
	// it with the stored hash instead of refusing the shape.
	if problem, bad := optionalLocal(body, "offer", "/body/offer"); bad {
		return problem, true
	}
	if problem, bad := optionalText(body, "note", "/body/note"); bad {
		return problem, true
	}
	if spaces, present := body["spaces"]; present {
		if problem, bad := checkAcceptSpaces(spaces); bad {
			return problem, true
		}
	}
	if problem, bad := personalIn(body, "/body"); bad {
		return problem, true
	}
	return Problem{}, false
}

func checkExecuteBody(body map[string]any) (Problem, bool) {
	if problem, bad := unknownKey(body, executeBodyKeys, "/body"); bad {
		return problem, true
	}
	obligation, _ := body["obligation"].(string)
	if !obligationPattern.MatchString(obligation) {
		return Invalid("The message is not valid ORP 0.3.", "/body/obligation"), true
	}
	if raw, present := body["proof_url"]; present {
		text, ok := raw.(string)
		if !ok || !validURL(text) {
			return Invalid("The message is not valid ORP 0.3.", "/body/proof_url"), true
		}
	}
	if problem, bad := optionalLocal(body, "space", "/body/space"); bad {
		return problem, true
	}
	if problem, bad := optionalTimestamp(body, "at", "/body/at"); bad {
		return problem, true
	}
	if problem, bad := optionalText(body, "note", "/body/note"); bad {
		return problem, true
	}
	if problem, bad := personalIn(body, "/body"); bad {
		return problem, true
	}
	return Problem{}, false
}

func checkAcceptSpaces(value any) (Problem, bool) {
	raw, ok := value.([]any)
	if !ok || len(raw) > 10 {
		return Invalid("The message is not valid ORP 0.3.", "/body/spaces"), true
	}
	for i, item := range raw {
		path := "/body/spaces/" + itoa(i)
		obj, ok := item.(map[string]any)
		if !ok {
			return Invalid("The message is not valid ORP 0.3.", path), true
		}
		if problem, bad := unknownKey(obj, map[string]bool{"obligation": true, "space": true}, path); bad {
			return problem, true
		}
		obligation, _ := obj["obligation"].(string)
		space, _ := obj["space"].(string)
		if !obligationPattern.MatchString(obligation) || !localPattern.MatchString(space) {
			return Invalid("The message is not valid ORP 0.3.", path), true
		}
	}
	return Problem{}, false
}

// CheckTerms reports whether terms are a closed ORP 0.3 object this host can store.
func CheckTerms(terms map[string]any, path string) (Problem, bool) {
	return checkTerms(terms, path)
}

func checkTerms(terms map[string]any, path string) (Problem, bool) {
	allowed := map[string]bool{
		"title": true, "kind": true, "obligations": true, "categories": true,
		"disclosure": true, "starts": true, "ends": true,
	}
	if problem, bad := unknownKey(terms, allowed, path); bad {
		return problem, true
	}
	title, _ := terms["title"].(string)
	if title == "" || utf8.RuneCountInString(title) > 120 {
		return Invalid("The message is not valid ORP 0.3.", path+"/title"), true
	}
	if kind, present := terms["kind"]; present {
		text, ok := kind.(string)
		if !ok || !dealKinds[text] {
			return Invalid("The message is not valid ORP 0.3.", path+"/kind"), true
		}
	}
	raw, ok := terms["obligations"].([]any)
	if !ok || len(raw) < 1 || len(raw) > 10 {
		return Invalid("The message is not valid ORP 0.3.", path+"/obligations"), true
	}
	needsDisclosure := false
	seen := map[string]bool{}
	for i, item := range raw {
		obj, ok := item.(map[string]any)
		if !ok {
			return Invalid("The message is not valid ORP 0.3.", path+"/obligations/"+itoa(i)), true
		}
		if problem, bad := checkObligation(obj, path+"/obligations/"+itoa(i)); bad {
			return problem, true
		}
		id, _ := obj["id"].(string)
		if seen[id] {
			return Invalid("The message is not valid ORP 0.3.", path+"/obligations/"+itoa(i)+"/id"), true
		}
		seen[id] = true
		switch obj["action"] {
		case "place", "comment", "repost":
			needsDisclosure = true
		}
	}
	if categories, present := terms["categories"]; present {
		if problem, bad := checkCategories(categories, path+"/categories"); bad {
			return problem, true
		}
	}
	if disclosure, present := terms["disclosure"]; present {
		obj, ok := disclosure.(map[string]any)
		if !ok {
			return Invalid("The message is not valid ORP 0.3.", path+"/disclosure"), true
		}
		if problem, bad := checkDisclosure(obj, path+"/disclosure"); bad {
			return problem, true
		}
	} else if needsDisclosure {
		return Invalid("The message is not valid ORP 0.3.", path+"/disclosure"), true
	}
	if problem, bad := optionalTimestamp(terms, "starts", path+"/starts"); bad {
		return problem, true
	}
	if problem, bad := optionalTimestamp(terms, "ends", path+"/ends"); bad {
		return problem, true
	}
	if problem, bad := personalIn(terms, path); bad {
		return problem, true
	}
	return Problem{}, false
}

func checkObligation(obj map[string]any, path string) (Problem, bool) {
	action, _ := obj["action"].(string)
	switch action {
	case "place":
		return checkPlace(obj, path)
	case "repost":
		return checkRepost(obj, path)
	case "comment":
		return checkComment(obj, path)
	case "pay":
		return checkPay(obj, path)
	case "credit":
		return checkCredit(obj, path)
	case "share":
		return checkShare03(obj, path)
	default:
		return Invalid("The message is not valid ORP 0.3.", path+"/action"), true
	}
}

// checkShare03 validates a share obligation: the obligor hands the counterparty
// an aggregate data set described by data, optionally by a due time.
func checkShare03(obj map[string]any, path string) (Problem, bool) {
	allowed := map[string]bool{
		"id": true, "by": true, "action": true, "data": true, "due": true, "proof": true,
	}
	if problem, bad := unknownKey(obj, allowed, path); bad {
		return problem, true
	}
	if problem, bad := obligationIdentity(obj, path); bad {
		return problem, true
	}
	if _, present := obj["data"]; !present {
		return Invalid("The message is not valid ORP 0.3.", path+"/data"), true
	}
	if problem, bad := optionalDeadline(obj, path+"/due"); bad {
		return problem, true
	}
	if problem, bad := proofList(obj, path, false); bad {
		return problem, true
	}
	return Problem{}, false
}

func checkPlace(obj map[string]any, path string) (Problem, bool) {
	allowed := map[string]bool{
		"id": true, "by": true, "action": true, "space": true, "content": true,
		"destination": true, "count": true, "clicks": true, "due": true, "keep": true, "proof": true,
	}
	if problem, bad := unknownKey(obj, allowed, path); bad {
		return problem, true
	}
	if problem, bad := obligationIdentity(obj, path); bad {
		return problem, true
	}
	if problem, bad := proofList(obj, path, true); bad {
		return problem, true
	}
	return checkPlaceExtras(obj, path)
}

func checkPlaceExtras(obj map[string]any, path string) (Problem, bool) {
	if problem, bad := optionalSpace(obj, path); bad {
		return problem, true
	}
	if problem, bad := optionalContent(obj, path); bad {
		return problem, true
	}
	if problem, bad := optionalURL(obj, "destination", path+"/destination"); bad {
		return problem, true
	}
	if problem, bad := optionalCount(obj, "count", path+"/count", 1, 100); bad {
		return problem, true
	}
	if problem, bad := optionalCount(obj, "clicks", path+"/clicks", 1, 10000000); bad {
		return problem, true
	}
	if problem, bad := optionalDeadline(obj, path+"/due"); bad {
		return problem, true
	}
	if problem, bad := optionalDurationKey(obj, "keep", path+"/keep"); bad {
		return problem, true
	}
	return Problem{}, false
}

func checkRepost(obj map[string]any, path string) (Problem, bool) {
	allowed := map[string]bool{
		"id": true, "by": true, "action": true, "target": true, "space": true,
		"quote": true, "due": true, "keep": true, "proof": true,
	}
	if problem, bad := unknownKey(obj, allowed, path); bad {
		return problem, true
	}
	if problem, bad := obligationIdentity(obj, path); bad {
		return problem, true
	}
	if problem, bad := requiredURL(obj, "target", path+"/target"); bad {
		return problem, true
	}
	if problem, bad := proofList(obj, path, true); bad {
		return problem, true
	}
	if problem, bad := optionalSpace(obj, path); bad {
		return problem, true
	}
	if quote, present := obj["quote"]; present {
		if _, ok := quote.(bool); !ok {
			return Invalid("The message is not valid ORP 0.3.", path+"/quote"), true
		}
	}
	if problem, bad := optionalDeadline(obj, path+"/due"); bad {
		return problem, true
	}
	if problem, bad := optionalDurationKey(obj, "keep", path+"/keep"); bad {
		return problem, true
	}
	return Problem{}, false
}

func checkComment(obj map[string]any, path string) (Problem, bool) {
	allowed := map[string]bool{
		"id": true, "by": true, "action": true, "target": true, "content": true,
		"due": true, "keep": true, "proof": true,
	}
	if problem, bad := unknownKey(obj, allowed, path); bad {
		return problem, true
	}
	if problem, bad := obligationIdentity(obj, path); bad {
		return problem, true
	}
	if problem, bad := requiredURL(obj, "target", path+"/target"); bad {
		return problem, true
	}
	if problem, bad := proofList(obj, path, true); bad {
		return problem, true
	}
	if problem, bad := optionalContent(obj, path); bad {
		return problem, true
	}
	if problem, bad := optionalDeadline(obj, path+"/due"); bad {
		return problem, true
	}
	if problem, bad := optionalDurationKey(obj, "keep", path+"/keep"); bad {
		return problem, true
	}
	return Problem{}, false
}

func checkPay(obj map[string]any, path string) (Problem, bool) {
	allowed := map[string]bool{
		"id": true, "by": true, "action": true, "per": true, "amount": true,
		"rate_bps": true, "cap": true, "window": true, "hold": true, "after": true,
		"due": true, "rail": true, "proof": true,
	}
	if problem, bad := unknownKey(obj, allowed, path); bad {
		return problem, true
	}
	if problem, bad := obligationIdentity(obj, path); bad {
		return problem, true
	}
	per, _ := obj["per"].(string)
	if per != "deal" && per != "placement" && per != "click" && per != "conversion" {
		return Invalid("The message is not valid ORP 0.3.", path+"/per"), true
	}
	_, hasAmount := obj["amount"]
	_, hasRate := obj["rate_bps"]
	if !hasAmount && !hasRate {
		return Invalid("The message is not valid ORP 0.3.", path+"/amount"), true
	}
	if hasAmount {
		if problem, bad := checkMoney(obj["amount"], path+"/amount"); bad {
			return problem, true
		}
	}
	if hasRate {
		n, ok := whole(obj["rate_bps"])
		if !ok || n < 1 || n > 10000 {
			return Invalid("The message is not valid ORP 0.3.", path+"/rate_bps"), true
		}
	}
	if cap, present := obj["cap"]; present {
		if problem, bad := checkMoney(cap, path+"/cap"); bad {
			return problem, true
		}
	}
	if problem, bad := optionalDurationKey(obj, "window", path+"/window"); bad {
		return problem, true
	}
	if problem, bad := optionalDurationKey(obj, "hold", path+"/hold"); bad {
		return problem, true
	}
	if problem, bad := optionalObligationRef(obj, "after", path+"/after"); bad {
		return problem, true
	}
	if problem, bad := optionalDeadline(obj, path+"/due"); bad {
		return problem, true
	}
	if rail, present := obj["rail"]; present {
		text, ok := rail.(string)
		if !ok || !railPattern.MatchString(text) {
			return Invalid("The message is not valid ORP 0.3.", path+"/rail"), true
		}
	}
	if _, present := obj["proof"]; present {
		if problem, bad := proofList(obj, path, false); bad {
			return problem, true
		}
	}
	return Problem{}, false
}

func checkCredit(obj map[string]any, path string) (Problem, bool) {
	allowed := map[string]bool{
		"id": true, "by": true, "action": true, "credits": true, "after": true,
	}
	if problem, bad := unknownKey(obj, allowed, path); bad {
		return problem, true
	}
	if problem, bad := obligationIdentity(obj, path); bad {
		return problem, true
	}
	n, ok := whole(obj["credits"])
	if !ok || n < 1 || n > 1000000 {
		return Invalid("The message is not valid ORP 0.3.", path+"/credits"), true
	}
	if problem, bad := optionalObligationRef(obj, "after", path+"/after"); bad {
		return problem, true
	}
	return Problem{}, false
}

func obligationIdentity(obj map[string]any, path string) (Problem, bool) {
	id, _ := obj["id"].(string)
	if !obligationPattern.MatchString(id) {
		return Invalid("The message is not valid ORP 0.3.", path+"/id"), true
	}
	by, _ := obj["by"].(string)
	if by != "offerer" && by != "acceptor" {
		return Invalid("The message is not valid ORP 0.3.", path+"/by"), true
	}
	return Problem{}, false
}

func proofList(obj map[string]any, path string, required bool) (Problem, bool) {
	raw, present := obj["proof"]
	if !present {
		if required {
			return Invalid("The message is not valid ORP 0.3.", path+"/proof"), true
		}
		return Problem{}, false
	}
	items, ok := raw.([]any)
	if !ok || len(items) < 1 || len(items) > 6 {
		return Invalid("The message is not valid ORP 0.3.", path+"/proof"), true
	}
	seen := map[string]bool{}
	for i, item := range items {
		text, ok := item.(string)
		if !ok || seen[text] || !validProof(text) {
			return Invalid("The message is not valid ORP 0.3.", path+"/proof/"+itoa(i)), true
		}
		seen[text] = true
	}
	return Problem{}, false
}

func validProof(text string) bool {
	if proofMethods[text] {
		return true
	}
	return extPattern.MatchString(text) && len(text) <= 128
}

func checkDisclosure(obj map[string]any, path string) (Problem, bool) {
	if problem, bad := unknownKey(obj, map[string]bool{"label": true, "lang": true}, path); bad {
		return problem, true
	}
	label, _ := obj["label"].(string)
	if utf8.RuneCountInString(label) < 2 || utf8.RuneCountInString(label) > 80 {
		return Invalid("The message is not valid ORP 0.3.", path+"/label"), true
	}
	if lang, present := obj["lang"]; present {
		text, ok := lang.(string)
		if !ok || !langPattern.MatchString(text) {
			return Invalid("The message is not valid ORP 0.3.", path+"/lang"), true
		}
	}
	return Problem{}, false
}

func checkCategories(value any, path string) (Problem, bool) {
	raw, ok := value.([]any)
	if !ok || len(raw) > 5 {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	for i, item := range raw {
		text, ok := item.(string)
		if !ok || !categoryPattern.MatchString(text) || len(text) > 64 {
			return Invalid("The message is not valid ORP 0.3.", path+"/"+itoa(i)), true
		}
	}
	return Problem{}, false
}

func checkMoney(value any, path string) (Problem, bool) {
	obj, ok := value.(map[string]any)
	if !ok {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	if problem, bad := unknownKey(obj, map[string]bool{"amount_minor": true, "currency": true}, path); bad {
		return problem, true
	}
	n, ok := whole(obj["amount_minor"])
	if !ok || n < 0 || n > 100000000000 {
		return Invalid("The message is not valid ORP 0.3.", path+"/amount_minor"), true
	}
	currency, _ := obj["currency"].(string)
	if !currencyPattern.MatchString(currency) {
		return Invalid("The message is not valid ORP 0.3.", path+"/currency"), true
	}
	return Problem{}, false
}

func optionalContent(obj map[string]any, path string) (Problem, bool) {
	raw, present := obj["content"]
	if !present {
		return Problem{}, false
	}
	content, ok := raw.(map[string]any)
	if !ok {
		return Invalid("The message is not valid ORP 0.3.", path+"/content"), true
	}
	allowed := map[string]bool{
		"link": true, "code": true, "direct": true, "brief": true, "copy": true,
		"max_chars": true, "must_include": true, "must_not_include": true,
	}
	if problem, bad := unknownKey(content, allowed, path+"/content"); bad {
		return problem, true
	}
	for _, key := range []string{"link", "code", "direct"} {
		if value, present := content[key]; present {
			if _, ok := value.(bool); !ok {
				return Invalid("The message is not valid ORP 0.3.", path+"/content/"+key), true
			}
		}
	}
	if problem, bad := optionalBounded(content, "brief", path+"/content/brief", 1000); bad {
		return problem, true
	}
	if problem, bad := optionalBounded(content, "copy", path+"/content/copy", 2000); bad {
		return problem, true
	}
	if n, present := content["max_chars"]; present {
		value, ok := whole(n)
		if !ok || value < 1 || value > 100000 {
			return Invalid("The message is not valid ORP 0.3.", path+"/content/max_chars"), true
		}
	}
	if problem, bad := optionalPhrases(content, "must_include", path+"/content/must_include", 10); bad {
		return problem, true
	}
	if problem, bad := optionalPhrases(content, "must_not_include", path+"/content/must_not_include", 20); bad {
		return problem, true
	}
	return Problem{}, false
}

func optionalSpace(obj map[string]any, path string) (Problem, bool) {
	raw, present := obj["space"]
	if !present {
		return Problem{}, false
	}
	space, ok := raw.(map[string]any)
	if !ok || len(space) == 0 {
		return Invalid("The message is not valid ORP 0.3.", path+"/space"), true
	}
	if problem, bad := unknownKey(space, map[string]bool{"id": true, "kinds": true}, path+"/space"); bad {
		return problem, true
	}
	if id, present := space["id"]; present {
		text, ok := id.(string)
		if !ok || !localPattern.MatchString(text) {
			return Invalid("The message is not valid ORP 0.3.", path+"/space/id"), true
		}
	}
	if kinds, present := space["kinds"]; present {
		raw, ok := kinds.([]any)
		if !ok || len(raw) < 1 || len(raw) > 10 {
			return Invalid("The message is not valid ORP 0.3.", path+"/space/kinds"), true
		}
		seen := map[string]bool{}
		for i, item := range raw {
			text, ok := item.(string)
			if !ok || seen[text] || !validSpaceKind(text) {
				return Invalid("The message is not valid ORP 0.3.", path+"/space/kinds/"+itoa(i)), true
			}
			seen[text] = true
		}
	}
	return Problem{}, false
}

func optionalPhrases(obj map[string]any, key, path string, max int) (Problem, bool) {
	raw, present := obj[key]
	if !present {
		return Problem{}, false
	}
	items, ok := raw.([]any)
	if !ok || len(items) > max {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	for i, item := range items {
		text, ok := item.(string)
		if !ok || text == "" || utf8.RuneCountInString(text) > 200 {
			return Invalid("The message is not valid ORP 0.3.", path+"/"+itoa(i)), true
		}
	}
	return Problem{}, false
}

func optionalBounded(obj map[string]any, key, path string, max int) (Problem, bool) {
	raw, present := obj[key]
	if !present {
		return Problem{}, false
	}
	text, ok := raw.(string)
	if !ok || text == "" || utf8.RuneCountInString(text) > max {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	return Problem{}, false
}

func optionalCount(obj map[string]any, key, path string, min, max int) (Problem, bool) {
	raw, present := obj[key]
	if !present {
		return Problem{}, false
	}
	n, ok := whole(raw)
	if !ok || n < min || n > max {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	return Problem{}, false
}

func optionalDeadline(obj map[string]any, path string) (Problem, bool) {
	raw, present := obj["due"]
	if !present {
		return Problem{}, false
	}
	text, ok := raw.(string)
	if !ok || (!validTimestamp(text) && !validDuration(text)) {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	return Problem{}, false
}

func optionalDurationKey(obj map[string]any, key, path string) (Problem, bool) {
	raw, present := obj[key]
	if !present {
		return Problem{}, false
	}
	text, ok := raw.(string)
	if !ok || !validDuration(text) {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	return Problem{}, false
}

func optionalObligationRef(obj map[string]any, key, path string) (Problem, bool) {
	raw, present := obj[key]
	if !present {
		return Problem{}, false
	}
	text, ok := raw.(string)
	if !ok || !obligationPattern.MatchString(text) {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	return Problem{}, false
}

func optionalHash(obj map[string]any, key, path string) (Problem, bool) {
	raw, present := obj[key]
	if !present {
		return Problem{}, false
	}
	text, ok := raw.(string)
	if !ok || !hashPattern.MatchString(text) {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	return Problem{}, false
}

func optionalLocal(obj map[string]any, key, path string) (Problem, bool) {
	raw, present := obj[key]
	if !present {
		return Problem{}, false
	}
	text, ok := raw.(string)
	if !ok || !localPattern.MatchString(text) {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	return Problem{}, false
}

func optionalText(obj map[string]any, key, path string) (Problem, bool) {
	return optionalBounded(obj, key, path, 1000)
}

func optionalTimestamp(obj map[string]any, key, path string) (Problem, bool) {
	raw, present := obj[key]
	if !present {
		return Problem{}, false
	}
	text, ok := raw.(string)
	if !ok || !validTimestamp(text) {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	return Problem{}, false
}

func optionalURL(obj map[string]any, key, path string) (Problem, bool) {
	raw, present := obj[key]
	if !present {
		return Problem{}, false
	}
	text, ok := raw.(string)
	if !ok || !validURL(text) {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	return Problem{}, false
}

func requiredURL(obj map[string]any, key, path string) (Problem, bool) {
	text, _ := obj[key].(string)
	if !validURL(text) {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	return Problem{}, false
}

func unknownKey(obj map[string]any, allowed map[string]bool, path string) (Problem, bool) {
	for key := range obj {
		if !allowed[key] {
			return Invalid("The message is not valid ORP 0.3.", path+"/"+escapePointer(key)), true
		}
	}
	return Problem{}, false
}

func validDuration(value string) bool {
	if !durationPattern.MatchString(value) || value == "P" || value == "PT" || strings.HasSuffix(value, "T") {
		return false
	}
	return strings.ContainsAny(value, "0123456789")
}

func validTimestamp(value string) bool {
	if utf8.RuneCountInString(value) > 40 || value == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func validURL(value string) bool {
	if len(value) > 2048 || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func personalIn(value any, path string) (Problem, bool) {
	if found := personalAt(value, path); found != "" {
		return Invalid("The message contains personal data.", found), true
	}
	return Problem{}, false
}

func personalAt(value any, path string) string {
	switch v := value.(type) {
	case string:
		if emailPattern.MatchString(v) {
			if path == "" {
				return "/"
			}
			return path
		}
		return ""
	case []any:
		for i, item := range v {
			if found := personalAt(item, path+"/"+itoa(i)); found != "" {
				return found
			}
		}
		return ""
	case map[string]any:
		for key, item := range v {
			next := path + "/" + escapePointer(key)
			if personalKeys[key] {
				return next
			}
			if found := personalAt(item, next); found != "" {
				return found
			}
		}
		return ""
	default:
		return ""
	}
}

// Places reports whether role owes a place obligation.
func Places(terms map[string]any, role string) bool {
	raw, _ := terms["obligations"].([]any)
	for _, item := range raw {
		obj, _ := item.(map[string]any)
		if obj["action"] == "place" && obj["by"] == role {
			return true
		}
	}
	return false
}

// DisclosureLabel reads the terms label. An empty string means there is none.
func DisclosureLabel(terms map[string]any) string {
	disclosure, _ := terms["disclosure"].(map[string]any)
	label, _ := disclosure["label"].(string)
	return label
}

// TermsTitle reads the terms title.
func TermsTitle(terms map[string]any) string {
	title, _ := terms["title"].(string)
	return title
}

// ObligationMap finds one obligation by id.
func ObligationMap(terms map[string]any, id string) map[string]any {
	raw, _ := terms["obligations"].([]any)
	for _, item := range raw {
		obj, _ := item.(map[string]any)
		if obj["id"] == id {
			return obj
		}
	}
	return nil
}

// ProofAllows reports whether the obligation lists method.
func ProofAllows(obligation map[string]any, method string) bool {
	raw, _ := obligation["proof"].([]any)
	for _, item := range raw {
		if item == method {
			return true
		}
	}
	return false
}

// PendingObligations is the snapshot list for a new deal.
func PendingObligations(terms map[string]any) []any {
	raw, _ := terms["obligations"].([]any)
	out := make([]any, 0, len(raw))
	for _, item := range raw {
		obj, _ := item.(map[string]any)
		id, _ := obj["id"].(string)
		out = append(out, map[string]any{"id": id, "state": "pending"})
	}
	return out
}

// CatalogTerms is the standing terms a suggest row would show.
func CatalogTerms(candidate Candidate) (map[string]any, string, bool) {
	terms, _, ok := termsOf(candidate)
	if !ok {
		return nil, "", false
	}
	if _, bad := checkTerms(terms, "/terms"); bad {
		return nil, "", false
	}
	return terms, TermsHash(terms), true
}
