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
	r.POST("/orp/deals/:id/activate", h.Activate)
	r.GET("/orp/proof/:id", h.Proof)
	r.GET("/orp/approve/:id", h.Approve)
	r.POST("/orp/approve/:id", h.ApprovePost)
	r.GET("/go/:token", h.Go)
	r.HEAD("/go/:token", h.Go)
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
	if !found || !h.onDealParty(party, deal) {
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
	var approvals []orp.Approval
	if doc, err := orp.DecodeMany(deal.Parties); err == nil {
		approvals = doc.Approvals
	}
	if h.Entry != nil && h.Entry.DB != nil {
		rows, err := h.Entry.DB.DealOwnerApprovals(c.Request.Context(), deal.ID)
		if err == nil {
			for _, row := range rows {
				approvals = append(approvals, orp.Approval{Party: row.Party, Step: row.Step, By: row.By})
			}
		}
	}
	writeHTML(c, http.StatusOK, orp.ProofPage(orp.DisclosureLabel(terms), deal.State, linkObligationVerified(deal.Obligations), approvals))
}

func linkObligationVerified(raw json.RawMessage) bool {
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) != nil {
		return false
	}
	for _, row := range rows {
		receipt, _ := row["receipt"].(string)
		if row["state"] == "verified" && receipt != "" {
			return true
		}
	}
	return false
}

// Activate moves an accepted deal from awaiting to active.
// The caller is a party on the deal. The counterparty accept must already be
// stored, so neither the offerer nor the acceptor can activate before it.
// Any other state is state_conflict.
func (h *ORPHost) Activate(c *gin.Context) {
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
	if !found || !h.onDealParty(party, deal) {
		h.writeProblem(c, dns, party, "", orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}, false)
		return
	}
	if len(deal.Parties) > 0 {
		h.writeProblem(c, dns, party, "", orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}, false)
		return
	}
	if deal.State != "awaiting" {
		h.writeProblem(c, dns, party, "", orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}, false)
		return
	}
	accepted, err := h.Entry.DB.ORPAcceptStored(c.Request.Context(), deal.AcceptorParty, deal.ID)
	if err != nil {
		h.writeProblem(c, dns, party, "", internalProblem(), false)
		return
	}
	if !accepted {
		h.writeProblem(c, dns, party, "", orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}, false)
		return
	}
	if h.activateNeedsOwner(c, dns, party, deal) {
		return
	}
	now := h.now()
	bound, err := bindMandates(deal.Bound, deal.OffererParty, deal.AcceptorParty, now)
	if err != nil {
		h.writeProblem(c, dns, party, "", internalProblem(), false)
		return
	}
	deal.State = "active"
	deal.Waiting = nil
	deal.Bound = bound
	deal.UpdatedAt = now.UTC()
	doc, err := h.dealMessage(dns, party, "", deal, now)
	if err != nil {
		h.writeProblem(c, dns, party, "", internalProblem(), false)
		return
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		h.writeProblem(c, dns, party, "", internalProblem(), false)
		return
	}
	changed, err := h.Entry.DB.ActivateORPDeal(c.Request.Context(), deal.ID, bound, deal.UpdatedAt, h.dealInbox(dns, "", deal, now))
	if err != nil {
		h.writeProblem(c, dns, party, "", internalProblem(), false)
		return
	}
	if !changed {
		h.writeProblem(c, dns, party, "", orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}, false)
		return
	}
	writeORPBytes(c, http.StatusOK, raw)
}

// Approve shows the terms. It never records an approval.
// When the waiting owner turned on human approval, the signed-in owner sees
// the terms and a form, including a 0.3 deal whose parties column is null.
// An agent bearer, a prefetch, and a signed-out visit are refused.
// Submitting the form is ApprovePost.
func (h *ORPHost) Approve(c *gin.Context) {
	setApproveFrameHeaders(c)
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
	page := orp.ApprovePage(orp.TermsTitle(terms), orp.DisclosureLabel(terms))
	if h.writeOwnerForm(c, deal, terms) {
		return
	}
	writeHTML(c, http.StatusOK, page)
}

// writeOwnerForm answers a step_ask wait the owner opted into.
// It returns false when the public page is the right response.
func (h *ORPHost) writeOwnerForm(c *gin.Context, deal database.ORPDeal, terms map[string]any) bool {
	waiting, err := decodeObject(deal.Waiting)
	if err != nil || waiting == nil {
		return false
	}
	party, _ := waiting["party"].(string)
	_, dns := h.origin(c)
	ownerID, ok := orp.OwnerFromParty(dns, party)
	if !ok {
		return false
	}
	steps, err := h.Entry.DB.HumanApprovalSteps(c.Request.Context(), ownerID)
	if err != nil {
		writeHTML(c, http.StatusInternalServerError, "unavailable")
		return true
	}
	if !humanStepOn(steps, waiting) {
		return false
	}
	if purposePreview(c) {
		writePreview(c)
		return true
	}
	if strings.TrimSpace(c.GetHeader("Authorization")) != "" {
		writeOwnerApproval(c)
		return true
	}
	owner, signedIn := h.signedInOwner(c)
	if !signedIn {
		writeOwnerApproval(c)
		return true
	}
	if owner.ID != ownerID {
		c.JSON(http.StatusForbidden, gin.H{
			"reason":  "forbidden",
			"message": "This approval link is for the waiting owner.",
		})
		return true
	}
	writeHTML(c, http.StatusOK, orp.OwnerApprovePage(orp.TermsTitle(terms), orp.DisclosureLabel(terms), "/orp/approve/"+c.Param("id"), owner.CSRF))
	return true
}

// approveOwnerPost records approved_by owner. The caller has no agent bearer.
// A prefetch, a missing confirmation token, and the wrong owner do not record.
func (h *ORPHost) approveOwnerPost(c *gin.Context) {
	_, dns := h.origin(c)
	deal, found, err := h.Entry.DB.GetORPDealByEscalation(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": "unavailable"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"reason": "not_found", "message": "No such deal for this sender."})
		return
	}
	waiting, err := decodeObject(deal.Waiting)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": "unavailable"})
		return
	}
	if waiting == nil {
		c.JSON(http.StatusForbidden, gin.H{"reason": "forbidden", "message": "This page does not approve."})
		return
	}
	party, _ := waiting["party"].(string)
	ownerID, ok := orp.OwnerFromParty(dns, party)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"reason": "forbidden", "message": "This page does not approve."})
		return
	}
	steps, err := h.Entry.DB.HumanApprovalSteps(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": "unavailable"})
		return
	}
	if !humanStepOn(steps, waiting) {
		c.JSON(http.StatusForbidden, gin.H{"reason": "forbidden", "message": "This page does not approve."})
		return
	}
	owner, signedIn := h.signedInOwner(c)
	if !signedIn {
		writeOwnerApproval(c)
		return
	}
	if owner.ID != ownerID {
		c.JSON(http.StatusForbidden, gin.H{
			"reason":  "forbidden",
			"message": "This approval link is for the waiting owner.",
		})
		return
	}
	if !ownerFormCSRF(c, h.Entry.BaseURL, owner.CSRF) {
		c.JSON(http.StatusForbidden, gin.H{
			"reason":  "forbidden",
			"message": "This form needs the owner's confirmation.",
		})
		return
	}
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": "unavailable"})
		return
	}
	step, _ := waiting["step"].(string)
	if len(deal.Parties) == 0 || string(deal.Parties) == "null" {
		h.finishOwner03(c, dns, deal, ownerID, party, step)
		return
	}
	expectHash, expectAt := deal.TermsHash, deal.UpdatedAt
	now := h.now()
	h.noteApproval(&deal, party, step, "owner")
	h.signSeat(&deal, party, now)
	h.refreshMany(&deal, terms, now)
	msg := orp.Message{ORP: orp.Version04, Type: "accept", ID: orp.NewMessageID(), From: party}
	bodyHash := orp.ContentHash(map[string]any{"approve": c.Param("id")})
	h.publishMany(c, dns, msg, bodyHash, deal, mustMany(deal), expectHash, expectAt)
}

// finishOwner03 binds a 0.3 accept the way Activate does and records
// approved_by owner. Parties stays null, so the row stays a 0.3 deal.
func (h *ORPHost) finishOwner03(c *gin.Context, dns string, deal database.ORPDeal, ownerID, party, step string) {
	now := h.now()
	bound, err := bindMandates(deal.Bound, deal.OffererParty, deal.AcceptorParty, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": "unavailable"})
		return
	}
	if err = h.Entry.DB.RecordDealOwnerApproval(c.Request.Context(), ownerID, deal.ID, party, step); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": "unavailable"})
		return
	}
	deal.State = "active"
	deal.Waiting = nil
	deal.Bound = bound
	deal.Parties = nil
	deal.UpdatedAt = now.UTC()
	h.writeActive03(c, dns, party, deal)
}

func (h *ORPHost) writeActive03(c *gin.Context, dns, viewer string, deal database.ORPDeal) {
	doc, err := h.dealMessage(dns, viewer, "", deal, deal.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": "unavailable"})
		return
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": "unavailable"})
		return
	}
	changed, err := h.Entry.DB.ActivateORPDeal(c.Request.Context(), deal.ID, deal.Bound, deal.UpdatedAt, h.dealInbox(dns, "", deal, deal.UpdatedAt))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": "unavailable"})
		return
	}
	if !changed {
		c.JSON(http.StatusConflict, gin.H{"reason": "state_conflict", "message": "The deal is in the wrong state."})
		return
	}
	writeORPBytes(c, http.StatusOK, raw)
}

func (h *ORPHost) offer(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	if msg.ORP == orp.Version04 {
		h.offer04(c, dns, msg, hash, ownerID)
		return
	}
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
	if mode == database.ModeOff {
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
	// list follows placement publish. A new party has no saved row, so the
	// mode is ask, which is what join advertises. The offer waits and is not
	// discoverable, and the maker is not bound by mandate. A saved auto
	// still lists the offer.
	listed := mode == database.ModeAuto
	if !listed {
		origin, _ := h.origin(c)
		body["waiting"] = waitingOn(msg.From, "list", []any{"step_ask"}, origin, now)
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
			TermsHash: hashOfTerms, Limit: limit, CreatedAt: now.UTC(), Listed: listed,
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
	// SPEC opens a directed offer as proposed. awaiting starts when the
	// counterparty accept is stored, so the offerer cannot activate alone.
	deal.State = "proposed"
	h.commitDeal(c, dns, msg, hash, deal, now)
}

func (h *ORPHost) accept(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	if msg.ORP == orp.Version04 && h.accept04(c, dns, msg, hash, ownerID) {
		return
	}
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
	now := h.now()
	effect := database.ORPEffect{}
	// Only the counterparty accept of a proposed deal stores awaiting.
	// The offerer's own accept leaves the row proposed.
	if deal.State == "proposed" {
		if party != deal.AcceptorParty {
			h.finish(c, msg, hash, http.StatusConflict, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}), database.ORPEffect{})
			return
		}
		deal.State = "awaiting"
		deal.UpdatedAt = now.UTC()
		effect.UpdateDealID = deal.ID
		effect.UpdateState = "awaiting"
		effect.UpdatedAt = deal.UpdatedAt
	}
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
	effect.Inbox = h.dealInbox(dns, msg.ID, deal, now)
	h.finishRaw(c, msg, hash, http.StatusOK, raw, effect)
}

func (h *ORPHost) execute(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	deal, found, err := h.Entry.DB.GetORPDeal(c.Request.Context(), msg.Deal)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	party := orp.MemberParty(dns, ownerID)
	if !found || !h.onDealParty(party, deal) {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such deal for this sender."}), database.ORPEffect{})
		return
	}
	// Execute is a claim. Verification is a receipt, and only an active deal
	// can be claimed. awaiting (step_ask, claims_unchecked) stays untouched.
	if deal.State != "active" {
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
	if len(deal.Parties) > 0 {
		obligor = obligorBy(deal, role)
	} else if role == "offerer" {
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
	mode, err := h.placementMode(c, ownerID)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if obligation["action"] == "place" && mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("execute", "The owner has turned off placement publish, so this agent cannot claim a placement.")), database.ORPEffect{})
		return
	}
	now := h.now()
	obligations, err := markClaimed(deal.Obligations, obligationID)
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
	h.finishRaw(c, msg, hash, http.StatusOK, raw, database.ORPEffect{
		UpdateDealID:      deal.ID,
		UpdateObligations: obligations,
		UpdatedAt:         now.UTC(),
		Inbox:             h.dealInbox(dns, msg.ID, deal, now),
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
		if err != nil || orp.OfferExpired(terms, h.now()) || !orp.OfferMatches(terms, text) {
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
		if errors.Is(err, database.ErrORPState) {
			h.writeProblem(c, dns, msg.From, msg.ID, orp.Problem{Status: http.StatusConflict, Code: "state_conflict", Message: "The deal is in the wrong state."}, false)
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
	doc := orp.ErrorDocument(dns, msg.From, msg.ID, problem, h.now())
	if msg.ORP == orp.Version04 {
		doc["orp"] = orp.Version04
	}
	return doc
}

func (h *ORPHost) dealMessage(dns, viewer, re string, deal database.ORPDeal, now time.Time) (map[string]any, error) {
	if len(deal.Parties) > 0 {
		return h.manySnapshot(dns, viewer, re, deal, now)
	}
	body, err := dealBody(deal, now)
	if err != nil {
		return nil, err
	}
	return orp.DealMessage(dns, viewer, re, deal.ID, orp.DealView(body, viewer), now), nil
}

func (h *ORPHost) dealInbox(dns, re string, deal database.ORPDeal, now time.Time) []database.ORPInboxItem {
	if len(deal.Parties) > 0 {
		return h.manyInbox(dns, re, deal, "", "", now)
	}
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

// markClaimed moves a pending obligation to claimed. It does not verify it.
func markClaimed(raw json.RawMessage, obligationID string) (json.RawMessage, error) {
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	found := false
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		if row["id"] == obligationID {
			if row["state"] == "pending" || row["state"] == "" {
				row["state"] = "claimed"
			}
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
		Obligations:   database.JSONList(obligations),
		Bound:         database.JSONList(boundRaw),
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

// bindMandates records a mandate for each party that is not already bound.
// The deal schema allows two bound rows.
func bindMandates(raw json.RawMessage, offerer, acceptor string, now time.Time) (json.RawMessage, error) {
	rows := []map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, err
		}
	}
	have := map[string]bool{}
	for _, row := range rows {
		if party, _ := row["party"].(string); party != "" {
			have[party] = true
		}
	}
	for _, party := range []string{offerer, acceptor} {
		if party == "" || have[party] {
			continue
		}
		rows = append(rows, mandate(party, now))
		have[party] = true
	}
	if len(rows) > 2 {
		return nil, errors.New("too many bound parties")
	}
	return json.Marshal(rows)
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

// setApproveFrameHeaders keeps the approval document out of a frame.
// Approve and ApprovePost set it before any return.
func setApproveFrameHeaders(c *gin.Context) {
	c.Header("X-Frame-Options", "DENY")
	c.Header("Content-Security-Policy", "frame-ancestors 'none'; form-action 'self'; base-uri 'none'")
	c.Header("Referrer-Policy", "no-referrer")
}

// askHeld reports a step_ask wait whose owner turned ask on.
// party limits the check to a wait already on that party. An empty party matches any wait.
func (h *ORPHost) askHeld(c *gin.Context, deal database.ORPDeal, party string) (bool, error) {
	if len(deal.Waiting) == 0 || string(deal.Waiting) == "null" {
		return false, nil
	}
	waiting, err := decodeObject(deal.Waiting)
	if err != nil {
		return false, err
	}
	if waiting == nil || !reasonHas(waiting, "step_ask") {
		return false, nil
	}
	waitingParty, _ := waiting["party"].(string)
	if party != "" && waitingParty != party {
		return false, nil
	}
	_, dns := h.origin(c)
	ownerID, ok := orp.OwnerFromParty(dns, waitingParty)
	if !ok {
		return false, nil
	}
	steps, err := h.Entry.DB.HumanApprovalSteps(c.Request.Context(), ownerID)
	if err != nil {
		return false, err
	}
	return humanStepOn(steps, waiting), nil
}

// activateNeedsOwner refuses a 0.3 activate when the waiting owner holds ask
// and the wait is step_ask. The caller has already checked that an accept is stored.
func (h *ORPHost) activateNeedsOwner(c *gin.Context, dns, caller string, deal database.ORPDeal) bool {
	held, err := h.askHeld(c, deal, "")
	if err != nil {
		h.writeProblem(c, dns, caller, "", internalProblem(), false)
		return true
	}
	if !held {
		return false
	}
	writeOwnerApproval(c)
	return true
}
