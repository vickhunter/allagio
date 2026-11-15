package orp

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
)

var kidPattern = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,64}$`)

// ParseJoinKey checks a join key object. Only the public Ed25519 JWK fields
// kty, crv, x, kid, alg, and use are kept. Private material is refused.
func ParseJoinKey(value any, prefix string) (map[string]any, Problem, bool) {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, Invalid("The join key is not valid.", prefix), false
	}
	for key := range obj {
		if key == "d" || key == "k" {
			return nil, Invalid("A join key holds public material only.", prefix+"/"+escapePointer(key)), false
		}
		if key != "kty" && key != "crv" && key != "x" && key != "kid" && key != "alg" && key != "use" {
			return nil, Invalid("The join key is not valid.", prefix+"/"+escapePointer(key)), false
		}
	}
	if kty, _ := obj["kty"].(string); kty != "OKP" {
		return nil, Invalid("key.kty is OKP.", prefix+"/kty"), false
	}
	if crv, _ := obj["crv"].(string); crv != "Ed25519" {
		return nil, Invalid("key.crv is Ed25519.", prefix+"/crv"), false
	}
	x, _ := obj["x"].(string)
	raw, err := base64.RawURLEncoding.DecodeString(x)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, Invalid("key.x is a base64url Ed25519 public key.", prefix+"/x"), false
	}
	kid, _ := obj["kid"].(string)
	if !kidPattern.MatchString(kid) {
		return nil, Invalid("key.kid is 1 to 64 key characters.", prefix+"/kid"), false
	}
	if alg, present := obj["alg"]; present {
		if text, _ := alg.(string); text != "EdDSA" {
			return nil, Invalid("key.alg is EdDSA.", prefix+"/alg"), false
		}
	}
	if use, present := obj["use"]; present {
		if text, _ := use.(string); text != "sig" {
			return nil, Invalid("key.use is sig.", prefix+"/use"), false
		}
	}
	return map[string]any{
		"kty": "OKP", "crv": "Ed25519", "x": x, "kid": kid, "alg": "EdDSA", "use": "sig",
	}, Problem{}, true
}

// SignedType is a message a keyed party MUST sign (E9).
func SignedType(typ string) bool {
	switch typ {
	case "offer", "counter", "accept", "revoke", "payout":
		return true
	}
	return false
}

// KeyID is the kid claim of a stored key.
func KeyID(jwk map[string]any) string {
	kid, _ := jwk["kid"].(string)
	return kid
}

// KeyMatches reports whether a stored JWK verifies with kid.
func KeyMatches(jwk map[string]any, kid string) bool {
	stored, _ := jwk["kid"].(string)
	return stored == kid
}

// Signer signs host-issued records — the deal certificate and any signed
// receipt — so a party can verify them offline against the public JWK the
// discovery document lists under keys.
type Signer struct {
	Kid string
	Key ed25519.PrivateKey
}

// PublicJWK is the verifying half of the signer, shaped like a party JWK.
func (s *Signer) PublicJWK() map[string]any {
	pub, _ := s.Key.Public().(ed25519.PublicKey)
	return map[string]any{
		"kty": "OKP", "crv": "Ed25519",
		"x": base64.RawURLEncoding.EncodeToString(pub),
		"kid": s.Kid, "alg": "EdDSA", "use": "sig",
	}
}

// Sign returns obj with a sig field in the same compact form party envelopes
// use: base64url(header) + ".." + base64url(Ed25519 over the JCS of obj minus
// sig). VerifyEnvelopeSig checks the result.
func (s *Signer) Sign(obj map[string]any) map[string]any {
	unsigned := make(map[string]any, len(obj))
	for key, item := range obj {
		if key != "sig" {
			unsigned[key] = item
		}
	}
	header, _ := json.Marshal(map[string]any{"alg": "EdDSA", "kid": s.Kid})
	signature := ed25519.Sign(s.Key, []byte(Canonicalize(unsigned)))
	out := make(map[string]any, len(unsigned)+1)
	for key, item := range unsigned {
		out[key] = item
	}
	out["sig"] = base64.RawURLEncoding.EncodeToString(header) + ".." + base64.RawURLEncoding.EncodeToString(signature)
	return out
}

// VerifyEnvelopeSig checks sig against the public JWK over the JCS form of the
// envelope minus sig. The header is a base64url JSON object with alg EdDSA and
// a kid; the signature is Ed25519. A malformed header, an unknown kid, or a
// bad signature is false.
func VerifyEnvelopeSig(value any, keys []map[string]any) (kid string, ok bool) {
	obj, isObj := value.(map[string]any)
	if !isObj {
		return "", false
	}
	sig, _ := obj["sig"].(string)
	parts := strings.Split(sig, ".")
	if len(parts) != 3 || parts[1] != "" {
		return "", false
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	var header map[string]any
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return "", false
	}
	if alg, _ := header["alg"].(string); alg != "EdDSA" {
		return "", false
	}
	headerKid, _ := header["kid"].(string)
	if headerKid == "" {
		return "", false
	}
	var jwk map[string]any
	for _, key := range keys {
		if KeyMatches(key, headerKid) {
			jwk = key
			break
		}
	}
	if jwk == nil {
		return headerKid, false
	}
	pub, err := base64.RawURLEncoding.DecodeString(jwk["x"].(string))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return headerKid, false
	}
	signed, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signed) != ed25519.SignatureSize {
		return headerKid, false
	}
	unsigned := make(map[string]any, len(obj))
	for key, item := range obj {
		if key != "sig" {
			unsigned[key] = item
		}
	}
	return headerKid, ed25519.Verify(ed25519.PublicKey(pub), []byte(Canonicalize(unsigned)), signed)
}
