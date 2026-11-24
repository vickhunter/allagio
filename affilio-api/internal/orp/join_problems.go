package orp

import (
	"crypto/ed25519"
	"encoding/base64"
	"sort"
	"strings"
	"unicode/utf8"
)

// JoinErrorCodes is every stable code POST /orp/join can return. Agents
// branch on these. The same list is written into the agent docs.
var JoinErrorCodes = []string{
	"join_not_object",
	"join_disallowed_chars",
	"join_personal",
	"join_orp_version",
	"join_unknown_field",
	"join_key_shape",
	"join_key_private",
	"join_key_field",
	"join_key_kty",
	"join_key_crv",
	"join_key_x",
	"join_key_kid",
	"join_key_alg",
	"join_key_use",
	"join_agent_shape",
	"join_agent_field",
	"join_agent_name",
	"join_agent_version",
	"join_body_json",
	"join_body_size",
	"join_bearer_unknown",
	"join_key_held",
	"join_key_scope",
}

// JoinFailure is one failed join rule. Detail names the field and the rule
// so a log is readable. Code is what an agent branches on.
type JoinFailure struct {
	Code   string
	Field  string
	Rule   string
	Detail string
}

// NamedField is the field path, or "(body)" when the failure has no path.
func (f JoinFailure) NamedField() string {
	if strings.TrimSpace(f.Field) == "" {
		return "(body)"
	}
	return f.Field
}

func newJoinFailure(code, field, rule string) JoinFailure {
	named := field
	if strings.TrimSpace(named) == "" {
		named = "(body)"
	}
	return JoinFailure{
		Code:   code,
		Field:  field,
		Rule:   rule,
		Detail: "field " + named + " failed rule " + code + ": " + rule,
	}
}

// IsJoinCode reports a join identity failure. Those are problem+json on
// POST /orp/join. Rate limits and loopback refusals are not.
func IsJoinCode(code string) bool {
	switch code {
	case "join_bearer_unknown", "join_key_held", "join_key_scope":
		return true
	default:
		return false
	}
}

// JoinFailureFromProblem maps a body or identity problem onto one failure.
func JoinFailureFromProblem(problem Problem) JoinFailure {
	code := problem.Code
	rule := problem.Message
	if code == "" || code == "invalid_message" {
		code = "join_body_json"
		if rule == "" {
			rule = "The body is not JSON."
		}
		if rule == "The body is over 64 KiB." {
			code = "join_body_size"
		}
	}
	return newJoinFailure(code, problem.Field, rule)
}

// CollectJoin returns every failed rule. An empty object is a complete join.
// ParseJoin stays the single-error path suggest uses.
func CollectJoin(value any, prefix string) (JoinRequest, []JoinFailure) {
	if value == nil {
		return JoinRequest{}, []JoinFailure{newJoinFailure("join_not_object", prefix, "The join request is not valid.")}
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return JoinRequest{}, []JoinFailure{newJoinFailure("join_not_object", prefix, "The join request is not valid.")}
	}
	var failures []JoinFailure
	if found := DirtyPointer(value); found != "" {
		failures = append(failures, newJoinFailure("join_disallowed_chars", joinDirtyField(prefix, found), "The join request contains disallowed characters or markup."))
	}
	for key := range obj {
		switch {
		case personalJoinKeys[key]:
			failures = append(failures, newJoinFailure("join_personal", joinField(prefix, key), "Join asks for nothing personal."))
		case key == "orp":
			version, _ := obj[key].(string)
			if version != Version04 {
				failures = append(failures, newJoinFailure("join_orp_version", joinField(prefix, "orp"), "Use 0.4."))
			}
		case key == "agent" || key == "key":
		default:
			failures = append(failures, newJoinFailure("join_unknown_field", joinField(prefix, key), "The join request is not valid."))
		}
	}
	var req JoinRequest
	if key, present := obj["key"]; present {
		path := "/key"
		if prefix != "" {
			path = prefix + "/key"
		}
		parsed, keyFailures := inspectJoinKey(key, path)
		failures = append(failures, keyFailures...)
		if len(keyFailures) == 0 {
			req.Key = parsed
		}
	}
	if agent, present := obj["agent"]; present {
		path := "/agent"
		if prefix != "" {
			path = prefix + "/agent"
		}
		name, version, agentFailures := inspectJoinAgent(agent, path)
		failures = append(failures, agentFailures...)
		if len(agentFailures) == 0 {
			req.AgentName = name
			req.AgentVersion = version
		}
	}
	sortJoinFailures(failures)
	return req, failures
}

func inspectJoinAgent(value any, path string) (string, string, []JoinFailure) {
	obj, ok := value.(map[string]any)
	if !ok {
		return "", "", []JoinFailure{newJoinFailure("join_agent_shape", path, "The message is not valid ORP 0.4.")}
	}
	var failures []JoinFailure
	for key := range obj {
		if key != "name" && key != "version" {
			failures = append(failures, newJoinFailure("join_agent_field", path+"/"+escapePointer(key), "The message is not valid ORP 0.4."))
		}
	}
	name, _ := obj["name"].(string)
	if !agentPattern.MatchString(name) {
		failures = append(failures, newJoinFailure("join_agent_name", path+"/name", "The message is not valid ORP 0.4. "+AgentNameRule))
	}
	version := ""
	if raw, present := obj["version"]; present {
		text, isString := raw.(string)
		if !isString || utf8.RuneCountInString(text) > 32 {
			failures = append(failures, newJoinFailure("join_agent_version", path+"/version", "The message is not valid ORP 0.4."))
		} else {
			version = text
		}
	}
	return name, version, failures
}

func inspectJoinKey(value any, prefix string) (map[string]any, []JoinFailure) {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, []JoinFailure{newJoinFailure("join_key_shape", prefix, "The join key is not valid.")}
	}
	var failures []JoinFailure
	for key := range obj {
		switch key {
		case "d", "k":
			failures = append(failures, newJoinFailure("join_key_private", prefix+"/"+escapePointer(key), "A join key holds public material only."))
		case "kty", "crv", "x", "kid", "alg", "use":
		default:
			failures = append(failures, newJoinFailure("join_key_field", prefix+"/"+escapePointer(key), "The join key is not valid."))
		}
	}
	if kty, _ := obj["kty"].(string); kty != "OKP" {
		failures = append(failures, newJoinFailure("join_key_kty", prefix+"/kty", "key.kty is OKP."))
	}
	if crv, _ := obj["crv"].(string); crv != "Ed25519" {
		failures = append(failures, newJoinFailure("join_key_crv", prefix+"/crv", "key.crv is Ed25519."))
	}
	x, _ := obj["x"].(string)
	raw, err := base64.RawURLEncoding.DecodeString(x)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		failures = append(failures, newJoinFailure("join_key_x", prefix+"/x", "key.x is a base64url Ed25519 public key."))
	}
	kid, _ := obj["kid"].(string)
	if !kidPattern.MatchString(kid) {
		failures = append(failures, newJoinFailure("join_key_kid", prefix+"/kid", "key.kid is 1 to 64 key characters."))
	}
	if alg, present := obj["alg"]; present {
		if text, _ := alg.(string); text != "EdDSA" {
			failures = append(failures, newJoinFailure("join_key_alg", prefix+"/alg", "key.alg is EdDSA."))
		}
	}
	if use, present := obj["use"]; present {
		if text, _ := use.(string); text != "sig" {
			failures = append(failures, newJoinFailure("join_key_use", prefix+"/use", "key.use is sig."))
		}
	}
	sortJoinFailures(failures)
	if len(failures) > 0 {
		return nil, failures
	}
	return map[string]any{
		"kty": "OKP", "crv": "Ed25519", "x": x, "kid": kid, "alg": "EdDSA", "use": "sig",
	}, nil
}

func joinField(prefix, key string) string {
	token := "/" + escapePointer(key)
	if prefix == "" {
		return token
	}
	return prefix + token
}

func joinDirtyField(prefix, found string) string {
	if prefix == "" {
		return found
	}
	if found == "" || found == "/" {
		return prefix
	}
	return prefix + found
}

func sortJoinFailures(failures []JoinFailure) {
	sort.Slice(failures, func(i, j int) bool {
		if failures[i].NamedField() != failures[j].NamedField() {
			return failures[i].NamedField() < failures[j].NamedField()
		}
		return failures[i].Code < failures[j].Code
	})
}

// JoinProblemDocument is an RFC 9457 problem for one or more join rules.
// orp is "0.4" only when a failed rule is an ORP 0.4 rule.
func JoinProblemDocument(failures []JoinFailure) map[string]any {
	if len(failures) == 0 {
		failures = []JoinFailure{newJoinFailure("join_not_object", "", "The join request is not valid.")}
	}
	sortJoinFailures(failures)
	errors := make([]any, len(failures))
	orp04 := false
	for i, failure := range failures {
		if strings.Contains(failure.Rule, "ORP 0.4.") {
			orp04 = true
		}
		errors[i] = map[string]any{
			"code":   failure.Code,
			"field":  failure.NamedField(),
			"detail": failure.Detail,
		}
	}
	doc := map[string]any{
		"type":   "https://allagio.dev/problems/join",
		"title":  "The join request is not valid.",
		"status": 400,
		"detail": failures[0].Detail,
		"errors": errors,
	}
	if orp04 {
		doc["orp"] = Version04
	}
	return doc
}
