package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/database"
)

func paidTerms(hold string) map[string]any {
	pay := map[string]any{
		"id":     "pay",
		"action": "pay",
		"by":     "offerer",
		"per":    "placement",
		"after":  "place",
		"amount": map[string]any{"amount_minor": 1500, "currency": "EUR"},
	}
	if hold != "" {
		pay["hold"] = hold
	}
	return map[string]any{
		"title":      "zqtestmention paid notes",
		"kind":       "sponsored",
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"obligations": []any{
			map[string]any{
				"id":          "place",
				"action":      "place",
				"by":          "acceptor",
				"content":     map[string]any{"link": true},
				"destination": "https://example.com/notes",
				"proof":       []any{"link"},
			},
			pay,
		},
	}
}

func clickForDeal(t *testing.T, r http.Handler, dealID, token string) {
	t.Helper()
	dealRec, dealDoc := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(token))
	if dealRec.Code != http.StatusOK {
		t.Fatalf("deal %d %s", dealRec.Code, dealRec.Body.String())
	}
	rows, _ := dealDoc["body"].(map[string]any)["tracking"].([]any)
	if len(rows) == 0 {
		t.Fatalf("no tracking %s", dealRec.Body.String())
	}
	entry, _ := rows[0].(map[string]any)
	link, _ := entry["link"].(string)
	goToken := link[strings.LastIndex(link, "/go/")+4:]
	req := httptest.NewRequest(http.MethodGet, "/go/"+goToken, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("go redirect %d", w.Code)
	}
}

func inboxMessages(t *testing.T, r http.Handler, token string) []any {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodGet, "/orp/inbox", nil, bearerHeader(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("inbox %d %s", rec.Code, rec.Body.String())
	}
	messages, _ := body["messages"].([]any)
	return messages
}

func lastPayoutRecord(t *testing.T, messages []any) map[string]any {
	t.Helper()
	var found map[string]any
	for _, item := range messages {
		msg, _ := item.(map[string]any)
		if msg["type"] != "payout" {
			continue
		}
		found, _ = msg["body"].(map[string]any)
	}
	if found == nil {
		t.Fatal("no payout record in inbox")
	}
	return found
}

func TestORPPayoutLifecycle(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	if err := entry.DB.SaveStepModes(context.Background(), ownerOfParty(maker), "", database.StepModes{Payout: "auto"}); err != nil {
		t.Fatal(err)
	}

	offer := orpEnvelope("offer", "m_pay_offer_1", maker, map[string]any{"terms": paidTerms("")})
	offer["to"] = publisher
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(makerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("offer %d %s", rec.Code, rec.Body.String())
	}
	dealID, _ := body["deal"].(string)
	termsHash, _ := body["body"].(map[string]any)["terms_hash"].(string)

	acc := orpEnvelope("accept", "m_pay_accept_1", publisher, map[string]any{"terms_hash": termsHash})
	acc["deal"] = dealID
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp", acc, bearerHeader(pubToken)); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(makerToken)); rec.Code != http.StatusOK {
		t.Fatalf("activate %d %s", rec.Code, rec.Body.String())
	}

	clickForDeal(t, r, dealID, makerToken)

	sent := lastPayoutRecord(t, inboxMessages(t, r, pubToken))
	if sent["state"] != "sent" || sent["obligation"] != "pay" {
		t.Fatalf("payout record %v", sent)
	}
	payoutID, _ := sent["payout"].(string)
	amount, _ := sent["amount"].(map[string]any)
	if amount["amount_minor"] != float64(1500) || amount["currency"] != "EUR" || sent["basis"] != "receipt" {
		t.Fatalf("payout amount %v", sent)
	}

	makerRec := lastPayoutRecord(t, inboxMessages(t, r, makerToken))
	if makerRec["payout"] != payoutID || makerRec["state"] != "sent" {
		t.Fatalf("maker payout record %v", makerRec)
	}

	wrong := orpEnvelope("payout", "m_pay_wrong_1", maker, map[string]any{"payout": payoutID, "answer": "confirm"})
	wrong["deal"] = dealID
	rec, body = postJSON(t, r, http.MethodPost, "/orp", wrong, bearerHeader(makerToken))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("wrong payee %d %s", rec.Code, rec.Body.String())
	}

	ghost := orpEnvelope("payout", "m_pay_ghost_1", publisher, map[string]any{"payout": "po_missing0000", "answer": "confirm"})
	ghost["deal"] = dealID
	rec, body = postJSON(t, r, http.MethodPost, "/orp", ghost, bearerHeader(pubToken))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing payout %d %s", rec.Code, rec.Body.String())
	}

	confirm := orpEnvelope("payout", "m_pay_answer_1", publisher, map[string]any{"payout": payoutID, "answer": "confirm"})
	confirm["deal"] = dealID
	rec, body = postJSON(t, r, http.MethodPost, "/orp", confirm, bearerHeader(pubToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm %d %s", rec.Code, rec.Body.String())
	}
	if body["body"].(map[string]any)["state"] != "paid" {
		t.Fatalf("confirm state %s", rec.Body.String())
	}

	paid := lastPayoutRecord(t, inboxMessages(t, r, makerToken))
	if paid["state"] != "paid" {
		t.Fatalf("maker paid record %v", paid)
	}

	again := orpEnvelope("payout", "m_pay_again_1", publisher, map[string]any{"payout": payoutID, "answer": "confirm"})
	again["deal"] = dealID
	rec, _ = postJSON(t, r, http.MethodPost, "/orp", again, bearerHeader(pubToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("second confirm %d %s", rec.Code, rec.Body.String())
	}
}

func TestORPPayoutHoldAndAsk(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)

	offer := orpEnvelope("offer", "m_pay_offer_2", maker, map[string]any{"terms": paidTerms("PT1H")})
	offer["to"] = publisher
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(makerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("offer %d %s", rec.Code, rec.Body.String())
	}
	dealID, _ := body["deal"].(string)
	termsHash, _ := body["body"].(map[string]any)["terms_hash"].(string)

	acc := orpEnvelope("accept", "m_pay_accept_2", publisher, map[string]any{"terms_hash": termsHash})
	acc["deal"] = dealID
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp", acc, bearerHeader(pubToken)); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(makerToken)); rec.Code != http.StatusOK {
		t.Fatalf("activate %d %s", rec.Code, rec.Body.String())
	}

	clickForDeal(t, r, dealID, makerToken)

	held := lastPayoutRecord(t, inboxMessages(t, r, pubToken))
	if held["state"] != "held" || held["hold_until"] == nil {
		t.Fatalf("held record %v", held)
	}
	payoutID, _ := held["payout"].(string)

	early := orpEnvelope("payout", "m_pay_early_1", publisher, map[string]any{"payout": payoutID, "answer": "confirm"})
	early["deal"] = dealID
	rec, _ = postJSON(t, r, http.MethodPost, "/orp", early, bearerHeader(pubToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("held answer %d %s", rec.Code, rec.Body.String())
	}

	if _, err := entry.DB.Pool.Exec(context.Background(), `UPDATE orp_payouts SET hold_until = NOW() - INTERVAL '1 minute' WHERE id=$1`, payoutID); err != nil {
		t.Fatal(err)
	}
	if err := entry.DB.SaveStepModes(context.Background(), ownerOfParty(maker), "", database.StepModes{Payout: "ask"}); err != nil {
		t.Fatal(err)
	}

	messages := inboxMessages(t, r, makerToken)
	payable := lastPayoutRecord(t, messages)
	if payable["state"] != "payable" {
		t.Fatalf("advanced record %v", payable)
	}
	var stored string
	if err := entry.DB.Pool.QueryRow(context.Background(), `SELECT state FROM orp_payouts WHERE id=$1`, payoutID).Scan(&stored); err != nil || stored != "payable" {
		t.Fatalf("stored %q %v", stored, err)
	}
}

func TestORPPayoutNoBasis(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)

	offer := orpEnvelope("offer", "m_pay_offer_3", maker, map[string]any{"terms": paidTerms("")})
	offer["to"] = publisher
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(makerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("offer %d %s", rec.Code, rec.Body.String())
	}
	dealID, _ := body["deal"].(string)
	termsHash, _ := body["body"].(map[string]any)["terms_hash"].(string)

	acc := orpEnvelope("accept", "m_pay_accept_3", publisher, map[string]any{"terms_hash": termsHash})
	acc["deal"] = dealID
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp", acc, bearerHeader(pubToken)); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(makerToken)); rec.Code != http.StatusOK {
		t.Fatalf("activate %d %s", rec.Code, rec.Body.String())
	}

	for _, item := range inboxMessages(t, r, pubToken) {
		msg, _ := item.(map[string]any)
		if msg["type"] == "payout" {
			t.Fatalf("payout minted before verification: %v", msg["body"])
		}
	}
}

func TestORPPayoutPerDeal(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	if err := entry.DB.SaveStepModes(context.Background(), ownerOfParty(maker), "", database.StepModes{Payout: "auto"}); err != nil {
		t.Fatal(err)
	}

	terms := paidTerms("")
	obs, _ := terms["obligations"].([]any)
	pay, _ := obs[1].(map[string]any)
	pay["per"] = "deal"
	delete(pay, "after")

	offer := orpEnvelope("offer", "m_pay_offer_4", maker, map[string]any{"terms": terms})
	offer["to"] = publisher
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(makerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("offer %d %s", rec.Code, rec.Body.String())
	}
	dealID, _ := body["deal"].(string)
	termsHash, _ := body["body"].(map[string]any)["terms_hash"].(string)

	acc := orpEnvelope("accept", "m_pay_accept_4", publisher, map[string]any{"terms_hash": termsHash})
	acc["deal"] = dealID
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp", acc, bearerHeader(pubToken)); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(makerToken)); rec.Code != http.StatusOK {
		t.Fatalf("activate %d %s", rec.Code, rec.Body.String())
	}

	clickForDeal(t, r, dealID, makerToken)

	sent := lastPayoutRecord(t, inboxMessages(t, r, pubToken))
	if sent["state"] != "sent" || sent["basis"] != "receipt" {
		t.Fatalf("per-deal payout %v", sent)
	}
}
