package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
)

type party04 struct {
	token string
	party string
	owner string
}

type orp04Env struct {
	t    *testing.T
	r    http.Handler
	n    int
	now  func() time.Time
	last string
}

func newORP04(t *testing.T) (*database.DB, *ORPHost, http.Handler, func()) {
	t.Helper()
	db, cleanup := setupTestDB(t)
	entry := &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		AddressKey:   testAddressKey,
		ClaimsCheck:  "stub",
		TokenLimiter: NewIPRateLimiter(100, time.Minute),
	}
	host := &ORPHost{Entry: entry, SuggestLimiter: NewIPRateLimiter(100, time.Minute)}
	return db, host, mountORP(entry, host), cleanup
}

func joinAuto(t *testing.T, db *database.DB, r http.Handler) party04 {
	t.Helper()
	token, party := joinParty(t, r)
	owner := ownerOfParty(party)
	setPlacementPublish(t, db, owner, "auto")
	return party04{token: token, party: party, owner: owner}
}

func (e *orp04Env) stamp() time.Time {
	if e.now != nil {
		return e.now().UTC()
	}
	return time.Now().UTC()
}

func (e *orp04Env) send(p party04, version, typ, deal string, body map[string]any) (int, map[string]any) {
	e.t.Helper()
	e.n++
	id := "m_d102_" + strings.Repeat("a", 2) + itoa(e.n)
	if len(id) < 8 {
		id = "m_d102_msg_" + itoa(e.n)
	}
	e.last = id
	env := map[string]any{
		"orp": version, "type": typ, "id": id, "ts": e.stamp().Format(time.RFC3339), "from": p.party, "body": body,
	}
	if deal != "" {
		env["deal"] = deal
	}
	rec, doc := postJSON(e.t, e.r, http.MethodPost, "/orp", env, bearerHeader(p.token))
	return rec.Code, doc
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func docBody(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	body, _ := doc["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body missing %v", doc)
	}
	return body
}

func acceptSeat(e *orp04Env, p party04, snap map[string]any) (int, map[string]any) {
	e.t.Helper()
	body := docBody(e.t, snap)
	return e.send(p, "0.4", "accept", snap["deal"].(string), map[string]any{"terms_hash": body["terms_hash"]})
}

func eur(n int) map[string]any {
	return map[string]any{"amount_minor": n, "currency": "EUR"}
}

func testIdentity() map[string]any {
	return map[string]any{
		"hash":   orp.ContentHash(map[string]any{"illustrative": "facilitator"}),
		"issuer": "example-issuer",
	}
}

func seatState(t *testing.T, doc map[string]any, id string) (signed, required bool, found bool) {
	t.Helper()
	body := docBody(t, doc)
	parties, _ := body["parties"].([]any)
	for _, item := range parties {
		seat, _ := item.(map[string]any)
		if seat["id"] != id {
			continue
		}
		signed, _ = seat["signed"].(bool)
		required, _ = seat["required"].(bool)
		return signed, required, true
	}
	return false, false, false
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return -1
	}
}

func storedParties(t *testing.T, db *database.DB, dealID string) map[string]any {
	t.Helper()
	var raw []byte
	if err := db.Pool.QueryRow(context.Background(), `SELECT parties FROM orp_deals WHERE id=$1`, dealID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func clickTracked(t *testing.T, r http.Handler, link string) *httptest.ResponseRecorder {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, parsed.Path, nil)
	req.Header.Set("User-Agent", "D102TestAgent/9")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func listedOfferIDs(items []any) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		row, _ := item.(map[string]any)
		offer, _ := row["offer"].(map[string]any)
		id, _ := offer["id"].(string)
		if id != "" {
			out[id] = true
		}
	}
	return out
}

func TestORP04ThreeAndFive(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	e := &orp04Env{t: t, r: r}
	creator := joinAuto(t, db, r)
	curator := joinAuto(t, db, r)
	newsletter := joinAuto(t, db, r)
	terms := map[string]any{
		"title": "Referral notes chain", "kind": "partnership",
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"obligations": []any{map[string]any{
			"id": "place", "by": newsletter.party, "action": "place",
			"content":     map[string]any{"link": true, "brief": "Mention the notes app."},
			"destination": "https://example.com/notes", "proof": []any{"link"},
		}},
	}
	code, opened := e.send(creator, "0.4", "offer", "", map[string]any{"terms": terms, "parties": []any{
		map[string]any{"id": creator.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 50}},
		map[string]any{"id": curator.party, "role": "curator", "required": true, "terms": map[string]any{"percent": 30}},
		map[string]any{"id": newsletter.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 20}},
	}})
	if code != http.StatusOK || opened["orp"] != "0.4" || docBody(t, opened)["state"] != "pending" {
		t.Fatalf("open %d %s", code, mustJSON(opened))
	}
	signed, _, ok := seatState(t, opened, creator.party)
	if !ok || !signed {
		t.Fatalf("offerer unsigned %s", mustJSON(opened))
	}
	code, mid := acceptSeat(e, curator, opened)
	if code != http.StatusOK || docBody(t, mid)["state"] != "pending" {
		t.Fatalf("mid %d %s", code, mustJSON(mid))
	}
	code, done := acceptSeat(e, newsletter, mid)
	body := docBody(t, done)
	if code != http.StatusOK || body["state"] != "active" {
		t.Fatalf("active %d %s", code, mustJSON(done))
	}
	if body["terms_hash"] != orp.DealHash(body["terms"].(map[string]any), seatsFromWire(t, body["parties"])) {
		t.Fatal("terms hash drifted")
	}
	tracking, _ := body["tracking"].([]any)
	if len(tracking) != 1 || tracking[0].(map[string]any)["status"] != "live" {
		t.Fatalf("tracking %s", mustJSON(done))
	}
	link, _ := tracking[0].(map[string]any)["link"].(string)
	click := clickTracked(t, r, link)
	if click.Code != http.StatusFound || !strings.Contains(click.Header().Get("Location"), "via=") {
		t.Fatalf("click %d %s", click.Code, click.Header().Get("Location"))
	}
	dealID, _ := done["deal"].(string)
	stored := storedParties(t, db, dealID)
	rawStored := mustJSON(stored)
	if strings.Contains(rawStored, "@") || strings.Contains(rawStored, "D102TestAgent/9") {
		t.Fatalf("pii stored %s", rawStored)
	}
	clicks, _ := stored["clicks"].([]any)
	if len(clicks) != 1 || clicks[0].(map[string]any)["class"] != "human" {
		t.Fatalf("clicks %s", rawStored)
	}
	shares, _ := clicks[0].(map[string]any)["shares"].([]any)
	got := map[string]float64{}
	for _, item := range shares {
		row := item.(map[string]any)
		got[row["party"].(string)] = asFloat(row["percent"])
	}
	if got[creator.party] != 50 || got[curator.party] != 30 || got[newsletter.party] != 20 || len(got) != 3 {
		t.Fatalf("shares %#v", got)
	}
	via := click.Header().Get("Location")
	if !strings.Contains(via, clicks[0].(map[string]any)["id"].(string)) {
		t.Fatalf("redirect id %s stored %v", via, clicks[0])
	}
	proof, _ := postJSON(t, r, http.MethodGet, "/orp/proof/"+dealID, nil, nil)
	if proof.Code != http.StatusOK || !strings.Contains(proof.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("proof %d %s", proof.Code, proof.Header().Get("Content-Type"))
	}
	act, actBody := postJSON(t, r, http.MethodPost, "/orp/deals/"+dealID+"/activate", map[string]any{}, bearerHeader(newsletter.token))
	if act.Code != http.StatusConflict || actBody["body"].(map[string]any)["code"] != "state_conflict" {
		t.Fatalf("activate %d %s", act.Code, act.Body.String())
	}

	seller := joinAuto(t, db, r)
	buyer := joinAuto(t, db, r)
	mediator := joinAuto(t, db, r)
	door := joinAuto(t, db, r)
	notary := joinAuto(t, db, r)
	replacement := joinAuto(t, db, r)
	fee := eur(100000)
	homeTerms := map[string]any{
		"title": "Illustrative five-seat deal, test mode only", "kind": "partnership",
		"obligations": []any{map[string]any{"id": "share", "by": "signer", "action": "share"}},
	}
	homeParties := []any{
		map[string]any{"id": seller.party, "role": "offerer", "required": true, "terms": map[string]any{"amount": eur(0)}},
		map[string]any{"id": buyer.party, "role": "acceptor", "required": true, "terms": map[string]any{"amount": fee}},
		map[string]any{"id": mediator.party, "role": "facilitator", "required": true, "identity": testIdentity(), "terms": map[string]any{"amount": fee}},
		map[string]any{"id": door.party, "role": "service", "required": false, "terms": map[string]any{
			"amount": eur(4000), "per": "hour", "hours": 2,
			"deposit": map[string]any{"amount": eur(2000), "holder": "viewing-desk", "purpose": "viewing"},
		}},
		map[string]any{"id": notary.party, "role": "signer", "required": true},
	}
	code, home := e.send(seller, "0.4", "offer", "", map[string]any{"terms": homeTerms, "parties": homeParties})
	if code != http.StatusOK || docBody(t, home)["state"] != "pending" || strings.Contains(mustJSON(home), "@") {
		t.Fatalf("home %d %s", code, mustJSON(home))
	}
	code, home = acceptSeat(e, buyer, home)
	if code != http.StatusOK {
		t.Fatalf("buyer %d %s", code, mustJSON(home))
	}
	code, home = acceptSeat(e, mediator, home)
	if code != http.StatusOK {
		t.Fatalf("mediator %d %s", code, mustJSON(home))
	}
	code, home = acceptSeat(e, notary, home)
	if code != http.StatusOK || docBody(t, home)["state"] != "active" {
		t.Fatalf("notary %d %s", code, mustJSON(home))
	}
	doorSigned, doorRequired, found := seatState(t, home, door.party)
	if !found || doorSigned || doorRequired {
		t.Fatalf("door seat %v %v %v", doorSigned, doorRequired, found)
	}
	homeID, _ := home["deal"].(string)
	homeLink, _ := docBody(t, home)["tracking"].([]any)[0].(map[string]any)["link"].(string)
	homeClick := clickTracked(t, r, homeLink)
	if homeClick.Code != http.StatusFound {
		t.Fatalf("home click %d", homeClick.Code)
	}
	homeStored := storedParties(t, db, homeID)
	homeClicks, _ := homeStored["clicks"].([]any)
	homeShares, _ := homeClicks[0].(map[string]any)["shares"].([]any)
	amounts := map[string]float64{}
	for _, item := range homeShares {
		row := item.(map[string]any)
		amounts[row["party"].(string)] = asFloat(row["amount_minor"])
		if _, extra := row["percent"]; extra {
			t.Fatalf("percent on amount share %v", row)
		}
	}
	if amounts[seller.party] != 0 || amounts[buyer.party] != 100000 || amounts[mediator.party] != 100000 || amounts[door.party] != 4000 || amounts[notary.party] != 0 && false {
		t.Fatalf("home shares %#v", amounts)
	}
	if _, present := amounts[notary.party]; present || len(amounts) != 4 {
		t.Fatalf("notary included %#v", amounts)
	}
	if strings.Contains(mustJSON(homeStored), "D102TestAgent/9") || strings.Contains(mustJSON(homeStored), "@") {
		t.Fatal("home pii")
	}
	code, doorLeft := e.send(door, "0.4", "leave", homeID, map[string]any{})
	if code != http.StatusOK || docBody(t, doorLeft)["state"] != "active" || len(docBody(t, doorLeft)["parties"].([]any)) != 4 {
		t.Fatalf("door leave %d %s", code, mustJSON(doorLeft))
	}
	code, mediatorLeft := e.send(mediator, "0.4", "leave", homeID, map[string]any{})
	leftBody := docBody(t, mediatorLeft)
	if code != http.StatusOK || leftBody["state"] != "pending" {
		t.Fatalf("mediator leave %d %s", code, mustJSON(mediatorLeft))
	}
	vacancies, _ := leftBody["vacancies"].([]any)
	if len(vacancies) != 1 || vacancies[0] != "facilitator" {
		t.Fatalf("vacancy %v", vacancies)
	}
	code, seated := e.send(seller, "0.4", "seat", homeID, map[string]any{"seat": map[string]any{
		"id": replacement.party, "role": "facilitator", "required": true, "identity": testIdentity(), "terms": map[string]any{"amount": fee},
	}})
	if code != http.StatusOK || docBody(t, seated)["state"] != "pending" {
		t.Fatalf("seat %d %s", code, mustJSON(seated))
	}
	if vac, _ := docBody(t, seated)["vacancies"].([]any); len(vac) != 1 {
		t.Fatalf("seat cleared early %s", mustJSON(seated))
	}
	signed, _, _ = seatState(t, seated, replacement.party)
	if signed {
		t.Fatal("replacement signed by seat")
	}
	code, replaced := acceptSeat(e, replacement, seated)
	if code != http.StatusOK || docBody(t, replaced)["state"] != "active" || docBody(t, replaced)["vacancies"] != nil {
		t.Fatalf("replaced %d %s", code, mustJSON(replaced))
	}
	homeProof, _ := postJSON(t, r, http.MethodGet, "/orp/proof/"+homeID, nil, nil)
	if homeProof.Code != http.StatusOK {
		t.Fatalf("home proof %d", homeProof.Code)
	}
	mixedParties := make([]any, len(homeParties))
	copy(mixedParties, homeParties)
	mixedParties[2] = map[string]any{
		"id": mediator.party, "role": "facilitator", "required": true, "identity": testIdentity(),
		"terms": map[string]any{"percent": 1, "cap": fee},
	}
	code, mixed := e.send(seller, "0.4", "offer", "", map[string]any{"terms": homeTerms, "parties": mixedParties})
	if code != http.StatusOK || docBody(t, mixed)["state"] != "pending" {
		t.Fatalf("mixed %d %s", code, mustJSON(mixed))
	}
}

func seatsFromWire(t *testing.T, raw any) []orp.Seat {
	t.Helper()
	items, _ := raw.([]any)
	out := make([]orp.Seat, 0, len(items))
	encoded, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(encoded, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestORP04BadSplitAndHolder(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	e := &orp04Env{t: t, r: r}
	maker := joinAuto(t, db, r)
	other := joinAuto(t, db, r)
	terms := map[string]any{
		"title": "Bad notes split", "kind": "partnership",
		"obligations": []any{map[string]any{"id": "share", "by": "offerer", "action": "share"}},
	}
	beforeDeals := countRows(t, db, `SELECT COUNT(*) FROM orp_deals`)
	beforeInbox := countRows(t, db, `SELECT COUNT(*) FROM orp_inbox`)
	reject := func(parties []any, needle string) {
		t.Helper()
		code, doc := e.send(maker, "0.4", "offer", "", map[string]any{"terms": terms, "parties": parties})
		body := docBody(t, doc)
		if code != http.StatusBadRequest || body["code"] != "invalid_message" || !strings.Contains(body["message"].(string), needle) {
			t.Fatalf("reject %d %s", code, mustJSON(doc))
		}
		if countRows(t, db, `SELECT COUNT(*) FROM orp_deals`) != beforeDeals || countRows(t, db, `SELECT COUNT(*) FROM orp_inbox`) != beforeInbox {
			t.Fatal("bad offer wrote a deal or an inbox row")
		}
	}
	reject([]any{
		map[string]any{"id": maker.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 60}},
		map[string]any{"id": other.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 41}},
	}, "100")
	reject([]any{
		map[string]any{"id": maker.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 40, "amount": eur(500)}},
		map[string]any{"id": other.party, "role": "acceptor", "required": true, "terms": map[string]any{"amount": eur(500)}},
	}, "cap")
	reject([]any{
		map[string]any{"id": maker.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 40, "hours": 2, "cap": eur(1000)}},
		map[string]any{"id": other.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 40}},
	}, "100")
	for _, holder := range []string{"OfferRails", "offer-rails", "offer_rails", "offer rails", "Offer.Rails", "hosted api", "HOST"} {
		reject([]any{
			map[string]any{"id": maker.party, "role": "offerer", "required": true, "terms": map[string]any{"amount": eur(0)}},
			map[string]any{"id": other.party, "role": "service", "required": true, "terms": map[string]any{"deposit": map[string]any{"amount": eur(2000), "holder": holder, "purpose": "viewing"}}},
		}, "does not hold money")
	}
	reject([]any{
		map[string]any{"id": maker.party, "role": "offerer", "required": true},
		map[string]any{"id": other.party, "role": "service", "required": true, "terms": map[string]any{"deposit": map[string]any{"amount": eur(2000), "holder": orp.HostParty("127.0.0.1:8080"), "purpose": "viewing"}}},
	}, "does not hold money")
	code, pii := e.send(maker, "0.4", "offer", "", map[string]any{"terms": terms, "parties": []any{
		map[string]any{"id": maker.party, "role": "offerer", "required": true},
		map[string]any{"id": other.party, "role": "facilitator", "required": true, "identity": map[string]any{"hash": testIdentity()["hash"], "issuer": "example-issuer", "email": "person@example.com"}},
	}})
	if code != http.StatusBadRequest || docBody(t, pii)["code"] != "invalid_message" {
		t.Fatalf("pii %d %s", code, mustJSON(pii))
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_deals`) != beforeDeals {
		t.Fatal("pii wrote a deal")
	}
	code, hours := e.send(maker, "0.4", "offer", "", map[string]any{"terms": terms, "parties": []any{
		map[string]any{"id": maker.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 50, "hours": 3}},
		map[string]any{"id": other.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 50, "per": "hour", "deposit": map[string]any{"amount": eur(2000), "holder": "Viewing Desk", "purpose": "viewing"}}},
	}})
	if code != http.StatusOK {
		t.Fatalf("hours %d %s", code, mustJSON(hours))
	}
	code, mixed := e.send(maker, "0.4", "offer", "", map[string]any{"terms": terms, "parties": []any{
		map[string]any{"id": maker.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 40, "amount": eur(500), "cap": eur(1000)}},
		map[string]any{"id": other.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 40, "amount": eur(500), "cap": eur(1000)}},
	}})
	if code != http.StatusOK {
		t.Fatalf("mixed %d %s", code, mustJSON(mixed))
	}
}

func TestORP04Vacancy(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	e := &orp04Env{t: t, r: r}
	offerer := joinAuto(t, db, r)
	acceptor := joinAuto(t, db, r)
	required := joinAuto(t, db, r)
	optional := joinAuto(t, db, r)
	replacement := joinAuto(t, db, r)
	original := joinAuto(t, db, r)
	fee := eur(1000)
	share := map[string]any{
		"title": "Optional facilitator cannot clear a vacancy", "kind": "partnership",
		"obligations": []any{map[string]any{"id": "share", "by": "offerer", "action": "share"}},
	}
	code, opened := e.send(offerer, "0.4", "offer", "", map[string]any{"terms": share, "parties": []any{
		map[string]any{"id": offerer.party, "role": "offerer", "required": true, "terms": map[string]any{"amount": eur(0)}},
		map[string]any{"id": acceptor.party, "role": "acceptor", "required": true, "terms": map[string]any{"amount": fee}},
		map[string]any{"id": required.party, "role": "facilitator", "required": true, "terms": map[string]any{"amount": fee}},
		map[string]any{"id": optional.party, "role": "facilitator", "required": false, "terms": map[string]any{"amount": eur(1)}},
	}})
	if code != http.StatusOK {
		t.Fatalf("open %d %s", code, mustJSON(opened))
	}
	code, opened = acceptSeat(e, acceptor, opened)
	code, active := acceptSeat(e, required, opened)
	if code != http.StatusOK || docBody(t, active)["state"] != "active" {
		t.Fatalf("active %d %s", code, mustJSON(active))
	}
	dealID, _ := active["deal"].(string)
	code, left := e.send(required, "0.4", "leave", dealID, map[string]any{})
	if code != http.StatusOK || docBody(t, left)["state"] != "pending" {
		t.Fatalf("leave %d %s", code, mustJSON(left))
	}
	code, optionalSigned := acceptSeat(e, optional, left)
	if code != http.StatusOK || docBody(t, optionalSigned)["state"] != "pending" {
		t.Fatalf("optional %d %s", code, mustJSON(optionalSigned))
	}
	if vac, _ := docBody(t, optionalSigned)["vacancies"].([]any); len(vac) != 1 || vac[0] != "facilitator" {
		t.Fatalf("optional cleared %s", mustJSON(optionalSigned))
	}
	code, seated := e.send(offerer, "0.4", "seat", dealID, map[string]any{"seat": map[string]any{
		"id": replacement.party, "role": "facilitator", "required": true, "terms": map[string]any{"amount": fee},
	}})
	if code != http.StatusOK || docBody(t, seated)["vacancies"] == nil {
		t.Fatalf("seat %d %s", code, mustJSON(seated))
	}
	code, replaced := acceptSeat(e, replacement, seated)
	if code != http.StatusOK || docBody(t, replaced)["state"] != "active" || docBody(t, replaced)["vacancies"] != nil {
		t.Fatalf("replaced %d %s", code, mustJSON(replaced))
	}

	code, opened = e.send(offerer, "0.4", "offer", "", map[string]any{
		"terms": map[string]any{
			"title": "Original seat does not fill a vacancy", "kind": "partnership",
			"obligations": []any{map[string]any{"id": "share", "by": "offerer", "action": "share"}},
		},
		"parties": []any{
			map[string]any{"id": offerer.party, "role": "offerer", "required": true, "terms": map[string]any{"amount": eur(0)}},
			map[string]any{"id": acceptor.party, "role": "acceptor", "required": true, "terms": map[string]any{"amount": fee}},
			map[string]any{"id": required.party, "role": "facilitator", "required": true, "terms": map[string]any{"amount": fee}},
			map[string]any{"id": original.party, "role": "facilitator", "required": true, "terms": map[string]any{"amount": fee}},
		},
	})
	if code != http.StatusOK || docBody(t, opened)["state"] != "pending" {
		t.Fatalf("original open %d %s", code, mustJSON(opened))
	}
	dealID, _ = opened["deal"].(string)
	code, left = e.send(required, "0.4", "leave", dealID, map[string]any{})
	if code != http.StatusOK || docBody(t, left)["state"] != "pending" {
		t.Fatalf("original leave %d %s", code, mustJSON(left))
	}
	code, signed := acceptSeat(e, original, left)
	if code != http.StatusOK || docBody(t, signed)["state"] != "pending" {
		t.Fatalf("original sign %d %s", code, mustJSON(signed))
	}
	if vac, _ := docBody(t, signed)["vacancies"].([]any); len(vac) != 1 {
		t.Fatalf("original cleared %s", mustJSON(signed))
	}

	code, split := e.send(offerer, "0.4", "offer", "", map[string]any{
		"terms": map[string]any{
			"title": "Percent split must survive a leave", "kind": "partnership",
			"obligations": []any{map[string]any{"id": "share", "by": "offerer", "action": "share"}},
		},
		"parties": []any{
			map[string]any{"id": offerer.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 50}},
			map[string]any{"id": acceptor.party, "role": "curator", "required": true, "terms": map[string]any{"percent": 30}},
			map[string]any{"id": optional.party, "role": "service", "required": false, "terms": map[string]any{"percent": 20}},
		},
	})
	if code != http.StatusOK {
		t.Fatalf("split open %d %s", code, mustJSON(split))
	}
	code, split = acceptSeat(e, acceptor, split)
	if code != http.StatusOK || docBody(t, split)["state"] != "active" {
		t.Fatalf("split active %d %s", code, mustJSON(split))
	}
	dealID, _ = split["deal"].(string)
	code, denied := e.send(optional, "0.4", "leave", dealID, map[string]any{})
	if code != http.StatusBadRequest || !strings.Contains(docBody(t, denied)["message"].(string), "100") {
		t.Fatalf("split leave %d %s", code, mustJSON(denied))
	}
	read, readBody := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(offerer.token))
	if read.Code != http.StatusOK || readBody["body"].(map[string]any)["state"] != "active" || len(readBody["body"].(map[string]any)["parties"].([]any)) != 3 {
		t.Fatalf("unchanged %d %s", read.Code, read.Body.String())
	}
}

func TestORP04ExpiredOffer(t *testing.T) {
	db, host, r, cleanup := newORP04(t)
	defer cleanup()
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	host.Now = func() time.Time { return now }
	e := &orp04Env{t: t, r: r, now: func() time.Time { return now }}
	maker := joinAuto(t, db, r)
	reader := joinAuto(t, db, r)
	classicTerms := testTerms()
	classicTerms["title"] = "Notes classic window"
	classicEnv := orpEnvelope("offer", "m_d102_classic", maker.party, map[string]any{"terms": classicTerms})
	classicEnv["ts"] = now.Format(time.RFC3339)
	classicRec, classicDoc := postJSON(t, r, http.MethodPost, "/orp", classicEnv, bearerHeader(maker.token))
	if classicRec.Code != http.StatusOK || classicDoc["orp"] != "0.3" {
		t.Fatalf("classic %d %s", classicRec.Code, classicRec.Body.String())
	}
	classicID, _ := classicDoc["body"].(map[string]any)["id"].(string)
	terms := map[string]any{
		"title": "Notes freshness window", "kind": "affiliate",
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"claims":     map[string]any{"commission": 30, "destination": "https://example.com/notes"},
		"obligations": []any{map[string]any{
			"id": "place", "by": "acceptor", "action": "place",
			"content":     map[string]any{"link": true, "brief": "Mention the notes app."},
			"destination": "https://example.com/notes", "proof": []any{"link"},
		}},
	}
	code, listed := e.send(maker, "0.4", "offer", "", map[string]any{"terms": terms})
	if code != http.StatusOK || listed["orp"] != "0.4" {
		t.Fatalf("listed %d %s", code, mustJSON(listed))
	}
	offer := docBody(t, listed)
	storedTerms := offer["terms"].(map[string]any)
	claims := storedTerms["claims"].(map[string]any)
	if claims["commission"] != "unknown" || claims["destination"] != "unknown" {
		t.Fatalf("claims %#v", claims)
	}
	if _, ok := claims["audience"]; ok {
		t.Fatal("invented audience")
	}
	checked, err := time.Parse(time.RFC3339, storedTerms["checked_at"].(string))
	expires, err2 := time.Parse(time.RFC3339, storedTerms["expires_at"].(string))
	if err != nil || err2 != nil || expires.Sub(checked) != 90*24*time.Hour {
		t.Fatalf("window %v %v %s %s", err, err2, storedTerms["checked_at"], storedTerms["expires_at"])
	}
	offerID, _ := offer["id"].(string)
	visible := func() (discover, suggest bool) {
		t.Helper()
		code, found := e.send(reader, "0.4", "discover", "", map[string]any{"want": "offers", "query": "notes", "limit": 5})
		if code != http.StatusOK {
			t.Fatalf("discover %d %s", code, mustJSON(found))
		}
		ids := listedOfferIDs(docBody(t, found)["items"].([]any))
		rec, suggested := postJSON(t, r, http.MethodPost, "/orp/suggest", map[string]any{"text": "notes for founders", "limit": 5}, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("suggest %d %s", rec.Code, rec.Body.String())
		}
		return ids[offerID], listedOfferIDs(suggested["items"].([]any))[offerID]
	}
	inDiscover, inSuggest := visible()
	if !inDiscover || !inSuggest || !listedOfferIDs(func() []any {
		_, found := e.send(reader, "0.4", "discover", "", map[string]any{"want": "offers", "query": "notes", "limit": 5})
		return docBody(t, found)["items"].([]any)
	}())[classicID] {
		t.Fatalf("fresh discover=%v suggest=%v", inDiscover, inSuggest)
	}
	now = expires
	inDiscover, inSuggest = visible()
	_, found := e.send(reader, "0.4", "discover", "", map[string]any{"want": "offers", "query": "notes", "limit": 5})
	if inDiscover || inSuggest || !listedOfferIDs(docBody(t, found)["items"].([]any))[classicID] {
		t.Fatalf("expired discover=%v suggest=%v classic %v", inDiscover, inSuggest, listedOfferIDs(docBody(t, found)["items"].([]any)))
	}
	code, confirmed := e.send(maker, "0.4", "offer", "", map[string]any{"id": offerID, "confirm": true})
	confirmedTerms := docBody(t, confirmed)["terms"].(map[string]any)
	if code != http.StatusOK || confirmedTerms["claims"].(map[string]any)["commission"] != "unknown" {
		t.Fatalf("confirm %d %s", code, mustJSON(confirmed))
	}
	checked, _ = time.Parse(time.RFC3339, confirmedTerms["checked_at"].(string))
	expires, _ = time.Parse(time.RFC3339, confirmedTerms["expires_at"].(string))
	if expires.Sub(checked) != 90*24*time.Hour {
		t.Fatal("confirm window")
	}
	inDiscover, inSuggest = visible()
	if !inDiscover || !inSuggest {
		t.Fatalf("restored discover=%v suggest=%v", inDiscover, inSuggest)
	}
}

func TestORP04ApproveAtomic(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	e := &orp04Env{t: t, r: r}
	maker := joinAuto(t, db, r)
	acceptor := joinAuto(t, db, r)
	terms := map[string]any{
		"title": "Atomic notes placement", "kind": "partnership",
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"obligations": []any{map[string]any{
			"id": "place", "by": acceptor.party, "action": "place",
			"content": map[string]any{"link": true}, "destination": "https://example.com/notes", "proof": []any{"link"},
		}},
	}
	code, opened := e.send(maker, "0.4", "offer", "", map[string]any{"terms": terms, "parties": []any{
		map[string]any{"id": maker.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 50}},
		map[string]any{"id": acceptor.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 50}},
	}})
	if code != http.StatusOK || docBody(t, opened)["state"] != "pending" {
		t.Fatalf("open %d %s", code, mustJSON(opened))
	}
	dealID, _ := opened["deal"].(string)
	inboxBefore := countRows(t, db, `SELECT COUNT(*) FROM orp_inbox`)
	db.ORPBeforeCommit = func(context.Context) error { return errors.New("boom") }
	code, failed := acceptSeat(e, acceptor, opened)
	if code != http.StatusInternalServerError {
		t.Fatalf("fail %d %s", code, mustJSON(failed))
	}
	db.ORPBeforeCommit = nil
	if countRows(t, db, `SELECT COUNT(*) FROM orp_inbox`) != inboxBefore {
		t.Fatal("failed accept wrote inbox")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_messages WHERE message_id=$1`, e.last) != 0 {
		t.Fatal("failed accept stored the message")
	}
	var state string
	var parties []byte
	if err := db.Pool.QueryRow(context.Background(), `SELECT state, parties FROM orp_deals WHERE id=$1`, dealID).Scan(&state, &parties); err != nil {
		t.Fatal(err)
	}
	if state != "pending" || strings.Contains(string(parties), `"token"`) || jsonSigned(parties, acceptor.party) {
		t.Fatalf("residue state=%s %s", state, parties)
	}

	asker := joinPartyOnly(t, r)
	code, waiting := e.send(maker, "0.4", "offer", "", map[string]any{"terms": terms, "parties": []any{
		map[string]any{"id": maker.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 60}},
		map[string]any{"id": asker.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 40}},
	}})
	if code != http.StatusOK {
		t.Fatalf("ask offer %d %s", code, mustJSON(waiting))
	}
	askID, _ := waiting["deal"].(string)
	code, held := acceptSeat(e, asker, waiting)
	heldBody := docBody(t, held)
	if code != http.StatusOK || heldBody["state"] != "awaiting" {
		t.Fatalf("held %d %s", code, mustJSON(held))
	}
	waitingOn, _ := heldBody["waiting"].(map[string]any)
	esc, _ := waitingOn["escalation"].(string)
	getRec, _ := postJSON(t, r, http.MethodGet, "/orp/approve/"+esc, nil, nil)
	if getRec.Code != http.StatusOK || !strings.Contains(getRec.Body.String(), "does not approve") {
		t.Fatalf("get approve %d %s", getRec.Code, getRec.Body.String())
	}
	var afterGet string
	_ = db.Pool.QueryRow(context.Background(), `SELECT state FROM orp_deals WHERE id=$1`, askID).Scan(&afterGet)
	if afterGet != "awaiting" {
		t.Fatalf("get approved %s", afterGet)
	}
	inboxBefore = countRows(t, db, `SELECT COUNT(*) FROM orp_inbox`)
	db.ORPBeforeCommit = func(context.Context) error { return errors.New("boom") }
	postRec, _ := postJSON(t, r, http.MethodPost, "/orp/approve/"+esc, map[string]any{}, bearerHeader(asker.token))
	db.ORPBeforeCommit = nil
	if postRec.Code != http.StatusInternalServerError {
		t.Fatalf("post approve %d %s", postRec.Code, postRec.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_inbox`) != inboxBefore {
		t.Fatal("failed approve wrote inbox")
	}
	var askState string
	var askParties []byte
	if err := db.Pool.QueryRow(context.Background(), `SELECT state, parties FROM orp_deals WHERE id=$1`, askID).Scan(&askState, &askParties); err != nil {
		t.Fatal(err)
	}
	if askState != "awaiting" || strings.Contains(string(askParties), `"token"`) || jsonSigned(askParties, asker.party) {
		t.Fatalf("approve residue %s %s", askState, askParties)
	}

	other := joinAuto(t, db, r)
	directed := orpEnvelope("offer", "m_d102_directed", maker.party, map[string]any{"terms": testTerms()})
	directed["to"] = other.party
	dirRec, dirBody := postJSON(t, r, http.MethodPost, "/orp", directed, bearerHeader(maker.token))
	if dirRec.Code != http.StatusOK || dirBody["orp"] != "0.3" {
		t.Fatalf("directed %d %s", dirRec.Code, dirRec.Body.String())
	}
	dirDeal, _ := dirBody["deal"].(string)
	if err := db.Pool.QueryRow(context.Background(), `SELECT waiting->>'escalation' FROM orp_deals WHERE id=$1`, dirDeal).Scan(&esc); err != nil || esc == "" {
		t.Fatalf("escalation %v %s", err, dirRec.Body.String())
	}
	inboxBefore = countRows(t, db, `SELECT COUNT(*) FROM orp_inbox`)
	oldRec, oldBody := postJSON(t, r, http.MethodPost, "/orp/approve/"+esc, map[string]any{}, bearerHeader(other.token))
	if oldRec.Code != http.StatusForbidden || !strings.Contains(oldBody["body"].(map[string]any)["message"].(string), "This page does not approve.") {
		t.Fatalf("0.3 approve %d %s", oldRec.Code, oldRec.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_inbox`) != inboxBefore {
		t.Fatal("0.3 approve wrote inbox")
	}
	var oldState string
	var partiesNull *[]byte
	if err := db.Pool.QueryRow(context.Background(), `SELECT state, parties FROM orp_deals WHERE id=$1`, dirDeal).Scan(&oldState, &partiesNull); err != nil {
		t.Fatal(err)
	}
	if oldState != "proposed" || partiesNull != nil {
		t.Fatalf("0.3 residue %s %v", oldState, partiesNull)
	}
}

func jsonSigned(raw []byte, party string) bool {
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		return false
	}
	seats, _ := doc["seats"].([]any)
	for _, item := range seats {
		seat, _ := item.(map[string]any)
		if seat["id"] == party {
			return seat["signed"] == true
		}
	}
	return false
}

func joinPartyOnly(t *testing.T, r http.Handler) party04 {
	t.Helper()
	token, party := joinParty(t, r)
	return party04{token: token, party: party, owner: ownerOfParty(party)}
}

func TestORP04Capabilities(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	e := &orp04Env{t: t, r: r}
	rec, doc := postJSON(t, r, http.MethodGet, "/.well-known/orp.json", nil, nil)
	if rec.Code != http.StatusOK || doc["name"] != "In testing" {
		t.Fatalf("discovery %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := doc["conformance"]; ok {
		t.Fatal("conformance claimed")
	}
	versions, _ := doc["orp"].([]any)
	if len(versions) != 2 || versions[0] != "0.4" || versions[1] != "0.3" {
		t.Fatalf("orp %v", versions)
	}
	desc, _ := doc["description"].(string)
	if !strings.Contains(desc, "Many-party, leave, seat.") || len(desc) > 500 || !strings.Contains(desc, "In testing") {
		t.Fatalf("description %d %s", len(desc), desc)
	}
	caps := doc["capabilities"].(map[string]any)
	if caps["checks"].(map[string]any)["claims"] != "stub" {
		t.Fatalf("claims %v", caps["checks"])
	}
	if !messagesHave(caps["messages"].([]any), "leave", "seat") {
		t.Fatalf("messages %v", caps["messages"])
	}
	party := joinAuto(t, db, r)
	code, hello := e.send(party, "0.3", "hello", "", map[string]any{"versions": []any{"0.4", "0.3"}})
	helloBody := docBody(t, hello)
	helloVersions, _ := helloBody["versions"].([]any)
	if code != http.StatusOK || hello["orp"] != "0.3" || len(helloVersions) != 1 || helloVersions[0] != "0.3" {
		t.Fatalf("0.3 hello %d %s", code, mustJSON(hello))
	}
	if messagesHave(helloBody["capabilities"].(map[string]any)["messages"].([]any), "leave") || messagesHave(helloBody["capabilities"].(map[string]any)["messages"].([]any), "seat") {
		t.Fatalf("0.3 hello listed leave %s", mustJSON(hello))
	}
	code, hello = e.send(party, "0.4", "hello", "", map[string]any{"versions": []any{"0.4"}})
	helloBody = docBody(t, hello)
	helloVersions, _ = helloBody["versions"].([]any)
	if code != http.StatusOK || hello["orp"] != "0.4" || len(helloVersions) != 1 || helloVersions[0] != "0.4" {
		t.Fatalf("0.4 hello %d %s", code, mustJSON(hello))
	}
	if !messagesHave(helloBody["capabilities"].(map[string]any)["messages"].([]any), "leave", "seat") {
		t.Fatalf("0.4 hello messages %s", mustJSON(hello))
	}
}

func messagesHave(items []any, want ...string) bool {
	have := map[string]bool{}
	for _, item := range items {
		text, _ := item.(string)
		have[text] = true
	}
	for _, item := range want {
		if !have[item] {
			return false
		}
	}
	return true
}

func shareTerms04() map[string]any {
	return map[string]any{
		"title": "Ask mode notes", "kind": "partnership",
		"obligations": []any{map[string]any{"id": "share", "by": "offerer", "action": "share"}},
	}
}

func percentParties(parties []party04) []any {
	out := make([]any, len(parties))
	for i, party := range parties {
		role := "member"
		if i == 0 {
			role = "offerer"
		} else if i == 1 {
			role = "acceptor"
		}
		percent := 1
		if i == 0 {
			percent = 100 - (len(parties) - 1)
		}
		out[i] = map[string]any{
			"id": party.party, "role": role, "required": true,
			"terms": map[string]any{"percent": percent},
		}
	}
	return out
}

func joinDefault(t *testing.T, r http.Handler) party04 {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("join %d %s", rec.Code, rec.Body.String())
	}
	steps, _ := body["steps"].(map[string]any)
	if steps["list"] != "ask" {
		t.Fatalf("fresh join list mode %s", rec.Body.String())
	}
	token, _ := body["token"].(string)
	party, _ := body["party"].(string)
	return party04{token: token, party: party, owner: ownerOfParty(party)}
}

func storedBound(t *testing.T, db *database.DB, dealID string) string {
	t.Helper()
	var raw string
	if err := db.Pool.QueryRow(context.Background(), `SELECT bound::text FROM orp_deals WHERE id=$1`, dealID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertAskWait(t *testing.T, db *database.DB, code int, opened map[string]any, offerer string) {
	t.Helper()
	if code != http.StatusOK || opened["orp"] != "0.4" {
		t.Fatalf("ask offer %d %s", code, mustJSON(opened))
	}
	body := docBody(t, opened)
	if body["state"] != "awaiting" {
		t.Fatalf("state %v %s", body["state"], mustJSON(opened))
	}
	signed, _, found := seatState(t, opened, offerer)
	if !found || signed {
		t.Fatalf("offerer signature %s", mustJSON(opened))
	}
	if _, present := body["bound"]; present {
		t.Fatalf("bound on the wire %s", mustJSON(opened))
	}
	waiting, _ := body["waiting"].(map[string]any)
	reasons, _ := waiting["reasons"].([]any)
	if waiting["step"] != "list" || len(reasons) != 1 || reasons[0] != "step_ask" || waiting["url"] == "" || waiting["escalation"] == "" {
		t.Fatalf("waiting %v", waiting)
	}
	dealID, _ := opened["deal"].(string)
	if storedBound(t, db, dealID) != "[]" {
		t.Fatalf("stored bound %s", storedBound(t, db, dealID))
	}
}

func TestORP04OfferAsk(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	e := &orp04Env{t: t, r: r}
	for _, n := range []int{2, 3, 16} {
		parties := make([]party04, n)
		for i := range parties {
			parties[i] = joinPartyOnly(t, r)
			setPlacementPublish(t, db, parties[i].owner, "ask")
		}
		code, opened := e.send(parties[0], "0.4", "offer", "", map[string]any{
			"terms": shareTerms04(), "parties": percentParties(parties),
		})
		assertAskWait(t, db, code, opened, parties[0].party)
	}
	offerer := joinPartyOnly(t, r)
	acceptor := joinPartyOnly(t, r)
	signer := joinPartyOnly(t, r)
	for _, party := range []party04{offerer, acceptor, signer} {
		setPlacementPublish(t, db, party.owner, "ask")
	}
	code, opened := e.send(offerer, "0.4", "offer", "", map[string]any{
		"terms": shareTerms04(),
		"parties": []any{
			map[string]any{"id": offerer.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 50}},
			map[string]any{"id": acceptor.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 50}},
			map[string]any{"id": signer.party, "role": "signer", "required": false},
		},
	})
	assertAskWait(t, db, code, opened, offerer.party)
	dealID, _ := opened["deal"].(string)
	code, accepted := acceptSeat(e, acceptor, opened)
	acceptedBody := docBody(t, accepted)
	acceptedSigned, _, _ := seatState(t, accepted, acceptor.party)
	waiting, _ := acceptedBody["waiting"].(map[string]any)
	reasons, _ := waiting["reasons"].([]any)
	if code != http.StatusOK || acceptedSigned || acceptedBody["state"] == "active" || waiting["step"] != "accept" || !messagesHave(reasons, "step_ask") || storedBound(t, db, dealID) != "[]" {
		t.Fatalf("ask accept %d %s bound %s", code, mustJSON(accepted), storedBound(t, db, dealID))
	}
	code, left := e.send(signer, "0.4", "leave", dealID, map[string]any{})
	if code != http.StatusOK || len(docBody(t, left)["parties"].([]any)) != 2 || storedBound(t, db, dealID) != "[]" {
		t.Fatalf("ask leave %d %s", code, mustJSON(left))
	}
	extra := joinPartyOnly(t, r)
	setPlacementPublish(t, db, extra.owner, "ask")
	code, seated := e.send(offerer, "0.4", "seat", dealID, map[string]any{"seat": map[string]any{
		"id": extra.party, "role": "member", "required": false,
	}})
	if code != http.StatusOK || len(docBody(t, seated)["parties"].([]any)) != 3 || storedBound(t, db, dealID) != "[]" {
		t.Fatalf("ask seat %d %s", code, mustJSON(seated))
	}

	first := joinPartyOnly(t, r)
	second := joinPartyOnly(t, r)
	third := joinPartyOnly(t, r)
	for _, party := range []party04{first, second, third} {
		setPlacementPublish(t, db, party.owner, "ask")
	}
	code, opened = e.send(first, "0.4", "offer", "", map[string]any{
		"terms": shareTerms04(),
		"parties": []any{
			map[string]any{"id": first.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 50}},
			map[string]any{"id": second.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 30}},
			map[string]any{"id": third.party, "role": "curator", "required": true, "terms": map[string]any{"percent": 20}},
		},
	})
	assertAskWait(t, db, code, opened, first.party)
	code, opened = postApprove(t, r, first.token, opened)
	if code != http.StatusOK || docBody(t, opened)["state"] != "pending" || waitingNames(t, opened, first.party) {
		t.Fatalf("offerer approve %d %s", code, mustJSON(opened))
	}
	signed, _, found := seatState(t, opened, first.party)
	if !found || !signed {
		t.Fatalf("offerer still unsigned %s", mustJSON(opened))
	}
	code, opened = acceptSeat(e, second, opened)
	if code != http.StatusOK || !waitingNames(t, opened, second.party) {
		t.Fatalf("second hold %d %s", code, mustJSON(opened))
	}
	code, opened = postApprove(t, r, second.token, opened)
	if code != http.StatusOK || docBody(t, opened)["state"] != "pending" || waitingNames(t, opened, second.party) {
		t.Fatalf("second approve %d %s", code, mustJSON(opened))
	}
	code, opened = acceptSeat(e, third, opened)
	if code != http.StatusOK || !waitingNames(t, opened, third.party) {
		t.Fatalf("third hold %d %s", code, mustJSON(opened))
	}
	code, opened = postApprove(t, r, third.token, opened)
	if code != http.StatusOK || docBody(t, opened)["state"] != "active" || docBody(t, opened)["waiting"] != nil {
		t.Fatalf("last approve %d %s", code, mustJSON(opened))
	}
}

func waitingNames(t *testing.T, doc map[string]any, party string) bool {
	t.Helper()
	waiting, _ := docBody(t, doc)["waiting"].(map[string]any)
	return waiting["party"] == party
}

func postApprove(t *testing.T, r http.Handler, token string, doc map[string]any) (int, map[string]any) {
	t.Helper()
	waiting, _ := docBody(t, doc)["waiting"].(map[string]any)
	esc, _ := waiting["escalation"].(string)
	if esc == "" {
		t.Fatalf("no escalation %s", mustJSON(doc))
	}
	rec, body := postJSON(t, r, http.MethodPost, "/orp/approve/"+esc, map[string]any{}, bearerHeader(token))
	return rec.Code, body
}

func TestORP03DirectedPlaceAsk(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	offerer := joinDefault(t, r)
	acceptor := joinDefault(t, r)
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE owner_id=$1 AND site_id IS NULL`, offerer.owner) != 0 {
		t.Fatal("fresh join saved an owner mode row")
	}
	terms := testTerms()
	obligations := terms["obligations"].([]any)
	place := obligations[0].(map[string]any)
	place["by"] = "offerer"
	env := orpEnvelope("offer", "m_d102_placeby", offerer.party, map[string]any{"terms": terms})
	env["to"] = acceptor.party
	rec, doc := postJSON(t, r, http.MethodPost, "/orp", env, bearerHeader(offerer.token))
	body := docBody(t, doc)
	if rec.Code != http.StatusOK || doc["orp"] != "0.3" || doc["type"] != "deal" || body["state"] != "proposed" || body["bound"] != nil {
		t.Fatalf("directed %d %s", rec.Code, rec.Body.String())
	}
	dealID, _ := doc["deal"].(string)
	var bound string
	var partiesNull bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT bound::text, parties IS NULL FROM orp_deals WHERE id=$1`, dealID).Scan(&bound, &partiesNull); err != nil {
		t.Fatal(err)
	}
	if bound != "[]" || !partiesNull {
		t.Fatalf("stored bound %s parties null %v", bound, partiesNull)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_deals WHERE id=$1`, dealID) != 1 {
		t.Fatal("deal row missing")
	}
}

func TestORP04OfferAuto(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	e := &orp04Env{t: t, r: r}
	for _, n := range []int{2, 3, 16} {
		parties := make([]party04, n)
		for i := range parties {
			parties[i] = joinAuto(t, db, r)
		}
		code, opened := e.send(parties[0], "0.4", "offer", "", map[string]any{
			"terms": shareTerms04(), "parties": percentParties(parties),
		})
		body := docBody(t, opened)
		signed, _, found := seatState(t, opened, parties[0].party)
		if code != http.StatusOK || opened["orp"] != "0.4" || body["state"] != "pending" || !found || !signed || body["waiting"] != nil {
			t.Fatalf("auto %d %s", code, mustJSON(opened))
		}
		dealID, _ := opened["deal"].(string)
		if !strings.Contains(storedBound(t, db, dealID), parties[0].party) {
			t.Fatalf("auto bound %s", storedBound(t, db, dealID))
		}
	}
	offerer := joinAuto(t, db, r)
	acceptor := joinAuto(t, db, r)
	member := joinAuto(t, db, r)
	code, opened := e.send(offerer, "0.4", "offer", "", map[string]any{
		"terms": shareTerms04(),
		"parties": []any{
			map[string]any{"id": offerer.party, "role": "offerer", "required": true, "terms": map[string]any{"amount": eur(0)}},
			map[string]any{"id": acceptor.party, "role": "acceptor", "required": true, "terms": map[string]any{"amount": eur(100)}},
			map[string]any{"id": member.party, "role": "member", "required": false},
		},
	})
	dealID, _ := opened["deal"].(string)
	if code != http.StatusOK || docBody(t, opened)["state"] != "pending" {
		t.Fatalf("amount offer %d %s", code, mustJSON(opened))
	}
	code, left := e.send(offerer, "0.4", "leave", dealID, map[string]any{})
	if code != http.StatusOK || docBody(t, left)["state"] != "pending" || storedBound(t, db, dealID) != "[]" {
		t.Fatalf("last signature leave %d %s bound %s", code, mustJSON(left), storedBound(t, db, dealID))
	}
	extra := joinAuto(t, db, r)
	code, seated := e.send(acceptor, "0.4", "seat", dealID, map[string]any{"seat": map[string]any{
		"id": extra.party, "role": "member", "required": false,
	}})
	if code != http.StatusOK || storedBound(t, db, dealID) != "[]" {
		t.Fatalf("seat after empty bound %d %s", code, mustJSON(seated))
	}
}

func TestORP04FreshJoinDefault(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	e := &orp04Env{t: t, r: r}
	offerer := joinDefault(t, r)
	acceptor := joinDefault(t, r)
	signer := joinDefault(t, r)
	if countRows(t, db, `SELECT COUNT(*) FROM agent_step_modes WHERE owner_id=$1 AND placement_publish='auto'`, offerer.owner) != 0 {
		t.Fatal("fresh join was set to auto")
	}
	code, opened := e.send(offerer, "0.4", "offer", "", map[string]any{
		"terms": shareTerms04(),
		"parties": []any{
			map[string]any{"id": offerer.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 50}},
			map[string]any{"id": acceptor.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 50}},
			map[string]any{"id": signer.party, "role": "signer", "required": false},
		},
	})
	assertAskWait(t, db, code, opened, offerer.party)
	dealID, _ := opened["deal"].(string)
	code, accepted := acceptSeat(e, acceptor, opened)
	if code != http.StatusOK || docBody(t, accepted)["state"] == "active" || storedBound(t, db, dealID) != "[]" {
		t.Fatalf("default accept %d %s", code, mustJSON(accepted))
	}
	signed, _, _ := seatState(t, accepted, acceptor.party)
	if signed {
		t.Fatalf("default acceptor signed %s", mustJSON(accepted))
	}
	code, left := e.send(signer, "0.4", "leave", dealID, map[string]any{})
	if code != http.StatusOK || storedBound(t, db, dealID) != "[]" {
		t.Fatalf("default leave %d %s", code, mustJSON(left))
	}
	extra := joinDefault(t, r)
	code, seated := e.send(offerer, "0.4", "seat", dealID, map[string]any{"seat": map[string]any{
		"id": extra.party, "role": "member", "required": false,
	}})
	if code != http.StatusOK || storedBound(t, db, dealID) != "[]" {
		t.Fatalf("default seat %d %s", code, mustJSON(seated))
	}
}

func clickClassed(t *testing.T, r http.Handler, link, ua string) *httptest.ResponseRecorder {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, parsed.Path, nil)
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func placeObligation(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	rows, _ := body["obligations"].([]any)
	for _, item := range rows {
		row, _ := item.(map[string]any)
		if row["id"] == "place" {
			return row
		}
	}
	t.Fatalf("place obligation missing %s", mustJSON(map[string]any{"body": body}))
	return nil
}

func sharePercents(t *testing.T, click map[string]any) map[string]float64 {
	t.Helper()
	shares, _ := click["shares"].([]any)
	got := map[string]float64{}
	for _, item := range shares {
		row, _ := item.(map[string]any)
		party, _ := row["party"].(string)
		got[party] = asFloat(row["percent"])
	}
	return got
}

// TestORP04LinkClickVerifies is the cold-walk failure: an ask-mode 3-party
// deal goes active, the tracked link 302s, and the first human click still
// leaves place pending with no receipt and an unchanged proof page.
func TestORP04LinkClickVerifies(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	e := &orp04Env{t: t, r: r}
	offerer := joinDefault(t, r)
	curator := joinDefault(t, r)
	acceptor := joinDefault(t, r)
	terms := map[string]any{
		"title": "Referral notes chain", "kind": "partnership",
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"obligations": []any{map[string]any{
			"id": "place", "by": acceptor.party, "action": "place",
			"content":     map[string]any{"link": true, "brief": "Mention the notes app."},
			"destination": "https://example.com/notes", "proof": []any{"link"},
		}},
	}
	code, opened := e.send(offerer, "0.4", "offer", "", map[string]any{"terms": terms, "parties": []any{
		map[string]any{"id": offerer.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 50}},
		map[string]any{"id": curator.party, "role": "curator", "required": true, "terms": map[string]any{"percent": 30}},
		map[string]any{"id": acceptor.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 20}},
	}})
	assertAskWait(t, db, code, opened, offerer.party)
	code, opened = postApprove(t, r, offerer.token, opened)
	if code != http.StatusOK || docBody(t, opened)["state"] != "pending" {
		t.Fatalf("offerer approve %d %s", code, mustJSON(opened))
	}
	code, opened = acceptSeat(e, curator, opened)
	if code != http.StatusOK {
		t.Fatalf("curator accept %d %s", code, mustJSON(opened))
	}
	code, opened = postApprove(t, r, curator.token, opened)
	if code != http.StatusOK || docBody(t, opened)["state"] != "pending" {
		t.Fatalf("curator approve %d %s", code, mustJSON(opened))
	}
	code, opened = acceptSeat(e, acceptor, opened)
	if code != http.StatusOK {
		t.Fatalf("acceptor accept %d %s", code, mustJSON(opened))
	}
	code, opened = postApprove(t, r, acceptor.token, opened)
	body := docBody(t, opened)
	if code != http.StatusOK || body["state"] != "active" || body["waiting"] != nil {
		t.Fatalf("active %d %s", code, mustJSON(opened))
	}
	tracking, _ := body["tracking"].([]any)
	if len(tracking) != 1 {
		t.Fatalf("tracking %s", mustJSON(opened))
	}
	link, _ := tracking[0].(map[string]any)["link"].(string)
	dealID, _ := opened["deal"].(string)
	proofBefore, _ := postJSON(t, r, http.MethodGet, "/orp/proof/"+dealID, nil, nil)
	beforeHTML := proofBefore.Body.String()
	browser := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

	bot := clickClassed(t, r, link, "")
	if bot.Code != http.StatusFound {
		t.Fatalf("bot click %d", bot.Code)
	}
	botDealRec, botDeal := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(offerer.token))
	botPlace := placeObligation(t, docBody(t, botDeal))
	if botDealRec.Code != http.StatusOK || botPlace["state"] != "pending" || botPlace["receipt"] != nil {
		t.Fatalf("bot verified the link %d %s", botDealRec.Code, botDealRec.Body.String())
	}

	click := clickClassed(t, r, link, browser)
	if click.Code != http.StatusFound {
		t.Fatalf("human click %d %s", click.Code, click.Body.String())
	}
	loc := click.Header().Get("Location")
	if !strings.Contains(loc, "via=") || !strings.Contains(loc, "ref=") {
		t.Fatalf("location %s", loc)
	}
	dealRec, deal := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(offerer.token))
	place := placeObligation(t, docBody(t, deal))
	receipt, _ := place["receipt"].(string)
	if dealRec.Code != http.StatusOK || place["state"] != "verified" || receipt == "" {
		t.Fatalf("link obligation %d %s", dealRec.Code, dealRec.Body.String())
	}
	if click.Header().Get("Cache-Control") != "no-store" || click.Header().Get("Referrer-Policy") != "no-referrer" || click.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("redirect headers cache %q referrer %q robots %q", click.Header().Get("Cache-Control"), click.Header().Get("Referrer-Policy"), click.Header().Get("X-Robots-Tag"))
	}
	proofAfter, _ := postJSON(t, r, http.MethodGet, "/orp/proof/"+dealID, nil, nil)
	if proofAfter.Code != http.StatusOK || proofAfter.Body.String() == beforeHTML {
		t.Fatalf("proof unchanged %d", proofAfter.Code)
	}
	stored := storedParties(t, db, dealID)
	rawStored := mustJSON(stored)
	if strings.Contains(rawStored, browser) || strings.Contains(rawStored, "@") {
		t.Fatalf("pii stored %s", rawStored)
	}
	again := clickClassed(t, r, link, browser)
	if again.Code != http.StatusFound || again.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("repeat %d robots %q", again.Code, again.Header().Get("X-Robots-Tag"))
	}
	againRec, againDeal := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(acceptor.token))
	againPlace := placeObligation(t, docBody(t, againDeal))
	if againRec.Code != http.StatusOK || againPlace["state"] != "verified" || againPlace["receipt"] != receipt {
		t.Fatalf("repeat receipt %s", againRec.Body.String())
	}
	clicks, _ := storedParties(t, db, dealID)["clicks"].([]any)
	human := 0
	for _, item := range clicks {
		row, _ := item.(map[string]any)
		if row["class"] != "human" {
			continue
		}
		human++
		got := sharePercents(t, row)
		if got[offerer.party] != 50 || got[curator.party] != 30 || got[acceptor.party] != 20 || len(got) != 3 {
			t.Fatalf("shares doubled %#v", got)
		}
	}
	if human != 2 {
		t.Fatalf("human clicks %d in %s", human, mustJSON(storedParties(t, db, dealID)))
	}
	inboxRec, inbox := postJSON(t, r, http.MethodGet, "/orp/inbox", nil, bearerHeader(offerer.token))
	messages, _ := inbox["messages"].([]any)
	receipts := 0
	for _, item := range messages {
		msg, _ := item.(map[string]any)
		if msg["type"] != "receipt" {
			continue
		}
		receipts++
		if msg["id"] != receipt || msg["orp"] != "0.4" {
			t.Fatalf("receipt message %s", mustJSON(msg))
		}
		inner, _ := msg["body"].(map[string]any)
		if inner["obligation"] != "place" || inner["result"] != "verified" || inner["method"] != "link" {
			t.Fatalf("receipt body %v", inner)
		}
		if strings.Contains(mustJSON(msg), browser) {
			t.Fatalf("receipt stored the user agent %s", mustJSON(msg))
		}
	}
	if inboxRec.Code != http.StatusOK || receipts != 1 {
		t.Fatalf("inbox receipts %d %s", receipts, inboxRec.Body.String())
	}
	unknown := clickClassed(t, r, "http://127.0.0.1:8080/go/missing-token", browser)
	if unknown.Code != http.StatusFound || unknown.Header().Get("X-Robots-Tag") != "noindex" || unknown.Header().Get("Cache-Control") != "no-store" || unknown.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("unknown redirect %d cache %q robots %q", unknown.Code, unknown.Header().Get("Cache-Control"), unknown.Header().Get("X-Robots-Tag"))
	}
}

func TestORP04InvalidNamesVersion(t *testing.T) {
	_, _, r, cleanup := newORP04(t)
	defer cleanup()
	from := "orp:127.0.0.1/party"
	body := map[string]any{"versions": []any{"0.4"}}
	short04, short04Body := postJSON(t, r, http.MethodPost, "/orp", map[string]any{
		"orp": "0.4", "type": "hello", "id": "short1", "ts": time.Now().UTC().Format(time.RFC3339), "from": from, "body": body,
	}, nil)
	inner, _ := short04Body["body"].(map[string]any)
	if short04.Code != http.StatusBadRequest || short04Body["orp"] != "0.4" || inner["message"] != "The message is not valid ORP 0.4." || inner["field"] != "/id" {
		t.Fatalf("0.4 id %d %s", short04.Code, short04.Body.String())
	}
	extra, extraBody := postJSON(t, r, http.MethodPost, "/orp", map[string]any{
		"orp": "0.4", "type": "hello", "id": "m_d102_extra", "ts": time.Now().UTC().Format(time.RFC3339), "from": from, "body": body, "note": "nope",
	}, nil)
	extraInner, _ := extraBody["body"].(map[string]any)
	if extra.Code != http.StatusBadRequest || extraInner["message"] != "The message is not valid ORP 0.4." || extraInner["field"] != "/note" {
		t.Fatalf("0.4 key %d %s", extra.Code, extra.Body.String())
	}
	agent, agentBody := postJSON(t, r, http.MethodPost, "/orp", map[string]any{
		"orp": "0.4", "type": "hello", "id": "m_d102_agent", "ts": time.Now().UTC().Format(time.RFC3339), "from": from, "body": body,
		"agent": map[string]any{"name": ""},
	}, nil)
	agentInner, _ := agentBody["body"].(map[string]any)
	name04 := "The message is not valid ORP 0.4. " + orp.AgentNameRule
	if agent.Code != http.StatusBadRequest || agentInner["message"] != name04 || agentInner["field"] != "/agent/name" || strings.Contains(agentInner["message"].(string), "ORP 0.3.") {
		t.Fatalf("0.4 agent %d %s", agent.Code, agent.Body.String())
	}
	spaced, spacedBody := postJSON(t, r, http.MethodPost, "/orp", map[string]any{
		"orp": "0.4", "type": "hello", "id": "m_d102_space", "ts": time.Now().UTC().Format(time.RFC3339), "from": from, "body": body,
		"agent": map[string]any{"name": "Cold Agent"},
	}, nil)
	spacedInner, _ := spacedBody["body"].(map[string]any)
	if spaced.Code == http.StatusBadRequest && spacedInner["field"] == "/agent/name" {
		t.Fatalf("space in the name was refused %s", spaced.Body.String())
	}
	paren, parenBody := postJSON(t, r, http.MethodPost, "/orp", map[string]any{
		"orp": "0.4", "type": "hello", "id": "m_d102_paren", "ts": time.Now().UTC().Format(time.RFC3339), "from": from, "body": body,
		"agent": map[string]any{"name": "Cold Agent (walk)"},
	}, nil)
	parenInner, _ := parenBody["body"].(map[string]any)
	if paren.Code != http.StatusBadRequest || parenInner["message"] != name04 || parenInner["field"] != "/agent/name" {
		t.Fatalf("0.4 parentheses %d %s", paren.Code, paren.Body.String())
	}
	paren03, paren03Body := postJSON(t, r, http.MethodPost, "/orp", map[string]any{
		"orp": "0.3", "type": "hello", "id": "m_d102_paren3", "ts": time.Now().UTC().Format(time.RFC3339), "from": from, "body": map[string]any{"versions": []any{"0.3"}},
		"agent": map[string]any{"name": "Cold Agent (walk)"},
	}, nil)
	paren03Inner, _ := paren03Body["body"].(map[string]any)
	name03 := "The message is not valid ORP 0.3. " + orp.AgentNameRule
	if paren03.Code != http.StatusBadRequest || paren03Inner["message"] != name03 || paren03Inner["field"] != "/agent/name" || strings.Contains(paren03Inner["message"].(string), "ORP 0.4.") {
		t.Fatalf("0.3 parentheses %d %s", paren03.Code, paren03.Body.String())
	}
	short03, short03Body := postJSON(t, r, http.MethodPost, "/orp", map[string]any{
		"orp": "0.3", "type": "hello", "id": "short1", "ts": time.Now().UTC().Format(time.RFC3339), "from": from, "body": map[string]any{"versions": []any{"0.3"}},
	}, nil)
	inner03, _ := short03Body["body"].(map[string]any)
	if short03.Code != http.StatusBadRequest || inner03["message"] != "The message is not valid ORP 0.3." || inner03["field"] != "/id" {
		t.Fatalf("0.3 id %d %s", short03.Code, short03.Body.String())
	}
}

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func openActivePlace(t *testing.T, db *database.DB, r http.Handler) (dealID, link string, offerer party04) {
	t.Helper()
	e := &orp04Env{t: t, r: r}
	offerer = joinAuto(t, db, r)
	acceptor := joinAuto(t, db, r)
	terms := map[string]any{
		"title": "Referral notes chain", "kind": "partnership",
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"obligations": []any{map[string]any{
			"id": "place", "by": acceptor.party, "action": "place",
			"content":     map[string]any{"link": true, "brief": "Mention the notes app."},
			"destination": "https://example.com/notes", "proof": []any{"link"},
		}},
	}
	code, opened := e.send(offerer, "0.4", "offer", "", map[string]any{"terms": terms, "parties": []any{
		map[string]any{"id": offerer.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 50}},
		map[string]any{"id": acceptor.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 50}},
	}})
	if code != http.StatusOK {
		t.Fatalf("offer %d %s", code, mustJSON(opened))
	}
	code, opened = acceptSeat(e, acceptor, opened)
	body := docBody(t, opened)
	if code != http.StatusOK || body["state"] != "active" {
		t.Fatalf("active %d %s", code, mustJSON(opened))
	}
	tracking, _ := body["tracking"].([]any)
	if len(tracking) != 1 {
		t.Fatalf("tracking %s", mustJSON(opened))
	}
	link, _ = tracking[0].(map[string]any)["link"].(string)
	dealID, _ = opened["deal"].(string)
	if link == "" || dealID == "" {
		t.Fatalf("link %q deal %q", link, dealID)
	}
	return dealID, link, offerer
}

func readPlace(t *testing.T, r http.Handler, dealID, token string) map[string]any {
	t.Helper()
	rec, doc := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("deal %d %s", rec.Code, rec.Body.String())
	}
	return placeObligation(t, docBody(t, doc))
}

func setDealState(t *testing.T, db *database.DB, dealID, state string) {
	t.Helper()
	tag, err := db.Pool.Exec(context.Background(), `UPDATE orp_deals SET state=$2 WHERE id=$1`, dealID, state)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("set state %s: %v rows %d", state, err, tag.RowsAffected())
	}
}

func setPlaceState(t *testing.T, db *database.DB, dealID, state string) {
	t.Helper()
	var raw []byte
	if err := db.Pool.QueryRow(context.Background(), `SELECT obligations FROM orp_deals WHERE id=$1`, dealID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range rows {
		if rows[i]["id"] != "place" {
			continue
		}
		rows[i]["state"] = state
		delete(rows[i], "receipt")
		found = true
	}
	if !found {
		t.Fatal("place missing")
	}
	next, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), `UPDATE orp_deals SET obligations=$2 WHERE id=$1`, dealID, next); err != nil {
		t.Fatal(err)
	}
}

func storedClicks(t *testing.T, db *database.DB, dealID string) []any {
	t.Helper()
	clicks, _ := storedParties(t, db, dealID)["clicks"].([]any)
	return clicks
}

func receiptCount(t *testing.T, db *database.DB) int {
	t.Helper()
	return countRows(t, db, `SELECT COUNT(*) FROM orp_inbox WHERE message->>'type' = 'receipt'`)
}

func TestORP04BotUADoesNotVerify(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	dealID, link, offerer := openActivePlace(t, db, r)
	first := clickClassed(t, r, link, "Googlebot/2.1")
	if first.Code != http.StatusFound || !strings.Contains(first.Header().Get("Location"), "example.com/notes") {
		t.Fatalf("googlebot %d %s", first.Code, first.Header().Get("Location"))
	}
	place := readPlace(t, r, dealID, offerer.token)
	if place["state"] != "pending" || place["receipt"] != nil {
		t.Fatalf("googlebot verified %#v", place)
	}
	clicks := storedClicks(t, db, dealID)
	if len(clicks) != 1 || clicks[0].(map[string]any)["class"] != "bot" {
		t.Fatalf("googlebot class %#v", clicks)
	}
	for _, ua := range []string{"Twitterbot/1.0", "curl/8.0.1"} {
		rec := clickClassed(t, r, link, ua)
		if rec.Code != http.StatusFound {
			t.Fatalf("%s %d", ua, rec.Code)
		}
	}
	place = readPlace(t, r, dealID, offerer.token)
	if place["state"] != "pending" || place["receipt"] != nil || receiptCount(t, db) != 0 {
		t.Fatalf("bots verified %#v receipts %d", place, receiptCount(t, db))
	}
	for _, item := range storedClicks(t, db, dealID) {
		row, _ := item.(map[string]any)
		if row["class"] != "bot" {
			t.Fatalf("class %#v", row)
		}
	}
	human := clickClassed(t, r, link, browserUA)
	if human.Code != http.StatusFound {
		t.Fatalf("human %d", human.Code)
	}
	place = readPlace(t, r, dealID, offerer.token)
	if place["state"] != "verified" || place["receipt"] == nil {
		t.Fatalf("browser after bots %#v", place)
	}
}

func TestORP04InactiveDealDoesNotVerify(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	dealID, link, offerer := openActivePlace(t, db, r)
	for _, state := range []string{"pending", "ended"} {
		setDealState(t, db, dealID, state)
		rec := clickClassed(t, r, link, browserUA)
		if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "example.com/notes") {
			t.Fatalf("%s redirect %d %s", state, rec.Code, rec.Header().Get("Location"))
		}
		place := readPlace(t, r, dealID, offerer.token)
		if place["state"] != "pending" || place["receipt"] != nil {
			t.Fatalf("%s verified %#v", state, place)
		}
	}
	if n := receiptCount(t, db); n != 0 {
		t.Fatalf("receipts %d", n)
	}
	clicks := storedClicks(t, db, dealID)
	if len(clicks) != 2 {
		t.Fatalf("clicks %#v", clicks)
	}
	setDealState(t, db, dealID, "active")
	rec := clickClassed(t, r, link, browserUA)
	if rec.Code != http.StatusFound {
		t.Fatalf("active %d", rec.Code)
	}
	place := readPlace(t, r, dealID, offerer.token)
	if place["state"] != "verified" || place["receipt"] == nil {
		t.Fatalf("active verify %#v", place)
	}
}

func TestORP04ObligationStateGuard(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	dealID, link, offerer := openActivePlace(t, db, r)
	for _, state := range []string{"failed", "partial"} {
		setPlaceState(t, db, dealID, state)
		before := len(storedClicks(t, db, dealID))
		rec := clickClassed(t, r, link, browserUA)
		if rec.Code != http.StatusFound {
			t.Fatalf("%s %d", state, rec.Code)
		}
		place := readPlace(t, r, dealID, offerer.token)
		if place["state"] != state || place["receipt"] != nil {
			t.Fatalf("%s changed %#v", state, place)
		}
		if len(storedClicks(t, db, dealID)) != before+1 || receiptCount(t, db) != 0 {
			t.Fatalf("%s clicks %d receipts %d", state, len(storedClicks(t, db, dealID)), receiptCount(t, db))
		}
	}
	setPlaceState(t, db, dealID, "claimed")
	rec := clickClassed(t, r, link, browserUA)
	if rec.Code != http.StatusFound {
		t.Fatalf("claimed %d", rec.Code)
	}
	place := readPlace(t, r, dealID, offerer.token)
	receipt, _ := place["receipt"].(string)
	if place["state"] != "verified" || receipt == "" || receiptCount(t, db) != 2 {
		t.Fatalf("claimed verify %#v receipts %d", place, receiptCount(t, db))
	}
	again := clickClassed(t, r, link, browserUA)
	if again.Code != http.StatusFound {
		t.Fatalf("repeat %d", again.Code)
	}
	place = readPlace(t, r, dealID, offerer.token)
	if place["state"] != "verified" || place["receipt"] != receipt || receiptCount(t, db) != 2 {
		t.Fatalf("repeat receipt %#v count %d", place, receiptCount(t, db))
	}
}

func TestORP04HeadDoesNotRecord(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	dealID, link, offerer := openActivePlace(t, db, r)
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodHead, parsed.Path, nil)
	req.Header.Set("User-Agent", browserUA)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("head %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Referrer-Policy") != "no-referrer" || rec.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("head headers cache %q referrer %q robots %q", rec.Header().Get("Cache-Control"), rec.Header().Get("Referrer-Policy"), rec.Header().Get("X-Robots-Tag"))
	}
	if !strings.Contains(rec.Header().Get("Location"), "example.com/notes") {
		t.Fatalf("head location %s", rec.Header().Get("Location"))
	}
	place := readPlace(t, r, dealID, offerer.token)
	if place["state"] != "pending" || place["receipt"] != nil || len(storedClicks(t, db, dealID)) != 0 || receiptCount(t, db) != 0 {
		t.Fatalf("head recorded %#v clicks %#v", place, storedClicks(t, db, dealID))
	}
	if got := orp.ClickClass(http.MethodHead, browserUA, "", ""); got != "preview" {
		t.Fatalf("head class %s", got)
	}
	get := clickClassed(t, r, link, browserUA)
	if get.Code != http.StatusFound || get.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("get after head %d", get.Code)
	}
	place = readPlace(t, r, dealID, offerer.token)
	if place["state"] != "verified" || place["receipt"] == nil || len(storedClicks(t, db, dealID)) != 1 {
		t.Fatalf("get after head %#v clicks %d", place, len(storedClicks(t, db, dealID)))
	}
}

func TestORP04ClickShareRead(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	e := &orp04Env{t: t, r: r}
	offerer := joinDefault(t, r)
	curator := joinDefault(t, r)
	acceptor := joinDefault(t, r)
	terms := map[string]any{
		"title": "Referral notes chain", "kind": "partnership",
		"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
		"obligations": []any{map[string]any{
			"id": "place", "by": acceptor.party, "action": "place",
			"content":     map[string]any{"link": true, "brief": "Mention the notes app."},
			"destination": "https://example.com/notes", "proof": []any{"link"},
		}},
	}
	code, opened := e.send(offerer, "0.4", "offer", "", map[string]any{"terms": terms, "parties": []any{
		map[string]any{"id": offerer.party, "role": "offerer", "required": true, "terms": map[string]any{"percent": 50}},
		map[string]any{"id": curator.party, "role": "curator", "required": true, "terms": map[string]any{"percent": 30}},
		map[string]any{"id": acceptor.party, "role": "acceptor", "required": true, "terms": map[string]any{"percent": 20}},
	}})
	assertAskWait(t, db, code, opened, offerer.party)
	code, opened = postApprove(t, r, offerer.token, opened)
	if code != http.StatusOK {
		t.Fatalf("offerer approve %d %s", code, mustJSON(opened))
	}
	code, opened = acceptSeat(e, curator, opened)
	if code != http.StatusOK {
		t.Fatalf("curator accept %d %s", code, mustJSON(opened))
	}
	code, opened = postApprove(t, r, curator.token, opened)
	if code != http.StatusOK {
		t.Fatalf("curator approve %d %s", code, mustJSON(opened))
	}
	code, opened = acceptSeat(e, acceptor, opened)
	if code != http.StatusOK {
		t.Fatalf("acceptor accept %d %s", code, mustJSON(opened))
	}
	code, opened = postApprove(t, r, acceptor.token, opened)
	body := docBody(t, opened)
	if code != http.StatusOK || body["state"] != "active" {
		t.Fatalf("active %d %s", code, mustJSON(opened))
	}
	tracking, _ := body["tracking"].([]any)
	if len(tracking) != 1 {
		t.Fatalf("tracking %s", mustJSON(opened))
	}
	link, _ := tracking[0].(map[string]any)["link"].(string)
	dealID, _ := opened["deal"].(string)
	parties := []party04{offerer, curator, acceptor}
	assertShare := func(wantClicks float64, want map[string]float64) {
		t.Helper()
		var seen string
		for _, party := range parties {
			rec, doc := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(party.token))
			if rec.Code != http.StatusOK {
				t.Fatalf("deal %d %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), browserUA) || strings.Contains(rec.Body.String(), "Googlebot") || strings.Contains(rec.Body.String(), "@") {
				t.Fatalf("pii %s", rec.Body.String())
			}
			row := docBody(t, doc)["tracking"].([]any)[0].(map[string]any)
			if asFloat(row["clicks"]) != wantClicks {
				t.Fatalf("clicks %#v", row["clicks"])
			}
			got := map[string]float64{}
			shares, _ := row["click_share"].([]any)
			for _, item := range shares {
				one, _ := item.(map[string]any)
				id, _ := one["party"].(string)
				got[id] = asFloat(one["percent"])
			}
			raw, err := json.Marshal(row["click_share"])
			if err != nil {
				t.Fatal(err)
			}
			if seen == "" {
				seen = string(raw)
			} else if seen != string(raw) {
				t.Fatalf("share differed %s vs %s", seen, raw)
			}
			for id, value := range want {
				if got[id] != value || len(got) != len(want) {
					t.Fatalf("share %#v want %#v", got, want)
				}
			}
		}
	}
	zero := map[string]float64{offerer.party: 0, curator.party: 0, acceptor.party: 0}
	clickClassed(t, r, link, "")
	assertShare(0, zero)
	clickClassed(t, r, link, "Googlebot/2.1")
	assertShare(0, zero)
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	head := httptest.NewRequest(http.MethodHead, parsed.Path, nil)
	head.Header.Set("User-Agent", browserUA)
	headRec := httptest.NewRecorder()
	r.ServeHTTP(headRec, head)
	if headRec.Code != http.StatusFound {
		t.Fatalf("head %d", headRec.Code)
	}
	assertShare(0, zero)
	prefetch := httptest.NewRequest(http.MethodGet, parsed.Path, nil)
	prefetch.Header.Set("User-Agent", browserUA)
	prefetch.Header.Set("Sec-Purpose", "prefetch")
	prefetchRec := httptest.NewRecorder()
	r.ServeHTTP(prefetchRec, prefetch)
	if prefetchRec.Code != http.StatusFound {
		t.Fatalf("prefetch %d", prefetchRec.Code)
	}
	testClick := httptest.NewRequest(http.MethodGet, parsed.Path+"?orp_test=1", nil)
	testClick.Header.Set("User-Agent", browserUA)
	testRec := httptest.NewRecorder()
	r.ServeHTTP(testRec, testClick)
	if testRec.Code != http.StatusFound {
		t.Fatalf("orp_test %d", testRec.Code)
	}
	assertShare(0, zero)
	clickClassed(t, r, link, browserUA)
	clickClassed(t, r, link, browserUA)
	assertShare(2, map[string]float64{offerer.party: 1, curator.party: 0.6, acceptor.party: 0.4})
	shown, doc := postJSON(t, r, http.MethodGet, "/orp/deals/"+dealID, nil, bearerHeader(acceptor.token))
	if !strings.Contains(shown.Body.String(), `"percent":1.0`) || !strings.Contains(shown.Body.String(), `"percent":0.6`) || !strings.Contains(shown.Body.String(), `"percent":0.4`) {
		t.Fatalf("fixed point %s", shown.Body.String())
	}
	if err := orp.ValidateAgainst(filepath.Join(orpRepoRoot(t), "docs", "protocol", "schema", "0.4", "messages", "deal.schema.json"), doc); err != nil {
		t.Fatalf("schema %v", err)
	}

	env := orpEnvelope("offer", "m_d109_03offer", offerer.party, map[string]any{"terms": testTerms()})
	env["to"] = acceptor.party
	oldRec, oldDoc := postJSON(t, r, http.MethodPost, "/orp", env, bearerHeader(offerer.token))
	if oldRec.Code != http.StatusOK || oldDoc["orp"] != "0.3" || strings.Contains(oldRec.Body.String(), "click_share") || strings.Contains(oldRec.Body.String(), `"clicks"`) {
		t.Fatalf("0.3 offer %d %s", oldRec.Code, oldRec.Body.String())
	}
	oldID, _ := oldDoc["deal"].(string)
	oldGet, _ := postJSON(t, r, http.MethodGet, "/orp/deals/"+oldID, nil, bearerHeader(offerer.token))
	if oldGet.Code != http.StatusOK || strings.Contains(oldGet.Body.String(), "click_share") || strings.Contains(oldGet.Body.String(), `"clicks"`) {
		t.Fatalf("0.3 read %d %s", oldGet.Code, oldGet.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/deal.schema.json"), oldDoc); err != nil {
		t.Fatalf("0.3 schema %v", err)
	}
}

func TestORPClickWriteKeepsEmptyObligations(t *testing.T) {
	db, _, r, cleanup := newORP04(t)
	defer cleanup()
	dealID, _, _ := openActivePlace(t, db, r)
	for _, raw := range []json.RawMessage{nil, {}, json.RawMessage("null"), json.RawMessage("[]")} {
		if _, err := db.Pool.Exec(context.Background(), `UPDATE orp_deals SET obligations=$2 WHERE id=$1`, dealID, database.JSONList(raw)); err != nil {
			t.Fatal(err)
		}
		var isNull bool
		var text string
		if err := db.Pool.QueryRow(context.Background(), `SELECT obligations IS NULL, obligations::text FROM orp_deals WHERE id=$1`, dealID).Scan(&isNull, &text); err != nil {
			t.Fatal(err)
		}
		if isNull || text != "[]" {
			t.Fatalf("JSONList(%q) stored null=%v text=%s", raw, isNull, text)
		}
	}
}
