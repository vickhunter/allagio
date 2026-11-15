package handlers

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"affilio-api/internal/orp"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func TestDealCertificateSignedAndVerifiable(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	var ownerID string
	if err := db.Pool.QueryRow(ctx, `INSERT INTO owners(issuer,subject,email,name,source) VALUES('iss','sub','cert@example.com','Cert','direct') RETURNING id::text`).Scan(&ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	// A join bearer whose owner sits on the deal.
	partyGrant, err := db.CreateAgentJoinToken(ctx, services.HashAPIKey("certtok"), "orat_crt", "ip-cert", "http://allagio.test/mcp", "Agent", "join:cert", "join-cert@agents.invalid", nil)
	if err != nil {
		t.Fatalf("join token: %v", err)
	}
	party := orp.MemberParty("allagio.test", partyGrant.OwnerID)
	receipts := `[{"obligation":"place","id":"rcpt_1","method":"link","verified_at":"2026-11-15T00:00:00Z"}]`
	tracking := `{"tracking":"trk","clicks":[],"link_receipts":` + receipts + `}`
	if _, err := db.Pool.Exec(ctx, `INSERT INTO orp_deals(id, acceptor_owner, offerer_party, acceptor_party, terms, terms_hash, state, obligations, tracking, created_at, updated_at)
VALUES('deal_cert_1',$1,$2,'orp:test/p_accept','{"title":"deal"}','sha256:abc','active','[]',$3,NOW(),NOW())`, ownerID, party, tracking); err != nil {
		t.Fatalf("insert deal: %v", err)
	}

	seed := sha256.Sum256([]byte("test-host-seed"))
	signer := &orp.Signer{Kid: "host", Key: ed25519.NewKeyFromSeed(seed[:])}
	host := &ORPHost{Entry: &AgentEntry{DB: db}, Signer: signer}

	r := gin.New()
	r.GET("/.well-known/orp.json", host.Discovery)
	r.GET("/orp/deals/:id/certificate", host.DealCertificate)

	// No bearer → 401.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orp/deals/deal_cert_1/certificate", nil)
	req.Host = "allagio.test"
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated certificate status %d", rec.Code)
	}
	// A bearer whose owner is not on the deal → 404.
	if _, err := db.CreateAgentJoinToken(ctx, services.HashAPIKey("stranger"), "orat_st", "ip-str", "http://allagio.test/mcp", "Agent", "join:str", "join-str@agents.invalid", nil); err != nil {
		t.Fatalf("stranger token: %v", err)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/orp/deals/deal_cert_1/certificate", nil)
	req.Host = "allagio.test"
	req.Header.Set("Authorization", "Bearer stranger")
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("non-party certificate status %d", rec.Code)
	}
	// The deal party gets the signed certificate.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/orp/deals/deal_cert_1/certificate", nil)
	req.Host = "allagio.test"
	req.Header.Set("Authorization", "Bearer certtok")
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("certificate status %d: %s", rec.Code, rec.Body.String())
	}
	var cert map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &cert); err != nil {
		t.Fatal(err)
	}
	if cert["terms_hash"] != "sha256:abc" || cert["deal"] != "deal_cert_1" || cert["state"] != "active" {
		t.Fatalf("certificate fields: %v", cert)
	}
	if _, leaked := cert["terms"]; leaked {
		t.Fatal("certificate leaks terms; only terms_hash may be served")
	}
	if rs, _ := cert["receipts"].([]any); len(rs) != 1 {
		t.Fatalf("receipts: %v", cert["receipts"])
	}

	// The discovery doc publishes the verifying JWK; the signature verifies.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/.well-known/orp.json", nil)
	req.Host = "allagio.test"
	r.ServeHTTP(rec, req)
	var disco map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &disco); err != nil {
		t.Fatal(err)
	}
	rawKeys, _ := disco["keys"].([]any)
	if len(rawKeys) != 1 {
		t.Fatalf("discovery keys: %v", disco["keys"])
	}
	var keys []map[string]any
	for _, k := range rawKeys {
		if m, ok := k.(map[string]any); ok {
			keys = append(keys, m)
		}
	}
	if kid, ok := orp.VerifyEnvelopeSig(cert, keys); !ok || kid != "host" {
		t.Fatalf("signature did not verify: kid=%q ok=%v", kid, ok)
	}

	// Tampering breaks verification.
	cert["state"] = "disputed"
	if _, ok := orp.VerifyEnvelopeSig(cert, keys); ok {
		t.Fatal("tampered certificate still verifies")
	}
}

func TestDealCertificateWithoutSignerIs503(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	host := &ORPHost{Entry: &AgentEntry{DB: db}}
	r := gin.New()
	r.GET("/orp/deals/:id/certificate", host.DealCertificate)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orp/deals/x/certificate", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
