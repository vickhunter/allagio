package orp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxBodyBytes is the largest JSON body this layer reads.
	MaxBodyBytes = 65536
	clockSkew    = 15 * time.Minute
)

var (
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
	partyPattern   = regexp.MustCompile(`^orp:[A-Za-z0-9.\-]{1,253}(:[0-9]{1,5})?/[A-Za-z0-9_\-]{1,64}$`)
	messagePattern = regexp.MustCompile(`^[A-Za-z0-9_.:\-]{8,128}$`)
	localPattern   = regexp.MustCompile(`^[A-Za-z0-9_\-]{3,64}$`)
	hashPattern    = regexp.MustCompile(`^sha256-[A-Za-z0-9_\-]{43}$`)
	sigPattern     = regexp.MustCompile(`^[A-Za-z0-9_\-]+\.\.[A-Za-z0-9_\-]+$`)
	extPattern     = regexp.MustCompile(`^[a-z0-9\-]+(\.[a-z0-9\-]+)+$`)
	agentPattern   = regexp.MustCompile(`^[A-Za-z0-9_. \-]{1,64}$`)
)

// AgentNameRule is the agent.name pattern. Spaces are allowed. Parentheses are not.
// A 0.4 invalid_message names 0.4 and includes this sentence. The pattern is not loosened.
const AgentNameRule = "agent.name matches ^[A-Za-z0-9_. -]{1,64}$: letters, digits, underscore, dot, space, or hyphen, 1 to 64 characters."

var coreTypes = map[string]bool{
	"hello": true, "discover": true, "results": true, "space": true,
	"request": true, "offer": true, "counter": true, "accept": true,
	"decline": true, "deal": true, "execute": true, "receipt": true,
	"outcome": true, "payout": true, "revoke": true, "consent": true,
	"dispute": true, "reputation": true, "error": true,
}

var envelopeKeys = map[string]bool{
	"orp": true, "type": true, "id": true, "ts": true, "from": true, "to": true,
	"deal": true, "re": true, "body": true, "agent": true, "prev": true,
	"sig": true, "ext": true, "critical": true,
}

var hostOnlyTypes = map[string]bool{
	"results": true, "deal": true, "outcome": true, "consent": true,
}

// Problem is an ORP error the handler writes without changing deal state.
type Problem struct {
	Status     int
	Code       string
	Message    string
	Field      string
	Supported  []string
	RetryAfter int
	Retryable  bool
	Step       string
}

func (p Problem) WithField(field string) Problem {
	p.Field = field
	return p
}

func Invalid(message, field string) Problem {
	return Problem{Status: 400, Code: "invalid_message", Message: message, Field: field}
}

// Message is an envelope that passed the shape checks this layer enforces.
type Message struct {
	ORP    string
	Type   string
	ID     string
	TS     time.Time
	From   string
	To     string
	Body   map[string]any
	Deal   string
	Re     string
	Prev   string
	HasSig bool
}

// ParseEnvelope applies E1–E4 shape rules that do not need a credential.
// A #68-shaped object fails here as invalid_message. Offer, accept, and
// execute bodies are checked here too. This layer still does not pay.
func ParseEnvelope(value any) (Message, Problem, bool) {
	obj, ok := value.(map[string]any)
	if !ok {
		return Message{}, Invalid("The body is not an ORP message.", ""), false
	}
	if found := DirtyPointer(value); found != "" {
		return Message{}, Invalid("The message contains disallowed characters or markup.", found), false
	}
	// Name the version the message already claims. A 0.4 envelope that fails
	// a shape check says ORP 0.4. Anything else, including a missing orp,
	// stays ORP 0.3 so existing 0.3 errors stay byte-identical.
	label := "The message is not valid ORP 0.3."
	if text, isString := obj["orp"].(string); isString && text == Version04 {
		label = "The message is not valid ORP 0.4."
	}
	for key := range obj {
		if !envelopeKeys[key] {
			return Message{}, Invalid(label, "/"+escapePointer(key)), false
		}
	}
	for _, key := range []string{"orp", "type", "id", "ts", "from", "body"} {
		if _, present := obj[key]; !present {
			return Message{}, Invalid(label, "/"+key), false
		}
	}
	orp, ok := obj["orp"].(string)
	if !ok || !versionPattern.MatchString(orp) || utf8.RuneCountInString(orp) > 16 {
		return Message{}, Invalid(label, "/orp"), false
	}
	if orp != Version && orp != Version04 {
		return Message{}, Problem{Status: 400, Code: "unsupported_version", Message: "This host speaks 0.3.", Supported: []string{Version}}, false
	}
	if orp == Version04 {
		label = "The message is not valid ORP 0.4."
	} else {
		label = "The message is not valid ORP 0.3."
	}
	typ, ok := obj["type"].(string)
	if !ok || typ == "" {
		return Message{}, Invalid(label, "/type"), false
	}
	if orp == Version04 && (typ == "leave" || typ == "seat") {
		// leave and seat are 0.4 message types. A 0.3 leave stays invalid_message.
	} else if !coreTypes[typ] {
		if !extPattern.MatchString(typ) || len(typ) > 128 {
			return Message{}, Invalid(label, "/type"), false
		}
		return Message{}, Problem{Status: 400, Code: "unsupported_type", Message: "The type is not supported."}, false
	}
	id, ok := obj["id"].(string)
	if !ok || !messagePattern.MatchString(id) {
		return Message{}, Invalid(label, "/id"), false
	}
	tsRaw, ok := obj["ts"].(string)
	if !ok || utf8.RuneCountInString(tsRaw) > 40 {
		return Message{}, Invalid(label, "/ts"), false
	}
	ts, err := time.Parse(time.RFC3339, tsRaw)
	if err != nil {
		return Message{}, Invalid(label, "/ts"), false
	}
	from, ok := obj["from"].(string)
	if !ok || !partyPattern.MatchString(from) || len(from) > 330 {
		return Message{}, Invalid(label, "/from"), false
	}
	body, ok := obj["body"].(map[string]any)
	if !ok {
		return Message{}, Invalid(label, "/body"), false
	}
	msg := Message{ORP: orp, Type: typ, ID: id, TS: ts, From: from, Body: body}
	if to, present := obj["to"]; present {
		text, ok := to.(string)
		if !ok || !partyPattern.MatchString(text) {
			return Message{}, Invalid(label, "/to"), false
		}
		msg.To = text
	}
	if deal, present := obj["deal"]; present {
		text, ok := deal.(string)
		if !ok || !localPattern.MatchString(text) {
			return Message{}, Invalid(label, "/deal"), false
		}
		msg.Deal = text
	}
	if re, present := obj["re"]; present {
		text, ok := re.(string)
		if !ok || !messagePattern.MatchString(text) {
			return Message{}, Invalid(label, "/re"), false
		}
		msg.Re = text
	}
	if prev, present := obj["prev"]; present {
		text, ok := prev.(string)
		if !ok || !hashPattern.MatchString(text) {
			return Message{}, Invalid(label, "/prev"), false
		}
		msg.Prev = text
	}
	if sig, present := obj["sig"]; present {
		text, ok := sig.(string)
		if !ok || !sigPattern.MatchString(text) || len(text) > 4096 {
			return Message{}, Invalid(label, "/sig"), false
		}
		msg.HasSig = true
	}
	if agent, present := obj["agent"]; present {
		if problem, bad := checkAgent(agent, "/agent"); bad {
			problem, _ = relabelVersion(orp, problem, bad)
			return Message{}, problem, false
		}
	}
	if ext, present := obj["ext"]; present {
		if problem, bad := checkExt(ext); bad {
			problem, _ = relabelVersion(orp, problem, bad)
			return Message{}, problem, false
		}
	}
	if critical, present := obj["critical"]; present {
		names, problem, bad := checkCritical(critical)
		if bad {
			problem, _ = relabelVersion(orp, problem, bad)
			return Message{}, problem, false
		}
		if len(names) > 0 {
			return Message{}, Problem{Status: 400, Code: "unsupported_extension", Message: "A critical extension is unknown."}, false
		}
	}
	if problem, bad := checkHandledBody(orp, typ, body); bad {
		return Message{}, problem, false
	}
	if (typ == "execute" || typ == "leave" || typ == "seat" || typ == "decline" || typ == "dispute" || typ == "payout") && msg.Deal == "" {
		return Message{}, Invalid(label, "/deal"), false
	}
	if typ == "revoke" {
		scope, _ := body["scope"].(string)
		if scope == "offer" {
			if offer, _ := body["offer"].(string); offer == "" {
				return Message{}, Invalid(label, "/body/offer"), false
			}
		} else if msg.Deal == "" {
			return Message{}, Invalid(label, "/deal"), false
		}
	}
	if typ == "request" && msg.To == "" {
		return Message{}, Invalid(label, "/to"), false
	}
	if typ == "accept" && msg.Deal == "" {
		offer, _ := body["offer"].(string)
		if offer == "" {
			return Message{}, Invalid(label, "/body/offer"), false
		}
	}
	if typ == "accept" && msg.Deal != "" {
		if _, present := body["offer"]; present {
			return Message{}, Invalid(label, "/deal"), false
		}
	}
	return msg, Problem{}, true
}

// ClockProblem is E6. now is the host clock.
func ClockProblem(ts, now time.Time) (Problem, bool) {
	if now.Sub(ts) > clockSkew || ts.Sub(now) > clockSkew {
		return Problem{Status: 400, Code: "stale", Message: "ts is more than 15 minutes from the host clock."}, true
	}
	return Problem{}, false
}

func checkHandledBody(version, typ string, body map[string]any) (Problem, bool) {
	switch typ {
	case "hello":
		return checkHelloFor(version, body)
	case "discover":
		problem, bad := checkDiscoverBody(body)
		return relabelVersion(version, problem, bad)
	case "offer":
		if version == Version04 {
			return checkOfferV4(body)
		}
		return checkOfferBody(body)
	case "accept":
		problem, bad := checkAcceptBody(body)
		return relabelVersion(version, problem, bad)
	case "execute":
		problem, bad := checkExecuteBody(body)
		return relabelVersion(version, problem, bad)
	case "leave":
		return checkLeaveBody(body)
	case "seat":
		return checkSeatBody(body)
	case "space":
		problem, bad := checkSpaceBody(body)
		return relabelVersion(version, problem, bad)
	case "request":
		problem, bad := checkRequestBody(body)
		return relabelVersion(version, problem, bad)
	case "payout":
		problem, bad := checkPayoutBody(body)
		return relabelVersion(version, problem, bad)
	case "receipt":
		problem, bad := checkReceiptBody(body)
		return relabelVersion(version, problem, bad)
	default:
		return Problem{}, false
	}
}

var spaceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func spaceStr(obj map[string]any, key string, max int, required bool) (Problem, bool) {
	value, present := obj[key]
	if !present || value == nil {
		if required {
			return Invalid("The message is not valid ORP 0.3.", "/body/space/"+key), true
		}
		return Problem{}, false
	}
	text, ok := value.(string)
	if !ok || (required && text == "") || utf8.RuneCountInString(text) > max {
		return Invalid("The message is not valid ORP 0.3.", "/body/space/"+key), true
	}
	return Problem{}, false
}

func spaceStringList(obj map[string]any, key string) (Problem, bool) {
	value, present := obj[key]
	if !present || value == nil {
		return Problem{}, false
	}
	list, ok := value.([]any)
	if !ok || len(list) > 20 {
		return Invalid("The message is not valid ORP 0.3.", "/body/space/"+key), true
	}
	for _, item := range list {
		text, ok := item.(string)
		if !ok || text == "" || utf8.RuneCountInString(text) > 100 {
			return Invalid("The message is not valid ORP 0.3.", "/body/space/"+key), true
		}
	}
	return Problem{}, false
}

func checkSpaceBody(body map[string]any) (Problem, bool) {
	for key := range body {
		switch key {
		case "action", "space", "id":
		default:
			return Invalid("The message is not valid ORP 0.3.", "/body/"+escapePointer(key)), true
		}
	}
	action, _ := body["action"].(string)
	if action != "put" && action != "withdraw" {
		return Invalid("The message is not valid ORP 0.3.", "/body/action"), true
	}
	if action == "withdraw" {
		id, _ := body["id"].(string)
		if id == "" {
			if sp, ok := body["space"].(map[string]any); ok {
				id, _ = sp["id"].(string)
			}
		}
		if !spaceIDPattern.MatchString(id) {
			return Invalid("The message is not valid ORP 0.3.", "/body/id"), true
		}
		return Problem{}, false
	}
	sp, ok := body["space"].(map[string]any)
	if !ok {
		return Invalid("The message is not valid ORP 0.3.", "/body/space"), true
	}
	for key := range sp {
		switch key {
		case "id", "kind", "platform", "title", "locator", "topics", "languages",
			"audience", "accepts", "disclosure", "rules":
		default:
			return Invalid("The message is not valid ORP 0.3.", "/body/space/"+escapePointer(key)), true
		}
	}
	if _, present := sp["verified"]; present {
		return Invalid("The message is not valid ORP 0.3.", "/body/space/verified"), true
	}
	id, _ := sp["id"].(string)
	if !spaceIDPattern.MatchString(id) {
		return Invalid("The message is not valid ORP 0.3.", "/body/space/id"), true
	}
	for _, key := range []string{"kind", "platform", "title"} {
		if problem, bad := spaceStr(sp, key, 200, true); bad {
			return problem, true
		}
	}
	if problem, bad := spaceStr(sp, "locator", 2000, false); bad {
		return problem, true
	}
	if locator, _ := sp["locator"].(string); locator != "" && !strings.HasPrefix(locator, "https://") {
		return Invalid("The message is not valid ORP 0.3.", "/body/space/locator"), true
	}
	for _, key := range []string{"topics", "languages", "accepts"} {
		if problem, bad := spaceStringList(sp, key); bad {
			return problem, true
		}
	}
	if disc, present := sp["disclosure"]; present {
		obj, ok := disc.(map[string]any)
		if !ok {
			return Invalid("The message is not valid ORP 0.3.", "/body/space/disclosure"), true
		}
		for key := range obj {
			if key != "label" && key != "lang" {
				return Invalid("The message is not valid ORP 0.3.", "/body/space/disclosure/"+escapePointer(key)), true
			}
		}
		if problem, bad := spaceStr(obj, "label", 100, false); bad {
			return problem, true
		}
		if problem, bad := spaceStr(obj, "lang", 16, false); bad {
			return problem, true
		}
	}
	if rules, present := sp["rules"]; present {
		if _, ok := rules.(map[string]any); !ok {
			return Invalid("The message is not valid ORP 0.3.", "/body/space/rules"), true
		}
	}
	if aud, present := sp["audience"]; present {
		if _, ok := aud.(map[string]any); !ok {
			return Invalid("The message is not valid ORP 0.3.", "/body/space/audience"), true
		}
	}
	return Problem{}, false
}

func checkPayoutBody(body map[string]any) (Problem, bool) {
	for key := range body {
		switch key {
		case "payout", "answer", "note":
		default:
			return Invalid("The message is not valid ORP 0.3.", "/body/"+escapePointer(key)), true
		}
	}
	pid, _ := body["payout"].(string)
	if !localPattern.MatchString(pid) {
		return Invalid("The message is not valid ORP 0.3.", "/body/payout"), true
	}
	answer, _ := body["answer"].(string)
	if answer != "confirm" && answer != "not_received" {
		return Invalid("The message is not valid ORP 0.3.", "/body/answer"), true
	}
	if note, present := body["note"]; present {
		text, ok := note.(string)
		if !ok || utf8.RuneCountInString(text) > 2000 {
			return Invalid("The message is not valid ORP 0.3.", "/body/note"), true
		}
	}
	return Problem{}, false
}

func checkRequestBody(body map[string]any) (Problem, bool) {
	for key := range body {
		switch key {
		case "space", "context", "kind", "terms":
		default:
			return Invalid("The message is not valid ORP 0.3.", "/body/"+escapePointer(key)), true
		}
	}
	if space, present := body["space"]; present {
		text, ok := space.(string)
		if !ok || !spaceIDPattern.MatchString(text) {
			return Invalid("The message is not valid ORP 0.3.", "/body/space"), true
		}
	}
	if ctx, present := body["context"]; present {
		if _, ok := ctx.(map[string]any); !ok {
			return Invalid("The message is not valid ORP 0.3.", "/body/context"), true
		}
	}
	if kind, present := body["kind"]; present {
		text, ok := kind.(string)
		if !ok || utf8.RuneCountInString(text) > 64 {
			return Invalid("The message is not valid ORP 0.3.", "/body/kind"), true
		}
	}
	return Problem{}, false
}

func checkHelloFor(version string, body map[string]any) (Problem, bool) {
	if version != Version04 {
		return checkHelloBody(body)
	}
	problem, bad := checkHelloBody(body)
	if bad && problem.Code != "unsupported_version" {
		problem.Message = strings.ReplaceAll(problem.Message, "ORP 0.3.", "ORP 0.4.")
		return problem, true
	}
	raw, _ := body["versions"].([]any)
	for _, item := range raw {
		if item == Version04 {
			return Problem{}, false
		}
	}
	return Problem{Status: 400, Code: "unsupported_version", Message: "This host speaks 0.4 and 0.3.", Supported: []string{Version04, Version}}, true
}

func relabelVersion(version string, problem Problem, bad bool) (Problem, bool) {
	if bad && version == Version04 {
		problem.Message = strings.ReplaceAll(problem.Message, "ORP 0.3.", "ORP 0.4.")
	}
	return problem, bad
}

func checkLeaveBody(body map[string]any) (Problem, bool) {
	for key := range body {
		if key != "note" {
			return Invalid("The message is not valid ORP 0.4.", "/body/"+escapePointer(key)), true
		}
	}
	if problem, bad := optionalText(body, "note", "/body/note"); bad {
		problem.Message = strings.ReplaceAll(problem.Message, "ORP 0.3.", "ORP 0.4.")
		return problem, true
	}
	return Problem{}, false
}

func checkSeatBody(body map[string]any) (Problem, bool) {
	for key := range body {
		if key != "seat" {
			return Invalid("The message is not valid ORP 0.4.", "/body/"+escapePointer(key)), true
		}
	}
	if _, ok := body["seat"].(map[string]any); !ok {
		return Invalid("The message is not valid ORP 0.4.", "/body/seat"), true
	}
	return Problem{}, false
}

func checkHelloBody(body map[string]any) (Problem, bool) {
	for key := range body {
		switch key {
		case "versions", "capabilities", "party", "limits":
		default:
			return Invalid("The message is not valid ORP 0.3.", "/body/"+escapePointer(key)), true
		}
	}
	raw, ok := body["versions"].([]any)
	if !ok || len(raw) < 1 || len(raw) > 8 {
		return Invalid("The message is not valid ORP 0.3.", "/body/versions"), true
	}
	seen := map[string]bool{}
	has := false
	for i, item := range raw {
		text, ok := item.(string)
		if !ok || !versionPattern.MatchString(text) || seen[text] {
			return Invalid("The message is not valid ORP 0.3.", "/body/versions/"+itoa(i)), true
		}
		seen[text] = true
		if text == Version {
			has = true
		}
	}
	if !has {
		return Problem{Status: 400, Code: "unsupported_version", Message: "This host speaks 0.3.", Supported: []string{Version}}, true
	}
	return Problem{}, false
}

func checkDiscoverBody(body map[string]any) (Problem, bool) {
	for key := range body {
		switch key {
		case "want", "query", "context", "filters", "limit", "cursor":
		default:
			return Invalid("The message is not valid ORP 0.3.", "/body/"+escapePointer(key)), true
		}
	}
	want, _ := body["want"].(string)
	if want != "offers" && want != "spaces" {
		return Invalid("The message is not valid ORP 0.3.", "/body/want"), true
	}
	if query, present := body["query"]; present {
		text, ok := query.(string)
		if !ok || text == "" || utf8.RuneCountInString(text) > 500 {
			return Invalid("The message is not valid ORP 0.3.", "/body/query"), true
		}
	}
	if limit, present := body["limit"]; present {
		n, ok := whole(limit)
		if !ok || n < 1 || n > 50 {
			return Invalid("The message is not valid ORP 0.3.", "/body/limit"), true
		}
	}
	return Problem{}, false
}

func checkAgent(value any, path string) (Problem, bool) {
	obj, ok := value.(map[string]any)
	if !ok {
		return Invalid("The message is not valid ORP 0.3.", path), true
	}
	for key := range obj {
		if key != "name" && key != "version" {
			return Invalid("The message is not valid ORP 0.3.", path+"/"+escapePointer(key)), true
		}
	}
	name, _ := obj["name"].(string)
	if !agentPattern.MatchString(name) {
		return Invalid("The message is not valid ORP 0.3. "+AgentNameRule, path+"/name"), true
	}
	if version, present := obj["version"]; present {
		text, ok := version.(string)
		if !ok || utf8.RuneCountInString(text) > 32 {
			return Invalid("The message is not valid ORP 0.3.", path+"/version"), true
		}
	}
	return Problem{}, false
}

func checkExt(value any) (Problem, bool) {
	obj, ok := value.(map[string]any)
	if !ok || len(obj) > 16 {
		return Invalid("The message is not valid ORP 0.3.", "/ext"), true
	}
	for key := range obj {
		if !extPattern.MatchString(key) || len(key) > 128 {
			return Invalid("The message is not valid ORP 0.3.", "/ext/"+escapePointer(key)), true
		}
	}
	return Problem{}, false
}

func checkCritical(value any) ([]string, Problem, bool) {
	raw, ok := value.([]any)
	if !ok {
		return nil, Invalid("The message is not valid ORP 0.3.", "/critical"), true
	}
	seen := map[string]bool{}
	names := make([]string, 0, len(raw))
	for i, item := range raw {
		text, ok := item.(string)
		if !ok || !extPattern.MatchString(text) || len(text) > 128 || seen[text] {
			return nil, Invalid("The message is not valid ORP 0.3.", "/critical/"+itoa(i)), true
		}
		seen[text] = true
		names = append(names, text)
	}
	return names, Problem{}, false
}

func whole(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		if v != float64(int(v)) || v < -1e9 || v > 1e9 {
			return 0, false
		}
		return int(v), true
	case int:
		return v, true
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}

// DecodeJSON parses one JSON value. Numbers stay as json.Number so an integer
// limit is not mistaken for a float. An empty body is an empty object.
func DecodeJSON(body []byte) (any, Problem, bool) {
	if len(body) > MaxBodyBytes {
		return nil, Invalid("The body is over 64 KiB.", ""), false
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return map[string]any{}, Problem{}, true
	}
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, Invalid("The body is not JSON.", ""), false
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return nil, Invalid("The body is not JSON.", ""), false
	}
	return normalizeNumbers(value), Problem{}, true
}

func normalizeNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil && strconv.FormatInt(i, 10) == v.String() {
			return float64(i)
		}
		f, err := v.Float64()
		if err != nil {
			return v.String()
		}
		return f
	case map[string]any:
		for key, item := range v {
			v[key] = normalizeNumbers(item)
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = normalizeNumbers(item)
		}
		return v
	default:
		return value
	}
}

// NewMessageID is a host-chosen message id.
func NewMessageID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "m_host_fallback"
	}
	return "m_host_" + hex.EncodeToString(buf[:])
}

// ErrorDocument is an error envelope. It changes no deal state.
func ErrorDocument(dns string, to, re string, problem Problem, now time.Time) map[string]any {
	body := map[string]any{
		"code":      problem.Code,
		"message":   problem.Message,
		"retryable": problem.Retryable || problem.Code == "rate_limited" || problem.Code == "internal",
	}
	if problem.Field != "" {
		body["field"] = problem.Field
	}
	if len(problem.Supported) > 0 {
		supported := make([]any, len(problem.Supported))
		for i, item := range problem.Supported {
			supported[i] = item
		}
		body["supported"] = supported
	}
	if problem.RetryAfter > 0 {
		body["retry_after"] = problem.RetryAfter
	}
	if problem.Step != "" {
		body["step"] = problem.Step
	}
	doc := map[string]any{
		"orp":  Version,
		"type": "error",
		"id":   NewMessageID(),
		"ts":   now.UTC().Format(time.RFC3339),
		"from": HostParty(dns),
		"body": body,
	}
	if to != "" && partyPattern.MatchString(to) {
		doc["to"] = to
	}
	if re != "" && messagePattern.MatchString(re) {
		doc["re"] = re
	}
	return doc
}

// HelloReply answers a 0.3 hello. It names the party behind the bearer.
func HelloReply(dns, to, re, party string, claimsMode string, now time.Time) map[string]any {
	return HelloReplyFor(Version, dns, to, re, party, claimsMode, now)
}

// HelloReplyFor answers hello. A 0.4 hello lists 0.4 and adds leave and seat.
// A 0.3 hello stays exactly ["0.3"] and does not list leave or seat.
func HelloReplyFor(version, dns, to, re, party, claimsMode string, now time.Time) map[string]any {
	orpVersion := Version
	versions := []any{Version}
	caps := Capabilities(claimsMode)
	if version == Version04 {
		orpVersion = Version04
		versions = []any{Version04}
		caps = CapabilitiesFor(Version04, claimsMode)
	}
	return map[string]any{
		"orp":  orpVersion,
		"type": "hello",
		"id":   NewMessageID(),
		"ts":   now.UTC().Format(time.RFC3339),
		"from": HostParty(dns),
		"to":   to,
		"re":   re,
		"body": map[string]any{
			"versions":     versions,
			"capabilities": caps,
			"party":        party,
			"limits": map[string]any{
				"messages_per_minute": 30,
				"max_body_bytes":      MaxBodyBytes,
			},
		},
	}
}

// ResultsReply answers discover. items may be empty.
func ResultsReply(dns, to, re string, items []any, emptyReason string, now time.Time) map[string]any {
	if items == nil {
		items = []any{}
	}
	body := map[string]any{"items": items}
	if len(items) == 0 && emptyReason != "" {
		body["empty_reason"] = emptyReason
	}
	return map[string]any{
		"orp":  Version,
		"type": "results",
		"id":   NewMessageID(),
		"ts":   now.UTC().Format(time.RFC3339),
		"from": HostParty(dns),
		"to":   to,
		"re":   re,
		"body": body,
	}
}

// RefusesDeal is the reply for a core type this testing host does not run.
// No deal, offer, campaign, or placement is created.
func RefusesDeal() Problem {
	return Problem{Status: 403, Code: "forbidden", Message: "This host is in testing and does not run this message."}
}

// HostOnly is the reply when a party sends a host-only type.
func HostOnly() Problem {
	return Problem{Status: 403, Code: "forbidden", Message: "Only the host may send this."}
}

// Handled reports whether the thin layer replies with hello or discover results.
func Handled(typ string) bool {
	return typ == "hello" || typ == "discover"
}

// IsHostOnly reports host-only core types a party must not send.
func IsHostOnly(typ string) bool {
	return hostOnlyTypes[typ]
}
