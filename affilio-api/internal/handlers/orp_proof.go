package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

// proofFetchFunc reads a public page and reports its HTTP status so a check
// can tell unreachable (no answer at all) from not_public (a wall).
type proofFetchFunc func(ctx context.Context, rawURL string) (body string, status int, ok bool)

// FetchProof fetches a proof_url page for obligation verification. Nil uses
// the SSRF-guarded readiness fetcher. On ORPHost in orp_host.go.

func (h *ORPHost) proofFetch() proofFetchFunc {
	if h.FetchProof != nil {
		return h.FetchProof
	}
	return (&services.ReadinessEngine{}).FetchPublicPage
}

// proofVerdict is one check's outcome: the receipt result, the public found
// facts, the failure reason, and whether the obligation itself may move to a
// terminal state. A page that cannot be read proves nothing either way, so
// transient failures keep the obligation claimed.
type proofVerdict struct {
	result   string
	found    map[string]any
	reason   string
	terminal bool
}

// trackEntry is the one tracking row a place obligation mints at activate.
type trackEntry struct {
	Obligation  string `json:"obligation"`
	Token       string `json:"token"`
	Destination string `json:"destination"`
	Status      string `json:"status"`
	Code        string `json:"code"`
}

// trackingEntry finds the tracking row minted for one obligation in the
// deal's tracking document (0.3 column or the 0.4 parties doc).
func trackingEntry(deal database.ORPDeal, obligationID string) *trackEntry {
	raw := deal.Tracking
	if len(deal.Parties) > 0 && string(deal.Parties) != "null" {
		raw = deal.Parties
	}
	var doc struct {
		Tracking []trackEntry `json:"tracking"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	for i := range doc.Tracking {
		if doc.Tracking[i].Obligation == obligationID {
			return &doc.Tracking[i]
		}
	}
	return nil
}

// proofMethod picks the receipt method an execute proof_url earns: the
// platform's own check for X and YouTube pages when listed, else proof_url,
// else none (the execute stays a bare claim).
func proofMethod(obligation map[string]any, proofURL string) (method, fetchURL string) {
	if orp.ProofAllows(obligation, "platform_check") {
		if embed, xHost := xStatusEmbedURL(proofURL); xHost && embed != "" {
			return "platform_check", embed
		}
		if embed := youtubeEmbedURL(proofURL); embed != "" {
			return "platform_check", embed
		}
	}
	if orp.ProofAllows(obligation, "proof_url") {
		return "proof_url", proofURL
	}
	return "", ""
}

// youtubeEmbedURL maps a YouTube watch or share URL to its public oEmbed
// endpoint, matching the preview fetcher's choice.
func youtubeEmbedURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	yt := host == "youtube.com" || host == "youtu.be" ||
		strings.HasSuffix(host, ".youtube.com") || strings.HasSuffix(host, ".youtu.be")
	if !yt {
		return ""
	}
	return "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(rawURL)
}

// runProof fetches the page and evaluates the obligation's evidence on it.
// method is proof_url or platform_check; the embed URL is already resolved.
func (h *ORPHost) runProof(ctx context.Context, fetch proofFetchFunc, method, fetchURL, proofURL string, obligation, terms map[string]any, origin string, deal database.ORPDeal) proofVerdict {
	body, status, ok := fetch(ctx, fetchURL)
	if !ok {
		reason := "unreachable"
		if status >= 400 {
			reason = "not_public"
		}
		return proofVerdict{result: "failed", reason: reason, found: map[string]any{}}
	}
	page := body
	if method == "platform_check" {
		if embed, ok := embedText(body); ok {
			page = embed
		}
	}
	found, missing := proofEvidence(page, method, obligation, terms, origin, deal)
	verdict := proofVerdict{result: "verified", found: found, terminal: true}
	if missing != "" {
		verdict.result = "failed"
		verdict.reason = missing
	}
	return verdict
}

// embedText unwraps an oEmbed reply: the html field carries the post markup
// for X; every field is searchable text for YouTube.
func embedText(body string) (string, bool) {
	var doc map[string]any
	if json.Unmarshal([]byte(body), &doc) != nil {
		return "", false
	}
	if htmlText, _ := doc["html"].(string); htmlText != "" {
		return htmlText, true
	}
	return body, true
}

// proofEvidence checks the obligation's required elements on the fetched
// page. found carries only the schema's public keys; missing names the
// receipt reason when something required is absent.
func proofEvidence(page, method string, obligation, terms map[string]any, origin string, deal database.ORPDeal) (map[string]any, string) {
	action, _ := obligation["action"].(string)
	found := map[string]any{}
	switch action {
	case "place":
		return placeEvidence(page, obligation, terms, origin, deal)
	case "repost":
		target, _ := obligation["target"].(string)
		if pageHasTarget(page, target) {
			found["target"] = true
			return found, ""
		}
		return found, "not_found"
	case "comment":
		return commentEvidence(page, obligation)
	}
	return found, "not_found"
}

// placeEvidence checks a place obligation: a direct link needs destination
// plus rel, anything else needs the tracking link (or the minted code), and
// the terms disclosure text when one is required.
func placeEvidence(page string, obligation, terms map[string]any, origin string, deal database.ORPDeal) (map[string]any, string) {
	content, _ := obligation["content"].(map[string]any)
	found := map[string]any{}
	obligationID, _ := obligation["id"].(string)
	direct, _ := content["direct"].(bool)
	if direct {
		destination, _ := obligation["destination"].(string)
		dest, rel := directLink(page, destination)
		if dest {
			found["destination"] = true
			found["rel"] = rel
		} else {
			return found, "link_missing"
		}
		if !rel {
			return found, "rel_missing"
		}
	} else {
		entry := trackingEntry(deal, obligationID)
		linked := false
		if entry != nil && entry.Token != "" && strings.Contains(page, "/go/"+entry.Token) {
			found["link"] = true
			linked = true
		}
		if entry != nil && entry.Code != "" && strings.Contains(page, entry.Code) {
			found["code"] = true
			linked = true
		}
		if !linked {
			return found, "link_missing"
		}
	}
	if label := orp.DisclosureLabel(terms); label != "" && !strings.Contains(
		strings.ToLower(page), strings.ToLower(label)) {
		return found, "disclosure_missing"
	}
	if label := orp.DisclosureLabel(terms); label != "" {
		found["disclosure"] = true
	}
	return found, ""
}

// commentEvidence checks a comment obligation: the comment's own text when
// content fixes it, else a link to the target.
func commentEvidence(page string, obligation map[string]any) (map[string]any, string) {
	found := map[string]any{}
	content, _ := obligation["content"].(map[string]any)
	if copy, _ := content["copy"].(string); copy != "" {
		if containsFold(page, copy) {
			found["content"] = true
			return found, ""
		}
		return found, "content_mismatch"
	}
	if phrases, ok := content["must_include"].([]any); ok && len(phrases) > 0 {
		for _, phrase := range phrases {
			text, _ := phrase.(string)
			if text == "" || !containsFold(page, text) {
				return found, "content_mismatch"
			}
		}
		found["content"] = true
		return found, ""
	}
	target, _ := obligation["target"].(string)
	if target != "" && pageHasTarget(page, target) {
		found["target"] = true
		return found, ""
	}
	return found, "not_found"
}

// pageHasTarget reports whether the page carries the target URL itself, its
// canonical form, or — for an X status — the status id.
func pageHasTarget(page, target string) bool {
	if target == "" {
		return false
	}
	if strings.Contains(page, target) {
		return true
	}
	canonical := strings.TrimRight(strings.SplitN(target, "?", 2)[0], "/")
	if canonical != target && strings.Contains(page, canonical) {
		return true
	}
	if id := xStatusID(target); id != "" && strings.Contains(page, "status/"+id) {
		return true
	}
	return false
}

// xStatusID extracts the digits of an X/Twitter status URL path.
func xStatusID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || !xHosts[strings.ToLower(u.Hostname())] {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 3 || parts[1] != "status" {
		return ""
	}
	for _, r := range parts[2] {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return parts[2]
}

// containsFold folds whitespace and case on both sides, so a comment line
// matches its rendered form.
func containsFold(page, text string) bool {
	fold := func(s string) string {
		return strings.ToLower(strings.Join(strings.Fields(s), " "))
	}
	return strings.Contains(fold(page), fold(text))
}

// directLink parses the page's anchors: dest is a link to the obligation's
// destination, rel whether that anchor carries sponsored or nofollow.
func directLink(page, destination string) (dest, rel bool) {
	if destination == "" {
		return false, false
	}
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return false, false
	}
	want := normalizeHref(destination)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if dest && rel {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			href, rels := "", ""
			for _, attr := range n.Attr {
				switch attr.Key {
				case "href":
					href = attr.Val
				case "rel":
					rels = attr.Val
				}
			}
			if normalizeHref(href) == want {
				dest = true
				for _, r := range strings.Fields(strings.ToLower(rels)) {
					if r == "sponsored" || r == "nofollow" {
						rel = true
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return dest, rel
}

func normalizeHref(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}

// verifyProof runs the execute-time check: the obligation's proof list names
// a fetchable method, the execute carried proof_url, and the fetch and page
// checks produce a receipt body plus the next obligation rows. It returns
// nil when this execute stays a bare claim.
func (h *ORPHost) verifyProof(ctx context.Context, origin string, deal database.ORPDeal, obligation, terms map[string]any, proofURL string, obligations []map[string]any, now time.Time) (json.RawMessage, map[string]any, string) {
	if proofURL == "" {
		return nil, nil, ""
	}
	obligationID, _ := obligation["id"].(string)
	method, fetchURL := proofMethod(obligation, proofURL)
	if method == "" {
		return nil, nil, ""
	}
	verdict := h.runProof(ctx, h.proofFetch(), method, fetchURL, proofURL, obligation, terms, origin, deal)
	at := now.UTC().Format(time.RFC3339)
	body := map[string]any{
		"obligation": obligationID,
		"result":     verdict.result,
		"method":     method,
		"at":         at,
		"url":        proofURL,
		"found":      verdict.found,
	}
	if verdict.reason != "" {
		body["reason"] = verdict.reason
	}
	keepUntil := keepEnd(obligation, now)
	if verdict.result == "verified" && !keepUntil.IsZero() {
		next := recheckSchedule(now, keepUntil, true)
		body["next_check"] = next.UTC().Format(time.RFC3339)
	}
	receiptID := orp.NewMessageID()
	rows := make([]map[string]any, 0, len(obligations))
	for _, row := range obligations {
		if row["id"] == obligationID {
			if verdict.terminal {
				// P4: a later receipt never demotes verified or waived work.
				state, _ := row["state"].(string)
				if state != "verified" && state != "waived" {
					row["state"] = verdict.result
					row["receipt"] = receiptID
					row["method"] = method
					row["url"] = proofURL
					if verdict.result == "verified" && !keepUntil.IsZero() {
						next := recheckSchedule(now, keepUntil, true)
						row["next_check"] = next.UTC().Format(time.RFC3339)
						row["keep_until"] = keepUntil.UTC().Format(time.RFC3339)
					}
				}
			}
		}
		rows = append(rows, row)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return nil, nil, ""
	}
	return raw, body, receiptID
}

// keepEnd reads the obligation's keep duration against the verify time.
func keepEnd(obligation map[string]any, at time.Time) time.Time {
	keep, _ := obligation["keep"].(string)
	if keep == "" {
		return time.Time{}
	}
	dur, ok := orp.ParseISODuration(keep)
	if !ok {
		return time.Time{}
	}
	return at.Add(dur)
}

// recheckSchedule picks the next check inside P6: one day after
// verification, then weekly, and at the end of keep.
func recheckSchedule(now, keepUntil time.Time, first bool) time.Time {
	step := 7 * 24 * time.Hour
	if first {
		step = 24 * time.Hour
	}
	next := now.Add(step)
	if next.After(keepUntil) {
		return keepUntil
	}
	return next
}

// receiptInbox builds one receipt envelope per party owner on the deal, the
// same delivery a link receipt uses.
func receiptInbox(dns, re, dealID, version string, parties []outcomeParty, body map[string]any, receiptID string, now time.Time) []database.ORPInboxItem {
	out := make([]database.ORPInboxItem, 0, len(parties))
	seen := map[string]bool{}
	for _, target := range parties {
		if target.owner == "" || seen[target.owner] {
			continue
		}
		seen[target.owner] = true
		doc := orp.ReceiptMessage(dns, target.party, re, dealID, body, now)
		doc["id"] = receiptID
		if version == orp.Version04 {
			doc["orp"] = orp.Version04
		}
		raw, err := json.Marshal(doc)
		if err != nil {
			continue
		}
		var value any
		if err := json.Unmarshal(raw, &value); err == nil {
			if _, problem, ok := orp.ParseEnvelope(value); !ok {
				log.Printf("orp receipt deal %s party %s skipped: %s", dealID, target.party, problem.Message)
				continue
			}
		}
		out = append(out, database.ORPInboxItem{OwnerID: target.owner, Message: raw})
	}
	return out
}

// dealProofParties lists every party owner a receipt reaches: offerer and
// acceptor on a 0.3 row, seat owners on a 0.4 row.
func dealProofParties(deal database.ORPDeal) (parties []outcomeParty, version string) {
	version = orp.Version
	if len(deal.Parties) > 0 && string(deal.Parties) != "null" {
		if doc, err := orp.DecodeMany(deal.Parties); err == nil {
			return outcomeParties(deal, doc, orp.Version04), orp.Version04
		}
	}
	return outcomeParties(deal, orp.Many{}, orp.Version), orp.Version
}

// receipt handles a party-sent receipt (§13 P2). The host issues every
// method except attestation; only the obligor's counterparty may attest, and
// an attestation moves the obligation within the deal only.
func (h *ORPHost) receipt(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	deal, party, ok := h.partyDeal(c, dns, msg, hash, ownerID)
	if !ok {
		return
	}
	if method, _ := msg.Body["method"].(string); method != "attestation" {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "Only the host issues this receipt."}), database.ORPEffect{})
		return
	}
	if deal.State != "active" {
		h.stateConflict(c, dns, msg, hash)
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
	if obligor == party {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "A receipt from the obligor about its own obligation is refused."}), database.ORPEffect{})
		return
	}
	if !orp.ProofAllows(obligation, "attestation") {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "The obligation does not accept attestation."}), database.ORPEffect{})
		return
	}
	var rows []map[string]any
	if err := json.Unmarshal(deal.Obligations, &rows); err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	var current string
	for _, row := range rows {
		if row["id"] == obligationID {
			current, _ = row["state"].(string)
		}
	}
	// A pending or claimed obligation may be attested; a terminal one is a
	// conflict.
	if current != "pending" && current != "claimed" && current != "" {
		h.stateConflict(c, dns, msg, hash)
		return
	}
	result, _ := msg.Body["result"].(string)
	now := h.now()
	prior := deal.UpdatedAt
	for _, row := range rows {
		if row["id"] != obligationID {
			continue
		}
		row["state"] = result
		row["receipt"] = msg.ID
		row["method"] = "attestation"
		row["attester"] = party
		row["at"] = now.UTC().Format(time.RFC3339)
	}
	obligations, err := json.Marshal(rows)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	deal.Obligations = obligations
	deal.UpdatedAt = now.UTC()
	effect := database.ORPEffect{
		UpdateDealID:      deal.ID,
		UpdateObligations: obligations,
		ExpectUpdatedAt:   prior,
		UpdatedAt:         now.UTC(),
		Inbox:             h.dealInbox(dns, msg.ID, deal, now),
	}
	effect.Inbox = append(effect.Inbox, h.forwardInbox(msg, deal)...)
	if rawMsg, err := json.Marshal(msg); err == nil && ownerID != "" {
		effect.Inbox = append(effect.Inbox, database.ORPInboxItem{OwnerID: ownerID, Message: rawMsg})
	}
	h.finishDeal(c, dns, msg, hash, party, deal, now, effect)
}

// sweepORPRechecks is the P6 pass inside the daily sweep. Each verified
// obligation whose next_check passed has its proof page refetched: a page
// that cannot be read reschedules a day out, a page still carrying the
// obligation books the next check, and a page that lost it fails the
// obligation, opens a removed_early dispute, and reverses held payouts.
func sweepORPRechecks(ctx context.Context, db *database.DB, dns string, fetch proofFetchFunc, now time.Time) (int, error) {
	deals, err := db.ListORPRecheckDue(ctx, now, 100)
	if err != nil {
		return 0, err
	}
	checked := 0
	for _, deal := range deals {
		n, err := recheckDeal(ctx, db, dns, deal, fetch, now)
		if err != nil {
			log.Printf("orp recheck deal %s: %v", deal.ID, err)
			continue
		}
		checked += n
	}
	return checked, nil
}

func recheckDeal(ctx context.Context, db *database.DB, dns string, deal database.ORPDeal, fetch proofFetchFunc, now time.Time) (int, error) {
	var rows []map[string]any
	if err := json.Unmarshal(deal.Obligations, &rows); err != nil {
		return 0, err
	}
	terms, err := decodeObject(deal.Terms)
	if err != nil {
		return 0, err
	}
	parties, version := dealProofParties(deal)
	var inbox []database.ORPInboxItem
	checked := 0
	changed := false
	disputed := false
	at := now.UTC().Format(time.RFC3339)
	for _, row := range rows {
		state, _ := row["state"].(string)
		nextRaw, _ := row["next_check"].(string)
		if state != "verified" || nextRaw == "" {
			continue
		}
		next, err := time.Parse(time.RFC3339, nextRaw)
		if err != nil || next.After(now) {
			continue
		}
		checked++
		obligationID, _ := row["id"].(string)
		method, _ := row["method"].(string)
		proofURL, _ := row["url"].(string)
		obligation := orp.ObligationMap(terms, obligationID)
		if obligation == nil || proofURL == "" || (method != "proof_url" && method != "platform_check") {
			delete(row, "next_check")
			changed = true
			continue
		}
		fetchURL := proofURL
		if method == "platform_check" {
			if _, resolved := proofMethod(obligation, proofURL); resolved != "" {
				fetchURL = resolved
			}
		}
		page, status, ok := fetch(ctx, fetchURL)
		if !ok {
			// No evidence either way: try again tomorrow.
			row["next_check"] = now.Add(24 * time.Hour).UTC().Format(time.RFC3339)
			changed = true
			_ = status
			continue
		}
		if method == "platform_check" {
			if embed, unwrapped := embedText(page); unwrapped {
				page = embed
			}
		}
		found, missing := proofEvidence(page, method, obligation, terms, "", deal)
		keepUntil, _ := time.Parse(time.RFC3339, str(row["keep_until"]))
		receiptID := orp.NewMessageID()
		body := map[string]any{
			"obligation": obligationID,
			"method":     method,
			"at":         at,
			"url":        proofURL,
			"found":      found,
		}
		row["receipt"] = receiptID
		if missing == "" {
			body["result"] = "verified"
			nextCheck := recheckSchedule(now, keepUntil, false)
			if !keepUntil.IsZero() && now.Before(keepUntil) && nextCheck.After(now) {
				row["next_check"] = nextCheck.UTC().Format(time.RFC3339)
				body["next_check"] = row["next_check"]
			} else {
				delete(row, "next_check")
				if err := db.ReleaseORPKeepPayouts(ctx, deal.ID, obligationID, now); err != nil {
					log.Printf("orp recheck release deal %s: %v", deal.ID, err)
				}
			}
		} else {
			row["state"] = "failed"
			body["result"] = "failed"
			body["reason"] = "removed"
			disputed = true
			for _, basis := range []string{obligationID, ""} {
				if err := db.ReverseORPKeepPayouts(ctx, deal.ID, basis); err != nil {
					log.Printf("orp recheck reverse deal %s: %v", deal.ID, err)
				}
			}
			for _, target := range parties {
				if target.owner == "" {
					continue
				}
				doc := map[string]any{
					"orp": version, "type": "dispute", "id": orp.NewMessageID(),
					"ts": orp.Stamp(now), "from": orp.HostParty(dns), "to": target.party,
					"deal": deal.ID,
					"body": map[string]any{
						"action":     "open",
						"obligation": obligationID,
						"claim":      "removed_early",
						"statement":  "The host rechecked the proof page and no longer finds the obligation.",
					},
				}
				raw, err := json.Marshal(doc)
				if err != nil {
					continue
				}
				var value any
				if err := json.Unmarshal(raw, &value); err == nil {
					if _, problem, ok := orp.ParseEnvelope(value); !ok {
						log.Printf("orp recheck dispute deal %s party %s skipped: %s", deal.ID, target.party, problem.Message)
						continue
					}
				}
				inbox = append(inbox, database.ORPInboxItem{OwnerID: target.owner, Message: raw})
			}
		}
		inbox = append(inbox, receiptInbox(dns, "", deal.ID, version, parties, body, receiptID, now)...)
		changed = true
	}
	if !changed {
		return checked, nil
	}
	obligations, err := json.Marshal(rows)
	if err != nil {
		return checked, err
	}
	state := deal.State
	if disputed {
		state = "disputed"
	}
	if ok, err := db.ApplyORPRecheck(ctx, deal.ID, obligations, state, deal.UpdatedAt, now.UTC(), inbox); err != nil {
		return checked, err
	} else if !ok {
		return checked, nil
	}
	return checked, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
