package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"golang.org/x/oauth2"
)

func TestOIDCVerifiesSignedIdentityAndNonce(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "pilot-key"))
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"", "nonce", "audience", "issuer", "expired", "unverified_email"} {
		t.Run(failure, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					json.NewEncoder(w).Encode(map[string]any{"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token", "jwks_uri": server.URL + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
				case "/keys":
					json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "pilot-key", Algorithm: "RS256", Use: "sig"}}})
				case "/token":
					if r.Method != "POST" || r.ParseForm() != nil || r.PostFormValue("code") != "valid-code" || r.PostFormValue("code_verifier") != "verifier" {
						t.Error("exchange lost authorization code or PKCE verifier")
					}
					claims := jwt.Claims{Issuer: server.URL, Subject: "stable-subject", Audience: jwt.Audience{"pilot-client"}, Expiry: jwt.NewNumericDate(time.Now().Add(time.Hour))}
					nonce := "expected-nonce"
					verified := true
					switch failure {
					case "nonce":
						nonce = "foreign-nonce"
					case "audience":
						claims.Audience = jwt.Audience{"another-client"}
					case "issuer":
						claims.Issuer = "https://foreign.example"
					case "expired":
						claims.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Hour))
					case "unverified_email":
						verified = false
					}
					token, err := jwt.Signed(signer).Claims(claims).Claims(map[string]any{"nonce": nonce, "email": "owner@example.com", "email_verified": verified, "name": "Pilot Owner"}).Serialize()
					if err != nil {
						t.Error(err)
					}
					json.NewEncoder(w).Encode(map[string]any{"access_token": "provider-token-not-forwarded", "token_type": "Bearer", "id_token": token})
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			ctx := oidc.ClientContext(context.Background(), server.Client())
			provider, err := oidc.NewProvider(ctx, server.URL)
			if err != nil {
				t.Fatal(err)
			}
			login := &OIDCLogin{issuer: server.URL, client: server.Client(), verifier: provider.Verifier(&oidc.Config{ClientID: "pilot-client"}), config: oauth2.Config{ClientID: "pilot-client", ClientSecret: "test-secret", Endpoint: provider.Endpoint(), RedirectURL: "https://pilot.example/auth/callback"}}
			o, err := login.Verify(context.Background(), "valid-code", "expected-nonce", "verifier")
			if failure == "" {
				if err != nil || o.Subject != "stable-subject" || o.Email != "owner@example.com" {
					t.Fatalf("valid identity rejected: %+v %v", o, err)
				}
			} else if err == nil {
				t.Fatalf("invalid identity accepted: %s", failure)
			}
		})
	}
}
