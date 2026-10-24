package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

const openTestDeals = 5

// Mount adds the test-mode deal routes. POST /orp stays the message endpoint.
func mountORPDeals(h *ORPHost, r gin.IRouter) {
	r.GET("/orp/inbox", h.Inbox)
	r.GET("/orp/deals/:id", h.Deal)
	r.GET("/orp/proof/:id", h.Proof)
	r.GET("/orp/approve/:id", h.Approve)
}

// Inbox is GET /orp/inbox. The bearer is the joined agent.
func (h *ORPHost) Inbox(c *gin.Context) {
	_, dns := h.origin(c)
	grant, ok := h.bearer(c, dns)
	if !ok {
		return
	}
	after := int64(0)
	if raw := strings.TrimSpace(c.Query("after")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			h.writeProblem(c, dns, "", "", orp.Invalid("The inbox cursor is not valid.", ""), false)
			return
		}
		after = n
	}
	raw, next, err := h.Entry.DB.ListORPInbox(c.Request.Context(), grant.OwnerID, after, 100)
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	if raw == nil {
		raw = []json.RawMessage{}
	}
	writeORP(c, http.StatusOK, map[string]any{"messages": raw, "next": strconv.FormatInt(next, 10)}, false)
}

// Deal is GET /orp/deals/:id for a party on the deal.
func (h *ORPHost) Deal(c *gin.Context) {
	_, dns := h.origin(c)
	grant, ok := h.bearer(c, dns)
	if !ok {
		return
	}
	party := orp.MemberParty(dns, grant.OwnerID)
	deal, found, err := h.Entry.DB.GetORPDeal(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeProblem(c, dns, party, "", internalProblem(), false)
		return
	}
	if !found || (deal.OffererParty != party && deal.AcceptorParty != party) {
		h.writeProblem(c, dns, party, "", orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}, false)
		return
	}
	doc, err := h.dealMessage(dns, party, "", deal, h.now())
	if err != nil {
		h.writeProblem(c, dns, party, "", internalProblem(), false)
		return
	}
	writeORP(c, http.StatusOK, doc, false)
}

// Proof is the public test page named by an execute proof_url.
func (h *ORPHost) Proof(c *gin.Context) {
	deal, found, err := h.Entry.DB.GetORPDeal(c.Request.Context(), c.Param("id"))
	if err != nil || !found {
		writeHTML(c, http.StatusNotFound, "not found")
		return
	}
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		writeHTML(c, http.StatusNotFound, "not found")
		return
	}
	writeHTML(c, http.StatusOK, orp.ProofPage(orp.DisclosureLabel(terms)))
}

// Approve shows the terms a person would decide. It does not approve.
func (h *ORPHost) Approve(c *gin.Context) {
	deal, found, err := h.Entry.DB.GetORPDealByEscalation(c.Request.Context(), c.Param("id"))
	if err != nil || !found {
		writeHTML(c, http.StatusNotFound, "not found")
		return
	}
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		writeHTML(c, http.StatusNotFound, "not found")
		return
	}
	writeHTML(c, http.StatusOK, orp.ApprovePage(orp.TermsTitle(terms), orp.DisclosureLabel(terms)))
}

func (h *ORPHost) offer(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	terms := msg.Body["terms"].(map[string]any)
	hashOfTerms := orp.TermsHash(terms)
	if given, _ := msg.Body["terms_hash"].(string); given != "" && given != hashOfTerms {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "terms_mismatch", Message: "terms_hash does not match the terms."}), database.ORPEffect{})
		return
	}
	mode, err := h.placementMode(c, ownerID)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if msg.To != "" {
		h.directedOffer(c, dns, msg, hash, ownerID, mode, terms, hashOfTerms)
		return
	}
	if orp.Places(terms, "offerer") && mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("list", "The owner has turned off placement publish, so this agent cannot list a placement.")), database.ORPEffect{})
		return
	}
	limit := openTestDeals
	if n, ok := wholeLimit(msg.Body["limit"]); ok {
		limit = n
	}
	now := h.now()
	offerID := orp.LocalID("ofr_")
	body := map[string]any{"id": offerID, "terms": terms, "terms_hash": hashOfTerms}
	if _, present := msg.Body["limit"]; present {
		body["limit"] = limit
	}
	if note, _ := msg.Body["note"].(string); note != "" {
		body["note"] = note
	}
	if expires, _ := msg.Body["expires"].(string); expires != "" {
		body["expires"] = expires
	}
	doc := orp.ListedOffer(dns, msg.From, msg.ID, body, now)
	termsRaw, err := json.Marshal(terms)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	docRaw, err := json.Marshal(doc)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	h.finishRaw(c, msg, hash, http.StatusOK, docRaw, database.ORPEffect{
		Offer: &database.ORPOffer{
			ID: offerID, OwnerID: ownerID, Party: msg.From, Terms: termsRaw,
			TermsHash: hashOfTerms, Limit: limit, CreatedAt: now.UTC(),
		},
		Inbox: []database.ORPInboxItem{{OwnerID: ownerID, Message: docRaw}},
	})
}

func (h *ORPHost) directedOffer(c *gin.Context, dns string, msg orp.Message, hash, ownerID, mode string, terms map[string]any, hashOfTerms string) {
	acceptorOwner, ok := orp.OwnerFromParty(dns, msg.To)
	if !ok || acceptorOwner == ownerID {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such party for this sender."}), database.ORPEffect{})
		return
	}
	exists, err := h.Entry.DB.OwnerExists(c.Request.Context(), acceptorOwner)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if !exists {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such party for this sender."}), database.ORPEffect{})
		return
	}
	if orp.Places(terms, "offerer") && mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("negotiate", "The owner has turned off placement publish, so this agent cannot offer a placement.")), database.ORPEffect{})
		return
	}
	acceptorMode, err := h.placementMode(c, acceptorOwner)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if orp.Places(terms, "acceptor") && acceptorMode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "That party is not taking this placement."}), database.ORPEffect{})
		return
	}
	if h.refuseRoom(c, dns, msg, hash, ownerID, acceptorOwner) {
		return
	}
	now := h.now()
	origin, _ := h.origin(c)
	reasons := holdReasons(acceptorMode, orp.Places(terms, "acceptor"))
	waiting := waitingOn(msg.To, "accept", reasons, origin, now)
	var bound []any
	if !(orp.Places(terms, "offerer") && mode == database.ModeAsk) {
		bound = []any{mandate(msg.From, now)}
	}
	deal := newTestDeal("", ownerID, acceptorOwner, msg.From, msg.To, terms, hashOfTerms, waiting, bound, now)
	h.commitDeal(c, dns, msg, hash, deal, now)
}

func (h *ORPHost) accept(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	if msg.Deal != "" {
		h.acceptDeal(c, dns, msg, hash, ownerID)
		return
	}
	offerID, _ := msg.Body["offer"].(string)
	given, _ := msg.Body["terms_hash"].(string)
	offer, found, err := h.Entry.DB.GetORPOffer(c.Request.Context(), offerID)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	var terms map[string]any
	var termsHash, offererParty, offererOwner string
	var offeredAt time.Time
	if found {
		terms, err = decodeObject(offer.Terms)
		if err != nil {
			h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
			return
		}
		termsHash = offer.TermsHash
		offererParty = offer.Party
		offererOwner = offer.OwnerID
		offeredAt = offer.CreatedAt
	} else {
		terms, termsHash, offererParty, found, err = h.catalogTerms(c, dns, offerID)
		if err != nil {
			h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
			return
		}
		if !found {
			h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such offer for this sender."}), database.ORPEffect{})
			return
		}
		offeredAt = h.now()
	}
	if given != termsHash {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "terms_mismatch", Message: "terms_hash does not match the terms."}), database.ORPEffect{})
		return
	}
	if offererParty == msg.From {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "A party cannot accept its own offer."}), database.ORPEffect{})
		return
	}
	mode, err := h.placementMode(c, ownerID)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if orp.Places(terms, "acceptor") && mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("accept", "The owner has turned off placement publish, so this agent cannot accept a placement.")), database.ORPEffect{})
		return
	}
	if existing, ok, err := h.Entry.DB.OpenORPDealForOffer(c.Request.Context(), offerID, ownerID); err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	} else if ok {
		_ = existing
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "This party already has a deal for this offer."}), database.ORPEffect{})
		return
	}
	if h.refuseRoom(c, dns, msg, hash, offererOwner, ownerID) {
		return
	}
	now := h.now()
	origin, _ := h.origin(c)
	waiting := waitingOn(msg.From, "accept", holdReasons(mode, orp.Places(terms, "acceptor")), origin, now)
	bound := []any{mandate(offererParty, offeredAt)}
	deal := newTestDeal(offerID, offererOwner, ownerID, offererParty, msg.From, terms, termsHash, waiting, bound, now)
	h.commitDeal(c, dns, msg, hash, deal, now)
}

func (h *ORPHost) acceptDeal(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	deal, found, err := h.Entry.DB.GetORPDeal(c.Request.Context(), msg.Deal)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	party := orp.MemberParty(dns, ownerID)
	if !found || (deal.OffererParty != party && deal.AcceptorParty != party) {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}), database.ORPEffect{})
		return
	}
	given, _ := msg.Body["terms_hash"].(string)
	if given != deal.TermsHash {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "terms_mismatch", Message: "terms_hash does not match the terms."}), database.ORPEffect{})
		return
	}
	if deal.State != "awaiting" && deal.State != "proposed" {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}), database.ORPEffect{})
		return
	}
	mode, err := h.placementMode(c, ownerID)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	role := "acceptor"
	if deal.OffererParty == party {
		role = "offerer"
	}
	if orp.Places(terms, role) && mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("accept", "The owner has turned off placement publish, so this agent cannot accept a placement.")), database.ORPEffect{})
		return
	}
	doc, err := h.dealMessage(dns, party, msg.ID, deal, h.now())
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	h.finishRaw(c, msg, hash, http.StatusOK, raw, database.ORPEffect{
		Inbox: h.dealInbox(dns, msg.ID, deal, h.now()),
	})
}

func (h *ORPHost) execute(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	deal, found, err := h.Entry.DB.GetORPDeal(c.Request.Context(), msg.Deal)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	party := orp.MemberParty(dns, ownerID)
	if !found || (deal.OffererParty != party && deal.AcceptorParty != party) {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}), database.ORPEffect{})
		return
	}
	if deal.State != "awaiting" {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}), database.ORPEffect{})
		return
	}
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	obligationID, _ := msg.Body["obligation"].(string)
	obligation := orp.ObligationMap(terms, obligationID)
	if obligation == nil {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such obligation for this sender."}), database.ORPEffect{})
		return
	}
	role, _ := obligation["by"].(string)
	var obligor string
	if role == "offerer" {
		obligor = deal.OffererParty
	} else {
		obligor = deal.AcceptorParty
	}
	if obligor != party {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "Only the obligor may claim this obligation."}), database.ORPEffect{})
		return
	}
	if obligation["action"] == "pay" {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "This testing host does not settle money."}), database.ORPEffect{})
		return
	}
	if !orp.ProofAllows(obligation, "proof_url") {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "This obligation does not accept proof_url."}), database.ORPEffect{})
		return
	}
	origin, _ := h.origin(c)
	want := orp.ProofURL(origin, deal.ID)
	if msg.Body["proof_url"] != want {
		h.finish(c, msg, hash, http.StatusBadRequest, h.errorDoc(dns, msg, orp.Invalid("The proof URL must be this host's test proof page.", "/body/proof_url")), database.ORPEffect{})
		return
	}
	mode, err := h.placementMode(c, ownerID)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if obligation["action"] == "place" && mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("execute", "The owner has turned off placement publish, so this agent cannot prove a placement.")), database.ORPEffect{})
		return
	}
	now := h.now()
	receiptBody := map[string]any{
		"obligation": obligationID,
		"result":     "verified",
		"method":     "proof_url",
		"at":         orp.Stamp(now),
		"url":        want,
		"found":      map[string]any{"disclosure": orp.DisclosureLabel(terms) != ""},
	}
	receiptID := ""
	obligations, err := markVerified(deal.Obligations, obligationID, &receiptID, dns, deal, msg.ID, receiptBody, now)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	deal.Obligations = obligations
	deal.UpdatedAt = now.UTC()
	doc, err := h.dealMessage(dns, party, msg.ID, deal, now)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	inbox := h.dealInbox(dns, msg.ID, deal, now)
	inbox = append(inbox, h.receiptInbox(dns, msg.ID, deal, receiptBody, receiptID, now)...)
	h.finishRaw(c, msg, hash, http.StatusOK, raw, database.ORPEffect{
		UpdateDealID:      deal.ID,
		UpdateObligations: obligations,
		UpdatedAt:         now.UTC(),
		Inbox:             inbox,
	})
}

func (h *ORPHost) commitDeal(c *gin.Context, dns string, msg orp.Message, hash string, deal database.ORPDeal, now time.Time) {
	doc, err := h.dealMessage(dns, msg.From, msg.ID, deal, now)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	saved, err := h.store(c, msg, hash, http.StatusOK, raw, database.ORPEffect{
		Deal:  &deal,
		Inbox: h.dealInbox(dns, msg.ID, deal, now),
	})
	if err != nil {
		if isUnique(err) {
			h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "This party already has a deal for this offer."}), database.ORPEffect{})
			return
		}
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	writeORPBytes(c, saved.Status, saved.Reply)
}

func (h *ORPHost) standingItems(c *gin.Context, text string) ([]any, error) {
	if h.Entry == nil || h.Entry.DB == nil {
		return nil, errors.New("no database")
	}
	offers, err := h.Entry.DB.ListORPOffers(c.Request.Context(), 50)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0)
	for _, offer := range offers {
		terms, err := decodeObject(offer.Terms)
		if err != nil || !orp.OfferMatches(terms, text) {
			continue
		}
		reason := "Standing test offer: " + orp.TermsTitle(terms) + "."
		if utf8.RuneCountInString(reason) > 200 {
			reason = string([]rune(reason)[:200])
		}
		items = append(items, map[string]any{
			"offer": map[string]any{
				"id":         offer.ID,
				"terms":      terms,
				"terms_hash": offer.TermsHash,
			},
			"party": offer.Party,
			"fit": map[string]any{
				"score":   70,
				"reasons": []any{reason},
			},
			"pool":   "test",
			"checks": map[string]any{"claims": "not_checked"},
		})
	}
	return items, nil
}

func (h *ORPHost) catalogTerms(c *gin.Context, dns, id string) (map[string]any, string, string, bool, error) {
	rows, err := h.candidates(c)
	if err != nil {
		return nil, "", "", false, err
	}
	for _, row := range rows {
		if row.ID != id {
			continue
		}
		terms, hash, ok := orp.CatalogTerms(row)
		if !ok {
			return nil, "", "", false, nil
		}
		return terms, hash, orp.CatalogParty(dns, row.Source, row.ID), true, nil
	}
	return nil, "", "", false, nil
}

func (h *ORPHost) refuseRoom(c *gin.Context, dns string, msg orp.Message, hash string, owners ...string) bool {
	blocked, problem := h.roomFor(c, owners...)
	if !blocked {
		return false
	}
	if problem.Code == "internal" {
		h.writeProblem(c, dns, msg.From, msg.ID, problem, false)
		return true
	}
	h.finish(c, msg, hash, problem.Status, h.errorDoc(dns, msg, problem), database.ORPEffect{})
	return true
}

func (h *ORPHost) roomFor(c *gin.Context, owners ...string) (bool, orp.Problem) {
	seen := map[string]bool{}
	for _, ownerID := range owners {
		if ownerID == "" || seen[ownerID] {
			continue
		}
		seen[ownerID] = true
		n, err := h.Entry.DB.CountOpenORPDeals(c.Request.Context(), ownerID)
		if err != nil {
			return true, internalProblem()
		}
		if n >= openTestDeals {
			return true, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "This party is limited to 5 open test deals."}
		}
	}
	return false, orp.Problem{}
}

func (h *ORPHost) placementMode(c *gin.Context, ownerID string) (string, error) {
	if h.Entry == nil || h.Entry.DB == nil || ownerID == "" {
		return database.ModeAsk, nil
	}
	modes, err := h.Entry.DB.LoadStepModes(c.Request.Context(), ownerID, "")
	if err != nil {
		return "", err
	}
	return modes.Get(database.StepPlacementPublish), nil
}

func (h *ORPHost) bearer(c *gin.Context, dns string) (*database.AgentGrant, bool) {
	grant, err := h.Entry.JoinGrant(c)
	if err != nil {
		h.writeProblem(c, dns, "", "", orp.Problem{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "No credential or a bad credential."}, false)
		return nil, false
	}
	return grant, true
}

func (h *ORPHost) seen(c *gin.Context, dns string, msg orp.Message, hash string) bool {
	saved, found, err := h.Entry.DB.LookupORPMessage(c.Request.Context(), msg.From, msg.ID, hash)
	if errors.Is(err, database.ErrORPReplay) {
		h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusConflict, Code: "replay", Message: "This message id was already used with different content."}, false)
		return true
	}
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return true
	}
	if found {
		writeORPBytes(c, saved.Status, saved.Reply)
		return true
	}
	return false
}

func (h *ORPHost) finish(c *gin.Context, msg orp.Message, hash string, status int, doc any, effect database.ORPEffect) {
	raw, err := json.Marshal(doc)
	if err != nil {
		_, dns := h.origin(c)
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	h.finishRaw(c, msg, hash, status, raw, effect)
}

func (h *ORPHost) finishRaw(c *gin.Context, msg orp.Message, hash string, status int, raw []byte, effect database.ORPEffect) {
	saved, err := h.store(c, msg, hash, status, raw, effect)
	if err != nil {
		_, dns := h.origin(c)
		if errors.Is(err, database.ErrORPReplay) {
			h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusConflict, Code: "replay", Message: "This message id was already used with different content."}, false)
			return
		}
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	writeORPBytes(c, saved.Status, saved.Reply)
}

func (h *ORPHost) store(c *gin.Context, msg orp.Message, hash string, status int, raw []byte, effect database.ORPEffect) (database.ORPSaved, error) {
	return h.Entry.DB.CommitORP(c.Request.Context(), database.ORPRecord{
		Party: msg.From, ID: msg.ID, Hash: hash, Status: status, Reply: raw,
	}, effect)
}

func (h *ORPHost) errorDoc(dns string, msg orp.Message, problem orp.Problem) map[string]any {
	return orp.ErrorDocument(dns, msg.From, msg.ID, problem, h.now())
}

func (h *ORPHost) dealMessage(dns, viewer, re string, deal database.ORPDeal, now time.Time) (map[string]any, error) {
	body, err := dealBody(deal, now)
	if err != nil {
		return nil, err
	}
	return orp.DealMessage(dns, viewer, re, deal.ID, orp.DealView(body, viewer), now), nil
}

func (h *ORPHost) dealInbox(dns, re string, deal database.ORPDeal, now time.Time) []database.ORPInboxItem {
	return []database.ORPInboxItem{
		h.inboxDeal(dns, deal.OffererParty, deal.OffererOwner, re, deal, now),
		h.inboxDeal(dns, deal.AcceptorParty, deal.AcceptorOwner, re, deal, now),
	}
}

func (h *ORPHost) inboxDeal(dns, party, owner, re string, deal database.ORPDeal, now time.Time) database.ORPInboxItem {
	if owner == "" {
		return database.ORPInboxItem{}
	}
	doc, err := h.dealMessage(dns, party, re, deal, now)
	if err != nil {
		return database.ORPInboxItem{}
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return database.ORPInboxItem{}
	}
	return database.ORPInboxItem{OwnerID: owner, Message: raw}
}

func (h *ORPHost) receiptInbox(dns, re string, deal database.ORPDeal, body map[string]any, receiptID string, now time.Time) []database.ORPInboxItem {
	return []database.ORPInboxItem{
		receiptItem(dns, deal.OffererParty, deal.OffererOwner, re, deal.ID, body, receiptID, now),
		receiptItem(dns, deal.AcceptorParty, deal.AcceptorOwner, re, deal.ID, body, receiptID, now),
	}
}

func receiptItem(dns, party, owner, re, dealID string, body map[string]any, receiptID string, now time.Time) database.ORPInboxItem {
	if owner == "" {
		return database.ORPInboxItem{}
	}
	doc := orp.ReceiptMessage(dns, party, re, dealID, body, now)
	if receiptID != "" {
		doc["id"] = receiptID
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return database.ORPInboxItem{}
	}
	return database.ORPInboxItem{OwnerID: owner, Message: raw}
}

func markVerified(raw json.RawMessage, obligationID string, receiptID *string, dns string, deal database.ORPDeal, re string, body map[string]any, now time.Time) (json.RawMessage, error) {
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	doc := orp.ReceiptMessage(dns, deal.AcceptorParty, re, deal.ID, body, now)
	*receiptID = doc["id"].(string)
	found := false
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		if row["id"] == obligationID {
			row["state"] = "verified"
			row["receipt"] = *receiptID
			found = true
		}
		out = append(out, row)
	}
	if !found {
		return nil, errors.New("obligation row missing")
	}
	return json.Marshal(out)
}

func newTestDeal(offerID, offererOwner, acceptorOwner, offererParty, acceptorParty string, terms map[string]any, termsHash string, waiting map[string]any, bound []any, now time.Time) database.ORPDeal {
	termsRaw, _ := json.Marshal(terms)
	waitingRaw, _ := json.Marshal(waiting)
	boundRaw, _ := json.Marshal(bound)
	obligations, _ := json.Marshal(orp.PendingObligations(terms))
	return database.ORPDeal{
		ID:            orp.LocalID("deal_"),
		OfferID:       offerID,
		OffererOwner:  offererOwner,
		AcceptorOwner: acceptorOwner,
		OffererParty:  offererParty,
		AcceptorParty: acceptorParty,
		Terms:         termsRaw,
		TermsHash:     termsHash,
		State:         "awaiting",
		Waiting:       waitingRaw,
		Obligations:   obligations,
		Bound:         boundRaw,
		UpdatedAt:     now.UTC(),
		CreatedAt:     now.UTC(),
	}
}

func dealBody(deal database.ORPDeal, now time.Time) (map[string]any, error) {
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"state":      deal.State,
		"offerer":    deal.OffererParty,
		"acceptor":   deal.AcceptorParty,
		"terms":      terms,
		"terms_hash": deal.TermsHash,
		"updated":    orp.Stamp(deal.UpdatedAt),
	}
	if deal.UpdatedAt.IsZero() {
		body["updated"] = orp.Stamp(now)
	}
	if deal.OfferID != "" {
		body["offer"] = deal.OfferID
	}
	if len(deal.Bound) > 0 && string(deal.Bound) != "null" && string(deal.Bound) != "[]" {
		var bound any
		if err = json.Unmarshal(deal.Bound, &bound); err != nil {
			return nil, err
		}
		body["bound"] = bound
	}
	if len(deal.Obligations) > 0 {
		var obligations any
		if err = json.Unmarshal(deal.Obligations, &obligations); err != nil {
			return nil, err
		}
		body["obligations"] = obligations
	}
	if len(deal.Waiting) > 0 && string(deal.Waiting) != "null" {
		waiting, err := decodeObject(deal.Waiting)
		if err != nil {
			return nil, err
		}
		body["waiting"] = waiting
	}
	return body, nil
}

func waitingOn(party, step string, reasons []any, origin string, now time.Time) map[string]any {
	esc := orp.LocalID("esc_")
	return map[string]any{
		"party":      party,
		"step":       step,
		"since":      orp.Stamp(now),
		"escalation": esc,
		"reasons":    reasons,
		"url":        orp.ApproveURL(origin, esc),
	}
}

func holdReasons(mode string, places bool) []any {
	reasons := []any{"claims_unchecked"}
	if places && mode == database.ModeAsk {
		reasons = []any{"step_ask", "claims_unchecked"}
	}
	return reasons
}

func mandate(party string, at time.Time) map[string]any {
	return map[string]any{
		"party":     party,
		"authority": map[string]any{"type": "mandate"},
		"at":        orp.Stamp(at),
	}
}

func stepOff(step, message string) orp.Problem {
	return orp.Problem{Status: http.StatusForbidden, Code: "step_off", Message: message, Step: step}
}

func wholeLimit(value any) (int, bool) {
	switch n := value.(type) {
	case float64:
		if n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	case int:
		return n, true
	default:
		return 0, false
	}
}

func decodeObject(raw json.RawMessage) (map[string]any, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func writeORPBytes(c *gin.Context, status int, body []byte) {
	c.Header("Cache-Control", "no-store")
	c.Data(status, "application/json; charset=utf-8", body)
}

func writeHTML(c *gin.Context, status int, body string) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Robots-Tag", "noindex")
	c.Data(status, "text/html; charset=utf-8", []byte(body))
}
