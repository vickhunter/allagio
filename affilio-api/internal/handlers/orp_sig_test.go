package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"affilio-api/internal/orp"
)

func signEnvelope(t *testing.T, doc map[string]any, priv ed25519.PrivateKey, kid string) map[string]any {
	t.Helper()
	unsigned := map[string]any{}
	for key, item := range doc {
		unsigned[key] = item
	}
	sig := ed25519.Sign(priv, []byte(orp.Canonicalize(unsigned)))
	header, _ := json.Marshal(map[string]any{"alg": "EdDSA", "kid": kid})
	doc["sig"] = base64.RawURLEncoding.EncodeToString(header) + ".." + base64.RawURLEncoding.EncodeToString(sig)
	return doc
}

func testJWK(pub ed25519.PublicKey, kid string) map[string]any {
	return map[string]any{
		"kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "use": "sig",
		"x":   base64.RawURLEncoding.EncodeToString(pub),
		"kid": kid,
	}
}

func joinWithKey(t *testing.T, r http.Handler, jwk map[string]any) (token, party string) {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodPost, "/orp/join",
		map[string]any{"agent": map[string]any{"name": "Sig Agent"}, "key": jwk}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("join with key %d %s", rec.Code, rec.Body.String())
	}
	token, _ = body["token"].(string)
	party, _ = body["party"].(string)
	if token == "" || party == "" {
		t.Fatalf("join body %s", rec.Body.String())
	}
	return token, party
}

func TestORPSignedHello(t *testing.T) {
	_, r := trustHost(t)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	token, party := joinWithKey(t, r, testJWK(pub, "sig-key-1"))

	doc := signEnvelope(t, orpEnvelope("hello", "m_sig_hello_1", party, map[string]any{"versions": []any{"0.3"}}), priv, "sig-key-1")
	rec, body := postJSON(t, r, http.MethodPost, "/orp", doc, bearerHeader(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("signed hello %d %s", rec.Code, rec.Body.String())
	}
	if body["type"] != "hello" {
		t.Fatalf("hello type %s", rec.Body.String())
	}
}

func TestORPSigRejected(t *testing.T) {
	_, r := trustHost(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	token, party := joinWithKey(t, r, testJWK(pub, "sig-key-2"))

	doc := signEnvelope(t, orpEnvelope("hello", "m_sig_bad_1", party, map[string]any{"versions": []any{"0.3"}}), priv, "sig-key-2")
	doc["id"] = "m_sig_bad_2"
	rec, body := postJSON(t, r, http.MethodPost, "/orp", doc, bearerHeader(token))
	if rec.Code != http.StatusUnauthorized || body["body"].(map[string]any)["code"] != "bad_signature" {
		t.Fatalf("mutated signed hello %d %s", rec.Code, rec.Body.String())
	}

	doc = signEnvelope(t, orpEnvelope("hello", "m_sig_kid_1", party, map[string]any{"versions": []any{"0.3"}}), priv, "other-kid")
	rec, _ = postJSON(t, r, http.MethodPost, "/orp", doc, bearerHeader(token))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown kid %d %s", rec.Code, rec.Body.String())
	}

	otherPub, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	_ = otherPub
	doc = signEnvelope(t, orpEnvelope("hello", "m_sig_wrong_1", party, map[string]any{"versions": []any{"0.3"}}), otherPriv, "sig-key-2")
	rec, _ = postJSON(t, r, http.MethodPost, "/orp", doc, bearerHeader(token))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key %d %s", rec.Code, rec.Body.String())
	}
}

func TestORPKeyedPartyMustSign(t *testing.T) {
	_, r := trustHost(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	token, party := joinWithKey(t, r, testJWK(pub, "sig-offer-1"))

	offer := orpEnvelope("offer", "m_sig_unsigned", party, standingOffer())
	offer["to"] = "orp:127.0.0.1:8080/nobody"
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(token))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned offer from keyed party %d %s", rec.Code, rec.Body.String())
	}
	_ = body

	hello := orpEnvelope("hello", "m_sig_stillok", party, map[string]any{"versions": []any{"0.3"}})
	rec, _ = postJSON(t, r, http.MethodPost, "/orp", hello, bearerHeader(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("unsigned hello from keyed party %d %s", rec.Code, rec.Body.String())
	}

	offer = signEnvelope(t, offer, priv, "sig-offer-1")
	rec, _ = postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(token))
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("signed offer refused %d %s", rec.Code, rec.Body.String())
	}
}

func TestORPPartyKeysEndpoint(t *testing.T) {
	_, r := trustHost(t)
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, party := joinWithKey(t, r, testJWK(pub, "sig-keys-1"))
	opaque := party[strings.LastIndex(party, "/")+1:]

	rec, body := postJSON(t, r, http.MethodGet, "/orp/keys/"+opaque, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("keys endpoint %d %s", rec.Code, rec.Body.String())
	}
	keys, _ := body["keys"].([]any)
	if len(keys) != 1 {
		t.Fatalf("keys body %s", rec.Body.String())
	}
	key, _ := keys[0].(map[string]any)
	if key["kid"] != "sig-keys-1" || key["crv"] != "Ed25519" {
		t.Fatalf("key body %s", rec.Body.String())
	}

	other := "f"
	if strings.HasSuffix(opaque, other) {
		other = "e"
	}
	rec, _ = postJSON(t, r, http.MethodGet, "/orp/keys/"+opaque[:len(opaque)-1]+other, nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown opaque %d %s", rec.Code, rec.Body.String())
	}
}

func TestORPSigNoRegisteredKey(t *testing.T) {
	_, r := trustHost(t)
	token, party := joinParty(t, r)
	doc := orpEnvelope("hello", "m_sig_none_1", party, map[string]any{"versions": []any{"0.3"}})
	doc["sig"] = "e30..AAAA"
	rec, body := postJSON(t, r, http.MethodPost, "/orp", doc, bearerHeader(token))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sig without key %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "no registered key") {
		t.Fatalf("expected no-key guidance, got %s", rec.Body.String())
	}
	_ = body
}

func TestORPJoinKeyValidation(t *testing.T) {
	_, r := trustHost(t)
	goodX := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	cases := []map[string]any{
		{"kty": "OKP", "crv": "Ed25519", "x": goodX, "kid": "k1", "d": "private"},
		{"kty": "OKP", "crv": "P-256", "x": goodX, "kid": "k1"},
		{"kty": "OKP", "crv": "Ed25519", "x": "short", "kid": "k1"},
		{"kty": "OKP", "crv": "Ed25519", "x": goodX},
		{"kty": "RSA", "crv": "Ed25519", "x": goodX, "kid": "k1"},
	}
	for i, key := range cases {
		rec, _ := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"key": key}, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("case %d: got %d body %s", i, rec.Code, rec.Body.String())
		}
	}
}
