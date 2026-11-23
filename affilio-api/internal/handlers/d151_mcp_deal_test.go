package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestD151FreshTokenStepsAreAskAndSwapIsMCPOnly(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	server := httptest.NewUnstartedServer(r)
	base := "http://" + server.Listener.Addr().String()
	entry := &AgentEntry{
		DB: db, BaseURL: base, AddressKey: testAddressKey, ClaimsCheck: "off-local",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	host := &ORPHost{Entry: entry, FetchProof: func(context.Context, string) (string, int, bool) {
		return "<html><p>Shout-out swap</p></html>", 200, true
	}}
	entry.Mount(r)
	host.Mount(r)
	hosted := &HostedHandler{DB: db, BaseURL: base, LoopbackURL: base}
	r.Any("/mcp", hosted.MCP)
	server.Start()
	t.Cleanup(server.Close)

	mint := func() (token, party, owner string) {
		t.Helper()
		rec, body := postJSON(t, r, http.MethodPost, "/v1/agent-tokens", map[string]any{}, nil)
		token = assertJoinIssued(t, rec, body)
		party, _ = body["party"].(string)
		owner, _ = body["owner_id"].(string)
		if party == "" || owner == "" {
			t.Fatalf("token body %s", rec.Body.String())
		}
		return token, party, owner
	}
	tokenA, partyA, ownerA := mint()
	tokenB, partyB, _ := mint()
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE owner_id=$1`, ownerA) != 0 {
		t.Fatal("fresh token already had a step-mode row")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sessionA := connectMCP(t, ctx, server.URL+"/mcp", tokenA)
	t.Cleanup(func() { _ = sessionA.Close() })
	sessionB := connectMCP(t, ctx, server.URL+"/mcp", tokenB)
	t.Cleanup(func() { _ = sessionB.Close() })

	assertAskModes(t, ctx, sessionA)
	assertAskModes(t, ctx, sessionB)
	assertDealTools(t, ctx, sessionA)

	terms := map[string]any{
		"title": "TEST shout-out swap", "kind": "swap",
		"disclosure": map[string]any{"label": "Shout-out swap", "lang": "en"},
		"obligations": []any{
			map[string]any{"id": "place_a", "by": "offerer", "action": "place", "destination": "https://example.org/", "proof": []any{"proof_url"}},
			map[string]any{"id": "place_b", "by": "acceptor", "action": "place", "destination": "https://example.com/", "proof": []any{"proof_url"}},
		},
	}
	offered := callSessionTool(t, ctx, sessionA, "propose_deal", map[string]any{
		"party": partyA, "to": partyB, "terms": terms,
		"note": "TEST ONLY swap",
	})
	dealID, _ := offered["deal"].(string)
	offeredBody, _ := offered["body"].(map[string]any)
	termsHash, _ := offeredBody["terms_hash"].(string)
	if dealID == "" || offeredBody["state"] != "proposed" || termsHash == "" {
		t.Fatalf("offer %s", mustJSON(offered))
	}

	early, earlyText, earlyErr := callSessionToolRaw(t, ctx, sessionB, "approve_deal", map[string]any{"deal_id": dealID})
	if !earlyErr || early["deal"] != nil {
		t.Fatalf("premature approve_deal activated: err=%v %s %s", earlyErr, earlyText, mustJSON(early))
	}
	held := callSessionTool(t, ctx, sessionB, "check_deals", map[string]any{"deal_id": dealID})
	heldBody, _ := held["body"].(map[string]any)
	if heldBody["state"] != "proposed" {
		t.Fatalf("premature activate changed the deal %s", mustJSON(held))
	}

	accepted := callSessionTool(t, ctx, sessionB, "respond_to_offer", map[string]any{
		"party": partyB, "deal": dealID, "terms_hash": termsHash,
	})
	acceptedBody, _ := accepted["body"].(map[string]any)
	waiting, _ := acceptedBody["waiting"].(map[string]any)
	reasons := stringList(waiting["reasons"])
	if acceptedBody["state"] != "awaiting" || !hasAll(reasons, "step_ask") {
		t.Fatalf("accept did not hold in ask mode %s", mustJSON(accepted))
	}
	if acceptedBody["state"] == "active" {
		t.Fatal("accept activated the deal")
	}

	activated := callSessionTool(t, ctx, sessionB, "approve_deal", map[string]any{"deal_id": dealID})
	activatedBody, _ := activated["body"].(map[string]any)
	if activatedBody["state"] != "active" {
		t.Fatalf("approve_deal %s", mustJSON(activated))
	}
	if _, stillWaiting := activatedBody["waiting"]; stillWaiting {
		t.Fatalf("active deal still waiting %s", mustJSON(activated))
	}

	executed := callSessionTool(t, ctx, sessionA, "execute_obligation", map[string]any{
		"party": partyA, "deal": dealID, "obligation": "place_a", "proof_url": "https://example.org/swap-proof",
	})
	executedBody, _ := executed["body"].(map[string]any)
	rows, _ := executedBody["obligations"].([]any)
	var placeA map[string]any
	for _, item := range rows {
		row, _ := item.(map[string]any)
		if row["id"] == "place_a" {
			placeA = row
		}
	}
	if executedBody["state"] != "active" || placeA["method"] != "proof_url" || placeA["url"] != "https://example.org/swap-proof" || placeA["state"] == "pending" {
		t.Fatalf("execute_obligation %s", mustJSON(executed))
	}
}

func TestD151ApprovePageDoesNotContradictActivate(t *testing.T) {
	full := llmsFullDocument()
	for _, phrase := range []string{
		`POST /orp/approve/{escalation} does not approve a 0.3 deal`,
		`the message is "This page does not approve."`,
		`The 0.3 agent's approval is POST /orp/deals/{deal}/activate, or the MCP tool approve_deal`,
		`Activate before the accept is stored is HTTP 409 state_conflict`,
	} {
		if !strings.Contains(full, phrase) {
			t.Fatalf("llms-full missing %q", phrase)
		}
	}
	if strings.Contains(full, "By default the waiting party's agent approves an ask with its bearer on POST /orp/approve/{escalation}.") {
		t.Fatal("llms-full still tells a 0.3 agent to approve by POST /orp/approve")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{
		DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey, ClaimsCheck: "off-local",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	host := &ORPHost{Entry: entry}
	r := mountORP(entry, host)

	offerer := joinDefault(t, r)
	acceptor := joinDefault(t, r)
	for _, owner := range []string{offerer.owner, acceptor.owner} {
		modes, err := db.LoadStepModes(context.Background(), owner, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, step := range []string{
			database.StepSiteSignup, database.StepOfferMatch, database.StepPlacementPublish,
			database.StepLinkChange, database.StepPayout, database.StepShare, database.StepRevoke,
		} {
			if modes.Get(step) != database.ModeAsk {
				t.Fatalf("%s default %s", step, modes.Get(step))
			}
		}
	}

	terms := map[string]any{
		"title": "Ask-mode swap", "kind": "swap",
		"disclosure": map[string]any{"label": "Shout-out swap", "lang": "en"},
		"obligations": []any{
			map[string]any{"id": "place_a", "by": "offerer", "action": "place", "destination": "https://example.org/", "proof": []any{"proof_url"}},
			map[string]any{"id": "place_b", "by": "acceptor", "action": "place", "destination": "https://example.com/", "proof": []any{"proof_url"}},
		},
	}
	offer := orpEnvelope("offer", "m_d151_offer1", offerer.party, map[string]any{"terms": terms})
	offer["to"] = acceptor.party
	offRec, offBody := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(offerer.token))
	dealID, _ := offBody["deal"].(string)
	dealBody, _ := offBody["body"].(map[string]any)
	hash, _ := dealBody["terms_hash"].(string)
	if offRec.Code != http.StatusOK || dealBody["state"] != "proposed" || dealID == "" {
		t.Fatalf("offer %d %s", offRec.Code, offRec.Body.String())
	}
	early, earlyBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(acceptor.token))
	earlyInner, _ := earlyBody["body"].(map[string]any)
	if early.Code != http.StatusConflict || earlyInner["code"] != "state_conflict" || dealState(t, db, dealID) != "proposed" {
		t.Fatalf("premature activate %d %s", early.Code, early.Body.String())
	}

	accept := orpEnvelope("accept", "m_d151_accept", acceptor.party, map[string]any{"terms_hash": hash})
	accept["deal"] = dealID
	accRec, accBody := postJSON(t, r, http.MethodPost, "/orp", accept, bearerHeader(acceptor.token))
	accDeal, _ := accBody["body"].(map[string]any)
	waiting, _ := accDeal["waiting"].(map[string]any)
	esc, _ := waiting["escalation"].(string)
	if accRec.Code != http.StatusOK || accDeal["state"] != "awaiting" || esc == "" || !hasAll(stringList(waiting["reasons"]), "step_ask") {
		t.Fatalf("accept %d %s", accRec.Code, accRec.Body.String())
	}

	page, _ := postJSON(t, r, http.MethodGet, "/orp/approve/"+esc, nil, nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "does not approve") || !strings.Contains(page.Body.String(), "On a 0.3 deal that call is POST /orp/deals/{deal}/activate") {
		t.Fatalf("page %d %s", page.Code, page.Body.String())
	}
	denied, deniedBody := postJSON(t, r, http.MethodPost, "/orp/approve/"+esc, map[string]any{}, bearerHeader(acceptor.token))
	deniedInner, _ := deniedBody["body"].(map[string]any)
	if denied.Code != http.StatusForbidden || deniedInner["message"] != "This page does not approve." || dealState(t, db, dealID) != "awaiting" {
		t.Fatalf("approve post %d %s state %s", denied.Code, denied.Body.String(), dealState(t, db, dealID))
	}

	act, actBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", nil, bearerHeader(acceptor.token))
	actDeal, _ := actBody["body"].(map[string]any)
	if act.Code != http.StatusOK || actDeal["state"] != "active" || dealState(t, db, dealID) != "active" {
		t.Fatalf("activate after accept %d %s", act.Code, act.Body.String())
	}
}

func assertAskModes(t *testing.T, ctx context.Context, session *mcp.ClientSession) {
	t.Helper()
	got := callSessionTool(t, ctx, session, "get_step_modes", map[string]any{})
	modes, _ := got["modes"].(map[string]any)
	for _, step := range []string{"site_signup", "offer_match", "placement_publish", "link_change", "payout", "share", "revoke"} {
		if modes[step] != "ask" {
			t.Fatalf("get_step_modes %s: %s", step, mustJSON(got))
		}
	}
}

func assertDealTools(t *testing.T, ctx context.Context, session *mcp.ClientSession) {
	t.Helper()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]*mcp.Tool{}
	for _, tool := range list.Tools {
		if tool.Name == "approve_deal" || tool.Name == "execute_obligation" {
			found[tool.Name] = tool
		}
	}
	approve := found["approve_deal"]
	execute := found["execute_obligation"]
	if approve == nil || execute == nil {
		t.Fatalf("tools/list missing deal tools, have approve=%v execute=%v", approve != nil, execute != nil)
	}
	approveRaw, _ := json.Marshal(approve)
	executeRaw, _ := json.Marshal(execute)
	if !strings.Contains(approve.Description, "awaiting") || !strings.Contains(approve.Description, "POST /orp/approve/{escalation} does not approve a 0.3 deal") || !strings.Contains(string(approveRaw), "deal_id") {
		t.Fatalf("approve_deal schema %s", approveRaw)
	}
	if !strings.Contains(execute.Description, "proof_url") || !strings.Contains(execute.Description, "record_placement_receipt") || !strings.Contains(execute.Description, "does not take a placement_id") {
		t.Fatalf("execute_obligation description %s", execute.Description)
	}
	for _, phrase := range []string{"proof_url", "obligation", `"party"`, `"deal"`} {
		if !strings.Contains(string(executeRaw), phrase) {
			t.Fatalf("execute_obligation schema missing %s in %s", phrase, executeRaw)
		}
	}
}

func callSessionTool(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	out, text, isErr := callSessionToolRaw(t, ctx, session, name, args)
	if isErr {
		t.Fatalf("%s: %s", name, text)
	}
	return out
}

func callSessionToolRaw(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) (map[string]any, string, bool) {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Fatalf("%s empty", name)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	out := map[string]any{}
	_ = json.Unmarshal([]byte(text), &out)
	return out, text, result.IsError
}
