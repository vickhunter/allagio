package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/database"

	"github.com/gin-gonic/gin"
)

func insertDealOwner(t *testing.T, db *database.DB, email string, siteID string) string {
	t.Helper()
	var ownerID string
	err := db.Pool.QueryRow(context.Background(),
		`INSERT INTO owners(issuer,subject,email,name,source) VALUES('agent',$2,$1,'Agent','direct') RETURNING id`,
		email, email).Scan(&ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if siteID != "" {
		if _, err := db.Pool.Exec(context.Background(),
			`UPDATE owners SET site_id=$2::uuid WHERE id=$1::uuid`, ownerID, siteID); err != nil {
			t.Fatal(err)
		}
	}
	return ownerID
}

func insertSiteForOwner(t *testing.T, db *database.DB, hash, domain string) string {
	t.Helper()
	var id string
	if err := db.Pool.QueryRow(context.Background(),
		`INSERT INTO sites(api_key_hash, api_key_prefix, domain, name) VALUES($1,$2,$3,$3) RETURNING id::text`,
		hash, hash[:6], domain).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertORPOffer(t *testing.T, db *database.DB, id, ownerID, terms string, listed bool) {
	t.Helper()
	if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO orp_offers(id, owner_id, party, terms, terms_hash, deal_limit, created_at, listed)
VALUES($1, $2::uuid, 'orp:allagio.test/' || $3, $4, 'sha256-' || $1, 3, NOW(), $5)`,
		id, ownerID, ownerID, terms, listed); err != nil {
		t.Fatal(err)
	}
}

func insertORPDeal(t *testing.T, db *database.DB, id, terms, offererOwner, offererParty, acceptorOwner, acceptorParty, state string) {
	t.Helper()
	offerer := "NULL"
	if offererOwner != "" {
		offerer = "'" + offererOwner + "'::uuid"
	}
	if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO orp_deals(id, offerer_owner, acceptor_owner, offerer_party, acceptor_party, terms, terms_hash, state, obligations, bound, created_at, updated_at)
VALUES($1, `+offerer+`, $2::uuid, $3, $4, $5, 'sha256-' || $1, $6, '[]', '[]', NOW(), NOW())`,
		id, acceptorOwner, offererParty, acceptorParty, terms, state); err != nil {
		t.Fatal(err)
	}
}

func TestPublicDealsWallLabels(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	siteID := insertSiteForOwner(t, db, "dwall-site", "site-wall.example")
	makerID := insertDealOwner(t, db, "maker-wall@example.com", "")
	pubID := insertDealOwner(t, db, "pub-wall@example.com", siteID)
	expiredID := insertDealOwner(t, db, "expired-wall@example.com", "")
	ghostID := insertDealOwner(t, db, "ghost-wall@example.com", "")

	insertORPOffer(t, db, "off_wall_named", makerID, `{"title":"Vibe Sales"}`, true)
	insertORPOffer(t, db, "off_wall_expired", expiredID, `{"title":"Expired Co","expires_at":"2000-01-01T00:00:00Z"}`, true)

	// Named maker ↔ anonymous publisher.
	insertORPDeal(t, db, "deal_wall_1",
		`{"kind":"affiliate","title":"deal"}`, makerID, "orp:allagio.test/"+makerID,
		pubID, "orp:allagio.test/"+pubID, "active")
	// Expired listing never names the maker.
	insertORPDeal(t, db, "deal_wall_2",
		`{"kind":"repost","title":"deal"}`, expiredID, "orp:allagio.test/"+expiredID,
		ghostID, "orp:allagio.test/"+ghostID, "awaiting")
	// A foreign party shows as an agent on its own host.
	insertORPDeal(t, db, "deal_wall_3",
		`{"kind":"link_exchange","title":"deal"}`, "", "orp:otherhost.net/x_foreign",
		pubID, "orp:allagio.test/"+pubID, "proposed")
	// No acceptor seat yet: syncColumns stores the offerer's owner as a
	// fallback AcceptorOwner, but with no acceptor party the right side must
	// stay anonymous rather than echo the listed offerer.
	insertORPDeal(t, db, "deal_wall_4",
		`{"kind":"sponsored","title":"deal"}`, makerID, "orp:allagio.test/"+makerID,
		makerID, "", "awaiting")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&AgentEntry{DB: db, BaseURL: "https://allagio.test"}).Mount(r)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/deals/recent", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") == "" {
		t.Fatal("no cache header")
	}
	var body struct {
		Deals []struct {
			Kind     string `json:"kind"`
			State    string `json:"state"`
			Parties  int    `json:"parties"`
			Offerer  struct {
				Listed bool   `json:"listed"`
				Name   string `json:"name"`
				Role   string `json:"role"`
				Host   string `json:"host"`
			} `json:"offerer"`
			Acceptor struct {
				Listed bool   `json:"listed"`
				Role   string `json:"role"`
			} `json:"acceptor"`
		} `json:"deals"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Deals) != 4 {
		t.Fatalf("expected 4 deals, got %d", len(body.Deals))
	}
	byKind := map[string]int{}
	for i, deal := range body.Deals {
		byKind[deal.Kind] = i
	}

	named := body.Deals[byKind["affiliate"]]
	if !named.Offerer.Listed || named.Offerer.Name != "Vibe Sales" {
		t.Fatalf("listed offerer %#v", named.Offerer)
	}
	if named.Acceptor.Role != "publisher" || named.Acceptor.Listed {
		t.Fatalf("publisher acceptor %#v", named.Acceptor)
	}
	if named.Parties != 2 || named.State != "active" {
		t.Fatalf("affiliate row %#v", named)
	}

	expired := body.Deals[byKind["repost"]]
	if expired.Offerer.Listed || expired.Offerer.Name != "" || expired.Offerer.Role != "maker" {
		t.Fatalf("expired listing should anonymize %#v", expired.Offerer)
	}
	if expired.Acceptor.Role != "agent" {
		t.Fatalf("no-footprint acceptor %#v", expired.Acceptor)
	}

	foreign := body.Deals[byKind["link_exchange"]]
	if foreign.Offerer.Role != "agent" || foreign.Offerer.Host != "otherhost.net" {
		t.Fatalf("foreign party %#v", foreign.Offerer)
	}

	open := body.Deals[byKind["sponsored"]]
	if !open.Offerer.Listed || open.Offerer.Name != "Vibe Sales" {
		t.Fatalf("open-seat offerer %#v", open.Offerer)
	}
	if open.Acceptor.Listed || open.Acceptor.Role != "agent" {
		t.Fatalf("empty acceptor seat must not name the offerer %#v", open.Acceptor)
	}

	// Deal ids must never appear in the payload.
	for _, needle := range []string{"deal_wall_1", "deal_wall_2", "deal_wall_3", makerID, pubID} {
		if strings.Contains(rec.Body.String(), needle) {
			t.Fatalf("response leaks %q", needle)
		}
	}
}
