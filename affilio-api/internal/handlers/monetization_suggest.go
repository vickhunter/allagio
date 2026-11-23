package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"affilio-api/internal/database"
	"affilio-api/internal/orp"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

const (
	// MonetizationSuggestPath is the ORP thin-layer route for suggest_monetization.
	// The body reuses the suggest request fields (text, platform, lang, limit).
	// The response is the drafting shape: up to three offers and a human prompt.
	// POST /orp/suggest is unchanged.
	MonetizationSuggestPath = "/orp/monetize"

	monetizationHumanPrompt = "You can monetize this, want to try?"
	monetizationDisclosure  = "Affiliate link. I may earn a commission if you buy through this link."
	monetizationOfferCap    = 3
)

var suggestionCodePattern = regexp.MustCompile(`^sg[0-9a-f]{20}$`)

// Monetize is POST /orp/monetize. A zero-friction agent token calls it with
// a draft. The draft is stripped of emails, handles, and phone numbers, then
// scored. Only the hash of the stripped text and the matched keywords are
// stored. Nothing is published.
func (h *ORPHost) Monetize(c *gin.Context) {
	if h.Entry == nil || h.Entry.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "suggest unavailable"})
		return
	}
	grant, err := h.Entry.JoinGrant(c)
	if err != nil {
		if errors.Is(err, errNoBearer) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid bearer token"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired bearer token"})
		return
	}
	body, problem, ok := readORPBody(c)
	if !ok {
		c.JSON(problemStatus(problem), gin.H{"error": problem.Message})
		return
	}
	if h.SuggestLimiter != nil && !h.SuggestLimiter.Allow(h.clientIP(c)) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}
	value, problem, ok := orp.DecodeJSON(body)
	if !ok {
		c.JSON(problemStatus(problem), gin.H{"error": problem.Message})
		return
	}
	req, surface, problem, ok := parseMonetization(value)
	if !ok {
		writeSuggestInputProblem(c, problem)
		return
	}
	stripped := services.StripDraftPII(req.Text)
	sum := sha256.Sum256([]byte(stripped))
	hash := hex.EncodeToString(sum[:])
	lang := requestLang(value)

	modes, err := h.Entry.DB.LoadStepModes(c.Request.Context(), grant.OwnerID, grant.SiteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "suggest unavailable"})
		return
	}
	mode, step, blocked := monetizationGate(modes)
	if blocked {
		if err = h.storeSuggestion(c, grant.OwnerID, hash, nil, surface, lang, nil, mode, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "suggest unavailable"})
			return
		}
		writeMonetization(c, monetizationBody{
			Offers:            []any{},
			HumanPrompt:       "",
			NeedsHumanConfirm: false,
			Mode:              mode,
			Step:              step,
			Reason:            "step_off",
			Message:           stepOffMessage(step),
		})
		return
	}

	allow, deny, err := h.categoryLimits(c, grant.SiteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "suggest unavailable"})
		return
	}
	rows, err := h.candidates(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "suggest unavailable"})
		return
	}
	limit := req.Limit
	if limit <= 0 || limit > monetizationOfferCap {
		limit = monetizationOfferCap
	}
	picked, keywords, filtered := pickMonetization(stripped, rows, allow, deny, limit)
	reason := ""
	if len(picked) == 0 {
		if filtered {
			reason = "filtered"
		} else {
			reason = "no_match"
		}
	}
	confirm := mode == database.ModeAsk
	offers, links := h.monetizationOffers(c, picked, confirm)
	if err = h.storeSuggestion(c, grant.OwnerID, hash, keywords, surface, lang, offerIDs(picked), mode, links); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "suggest unavailable"})
		return
	}
	prompt := ""
	if len(offers) > 0 {
		prompt = monetizationHumanPrompt
	}
	writeMonetization(c, monetizationBody{
		Offers:            offers,
		HumanPrompt:       prompt,
		NeedsHumanConfirm: confirm && len(offers) > 0,
		Mode:              mode,
		Step:              step,
		Reason:            reason,
	})
}

type monetizationBody struct {
	Offers            []any
	HumanPrompt       string
	NeedsHumanConfirm bool
	Mode              string
	Step              string
	Reason            string
	Message           string
}

func writeMonetization(c *gin.Context, body monetizationBody) {
	if body.Offers == nil {
		body.Offers = []any{}
	}
	out := gin.H{
		"offers":              body.Offers,
		"human_prompt":        body.HumanPrompt,
		"needs_human_confirm": body.NeedsHumanConfirm,
		"mode":                body.Mode,
		"step":                body.Step,
		"reason":              body.Reason,
	}
	if body.Message != "" {
		out["message"] = body.Message
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, out)
}

// monetizationGate applies offer_match and placement_publish.
// Off on either step returns no offers. Ask on either step marks the offers
// for a person. Auto still does not publish. A missing row is ask for both.
func monetizationGate(modes database.StepModes) (mode, step string, blocked bool) {
	if modes.Get(database.StepOfferMatch) == database.ModeOff {
		return database.ModeOff, database.StepOfferMatch, true
	}
	if modes.Get(database.StepPlacementPublish) == database.ModeOff {
		return database.ModeOff, database.StepPlacementPublish, true
	}
	if modes.Get(database.StepOfferMatch) == database.ModeAsk {
		return database.ModeAsk, database.StepOfferMatch, false
	}
	if modes.Get(database.StepPlacementPublish) == database.ModeAsk {
		return database.ModeAsk, database.StepPlacementPublish, false
	}
	return database.ModeAuto, database.StepPlacementPublish, false
}

func (h *ORPHost) categoryLimits(c *gin.Context, siteID string) (allow, deny []string, err error) {
	if siteID == "" {
		return nil, nil, nil
	}
	policy, err := h.Entry.DB.GetSitePolicy(c.Request.Context(), siteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return policy.AllowCategories, policy.DenyCategories, nil
}

type monetizationPick struct {
	Candidate orp.Candidate
	Why       string
	Score     int
}

func pickMonetization(text string, rows []orp.Candidate, allow, deny []string, limit int) ([]monetizationPick, []string, bool) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].ID < rows[j].ID
	})
	offers := make([]services.OfferTerms, 0, len(rows))
	indexed := make([]orp.Candidate, 0, len(rows))
	for _, row := range rows {
		if !httpsDestination(row.Website) {
			continue
		}
		offers = append(offers, services.OfferTerms{
			Name:     row.Name,
			Tagline:  strings.TrimSpace(row.Blurb + " " + row.Company),
			Category: row.Category,
			Keywords: row.Keywords,
		})
		indexed = append(indexed, row)
	}
	ranked := services.RankPageOffers(text, offers)
	filtered := false
	scored := false
	var picked []monetizationPick
	seenKeywords := map[string]struct{}{}
	var keywords []string
	for _, hit := range ranked {
		if hit.Relevance < services.RelevanceFloor {
			continue
		}
		scored = true
		row := indexed[hit.Index]
		if !categoryAllowed(allow, deny, row.Category) {
			filtered = true
			continue
		}
		if len(picked) >= limit {
			continue
		}
		why := orp.FitReason(text, row.Name, row.Blurb, row.Company, row.Category, strings.Join(row.Keywords, " "))
		picked = append(picked, monetizationPick{Candidate: row, Why: why, Score: hit.Relevance})
		for _, token := range services.OverlapTokens(text, offers[hit.Index]) {
			if _, ok := seenKeywords[token]; ok {
				continue
			}
			if len(token) > 64 {
				token = token[:64]
			}
			seenKeywords[token] = struct{}{}
			keywords = append(keywords, token)
		}
	}
	sort.Strings(keywords)
	if len(keywords) > 24 {
		keywords = keywords[:24]
	}
	if len(picked) == 0 && scored && filtered {
		return nil, nil, true
	}
	return picked, keywords, false
}

func categoryAllowed(allow, deny []string, category string) bool {
	category = strings.ToLower(strings.TrimSpace(category))
	for _, item := range deny {
		if category != "" && strings.EqualFold(strings.TrimSpace(item), category) {
			return false
		}
	}
	if len(allow) == 0 {
		return true
	}
	if category == "" {
		return false
	}
	for _, item := range allow {
		if strings.EqualFold(strings.TrimSpace(item), category) {
			return true
		}
	}
	return false
}

func (h *ORPHost) monetizationOffers(c *gin.Context, picked []monetizationPick, confirm bool) ([]any, []database.MonetizationLink) {
	origin, _ := h.origin(c)
	base := strings.TrimRight(origin, "/")
	offers := make([]any, 0, len(picked))
	links := make([]database.MonetizationLink, 0, len(picked))
	for _, one := range picked {
		code, err := newSuggestionCode()
		if err != nil {
			continue
		}
		link := base + "/v1/m/" + code
		offers = append(offers, gin.H{
			"offer_id":            one.Candidate.ID,
			"title":               one.Candidate.Name,
			"why_match":           one.Why,
			"tracked_link":        link,
			"disclosure_line":     monetizationDisclosure,
			"needs_human_confirm": confirm,
		})
		links = append(links, database.MonetizationLink{
			Code:        code,
			OfferID:     one.Candidate.ID,
			Destination: strings.TrimSpace(one.Candidate.Website),
		})
	}
	return offers, links
}

func (h *ORPHost) storeSuggestion(c *gin.Context, ownerID, hash string, keywords []string, surface, lang string, offerIDs []string, mode string, links []database.MonetizationLink) error {
	if keywords == nil {
		keywords = []string{}
	}
	if offerIDs == nil {
		offerIDs = []string{}
	}
	_, err := h.Entry.DB.SaveMonetizationSuggestion(c.Request.Context(), database.MonetizationSuggestion{
		OwnerID:     ownerID,
		DraftSHA256: hash,
		Keywords:    keywords,
		Surface:     surface,
		Lang:        lang,
		OfferIDs:    offerIDs,
		Mode:        mode,
		Links:       links,
	})
	return err
}

func offerIDs(picked []monetizationPick) []string {
	out := make([]string, 0, len(picked))
	for _, one := range picked {
		out = append(out, one.Candidate.ID)
	}
	return out
}

func newSuggestionCode() (string, error) {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "sg" + hex.EncodeToString(buf), nil
}

func writeSuggestInputProblem(c *gin.Context, problem orp.Problem) {
	status := problemStatus(problem)
	body := gin.H{"error": problem.Message}
	if problem.Code != "" {
		body["code"] = problem.Code
	}
	if problem.Field != "" {
		body["field"] = problem.Field
	}
	if problem.Field == "/surface" || problem.Field == "/platform" {
		body["allowed"] = append([]string(nil), orp.DraftSurfaces...)
	}
	c.JSON(status, body)
}

func problemStatus(problem orp.Problem) int {
	if problem.Status == 0 {
		return http.StatusBadRequest
	}
	return problem.Status
}

func requestLang(value any) string {
	obj, _ := value.(map[string]any)
	if text, _ := obj["lang"].(string); text != "" {
		return text
	}
	text, _ := obj["language"].(string)
	return text
}

// parseMonetization accepts the suggest request (text, platform, lang, limit)
// and the tool aliases (draft, surface, language). surface and platform
// share DraftSurfaces: x, blog, newsletter, email, and web. The draft stays
// in the returned request only.
func parseMonetization(value any) (orp.SuggestRequest, string, orp.Problem, bool) {
	obj, ok := value.(map[string]any)
	if !ok {
		return orp.SuggestRequest{}, "", orp.Invalid("The suggest request is not valid.", ""), false
	}
	cleaned := make(map[string]any, len(obj))
	for key, item := range obj {
		cleaned[key] = item
	}
	surface, problem, ok := takeSurface(cleaned)
	if !ok {
		return orp.SuggestRequest{}, "", problem, false
	}
	if problem, ok = aliasField(cleaned, "draft", "text"); !ok {
		return orp.SuggestRequest{}, "", problem, false
	}
	if problem, ok = aliasField(cleaned, "language", "lang"); !ok {
		return orp.SuggestRequest{}, "", problem, false
	}
	if surface != "" {
		if platform, _ := cleaned["platform"].(string); platform == "" {
			cleaned["platform"] = surface
		}
	}
	req, problem, ok := orp.ParseSuggest(cleaned)
	if !ok {
		return orp.SuggestRequest{}, "", problem, false
	}
	if surface == "" {
		if platform, _ := cleaned["platform"].(string); orp.DraftSurfaceAllowed(platform) {
			surface = platform
		}
	}
	return req, surface, orp.Problem{}, true
}

func aliasField(obj map[string]any, from, to string) (orp.Problem, bool) {
	raw, present := obj[from]
	delete(obj, from)
	if !present {
		return orp.Problem{}, true
	}
	text, ok := raw.(string)
	if !ok {
		return orp.Invalid("The suggest request is not valid.", "/"+from), false
	}
	if existing, also := obj[to].(string); also {
		if existing != text {
			return orp.Invalid("The suggest request is not valid.", "/"+from), false
		}
		return orp.Problem{}, true
	}
	if _, also := obj[to]; also {
		return orp.Invalid("The suggest request is not valid.", "/"+to), false
	}
	obj[to] = text
	return orp.Problem{}, true
}

func takeSurface(obj map[string]any) (string, orp.Problem, bool) {
	raw, present := obj["surface"]
	delete(obj, "surface")
	if !present {
		return "", orp.Problem{}, true
	}
	surface, ok := raw.(string)
	if !ok || !orp.DraftSurfaceAllowed(surface) {
		return "", orp.DraftSurfaceProblem("/surface"), false
	}
	if platform, _ := obj["platform"].(string); platform != "" && platform != surface {
		return "", orp.Invalid("surface and platform must match. Allowed values: x, blog, newsletter, email, web.", "/surface"), false
	}
	return surface, orp.Problem{}, true
}

// Monetization is GET /v1/m/:code. It counts a click and redirects to the
// offer. The draft is not loaded.
func (h *RedirectsHandler) Monetization(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Robots-Tag", "noindex, nofollow")
	code := strings.TrimSpace(c.Param("code"))
	if h.DB == nil || !suggestionCodePattern.MatchString(code) {
		c.Status(http.StatusNotFound)
		return
	}
	count := c.Request.Method != http.MethodHead
	destination, err := h.DB.FollowMonetizationLink(c.Request.Context(), code, count)
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	if !httpsDestination(destination) {
		c.Status(http.StatusNotFound)
		return
	}
	c.Redirect(http.StatusFound, destination)
}
