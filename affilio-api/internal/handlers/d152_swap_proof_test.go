package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"affilio-api/internal/orp"
)

func swapProofTerms(title string) map[string]any {
	return map[string]any{
		"title":      title,
		"kind":       "swap",
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"obligations": []any{
			map[string]any{
				"id": "place_a", "action": "place", "by": "offerer",
				"destination": "https://example.org/", "proof": []any{"proof_url"},
			},
			map[string]any{
				"id": "place_b", "action": "place", "by": "acceptor",
				"destination": "https://example.com/", "proof": []any{"proof_url"},
			},
			map[string]any{
				"id": "pay", "action": "pay", "by": "offerer", "per": "placement",
				"amount": map[string]any{"amount_minor": 1500, "currency": "EUR"},
			},
		},
	}
}

func dealDoc(t *testing.T, r *gin.Engine, token, dealID string) map[string]any {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("deal %d %s", rec.Code, rec.Body.String())
	}
	inner, _ := body["body"].(map[string]any)
	if inner == nil {
		t.Fatalf("deal body %s", rec.Body.String())
	}
	return inner
}

func trackingByObligation(t *testing.T, doc map[string]any) map[string]map[string]any {
	t.Helper()
	rows, _ := doc["tracking"].([]any)
	out := map[string]map[string]any{}
	for _, item := range rows {
		row, _ := item.(map[string]any)
		id, _ := row["obligation"].(string)
		out[id] = row
	}
	return out
}

func repComponent(t *testing.T, r *gin.Engine, token, from, subject, id, name string) float64 {
	t.Helper()
	query := orpEnvelope("reputation", id, from, map[string]any{"subject": subject})
	rec, body := postJSON(t, r, http.MethodPost, "/orp", query, bearerHeader(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("reputation %d %s", rec.Code, rec.Body.String())
	}
	inner, _ := body["body"].(map[string]any)
	comp, _ := inner["components"].(map[string]any)
	n, ok := comp[name].(float64)
	if !ok {
		t.Fatalf("%s component %v", name, comp)
	}
	return n
}

// Two test parties run one swap to verified on both sides with the test
// fixture and open the proof page showing both obligations.
func TestD152SwapReachesVerifiedOnBothSides(t *testing.T) {
	entry, r := proofHost(t, func(ctx context.Context, rawURL string) (string, int, bool) {
		t.Errorf("fixture proof was fetched over HTTP: %s", rawURL)
		return "", 0, false
	})
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	dealID := offerTerms(t, r, makerToken, maker, publisher, pubToken, "d152swap", swapProofTerms("TEST shout-out swap"))

	doc := dealDoc(t, r, makerToken, dealID)
	tracks := trackingByObligation(t, doc)
	if len(tracks) != 2 {
		t.Fatalf("tracking %v", doc["tracking"])
	}
	linkA, _ := tracks["place_a"]["link"].(string)
	linkB, _ := tracks["place_b"]["link"].(string)
	if !strings.Contains(linkA, "/go/") || !strings.Contains(linkB, "/go/") || linkA == linkB {
		t.Fatalf("links %s %s", linkA, linkB)
	}
	tokenA := linkA[strings.LastIndex(linkA, "/")+1:]
	tokenB := linkB[strings.LastIndex(linkB, "/")+1:]
	if tracks["place_a"]["destination"] != "https://example.org/" || tracks["place_b"]["destination"] != "https://example.com/" {
		t.Fatalf("destinations %v %v", tracks["place_a"], tracks["place_b"])
	}

	origin := entry.BaseURL
	urlA := orp.TestFixtureURL(origin, dealID, "place_a")
	urlB := orp.TestFixtureURL(origin, dealID, "place_b")
	codeA, _ := executeProof(t, r, makerToken, maker, dealID, "place_a", urlA, "d152_a")
	codeB, _ := executeProof(t, r, pubToken, publisher, dealID, "place_b", urlB, "d152_b")
	if *codeA != http.StatusOK || *codeB != http.StatusOK {
		t.Fatalf("execute %d %d", *codeA, *codeB)
	}
	rowA := obligationRow(t, entry.DB, dealID, "place_a")
	rowB := obligationRow(t, entry.DB, dealID, "place_b")
	for _, row := range []map[string]any{rowA, rowB} {
		if row["state"] != "verified" || row["method"] != "proof_url" || row["test_fixture"] != true || row["receipt"] == "" {
			t.Fatalf("row %v", row)
		}
	}
	if rowA["url"] != urlA || rowB["url"] != urlB {
		t.Fatalf("urls %v %v", rowA["url"], rowB["url"])
	}
	receiptA, _ := rowA["receipt"].(string)
	receiptB, _ := rowB["receipt"].(string)

	wire := dealDoc(t, r, pubToken, dealID)
	rawWire, _ := json.Marshal(wire["obligations"])
	if strings.Contains(string(rawWire), "test_fixture") {
		t.Fatalf("wire leaked test_fixture %s", rawWire)
	}

	pageRec, _ := postJSON(t, r, http.MethodGet, "/orp/proof/"+dealID, nil, nil)
	page := pageRec.Body.String()
	if pageRec.Code != http.StatusOK || !strings.Contains(pageRec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("proof %d %s", pageRec.Code, pageRec.Header().Get("Content-Type"))
	}
	for _, want := range []string{
		"obligation place_a state verified receipt " + receiptA + " url " + urlA,
		"obligation place_b state verified receipt " + receiptB + " url " + urlB,
		"Allagio does not hold money",
		"Affiliate link",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("proof page missing %q: %s", want, page)
		}
	}
	if strings.Contains(page, maker) || strings.Contains(page, publisher) || strings.Contains(page, "/go/") {
		t.Fatalf("public proof leaked a party or a tracked link: %s", page)
	}

	anon, _ := postJSON(t, r, http.MethodGet, urlA[len(origin):], nil, nil)
	if anon.Code != http.StatusUnauthorized {
		t.Fatalf("anon fixture %d %s", anon.Code, anon.Body.String())
	}
	bad, _ := postJSON(t, r, http.MethodGet, urlA[len(origin):], nil, map[string]string{"Authorization": "Bearer not-a-real-token"})
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("bad bearer %d %s", bad.Code, bad.Body.String())
	}
	strangerToken, _ := joinParty(t, r)
	stranger, _ := postJSON(t, r, http.MethodGet, urlA[len(origin):], nil, bearerHeader(strangerToken))
	if stranger.Code != http.StatusNotFound {
		t.Fatalf("stranger fixture %d %s", stranger.Code, stranger.Body.String())
	}
	owned, _ := postJSON(t, r, http.MethodGet, urlA[len(origin):], nil, bearerHeader(makerToken))
	ownedPage := owned.Body.String()
	if owned.Code != http.StatusOK || !strings.Contains(ownedPage, "TEST") || !strings.Contains(ownedPage, "does not count toward reputation or earnings") || !strings.Contains(ownedPage, "Affiliate link") || !strings.Contains(ownedPage, "/go/"+tokenA) {
		t.Fatalf("owner fixture %d %s", owned.Code, ownedPage)
	}
	other, _ := postJSON(t, r, http.MethodGet, urlB[len(origin):], nil, bearerHeader(pubToken))
	if other.Code != http.StatusOK || !strings.Contains(other.Body.String(), "/go/"+tokenB) || !strings.Contains(other.Body.String(), "TEST") {
		t.Fatalf("acceptor fixture %d %s", other.Code, other.Body.String())
	}
	cookie, _ := sessionForOwner(t, entry.DB, ownerOfParty(maker), entry.BaseURL)
	withCookie, _ := serveCookie(t, r, http.MethodGet, urlA[len(origin):], cookie, nil)
	if withCookie.Code != http.StatusOK || !strings.Contains(withCookie.Body.String(), "TEST") {
		t.Fatalf("owner cookie %d %s", withCookie.Code, withCookie.Body.String())
	}
	outsider := otherOwnerCookie(t, entry.DB, entry.BaseURL)
	hidden, _ := serveCookie(t, r, http.MethodGet, urlA[len(origin):], outsider, nil)
	if hidden.Code != http.StatusNotFound {
		t.Fatalf("other owner %d %s", hidden.Code, hidden.Body.String())
	}

	if repComponent(t, r, makerToken, maker, maker, "m_d152_rep_maker", "verified") != 0 || repComponent(t, r, makerToken, maker, maker, "m_d152_rep_maker2", "failed") != 0 {
		t.Fatal("fixture moved the offerer's reputation")
	}
	if repComponent(t, r, pubToken, publisher, publisher, "m_d152_rep_pub", "verified") != 0 || repComponent(t, r, pubToken, publisher, publisher, "m_d152_rep_pub2", "failed") != 0 {
		t.Fatal("fixture moved the acceptor's reputation")
	}
	inboxMessages(t, r, makerToken)
	if countRows(t, entry.DB, `SELECT COUNT(*) FROM orp_payouts WHERE deal_id=$1`, dealID) != 0 {
		t.Fatal("fixture verification minted a payout")
	}
}

func TestD152NonTestDealNeedsARealPublicPage(t *testing.T) {
	pages := map[string]string{}
	entry, r := proofHost(t, func(ctx context.Context, rawURL string) (string, int, bool) {
		if page, ok := pages[rawURL]; ok {
			return page, http.StatusOK, true
		}
		if strings.Contains(rawURL, "/orp/proof/") && !strings.Contains(rawURL, "/test/") {
			return `<html><p>This is a test proof. The deal is active. Allagio does not hold money.</p></html>`, http.StatusOK, true
		}
		return "", 0, false
	})
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	setPlacementPublish(t, entry.DB, ownerOfParty(maker), "auto")
	origin := entry.BaseURL

	blocked := offerTerms(t, r, makerToken, maker, publisher, pubToken, "d152live", swapProofTerms("Live swap"))
	doc := dealDoc(t, r, makerToken, blocked)
	tracks := trackingByObligation(t, doc)
	tokenA := strings.TrimPrefix(tracks["place_a"]["link"].(string), origin+"/go/")
	passing := `<html><p>Affiliate link</p><p>/go/` + tokenA + `</p></html>`
	fixtureURL := orp.TestFixtureURL(origin, blocked, "place_a")
	pages[fixtureURL] = passing
	code, _ := executeProof(t, r, makerToken, maker, blocked, "place_a", fixtureURL, "d152_fix")
	row := obligationRow(t, entry.DB, blocked, "place_a")
	if *code != http.StatusOK || row["state"] != "claimed" || row["receipt"] != nil {
		t.Fatalf("fixture verified a live deal %d %v", *code, row)
	}
	publicURL := origin + "/orp/proof/" + blocked
	code, _ = executeProof(t, r, pubToken, publisher, blocked, "place_b", publicURL, "d152_pub")
	row = obligationRow(t, entry.DB, blocked, "place_b")
	if *code != http.StatusOK || row["state"] == "verified" || row["state"] != "failed" {
		t.Fatalf("public proof page verified a live deal %v", row)
	}

	realID := offerTerms(t, r, makerToken, maker, publisher, pubToken, "d152real", swapProofTerms("Live notes"))
	realDoc := dealDoc(t, r, makerToken, realID)
	realTracks := trackingByObligation(t, realDoc)
	realToken := strings.TrimPrefix(realTracks["place_a"]["link"].(string), origin+"/go/")
	realURL := "https://publisher.example/review"
	pages[realURL] = `<html><p>Affiliate link</p><p>/go/` + realToken + `</p></html>`
	code, _ = executeProof(t, r, makerToken, maker, realID, "place_a", realURL, "d152_real")
	realRow := obligationRow(t, entry.DB, realID, "place_a")
	if *code != http.StatusOK || realRow["state"] != "verified" || realRow["method"] != "proof_url" || realRow["test_fixture"] == true {
		t.Fatalf("real page %v", realRow)
	}

	inboxMessages(t, r, makerToken)
	if countRows(t, entry.DB, `SELECT COUNT(*) FROM orp_payouts WHERE deal_id=$1`, blocked) != 0 {
		t.Fatal("unverified live swap minted a payout")
	}
	if countRows(t, entry.DB, `SELECT COUNT(*) FROM orp_payouts WHERE deal_id=$1`, realID) != 1 {
		t.Fatal("real proof did not mint the placement payout")
	}
	if repComponent(t, r, makerToken, maker, maker, "m_d152_live_rep", "verified") <= 0 {
		t.Fatal("real proof did not count for reputation")
	}
	if repComponent(t, r, pubToken, publisher, publisher, "m_d152_live_fail", "failed") <= 0 {
		t.Fatal("a failed public page did not count as failed")
	}
}
