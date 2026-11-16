package handlers

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"affilio-api/internal/chainpay"
	"affilio-api/internal/database"
	"affilio-api/internal/entry"
)

const (
	chainTestTx     = "0x2222222222222222222222222222222222222222222222222222222222222222"
	chainTestWallet = "0x00000000000000000000000000000000000000Ab"
)

// fakeBaseRPC is a Base Sepolia JSON-RPC that knows one USDC transfer.
type fakeBaseRPC struct {
	mu    sync.Mutex
	to    string
	units int64
	head  int64
}

func (f *fakeBaseRPC) setHead(n int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.head = n
}

func (f *fakeBaseRPC) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var req struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result any
		switch req.Method {
		case "eth_getTransactionReceipt":
			if len(req.Params) == 1 && req.Params[0] == chainTestTx {
				result = map[string]any{
					"status":      "0x1",
					"blockNumber": "0x64",
					"logs": []any{map[string]any{
						"address": chainpay.Networks["eip155:84532"].Token,
						"topics": []any{
							"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
							"0x000000000000000000000000" + strings.Repeat("b", 40),
							"0x000000000000000000000000" + strings.ToLower(strings.TrimPrefix(f.to, "0x")),
						},
						"data": "0x" + big.NewInt(f.units).Text(16),
					}},
				}
			}
		case "eth_blockNumber":
			result = "0x" + big.NewInt(f.head).Text(16)
		case "eth_getBlockByNumber":
			result = map[string]any{"timestamp": "0x" + big.NewInt(time.Now().Unix()).Text(16)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func chainTerms() map[string]any {
	terms := paidTerms("")
	obligations := terms["obligations"].([]any)
	pay := obligations[1].(map[string]any)
	pay["rail"] = "usdc.base-sepolia"
	pay["amount"] = map[string]any{"amount_minor": 1500, "currency": "USD"}
	return terms
}

func chainDeal(t *testing.T, r http.Handler, makerToken, maker, pubToken, publisher, suffix string) string {
	t.Helper()
	offer := orpEnvelope("offer", "m_chain_offer_"+suffix, maker, map[string]any{"terms": chainTerms()})
	offer["to"] = publisher
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(makerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("offer %d %s", rec.Code, rec.Body.String())
	}
	dealID, _ := body["deal"].(string)
	termsHash, _ := body["body"].(map[string]any)["terms_hash"].(string)
	acc := orpEnvelope("accept", "m_chain_accept_"+suffix, publisher, map[string]any{"terms_hash": termsHash})
	acc["deal"] = dealID
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp", acc, bearerHeader(pubToken)); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(makerToken)); rec.Code != http.StatusOK {
		t.Fatalf("activate %d %s", rec.Code, rec.Body.String())
	}
	clickForDeal(t, r, dealID, makerToken)
	return dealID
}

func TestORPChainPayoutVerifiedOnChain(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	rpc := &fakeBaseRPC{to: chainTestWallet, units: 15_000_000, head: 0x64}
	srv := rpc.serve(t)
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey, ClaimsCheck: "off-local", TokenLimiter: NewIPRateLimiter(8, time.Minute)}
	r := mountORP(entry, &ORPHost{Entry: entry, Chain: &chainpay.Verifier{RPC: map[string]string{"eip155:84532": srv.URL}, Confirmations: 3}})
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	ctx := context.Background()
	// auto would release a manual rail; a chain rail still waits for the ref.
	if err := db.SaveStepModes(ctx, ownerOfParty(maker), "", database.StepModes{Payout: "auto"}); err != nil {
		t.Fatal(err)
	}

	chainDeal(t, r, makerToken, maker, pubToken, publisher, "1")
	record := lastPayoutRecord(t, inboxMessages(t, r, pubToken))
	payoutID, _ := record["payout"].(string)
	if record["state"] != "payable" || record["rail"] != "usdc.base-sepolia" {
		t.Fatalf("payout record %v", record)
	}

	rec, doc := postJSON(t, r, http.MethodGet, "/orp/payouts/"+payoutID, nil, bearerHeader(makerToken))
	if rec.Code != http.StatusOK || len(doc["pay_to"].([]any)) != 0 {
		t.Fatalf("no wallet yet %d %s", rec.Code, rec.Body.String())
	}
	rec, _ = postJSON(t, r, http.MethodPost, "/orp/payouts/"+payoutID+"/sent", map[string]any{"ref": chainTestTx}, bearerHeader(makerToken))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "no wallet") {
		t.Fatalf("send without payee wallet %d %s", rec.Code, rec.Body.String())
	}

	if err := db.SetORPPayoutWallet(ctx, ownerOfParty(publisher), "eip155:84532", chainTestWallet); err != nil {
		t.Fatal(err)
	}
	rec, doc = postJSON(t, r, http.MethodGet, "/orp/payouts/"+payoutID, nil, bearerHeader(makerToken))
	payTo, _ := doc["pay_to"].([]any)
	if rec.Code != http.StatusOK || len(payTo) != 1 {
		t.Fatalf("pay_to %d %s", rec.Code, rec.Body.String())
	}
	target := payTo[0].(map[string]any)
	if target["address"] != chainTestWallet || target["amount_units"] != "15000000" || target["network"] != "eip155:84532" {
		t.Fatalf("pay_to target %v", target)
	}
	rec, doc = postJSON(t, r, http.MethodGet, "/orp/payouts/"+payoutID, nil, bearerHeader(pubToken))
	if _, present := doc["pay_to"]; rec.Code != http.StatusOK || present {
		t.Fatalf("payee must not get pay_to %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), chainTestWallet) {
		t.Fatal("payout record leaked the wallet")
	}

	rec, _ = postJSON(t, r, http.MethodPost, "/orp/payouts/"+payoutID+"/sent", map[string]any{"ref": chainTestTx}, bearerHeader(pubToken))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("payee send %d %s", rec.Code, rec.Body.String())
	}
	rec, _ = postJSON(t, r, http.MethodPost, "/orp/payouts/"+payoutID+"/sent", map[string]any{"ref": "0x1234"}, bearerHeader(makerToken))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad ref %d %s", rec.Code, rec.Body.String())
	}
	rec, _ = postJSON(t, r, http.MethodPost, "/orp/payouts/"+payoutID+"/sent", map[string]any{}, bearerHeader(makerToken))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing ref %d %s", rec.Code, rec.Body.String())
	}

	// One block deep: sent, still confirming.
	rec, doc = postJSON(t, r, http.MethodPost, "/orp/payouts/"+payoutID+"/sent", map[string]any{"ref": chainTestTx}, bearerHeader(makerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("send %d %s", rec.Code, rec.Body.String())
	}
	sent := doc["record"].(map[string]any)["body"].(map[string]any)
	chain := doc["chain"].(map[string]any)
	if sent["state"] != "sent" || sent["ref"] != chainTestTx || chain["status"] != "pending" || chain["reason"] != "confirming" || chain["tx"] != "eip155:84532:"+chainTestTx {
		t.Fatalf("sent doc %s", rec.Body.String())
	}

	// A wallet change after the ref does not fail the transfer already made.
	if err := db.SetORPPayoutWallet(ctx, ownerOfParty(publisher), "eip155:84532", "0x"+strings.Repeat("d", 40)); err != nil {
		t.Fatal(err)
	}

	// Deep enough: the next read marks it paid and both inboxes get it.
	rpc.setHead(0x64 + 5)
	rec, doc = postJSON(t, r, http.MethodGet, "/orp/payouts/"+payoutID, nil, bearerHeader(pubToken))
	paid := doc["record"].(map[string]any)["body"].(map[string]any)
	if rec.Code != http.StatusOK || paid["state"] != "paid" || doc["chain"].(map[string]any)["status"] != "verified" {
		t.Fatalf("paid doc %d %s", rec.Code, rec.Body.String())
	}
	for _, token := range []string{pubToken, makerToken} {
		last := lastPayoutRecord(t, inboxMessages(t, r, token))
		if last["state"] != "paid" || last["ref"] != chainTestTx {
			t.Fatalf("inbox paid record %v", last)
		}
	}
	rec, _ = postJSON(t, r, http.MethodPost, "/orp/payouts/"+payoutID+"/sent", map[string]any{"ref": chainTestTx}, bearerHeader(makerToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("send after paid %d %s", rec.Code, rec.Body.String())
	}

	// The same transaction cannot settle a second payout.
	chainDeal(t, r, makerToken, maker, pubToken, publisher, "2")
	second := lastPayoutRecord(t, inboxMessages(t, r, pubToken))
	secondID, _ := second["payout"].(string)
	if secondID == payoutID || second["state"] != "payable" {
		t.Fatalf("second payout %v", second)
	}
	rec, _ = postJSON(t, r, http.MethodPost, "/orp/payouts/"+secondID+"/sent", map[string]any{"ref": chainTestTx}, bearerHeader(makerToken))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "already settles") {
		t.Fatalf("reused tx %d %s", rec.Code, rec.Body.String())
	}
}

func TestORPChainPayoutMismatchTakesNewRef(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	rpc := &fakeBaseRPC{to: chainTestWallet, units: 1_000_000, head: 0x64 + 5}
	srv := rpc.serve(t)
	entry := &AgentEntry{DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey, ClaimsCheck: "off-local", TokenLimiter: NewIPRateLimiter(8, time.Minute)}
	host := &ORPHost{Entry: entry, Chain: &chainpay.Verifier{RPC: map[string]string{"eip155:84532": srv.URL}, Confirmations: 3}}
	r := mountORP(entry, host)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)
	ctx := context.Background()
	if err := db.SetORPPayoutWallet(ctx, ownerOfParty(publisher), "eip155:84532", chainTestWallet); err != nil {
		t.Fatal(err)
	}
	chainDeal(t, r, makerToken, maker, pubToken, publisher, "m")
	payoutID, _ := lastPayoutRecord(t, inboxMessages(t, r, pubToken))["payout"].(string)

	if err := db.SaveStepModes(ctx, ownerOfParty(maker), "", database.StepModes{Payout: "off"}); err != nil {
		t.Fatal(err)
	}
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp/payouts/"+payoutID+"/sent", map[string]any{"ref": chainTestTx}, bearerHeader(makerToken)); rec.Code != http.StatusForbidden {
		t.Fatalf("payout step off %d %s", rec.Code, rec.Body.String())
	}
	if err := db.SaveStepModes(ctx, ownerOfParty(maker), "", database.StepModes{Payout: "ask"}); err != nil {
		t.Fatal(err)
	}

	rec, doc := postJSON(t, r, http.MethodPost, "/orp/payouts/"+payoutID+"/sent", map[string]any{"ref": chainTestTx}, bearerHeader(makerToken))
	if rec.Code != http.StatusOK || doc["chain"].(map[string]any)["status"] != "mismatch" || doc["chain"].(map[string]any)["reason"] != "no_matching_transfer" {
		t.Fatalf("short transfer %d %s", rec.Code, rec.Body.String())
	}
	if n, err := SweepORPChainPayouts(ctx, db, entry.BaseURL, host.Chain, time.Now()); err != nil || n != 0 {
		t.Fatalf("sweep skips mismatch: %d %v", n, err)
	}

	// A ref that settled nothing does not reserve the transaction.
	chainDeal(t, r, makerToken, maker, pubToken, publisher, "m2")
	secondID, _ := lastPayoutRecord(t, inboxMessages(t, r, pubToken))["payout"].(string)
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp/payouts/"+secondID+"/sent", map[string]any{"ref": chainTestTx}, bearerHeader(makerToken)); rec.Code != http.StatusOK {
		t.Fatalf("unverified ref must not reserve the tx %d %s", rec.Code, rec.Body.String())
	}

	// The payer fixes the ref; the payee can still confirm by hand.
	other := "0x" + strings.Repeat("3", 64)
	rec, doc = postJSON(t, r, http.MethodPost, "/orp/payouts/"+payoutID+"/sent", map[string]any{"ref": other}, bearerHeader(makerToken))
	if rec.Code != http.StatusOK || doc["chain"].(map[string]any)["status"] != "pending" || doc["chain"].(map[string]any)["reason"] != "not_found" {
		t.Fatalf("corrected ref %d %s", rec.Code, rec.Body.String())
	}
	answer := orpEnvelope("payout", "m_chain_confirm_1", publisher, map[string]any{"payout": payoutID, "answer": "confirm"})
	first, _, err := db.GetORPPayout(ctx, payoutID)
	if err != nil {
		t.Fatal(err)
	}
	answer["deal"] = first.DealID
	rec, body := postJSON(t, r, http.MethodPost, "/orp", answer, bearerHeader(pubToken))
	if rec.Code != http.StatusOK || body["body"].(map[string]any)["state"] != "paid" {
		t.Fatalf("payee confirm %d %s", rec.Code, rec.Body.String())
	}
	if n, err := SweepORPChainPayouts(ctx, db, entry.BaseURL, host.Chain, time.Now()); err != nil || n != 0 {
		t.Fatalf("sweep after confirm: %d %v", n, err)
	}
	// The ref on a payout the payee confirmed cannot settle another one.
	if rec, _ := postJSON(t, r, http.MethodPost, "/orp/payouts/"+secondID+"/sent", map[string]any{"ref": other}, bearerHeader(makerToken)); rec.Code != http.StatusConflict {
		t.Fatalf("confirmed ref reused %d %s", rec.Code, rec.Body.String())
	}
}

func TestPayoutWalletsForm(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r := newStepModeRouter(db, entry.LiveClaimChecker{})
	owner, cookie := testOwnerSession(t, h, "wallet-owner", "wallet-owner@example.com")
	bad := ownerForm(t, h, r, cookie, "/owner/payout-wallets", url.Values{"csrf": {owner.CSRF}, "wallet_eip155:8453": {"not-an-address"}})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad address %d", bad.Code)
	}
	saved := ownerForm(t, h, r, cookie, "/owner/payout-wallets", url.Values{"csrf": {owner.CSRF}, "wallet_eip155:84532": {chainTestWallet}})
	if saved.Code != http.StatusSeeOther {
		t.Fatalf("save %d %s", saved.Code, saved.Body.String())
	}
	wallets, err := db.ORPPayoutWallets(context.Background(), owner.ID)
	if err != nil || wallets["eip155:84532"] != chainTestWallet || wallets["eip155:8453"] != "" {
		t.Fatalf("wallets %v %v", wallets, err)
	}
	req := httptest.NewRequest(http.MethodGet, "/owner/settings", nil)
	req.AddCookie(cookie)
	page := httptest.NewRecorder()
	r.ServeHTTP(page, req)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `id="payout-wallets"`) || !strings.Contains(page.Body.String(), chainTestWallet) {
		t.Fatalf("settings page %d", page.Code)
	}
	cleared := ownerForm(t, h, r, cookie, "/owner/payout-wallets", url.Values{"csrf": {owner.CSRF}})
	if cleared.Code != http.StatusSeeOther {
		t.Fatalf("clear %d", cleared.Code)
	}
	if wallets, _ := db.ORPPayoutWallets(context.Background(), owner.ID); len(wallets) != 0 {
		t.Fatalf("cleared wallets %v", wallets)
	}
}
