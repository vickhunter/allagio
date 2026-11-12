package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func defaultPageFetch(ctx context.Context, rawURL string) (string, bool) {
	body, _, ok := (&services.ReadinessEngine{}).FetchPublicPage(ctx, rawURL)
	return body, ok
}

// space registers, updates, or withdraws one space (ORP 0.3 §9.1). put writes
// the space and replies space listed with status listed or waiting — waiting
// when the owner's list (placement publish) step is ask, off refuses. Only
// the host sets verified: a locator host matching the owner's claimed site
// domain earns {"method":"domain"}.
func (h *ORPHost) space(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	if msg.ORP == orp.Version04 {
		h.finish(c, msg, hash, http.StatusBadRequest, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusBadRequest, Code: "invalid_message", Message: "The message is not valid ORP 0.4."}), database.ORPEffect{})
		return
	}
	action, _ := msg.Body["action"].(string)
	if action == "withdraw" {
		h.withdrawSpace(c, dns, msg, hash, ownerID)
		return
	}
	rawSpace, _ := msg.Body["space"].(map[string]any)
	spaceID, _ := rawSpace["id"].(string)
	mode, err := h.placementMode(c, ownerID)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	if mode == database.ModeOff {
		h.finish(c, msg, hash, http.StatusForbidden, h.errorDoc(dns, msg, stepOff("list", "The owner has turned off placement publish, so this agent cannot list a space.")), database.ORPEffect{})
		return
	}
	listed := mode == database.ModeAuto
	now := h.now()
	origin, _ := h.origin(c)
	if verified := h.spaceVerified(c, ownerID, msg.From, rawSpace); len(verified) > 0 {
		rawSpace["verified"] = verified
	}
	spaceRaw, _ := json.Marshal(rawSpace)
	body := map[string]any{
		"action": "listed",
		"space":  rawSpace,
		"status": "listed",
	}
	if !listed {
		body["status"] = "waiting"
		body["waiting"] = waitingOn(msg.From, "list", []any{"step_ask"}, origin, now)
	}
	doc := orp.SpaceReply(dns, msg.From, msg.ID, body, now)
	h.finish(c, msg, hash, http.StatusOK, doc, database.ORPEffect{
		Space:     &database.ORPSpace{ID: spaceID, OwnerID: ownerID, Party: msg.From, Space: spaceRaw, Listed: listed},
		UpdatedAt: now.UTC(),
	})
}

func (h *ORPHost) withdrawSpace(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	spaceID, _ := msg.Body["id"].(string)
	if spaceID == "" {
		if sp, ok := msg.Body["space"].(map[string]any); ok {
			spaceID, _ = sp["id"].(string)
		}
	}
	stored, err := h.Entry.DB.GetORPSpace(c.Request.Context(), ownerID, spaceID)
	if err != nil {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such space for this sender."}), database.ORPEffect{})
		return
	}
	var space map[string]any
	_ = json.Unmarshal(stored.Space, &space)
	now := h.now()
	doc := orp.SpaceReply(dns, msg.From, msg.ID, map[string]any{
		"action": "listed", "space": space, "status": "withdrawn",
	}, now)
	h.finish(c, msg, hash, http.StatusOK, doc, database.ORPEffect{
		SpaceWithdraw: &database.ORPSpaceRef{OwnerID: ownerID, ID: spaceID},
		UpdatedAt:     now.UTC(),
	})
}

// spaceVerified returns the host-set verified list for a space. The domain
// method wins when the locator host matches the owner's claimed site. On any
// other host the locator is fetched and a page carrying the party id earns
// bio_token on a social.profile or challenge_post on other kinds. An X
// status locator is checked through the publish.twitter.com oEmbed endpoint;
// other X URLs serve JS-gated pages and earn nothing. A fetch failure adds
// no method.
func (h *ORPHost) spaceVerified(c *gin.Context, ownerID, party string, rawSpace map[string]any) []any {
	locator, _ := rawSpace["locator"].(string)
	if locator == "" {
		return nil
	}
	domain, err := h.Entry.DB.OwnerSiteDomain(c.Request.Context(), ownerID)
	if err == nil && domain != "" {
		host := locator
		if i := strings.Index(host, "://"); i >= 0 {
			host = host[i+3:]
		}
		if i := strings.IndexAny(host, "/?#"); i >= 0 {
			host = host[:i]
		}
		host = strings.TrimPrefix(strings.ToLower(host), "www.")
		domain = strings.TrimPrefix(strings.ToLower(domain), "www.")
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return h.spaceVerification("domain")
		}
	}
	fetch := h.FetchPage
	if fetch == nil {
		fetch = defaultPageFetch
	}
	if embed, xHost := xStatusEmbedURL(locator); xHost {
		if embed == "" {
			return nil
		}
		body, ok := fetch(c.Request.Context(), embed)
		if !ok || !strings.Contains(body, party) {
			return nil
		}
		return h.spaceVerification("challenge_post")
	}
	body, ok := fetch(c.Request.Context(), locator)
	if !ok || !strings.Contains(body, party) {
		return nil
	}
	if kind, _ := rawSpace["kind"].(string); kind == "social.profile" {
		return h.spaceVerification("bio_token")
	}
	return h.spaceVerification("challenge_post")
}

func (h *ORPHost) spaceVerification(method string) []any {
	return []any{map[string]any{"method": method, "at": orp.Stamp(h.now())}}
}

var xHosts = map[string]bool{
	"x.com": true, "www.x.com": true, "mobile.x.com": true,
	"twitter.com": true, "www.twitter.com": true, "mobile.twitter.com": true,
}

// xStatusEmbedURL maps an X status locator, /<user>/status/<digits> on x.com
// or twitter.com, to the publish.twitter.com oEmbed endpoint the host fetches
// for the post text. The second result reports whether the locator is on an
// X host at all: every other X URL is JS-gated and is never fetched.
func xStatusEmbedURL(locator string) (string, bool) {
	u, err := url.Parse(locator)
	if err != nil {
		return "", false
	}
	if !xHosts[strings.ToLower(u.Hostname())] {
		return "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 3 || parts[1] != "status" {
		return "", true
	}
	if _, err := strconv.ParseUint(parts[2], 10, 64); err != nil {
		return "", true
	}
	return "https://publish.twitter.com/oembed?url=" + url.QueryEscape(locator) + "&omit_script=true", true
}

// request delivers a maker's terms request to a space owner's inbox. The spec
// answer is 202 with no body — the recipient's agent answers with an offer.
func (h *ORPHost) request(c *gin.Context, dns string, msg orp.Message, hash, ownerID string) {
	toOwner, ok := orp.OwnerFromParty(dns, msg.To)
	if !ok || toOwner == "" {
		h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such party."}), database.ORPEffect{})
		return
	}
	if spaceID, _ := msg.Body["space"].(string); spaceID != "" {
		stored, err := h.Entry.DB.GetORPSpace(c.Request.Context(), toOwner, spaceID)
		if err != nil || !stored.Listed {
			h.finish(c, msg, hash, http.StatusNotFound, h.errorDoc(dns, msg, orp.Problem{Status: http.StatusNotFound, Code: "not_found", Message: "No such space for that party."}), database.ORPEffect{})
			return
		}
	}
	raw, err := json.Marshal(map[string]any{
		"orp": msg.ORP, "type": "request", "id": msg.ID, "ts": orp.Stamp(h.now()),
		"from": msg.From, "to": msg.To, "body": msg.Body,
	})
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	saved, err := h.store(c, msg, hash, http.StatusAccepted, []byte(`{"delivered":true}`), database.ORPEffect{
		Inbox: []database.ORPInboxItem{{OwnerID: toOwner, Message: raw}},
	})
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	writeORPBytes(c, saved.Status, nil)
}

// discoverSpaces serves discover want:"spaces": listed spaces with locator
// stripped, scored on topic/text overlap like standing offers.
func (h *ORPHost) discoverSpaces(c *gin.Context, dns string, msg orp.Message, hash string) {
	spaces, err := h.Entry.DB.ListORPSpaces(c.Request.Context(), 50)
	if err != nil {
		h.writeProblem(c, dns, msg.From, msg.ID, internalProblem(), false)
		return
	}
	text, limit := orp.DiscoverHaystack(msg.Body)
	items := []any{}
	for _, stored := range spaces {
		var space map[string]any
		if json.Unmarshal(stored.Space, &space) != nil {
			continue
		}
		delete(space, "locator")
		title, _ := space["title"].(string)
		topics, _ := space["topics"].([]any)
		haystack := title + " " + joinAny(topics) + " " + joinAny(spaceStringKeys(space, "platform", "kind"))
		score, reason := spaceFit(text, haystack)
		if text != "" && score == 0 {
			continue
		}
		items = append(items, map[string]any{
			"space": space,
			"party": stored.Party,
			"fit":   map[string]any{"score": score, "reasons": []any{reason}},
		})
		if len(items) >= limit {
			break
		}
	}
	empty := ""
	if len(items) == 0 {
		empty = "no_match"
	}
	doc := orp.ResultsReply(dns, msg.From, msg.ID, items, empty, h.now())
	if msg.ORP == orp.Version04 {
		doc["orp"] = orp.Version04
	}
	h.finish(c, msg, hash, http.StatusOK, doc, database.ORPEffect{})
}

func joinAny(list []any) string {
	parts := make([]string, 0, len(list))
	for _, item := range list {
		if text, ok := item.(string); ok {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func spaceStringKeys(obj map[string]any, keys ...string) []any {
	out := []any{}
	for _, key := range keys {
		if text, ok := obj[key].(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func spaceFit(query, haystack string) (int, string) {
	hay := " " + strings.ToLower(haystack) + " "
	if strings.TrimSpace(query) == "" {
		return 60, "Listed space."
	}
	matched := []string{}
	for _, token := range strings.Fields(strings.ToLower(query)) {
		if len(token) > 2 && strings.Contains(hay, " "+token) {
			matched = append(matched, token)
		}
	}
	if len(matched) == 0 {
		return 0, ""
	}
	score := 60 + len(matched)*10
	if score > 90 {
		score = 90
	}
	return score, "Space text matches: " + strings.Join(matched, ", ") + "."
}

// ListSpaceForm is the owner's approval of a waiting space: the list step
// was ask, and this click lists it.
func (h *HostedHandler) ListSpaceForm(c *gin.Context) {
	o := ownerFrom(c)
	listed, err := h.DB.ListORPSpaceOwner(c.Request.Context(), o.ID, c.Param("id"))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if !listed {
		c.Status(http.StatusNotFound)
		return
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings")
}

func (h *HostedHandler) offersSection(c *gin.Context, o *database.Owner) (template.HTML, error) {
	offers, err := h.DB.ListORPOffersByOwner(c.Request.Context(), o.ID)
	if err != nil {
		return "", err
	}
	if len(offers) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section id="standing-offers"><h2>Your standing offers</h2><p>Offers your agent opened for discovery. A waiting offer lists when you approve it.</p>`)
	for _, offer := range offers {
		var terms map[string]any
		_ = json.Unmarshal(offer.Terms, &terms)
		title, _ := terms["title"].(string)
		b.WriteString(`<div class="deal"><h3>`)
		b.WriteString(template.HTMLEscapeString(title))
		b.WriteString(` <code>`)
		b.WriteString(template.HTMLEscapeString(offer.ID))
		b.WriteString(`</code></h3><p>`)
		if offer.Listed {
			b.WriteString(`listed`)
		} else {
			b.WriteString(`waiting for your approval</p><form method="post" action="/owner/offers/`)
			b.WriteString(template.HTMLEscapeString(offer.ID))
			b.WriteString(`/list"><input type="hidden" name="csrf" value="`)
			b.WriteString(template.HTMLEscapeString(o.CSRF))
			b.WriteString(`"><button>List it</button></form><p>`)
		}
		b.WriteString(`</p></div>`)
	}
	b.WriteString(`</section>`)
	return template.HTML(b.String()), nil
}

// ListOfferForm is the owner's approval of a waiting standing offer: the
// list step was ask, and this click lists it.
func (h *HostedHandler) ListOfferForm(c *gin.Context) {
	o := ownerFrom(c)
	listed, err := h.DB.ListORPOfferOwner(c.Request.Context(), o.ID, c.Param("id"))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if !listed {
		c.Status(http.StatusNotFound)
		return
	}
	c.Redirect(http.StatusSeeOther, "/owner/settings")
}

func (h *HostedHandler) spacesSection(c *gin.Context, o *database.Owner) (template.HTML, error) {
	spaces, err := h.DB.ListORPSpacesByOwner(c.Request.Context(), o.ID)
	if err != nil {
		return "", err
	}
	if len(spaces) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(`<section id="spaces"><h2>Your spaces</h2><p>Surfaces your agent registered for deal work. A waiting space lists when you approve it.</p>`)
	for _, sp := range spaces {
		var space map[string]any
		_ = json.Unmarshal(sp.Space, &space)
		title, _ := space["title"].(string)
		kind, _ := space["kind"].(string)
		b.WriteString(`<div class="deal"><h3>`)
		b.WriteString(template.HTMLEscapeString(title))
		b.WriteString(` <code>`)
		b.WriteString(template.HTMLEscapeString(sp.ID))
		b.WriteString(`</code></h3><p>`)
		b.WriteString(template.HTMLEscapeString(kind))
		if sp.Listed {
			b.WriteString(` — listed`)
		} else {
			b.WriteString(` — waiting for your approval</p><form method="post" action="/owner/spaces/`)
			b.WriteString(template.HTMLEscapeString(sp.ID))
			b.WriteString(`/list"><input type="hidden" name="csrf" value="`)
			b.WriteString(template.HTMLEscapeString(o.CSRF))
			b.WriteString(`"><button>List it</button></form><p>`)
		}
		b.WriteString(`</p></div>`)
	}
	b.WriteString(`</section>`)
	return template.HTML(b.String()), nil
}
