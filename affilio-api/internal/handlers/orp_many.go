package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

func (h *ORPHost) offer04(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	if confirm, _ := msg.Body["confirm"].(bool); confirm {
		h.confirmOffer(c, dns, msg, hash, ownerID)
		return
	}
	terms := msg.Body["terms"].(map[string]any)
	if raw, ok := msg.Body["parties"].([]any); ok {
		h.offerMany(c, dns, msg, hash, ownerID, terms, raw)
		return
	}
	h.offerFresh(c, dns, msg, hash, ownerID, terms)
}

func (h *ORPHost) offerFresh(c *gin.Context, dns string, msg orp.Message, hash, ownerID string, terms map[string]any) {
	given, _ := msg.Body["terms_hash"].(string)
	if given != "" && given != orp.TermsHash(terms) {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "terms_mismatch", Message: "terms_hash does not match the terms."}), database.ORPEffect{})
		return
	}
	mode, err := h.placementMode(c, ownerID)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	if mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("list", "The owner has turned off placement publish, so this agent cannot list a placement.")), database.ORPEffect{})
		return
	}
	now := h.now()
	stored := orp.NormalizeClaims(terms, now)
	hashOfTerms := orp.TermsHash(stored)
	limit := openTestDeals
	if n, ok := wholeLimit(msg.Body["limit"]); ok {
		limit = n
	}
	offerID := orp.LocalID("ofr_")
	body := map[string]any{"id": offerID, "terms": stored, "terms_hash": hashOfTerms}
	if _, present := msg.Body["limit"]; present {
		body["limit"] = limit
	}
	if note, _ := msg.Body["note"].(string); note != "" {
		body["note"] = note
	}
	listed := mode == database.ModeAuto
	if !listed {
		origin := h.publicOrigin(dns)
		body["waiting"] = waitingOn(msg.From, "list", []any{"step_ask", "claims_unchecked"}, origin, now)
	}
	doc := orp.ListedOffer(dns, msg.From, msg.ID, body, now)
	doc["orp"] = orp.Version04
	termsRaw, err := json.Marshal(stored)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	docRaw, err := json.Marshal(doc)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	h.finishRaw(c, msg, hash, http.StatusOK, docRaw, database.ORPEffect{
		Offer: &database.ORPOffer{
			ID: offerID, OwnerID: ownerID, Party: msg.From, Terms: termsRaw,
			TermsHash: hashOfTerms, Limit: limit, CreatedAt: now.UTC(), Listed: listed,
		},
		Inbox: []database.ORPInboxItem{{OwnerID: ownerID, Message: docRaw}},
	})
}

func (h *ORPHost) confirmOffer(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	id, _ := msg.Body["id"].(string)
	offer, found, err := h.Entry.DB.GetORPOfferOwned(c.Request.Context(), id, ownerID)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	if !found {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such offer for this sender."}), database.ORPEffect{})
		return
	}
	terms, err := decodeObject(offer.Terms)
	if err != nil || !orp.HasCheckedAt(terms) {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}), database.ORPEffect{})
		return
	}
	now := h.now()
	stored := orp.NormalizeClaims(terms, now)
	hashOfTerms := orp.TermsHash(stored)
	termsRaw, err := json.Marshal(stored)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	body := map[string]any{"id": offer.ID, "terms": stored, "terms_hash": hashOfTerms}
	doc := orp.ListedOffer(dns, msg.From, msg.ID, body, now)
	doc["orp"] = orp.Version04
	docRaw, err := json.Marshal(doc)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	offer.Terms = termsRaw
	offer.TermsHash = hashOfTerms
	offer.CreatedAt = now.UTC()
	h.finishRaw(c, msg, hash, http.StatusOK, docRaw, database.ORPEffect{
		OfferUpdate: &offer,
		Inbox:       []database.ORPInboxItem{{OwnerID: ownerID, Message: docRaw}},
	})
}

func (h *ORPHost) offerMany(c *gin.Context, dns string, msg orp.Message, hash, ownerID string, terms map[string]any, raw []any) {
	if problem, bad := orp.ValidateParties(raw, orp.HostParty(dns)); bad {
		h.finish(c, msg, hash, problem.Status, h.errorDoc(dns, msg, problem), database.ORPEffect{})
		return
	}
	seats, owners, problem, ok := h.seatsFrom(c, dns, raw)
	if !ok {
		if problem.Code == "internal" {
			h.failOpen(c, dns, msg, problem)
			return
		}
		h.finish(c, msg, hash, problem.Status, h.errorDoc(dns, msg, problem), database.ORPEffect{})
		return
	}
	if !seatPresent(seats, msg.From) {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "The sender is not a party on this deal."}), database.ORPEffect{})
		return
	}
	hashed := orp.DealHash(terms, seats)
	if given, _ := msg.Body["terms_hash"].(string); given != "" && given != hashed {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "terms_mismatch", Message: "terms_hash does not match the terms."}), database.ORPEffect{})
		return
	}
	mode, err := h.placementMode(c, ownerID)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	if mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("negotiate", "The owner has turned off placement publish, so this agent cannot offer a placement.")), database.ORPEffect{})
		return
	}
	if h.refuseRoom(c, dns, msg, hash, owners...) {
		return
	}
	now := h.now()
	doc := orp.Many{Seats: seats, Round: 1}
	deal := h.newManyDeal(dns, "", terms, hashed, nil, nil, doc, now)
	if mode == database.ModeAuto {
		h.signSeat(&deal, msg.From, now)
		h.refreshMany(&deal, terms, now)
	} else {
		// A fresh join has no saved step row, so placement publish is ask.
		// That is the 0.3 list path: the offer waits, the offerer is not
		// signed, and nothing is bound. Step negotiate signed nobody and
		// left bound nil, which the NOT NULL column rejected.
		h.holdMany(&deal, msg.From, "list", []any{"step_ask"}, now)
	}
	h.insertMany(c, dns, msg, hash, deal, now)
}

func (h *ORPHost) accept04(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) bool {
	if msg.Deal != "" {
		deal, found, err := h.Entry.DB.GetORPDeal(c.Request.Context(), msg.Deal)
		if err != nil {
			h.failOpen(c, dns, msg, internalProblem())
			return true
		}
		if found && len(deal.Parties) > 0 {
			h.acceptMany(c, dns, msg, hash, ownerID, deal)
			return true
		}
		return false
	}
	offerID, _ := msg.Body["offer"].(string)
	offer, found, err := h.Entry.DB.GetORPOffer(c.Request.Context(), offerID)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return true
	}
	if !found {
		return false
	}
	terms, err := decodeObject(offer.Terms)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return true
	}
	if !orp.HasCheckedAt(terms) {
		return false
	}
	h.acceptFresh(c, dns, msg, hash, ownerID, offer, terms)
	return true
}

func (h *ORPHost) acceptFresh(c *gin.Context, dns string, msg orp.Message, hash, ownerID string, offer database.ORPOffer, terms map[string]any) {
	given, _ := msg.Body["terms_hash"].(string)
	if given != offer.TermsHash {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "terms_mismatch", Message: "terms_hash does not match the terms."}), database.ORPEffect{})
		return
	}
	if offer.Party == msg.From {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "A party cannot accept its own offer."}), database.ORPEffect{})
		return
	}
	if orp.OfferExpired(terms, h.now()) {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such offer for this sender."}), database.ORPEffect{})
		return
	}
	mode, err := h.placementMode(c, ownerID)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	if mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("accept", "The owner has turned off placement publish, so this agent cannot accept a placement.")), database.ORPEffect{})
		return
	}
	if existing, ok, err := h.Entry.DB.OpenORPDealForOffer(c.Request.Context(), offer.ID, ownerID); err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	} else if ok {
		_ = existing
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "This party already has a deal for this offer."}), database.ORPEffect{})
		return
	}
	if h.refuseRoom(c, dns, msg, hash, offer.OwnerID, ownerID) {
		return
	}
	raw := []any{
		map[string]any{"id": offer.Party, "role": "offerer", "required": true},
		map[string]any{"id": msg.From, "role": "acceptor", "required": true},
	}
	seats, _, problem, ok := h.seatsFrom(c, dns, raw)
	if !ok {
		h.finish(c, msg, hash, problem.Status, h.errorDoc(dns, msg, problem), database.ORPEffect{})
		return
	}
	now := h.now()
	hashed := orp.DealHash(terms, seats)
	doc := orp.Many{Seats: seats, Round: 1}
	if offer.Listed {
		h.signSeatDoc(&doc, offer.Party, now)
	}
	deal := h.newManyDeal(dns, offer.ID, terms, hashed, nil, nil, doc, now)
	if offer.Listed {
		deal.Bound = rawJSON([]any{mandate(offer.Party, offer.CreatedAt)})
	}
	if mode == database.ModeAuto {
		h.signSeat(&deal, msg.From, now)
		h.refreshMany(&deal, terms, now)
	} else {
		h.holdMany(&deal, msg.From, "accept", manyReasons(mode), now)
	}
	h.insertMany(c, dns, msg, hash, deal, now)
}

func (h *ORPHost) acceptMany(c *gin.Context, dns string, msg orp.Message, hash, ownerID string, deal database.ORPDeal) {
	doc, err := orp.DecodeMany(deal.Parties)
	if err != nil || !seatPresent(doc.Seats, msg.From) {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}), database.ORPEffect{})
		return
	}
	given, _ := msg.Body["terms_hash"].(string)
	if given != deal.TermsHash {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "terms_mismatch", Message: "terms_hash does not match the terms."}), database.ORPEffect{})
		return
	}
	if seatSigned(doc.Seats, msg.From) {
		h.publishMany(c, dns, msg, hash, deal, doc, "")
		return
	}
	if deal.State != "pending" && deal.State != "awaiting" && deal.State != "active" {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}), database.ORPEffect{})
		return
	}
	mode, err := h.placementMode(c, ownerID)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	if mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("accept", "The owner has turned off placement publish, so this agent cannot accept a placement.")), database.ORPEffect{})
		return
	}
	expectHash, expectAt := deal.TermsHash, deal.UpdatedAt
	now := h.now()
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	if mode == database.ModeAsk {
		h.holdMany(&deal, msg.From, "accept", manyReasons(mode), now)
	} else {
		h.signSeat(&deal, msg.From, now)
		h.refreshMany(&deal, terms, now)
	}
	h.publishMany(c, dns, msg, hash, deal, mustMany(deal), expectHash, expectAt)
}

func (h *ORPHost) leave04(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	deal, doc, ok := h.loadMany(c, dns, msg, hash)
	if !ok {
		return
	}
	seat, found := findSeat(doc.Seats, msg.From)
	if !found {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}), database.ORPEffect{})
		return
	}
	if deal.State != "pending" && deal.State != "awaiting" && deal.State != "active" {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}), database.ORPEffect{})
		return
	}
	if len(doc.Seats) <= 2 {
		h.finish(c, msg, hash, http.StatusBadRequest, h.errorDoc(dns, msg, orp.Invalid("A deal keeps at least 2 parties.", "/body/parties")), database.ORPEffect{})
		return
	}
	remaining := make([]SeatView, 0, len(doc.Seats)-1)
	nextSeats := make([]orp.Seat, 0, len(doc.Seats)-1)
	for _, item := range doc.Seats {
		if item.ID == msg.From {
			continue
		}
		nextSeats = append(nextSeats, item)
		remaining = append(remaining, seatView(item))
	}
	if problem, bad := orp.ValidateParties(asParties(remaining), orp.HostParty(dns)); bad {
		h.finish(c, msg, hash, problem.Status, h.errorDoc(dns, msg, problem), database.ORPEffect{})
		return
	}
	expectHash, expectAt := deal.TermsHash, deal.UpdatedAt
	doc.Seats = nextSeats
	if seat.Required {
		doc.Vacancies = append(doc.Vacancies, orp.Vacancy{ID: orp.LocalID("vac_"), Role: seat.Role})
	}
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	now := h.now()
	deal.TermsHash = orp.DealHash(terms, doc.Seats)
	deal.Bound = dropBound(deal.Bound, msg.From)
	h.putMany(&deal, doc)
	h.refreshMany(&deal, terms, now)
	_ = ownerID
	h.publishMany(c, dns, msg, hash, deal, mustMany(deal), expectHash, expectAt)
}

func (h *ORPHost) seat04(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	deal, doc, ok := h.loadMany(c, dns, msg, hash)
	if !ok {
		return
	}
	if !seatPresent(doc.Seats, msg.From) {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}), database.ORPEffect{})
		return
	}
	if deal.State != "pending" && deal.State != "awaiting" && deal.State != "active" {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}), database.ORPEffect{})
		return
	}
	seatRaw, _ := msg.Body["seat"].(map[string]any)
	joined := append(asParties(seatsView(doc.Seats)), seatRaw)
	if problem, bad := orp.ValidateParties(joined, orp.HostParty(dns)); bad {
		h.finish(c, msg, hash, problem.Status, h.errorDoc(dns, msg, problem), database.ORPEffect{})
		return
	}
	id, _ := seatRaw["id"].(string)
	if seatPresent(doc.Seats, id) {
		h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}), database.ORPEffect{})
		return
	}
	seats, _, problem, ready := h.seatsFrom(c, dns, []any{seatRaw})
	if !ready {
		if problem.Code == "internal" {
			h.failOpen(c, dns, msg, problem)
			return
		}
		h.finish(c, msg, hash, problem.Status, h.errorDoc(dns, msg, problem), database.ORPEffect{})
		return
	}
	next := seats[0]
	next.Signed = false
	if next.Required {
		taken := map[string]bool{}
		for _, seat := range doc.Seats {
			if seat.Fills != "" {
				taken[seat.Fills] = true
			}
		}
		for _, vacancy := range doc.Vacancies {
			if vacancy.Role == next.Role && !taken[vacancy.ID] {
				next.Fills = vacancy.ID
				break
			}
		}
	}
	expectHash, expectAt := deal.TermsHash, deal.UpdatedAt
	doc.Seats = append(doc.Seats, next)
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	now := h.now()
	deal.TermsHash = orp.DealHash(terms, doc.Seats)
	h.putMany(&deal, doc)
	h.refreshMany(&deal, terms, now)
	_ = ownerID
	h.publishMany(c, dns, msg, hash, deal, mustMany(deal), expectHash, expectAt)
}

// ApprovePost signs the waiting party on a 0.4 deal. GET /orp/approve/:id
// still does not approve. A 0.3 deal is left untouched.
func (h *ORPHost) ApprovePost(c *gin.Context) {
	_, dns := h.origin(c)
	grant, ok := h.bearer(c, dns)
	if !ok {
		return
	}
	deal, found, err := h.Entry.DB.GetORPDealByEscalation(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeProblem(c, dns, "", "", internalProblem(), false)
		return
	}
	if !found {
		h.writeProblem(c, dns, "", "", orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}, false)
		return
	}
	if len(deal.Parties) == 0 {
		h.writeProblem(c, dns, "", "", orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "This page does not approve."}, false)
		return
	}
	party := orp.MemberParty(dns, grant.OwnerID)
	waiting, err := decodeObject(deal.Waiting)
	if err != nil {
		h.writeProblem(c, dns, party, "", internalProblem(), false)
		return
	}
	if waiting["party"] != party {
		h.writeProblem(c, dns, party, "", orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "Only the waiting party may approve."}, false)
		return
	}
	expectHash, expectAt := deal.TermsHash, deal.UpdatedAt
	now := h.now()
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		h.writeProblem(c, dns, party, "", internalProblem(), false)
		return
	}
	h.signSeat(&deal, party, now)
	h.refreshMany(&deal, terms, now)
	msg := orp.Message{ORP: orp.Version04, Type: "accept", ID: orp.NewMessageID(), From: party}
	bodyHash := orp.ContentHash(map[string]any{"approve": c.Param("id")})
	h.publishMany(c, dns, msg, bodyHash, deal, mustMany(deal), expectHash, expectAt)
}

// Go redirects a tracked 0.4 link and attributes the click by split.
func (h *ORPHost) Go(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex")
	origin, _ := h.origin(c)
	token := c.Param("token")
	purpose := c.GetHeader("Sec-Purpose") + " " + c.GetHeader("Purpose")
	class := orp.ClickClass(c.Request.Method, c.GetHeader("User-Agent"), purpose, c.Request.URL.RawQuery)
	clickID := orp.LocalID("clk_")
	// HEAD is class preview. It redirects with the same headers and does not record.
	record := !strings.EqualFold(c.Request.Method, http.MethodHead)
	dest, param, alias, found, err := h.Entry.DB.RecordORPClick(c.Request.Context(), token, class, clickID, h.now(), record)
	if err != nil || !found {
		c.Redirect(http.StatusFound, strings.TrimRight(origin, "/")+"/")
		return
	}
	target, err := url.Parse(dest)
	if err != nil || target.Scheme == "" || target.Host == "" {
		c.Redirect(http.StatusFound, strings.TrimRight(origin, "/")+"/")
		return
	}
	q := target.Query()
	if param == "" {
		param = "via"
	}
	q.Add(param, clickID)
	if alias != "" {
		q.Add(alias, clickID)
	}
	target.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, target.String())
}

func (h *ORPHost) loadMany(c *gin.Context, dns string, msg orp.Message, hash string) (database.ORPDeal, orp.Many, bool) {
	deal, found, err := h.Entry.DB.GetORPDeal(c.Request.Context(), msg.Deal)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return database.ORPDeal{}, orp.Many{}, false
	}
	if !found || len(deal.Parties) == 0 {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}), database.ORPEffect{})
		return database.ORPDeal{}, orp.Many{}, false
	}
	doc, err := orp.DecodeMany(deal.Parties)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return database.ORPDeal{}, orp.Many{}, false
	}
	return deal, doc, true
}

func (h *ORPHost) seatsFrom(c *gin.Context, dns string, raw []any) ([]orp.Seat, []string, orp.Problem, bool) {
	seats := make([]orp.Seat, 0, len(raw))
	owners := make([]string, 0, len(raw))
	for _, item := range raw {
		party := item.(map[string]any)
		id, _ := party["id"].(string)
		owner, ok := orp.OwnerFromParty(dns, id)
		if !ok {
			return nil, nil, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such party for this sender."}, false
		}
		exists, err := h.Entry.DB.OwnerExists(c.Request.Context(), owner)
		if err != nil {
			return nil, nil, internalProblem(), false
		}
		if !exists {
			return nil, nil, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such party for this sender."}, false
		}
		seat := orp.Seat{ID: id, Owner: owner, Role: party["role"].(string), Required: party["required"].(bool)}
		if terms, ok := party["terms"].(map[string]any); ok && len(terms) > 0 {
			seat.Terms = terms
		}
		if identity, ok := party["identity"].(map[string]any); ok && len(identity) > 0 {
			seat.Identity = identity
		}
		seats = append(seats, seat)
		owners = append(owners, owner)
	}
	return seats, owners, orp.Problem{}, true
}

func (h *ORPHost) newManyDeal(dns, offerID string, terms map[string]any, hash string, waiting map[string]any, bound []any, doc orp.Many, now time.Time) database.ORPDeal {
	termsRaw, _ := json.Marshal(terms)
	waitingRaw, _ := json.Marshal(waiting)
	boundRaw, _ := json.Marshal(bound)
	obligations, _ := json.Marshal(orp.PendingObligations(terms))
	parties, _ := json.Marshal(doc)
	deal := database.ORPDeal{
		ID:          orp.LocalID("deal_"),
		OfferID:     offerID,
		Terms:       termsRaw,
		TermsHash:   hash,
		State:       "pending",
		Waiting:     waitingRaw,
		Obligations: obligations,
		Bound:       boundRaw,
		Parties:     parties,
		UpdatedAt:   now.UTC(),
		CreatedAt:   now.UTC(),
	}
	h.syncColumns(&deal, doc)
	// bound is JSONB NOT NULL. An empty signature set is [], not SQL NULL.
	// nullJSON turns nil into NULL and the insert fails, which is why an
	// ask-mode offer (nobody signed yet) returned HTTP 500. Auto mode hid
	// it by writing the offerer's mandate first. The snapshot omits [].
	if len(bound) == 0 || string(deal.Bound) == "null" {
		deal.Bound = json.RawMessage(`[]`)
	}
	if waiting == nil {
		deal.Waiting = nil
	}
	return deal
}

func (h *ORPHost) syncColumns(deal *database.ORPDeal, doc orp.Many) {
	deal.OffererParty = ""
	deal.AcceptorParty = ""
	deal.OffererOwner = ""
	fallback := ""
	acceptor := ""
	for _, seat := range doc.Seats {
		if fallback == "" {
			fallback = seat.Owner
		}
		if seat.Role == "offerer" {
			deal.OffererParty = seat.ID
			deal.OffererOwner = seat.Owner
		}
		if seat.Role == "acceptor" {
			deal.AcceptorParty = seat.ID
			acceptor = seat.Owner
		}
	}
	if acceptor == "" {
		acceptor = fallback
	}
	deal.AcceptorOwner = acceptor
}

func (h *ORPHost) putMany(deal *database.ORPDeal, doc orp.Many) {
	if len(doc.Vacancies) == 0 {
		doc.Vacancies = nil
	}
	raw, _ := json.Marshal(doc)
	deal.Parties = raw
	h.syncColumns(deal, doc)
}

func (h *ORPHost) signSeat(deal *database.ORPDeal, party string, now time.Time) {
	doc := mustMany(*deal)
	if !h.signSeatDoc(&doc, party, now) {
		return
	}
	rows := decodeBound(deal.Bound)
	rows = append(rows, mandate(party, now))
	sort.Slice(rows, func(i, j int) bool {
		return rows[i]["party"].(string) < rows[j]["party"].(string)
	})
	deal.Bound = rawJSON(rows)
	h.putMany(deal, doc)
}

func (h *ORPHost) signSeatDoc(doc *orp.Many, party string, now time.Time) bool {
	_ = now
	for i := range doc.Seats {
		if doc.Seats[i].ID != party || doc.Seats[i].Signed {
			continue
		}
		doc.Seats[i].Signed = true
		if doc.Seats[i].Fills != "" {
			kept := doc.Vacancies[:0]
			for _, vacancy := range doc.Vacancies {
				if vacancy.ID != doc.Seats[i].Fills {
					kept = append(kept, vacancy)
				}
			}
			doc.Vacancies = kept
		}
		return true
	}
	return false
}

func (h *ORPHost) refreshMany(deal *database.ORPDeal, terms map[string]any, now time.Time) {
	doc := mustMany(*deal)
	next := orp.SignatureState(doc.Seats, doc.Vacancies)
	origin := h.publicOrigin("")
	if h.Entry != nil && h.Entry.BaseURL != "" {
		origin, _ = orp.Origin(h.Entry.BaseURL, "https", "")
	}
	if next == "active" && !doc.Live {
		doc.Tracking = orp.MintTracking(origin, deal.ID, terms, doc.Tracking)
		doc.Live = true
		deal.State = "active"
		deal.Waiting = nil
		deal.UpdatedAt = now.UTC()
		h.putMany(deal, doc)
		return
	}
	if next == "active" {
		deal.State = "active"
		deal.Waiting = nil
		deal.UpdatedAt = now.UTC()
		h.putMany(deal, doc)
		return
	}
	deal.State = "pending"
	deal.UpdatedAt = now.UTC()
	clearSignedWaiting(deal)
	h.putMany(deal, doc)
}

// clearSignedWaiting drops waiting once the named party has signed.
// waiting names a party that still owes a step. A later required seat
// can still be unsigned; that seat is not this waiting record.
func clearSignedWaiting(deal *database.ORPDeal) {
	if len(deal.Waiting) == 0 || string(deal.Waiting) == "null" {
		return
	}
	waiting, err := decodeObject(deal.Waiting)
	if err != nil {
		return
	}
	party, _ := waiting["party"].(string)
	if party == "" || !seatSigned(mustMany(*deal).Seats, party) {
		return
	}
	deal.Waiting = nil
}

func (h *ORPHost) holdMany(deal *database.ORPDeal, party, step string, reasons []any, now time.Time) {
	origin := h.publicOrigin("")
	if h.Entry != nil && h.Entry.BaseURL != "" {
		origin, _ = orp.Origin(h.Entry.BaseURL, "https", "")
	}
	waiting := waitingOn(party, step, reasons, origin, now)
	raw, _ := json.Marshal(waiting)
	deal.Waiting = raw
	deal.State = "awaiting"
	deal.UpdatedAt = now.UTC()
}

func (h *ORPHost) insertMany(c *gin.Context, dns string, msg orp.Message, hash string, deal database.ORPDeal, now time.Time) {
	doc, err := h.manySnapshot(dns, msg.From, msg.ID, deal, now)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	saved, err := h.store(c, msg, hash, http.StatusOK, raw, database.ORPEffect{
		Deal:  &deal,
		Inbox: h.manyInbox(dns, msg.ID, deal, msg.From, ownerOf(msg.From, dns), now),
	})
	if err != nil {
		if isUnique(err) {
			h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "This party already has a deal for this offer."}), database.ORPEffect{})
			return
		}
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	writeORPBytes(c, saved.Status, saved.Reply)
}

func (h *ORPHost) publishMany(c *gin.Context, dns string, msg orp.Message, hash string, deal database.ORPDeal, doc orp.Many, expectHash string, expectAt ...time.Time) {
	at := deal.UpdatedAt
	if len(expectAt) > 0 {
		at = expectAt[0]
	}
	if expectHash == "" {
		expectHash = deal.TermsHash
	}
	snap, err := h.manySnapshot(dns, msg.From, msg.ID, deal, h.now())
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		h.failOpen(c, dns, msg, internalProblem())
		return
	}
	_ = doc
	h.finishRaw(c, msg, hash, http.StatusOK, raw, database.ORPEffect{
		ManyUpdate:     &deal,
		ManyExpectHash: expectHash,
		ManyExpectAt:   at,
		Inbox:          h.manyInbox(dns, msg.ID, deal, msg.From, ownerOf(msg.From, dns), h.now()),
	})
}

func (h *ORPHost) manySnapshot(dns, viewer, re string, deal database.ORPDeal, now time.Time) (map[string]any, error) {
	doc, err := orp.DecodeMany(deal.Parties)
	if err != nil {
		return nil, err
	}
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		return nil, err
	}
	parties := make([]any, len(doc.Seats))
	for i, seat := range doc.Seats {
		item := map[string]any{
			"id": seat.ID, "role": seat.Role, "required": seat.Required, "signed": seat.Signed,
		}
		if len(seat.Terms) > 0 {
			item["terms"] = seat.Terms
		}
		if len(seat.Identity) > 0 {
			item["identity"] = seat.Identity
		}
		parties[i] = item
	}
	body := map[string]any{
		"state":      deal.State,
		"parties":    parties,
		"terms":      terms,
		"terms_hash": deal.TermsHash,
		"round":      doc.Round,
		"updated":    orp.Stamp(deal.UpdatedAt),
	}
	if deal.UpdatedAt.IsZero() {
		body["updated"] = orp.Stamp(now)
	}
	if doc.Round == 0 {
		body["round"] = 1
	}
	if deal.OffererParty != "" {
		body["offerer"] = deal.OffererParty
	}
	if deal.AcceptorParty != "" {
		body["acceptor"] = deal.AcceptorParty
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
	if len(doc.Vacancies) > 0 {
		roles := make([]any, len(doc.Vacancies))
		for i, vacancy := range doc.Vacancies {
			roles[i] = vacancy.Role
		}
		body["vacancies"] = roles
	}
	if len(deal.Obligations) > 0 {
		var obligations any
		if err = json.Unmarshal(deal.Obligations, &obligations); err != nil {
			return nil, err
		}
		body["obligations"] = obligations
	}
	if len(doc.Tracking) > 0 {
		origin := h.publicOrigin(dns)
		rows := make([]any, len(doc.Tracking))
		for i, entry := range doc.Tracking {
			item := map[string]any{
				"obligation":  entry.Obligation,
				"link":        strings.TrimRight(origin, "/") + "/go/" + entry.Token,
				"destination": entry.Destination,
				"status":      entry.Status,
				"param":       entry.Param,
			}
			if entry.Alias != "" {
				item["alias"] = entry.Alias
			}
			rows[i] = item
		}
		body["tracking"] = rows
	}
	if len(deal.Waiting) > 0 && string(deal.Waiting) != "null" {
		waiting, err := decodeObject(deal.Waiting)
		if err != nil {
			return nil, err
		}
		body["waiting"] = waiting
	}
	snap := orp.DealMessage(dns, viewer, re, deal.ID, orp.DealView(body, viewer), now)
	snap["orp"] = orp.Version04
	return snap, nil
}

func (h *ORPHost) manyInbox(dns, re string, deal database.ORPDeal, actorParty, actorOwner string, now time.Time) []database.ORPInboxItem {
	doc, err := orp.DecodeMany(deal.Parties)
	if err != nil {
		return nil
	}
	out := make([]database.ORPInboxItem, 0, len(doc.Seats)+1)
	seen := map[string]bool{}
	for _, seat := range doc.Seats {
		out = append(out, h.inboxMany(dns, seat.ID, seat.Owner, re, deal, now))
		seen[seat.ID] = true
	}
	if actorParty != "" && !seen[actorParty] && actorOwner != "" {
		out = append(out, h.inboxMany(dns, actorParty, actorOwner, re, deal, now))
	}
	return out
}

func (h *ORPHost) inboxMany(dns, party, owner, re string, deal database.ORPDeal, now time.Time) database.ORPInboxItem {
	if owner == "" {
		return database.ORPInboxItem{}
	}
	doc, err := h.manySnapshot(dns, party, re, deal, now)
	if err != nil {
		return database.ORPInboxItem{}
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return database.ORPInboxItem{}
	}
	return database.ORPInboxItem{OwnerID: owner, Message: raw}
}

func (h *ORPHost) freshPitches(c *gin.Context, text string) ([]any, error) {
	if h.Entry == nil || h.Entry.DB == nil {
		return nil, nil
	}
	offers, err := h.Entry.DB.ListORPOffers(c.Request.Context(), 50)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0)
	for _, offer := range offers {
		terms, err := decodeObject(offer.Terms)
		if err != nil || !orp.HasCheckedAt(terms) || orp.OfferExpired(terms, h.now()) || !orp.OfferMatches(terms, text) {
			continue
		}
		reason := "Standing test offer: " + orp.TermsTitle(terms) + "."
		if utf8.RuneCountInString(reason) > 200 {
			reason = string([]rune(reason)[:200])
		}
		pitch := "You can monetize this draft: " + orp.TermsTitle(terms) + "."
		if utf8.RuneCountInString(pitch) > 280 {
			pitch = string([]rune(pitch)[:280])
		}
		items = append(items, map[string]any{
			"offer":  map[string]any{"id": offer.ID, "terms": terms, "terms_hash": offer.TermsHash},
			"party":  offer.Party,
			"fit":    map[string]any{"score": 70, "reasons": []any{reason}},
			"checks": map[string]any{"claims": "not_checked"},
			"pitch":  pitch,
		})
	}
	return items, nil
}

func (h *ORPHost) publicOrigin(dns string) string {
	base := ""
	if h.Entry != nil {
		base = h.Entry.BaseURL
	}
	origin, host := orp.Origin(base, "http", dns)
	if dns == "" {
		return origin
	}
	_ = host
	return origin
}

func (h *ORPHost) failOpen(c *gin.Context, dns string, msg orp.Message, problem orp.Problem) {
	if problem.Status == 0 {
		problem.Status = http.StatusBadRequest
	}
	writeORP(c, problem.Status, h.errorDoc(dns, msg, problem), false)
}

func (h *ORPHost) onDealParty(party string, deal database.ORPDeal) bool {
	if len(deal.Parties) == 0 {
		return deal.OffererParty == party || deal.AcceptorParty == party
	}
	doc, err := orp.DecodeMany(deal.Parties)
	if err != nil {
		return false
	}
	return seatPresent(doc.Seats, party)
}

func manyReasons(mode string) []any {
	if mode == database.ModeAsk {
		return []any{"step_ask", "claims_unchecked"}
	}
	return []any{"claims_unchecked"}
}

func seatPresent(seats []orp.Seat, id string) bool {
	for _, seat := range seats {
		if seat.ID == id {
			return true
		}
	}
	return false
}

func seatSigned(seats []orp.Seat, id string) bool {
	for _, seat := range seats {
		if seat.ID == id {
			return seat.Signed
		}
	}
	return false
}

func findSeat(seats []orp.Seat, id string) (orp.Seat, bool) {
	for _, seat := range seats {
		if seat.ID == id {
			return seat, true
		}
	}
	return orp.Seat{}, false
}

func mustMany(deal database.ORPDeal) orp.Many {
	doc, _ := orp.DecodeMany(deal.Parties)
	return doc
}

func rawJSON(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func decodeBound(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var rows []map[string]any
	_ = json.Unmarshal(raw, &rows)
	return rows
}

func dropBound(raw json.RawMessage, party string) json.RawMessage {
	rows := decodeBound(raw)
	kept := make([]any, 0, len(rows))
	for _, row := range rows {
		if row["party"] == party {
			continue
		}
		kept = append(kept, row)
	}
	if len(kept) == 0 {
		return json.RawMessage(`[]`)
	}
	return rawJSON(kept)
}

func ownerOf(party, dns string) string {
	id, ok := orp.OwnerFromParty(dns, party)
	if !ok {
		return ""
	}
	return id
}

// SeatView is a party object ValidateParties can read.
type SeatView struct {
	ID       string
	Role     string
	Required bool
	Terms    map[string]any
	Identity map[string]any
}

func seatView(seat orp.Seat) SeatView {
	return SeatView{ID: seat.ID, Role: seat.Role, Required: seat.Required, Terms: seat.Terms, Identity: seat.Identity}
}

func seatsView(seats []orp.Seat) []SeatView {
	out := make([]SeatView, len(seats))
	for i, seat := range seats {
		out[i] = seatView(seat)
	}
	return out
}

func asParties(seats []SeatView) []any {
	out := make([]any, len(seats))
	for i, seat := range seats {
		item := map[string]any{"id": seat.ID, "role": seat.Role, "required": seat.Required}
		if len(seat.Terms) > 0 {
			item["terms"] = seat.Terms
		}
		if len(seat.Identity) > 0 {
			item["identity"] = seat.Identity
		}
		out[i] = item
	}
	return out
}
