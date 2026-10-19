package entry

import (
	"errors"
	"log"
	"regexp"
	"strings"
)

// ErrClaimCheckUnavailable is the stub's answer. It is not a claim failure.
var ErrClaimCheckUnavailable = errors.New("claim check unavailable")

// ClaimCheckReason is why yolo and auto stay on manual approval.
const ClaimCheckReason = "claim_check_unavailable"

// ClaimCheckMessage is the sentence returned with ClaimCheckReason.
const ClaimCheckMessage = "Automatic approval is off until claim checks are live."

// YoloAfterFirstReason is why a saved yolo site still answers manual
// until its first approved placement.
const YoloAfterFirstReason = "yolo_after_first_placement"

// YoloAfterFirstMessage tells the UI that yolo is picked and waiting.
const YoloAfterFirstMessage = "YOLO is picked and turns on after the first approved placement."

// ClaimChecker is the D-028 claim-check hook. Real reports whether this
// checker is the live one. The stub is not, so yolo and auto cannot go live.
type ClaimChecker interface {
	CheckClaims(copyText, offerData string) error
	Real() bool
}

// ClaimReasonNumberNotInOffer is logged when copy states a number the offer does not.
const ClaimReasonNumberNotInOffer = "number_not_in_offer"

// ClaimMismatch is a failed claim check. Reason is the logged reason code.
// Claim is the unsupported token from the copy, such as "90%".
type ClaimMismatch struct {
	Reason string
	Claim  string
}

func (e ClaimMismatch) Error() string { return "claims_from_offer: " + e.Reason }

// PublicMessage names the unsupported claim for a 422 body.
func (e ClaimMismatch) PublicMessage() string {
	if strings.TrimSpace(e.Claim) == "" {
		return "The copy states a claim the offer does not support."
	}
	return "The copy says \"" + e.Claim + "\", which the offer does not support."
}

var claimToken = regexp.MustCompile(`\d+(?:[.,]\d+)?%?`)

// StubClaimChecker is the production checker. It is not live, so yolo and auto
// stay on manual approval. Consistent copy passes. A number the offer does not
// contain fails, and that failure logs a reason code.
type StubClaimChecker struct{}

func (StubClaimChecker) CheckClaims(copyText, offerData string) error {
	offer := strings.ToLower(offerData)
	for _, token := range claimToken.FindAllString(copyText, -1) {
		token = strings.ToLower(token)
		if strings.Contains(offer, token) {
			continue
		}
		bare := strings.TrimSuffix(token, "%")
		if bare != "" && strings.Contains(offer, bare) {
			continue
		}
		log.Printf("claim check failed reason=%s", ClaimReasonNumberNotInOffer)
		return ClaimMismatch{Reason: ClaimReasonNumberNotInOffer, Claim: token}
	}
	return nil
}

func (StubClaimChecker) Real() bool { return false }

// LiveClaimChecker accepts every claim and allows automatic go-live.
type LiveClaimChecker struct{}

func (LiveClaimChecker) CheckClaims(string, string) error { return nil }
func (LiveClaimChecker) Real() bool                       { return true }

// UncheckedClaims skips claims_from_offer. Restore and retry call ClaimsFail
// in the same database transaction as the other re-checks.
type UncheckedClaims struct{}

func (UncheckedClaims) CheckClaims(string, string) error { return nil }
func (UncheckedClaims) Real() bool                       { return true }

// ClaimsFail is true when restore or retry must refuse the placement.
// A consistent offer returns nil from the production checker and passes.
// A mismatch is a real failure. Decide still ignores ErrClaimCheckUnavailable
// so yolo stays on manual approval while the checker is not live.
func ClaimsFail(claims ClaimChecker, copyText, offerData string) bool {
	if claims == nil {
		claims = StubClaimChecker{}
	}
	return claims.CheckClaims(copyText, offerData) != nil
}

// AutoRules is the owner policy for auto mode. Maker fields (site quality and
// niches) apply when an offer accepts a publisher.
type AutoRules struct {
	AllowMakers            []string
	DenyMakers             []string
	AllowCategories        []string
	DenyCategories         []string
	MinRelevance           int
	AllowedPositions       []string
	MaxPlacementsPer30Days int
	CopyEditsAllowed       bool
	MinSiteQuality         int
	AllowedNiches          []string
	// UseStoredCap keeps a saved cap of 0. An unset cap still defaults to 3.
	UseStoredCap bool
}

// PlacementFacts are the inputs for one placement decision.
type PlacementFacts struct {
	Disclosure         string
	RelSponsored       bool
	Copy               string
	OfferData          string
	ExistingBoxes      int
	Relevance          int
	OfferLive          bool
	RevokeWorks        bool
	Position           string
	MakerID            string
	Category           string
	CopyEdited         bool
	PlacementsIn30Days int
}

// PublisherFacts are the inputs for a maker accepting a publisher.
type PublisherFacts struct {
	OfferLive   bool
	SiteQuality int
	Niche       string
}

// Decision is the policy result. ApprovedBy is owner, policy:auto, or policy:yolo.
type Decision struct {
	Allowed     bool
	NeedsOwner  bool
	ApprovedBy  string
	MatchedRule string
	Failures    []string
	Reason      string
	Message     string
}

// PresentRule turns a stored match into the id and sentence the dashboard shows.
// hard_checks_passed and category:invoicing are the machine ids.
func PresentRule(rule string) (id, text string) {
	if name := ruleValue(rule, "allow_category:"); name != "" {
		return "category:" + name, "The category is " + name + "."
	}
	if name := ruleValue(rule, "allow_niche:"); name != "" {
		return "category:" + name, "The category is " + name + "."
	}
	switch rule {
	case "hard_checks", "hard_checks_passed", "auto_defaults", "min_site_quality":
		return "hard_checks_passed", "Passed every hard check."
	}
	if strings.Contains(rule, "allow_maker") {
		return "maker_allowed", "The maker is on the allow list."
	}
	if name := ruleValue(rule, "position:"); name != "" {
		return "position:" + name, "The position is " + name + "."
	}
	if rule == "" {
		return "", ""
	}
	return rule, "Matched the saved policy."
}

func ruleValue(rule, prefix string) string {
	for _, part := range strings.Split(rule, "+") {
		if strings.HasPrefix(part, prefix) {
			return strings.TrimPrefix(part, prefix)
		}
	}
	return ""
}

var allowedPositions = map[string]bool{
	"after_intro": true, "matched_section": true, "end": true,
	"main_post": true, "first_reply": true,
	"early_mention": true, "description": true, "pinned_comment": true,
}

func ValidPosition(position string) bool { return allowedPositions[position] }

func KnownPositions() []string {
	return []string{"after_intro", "matched_section", "end", "main_post", "first_reply", "early_mention", "description", "pinned_comment"}
}

// NormalizeMinRelevance applies the default of 40 and refuses values under 25.
func NormalizeMinRelevance(value int, provided bool) (int, error) {
	if !provided {
		return 40, nil
	}
	if value < 25 || value > 100 {
		return 0, errMinRelevance
	}
	return value, nil
}

type relevanceError struct{}

func (relevanceError) Error() string { return "min relevance must be from 25 to 100" }

var errMinRelevance = relevanceError{}

// CapLimit is the placement cap. A saved 0 is a real cap of zero. An unset
// cap defaults to 3.
func CapLimit(rules AutoRules) int {
	if rules.MaxPlacementsPer30Days == 0 && !rules.UseStoredCap {
		return 3
	}
	return rules.MaxPlacementsPer30Days
}

// MinRelevanceLimit is the policy minimum. An unset value defaults to 40.
func MinRelevanceLimit(rules AutoRules) int {
	if rules.MinRelevance == 0 {
		return 40
	}
	return rules.MinRelevance
}

func EvaluatePlacement(mode string, rules AutoRules, in PlacementFacts, claims ClaimChecker) Decision {
	if claims == nil {
		claims = StubClaimChecker{}
	}
	failures := placementHardFailures(in, rules, claims)
	if len(failures) > 0 {
		return Decision{Failures: failures}
	}
	if !claims.Real() && (mode == ModeYolo || mode == ModeAuto) {
		return Decision{NeedsOwner: true, Reason: ClaimCheckReason, Message: ClaimCheckMessage}
	}
	switch mode {
	case ModeManual:
		return Decision{NeedsOwner: true}
	case ModeYolo:
		return Decision{Allowed: true, ApprovedBy: "policy:yolo", MatchedRule: "hard_checks"}
	case ModeAuto:
		rule, ok, reason := matchPlacement(rules, in)
		if !ok {
			return Decision{NeedsOwner: true, Failures: []string{reason}}
		}
		return Decision{Allowed: true, ApprovedBy: "policy:auto", MatchedRule: rule}
	default:
		return Decision{NeedsOwner: true, Failures: []string{"unknown_mode"}}
	}
}

func EvaluatePublisher(mode string, rules AutoRules, in PublisherFacts, claims ClaimChecker) Decision {
	if claims == nil {
		claims = StubClaimChecker{}
	}
	if !in.OfferLive {
		return Decision{Failures: []string{"offer_live"}}
	}
	if !claims.Real() && (mode == ModeYolo || mode == ModeAuto) {
		return Decision{NeedsOwner: true, Reason: ClaimCheckReason, Message: ClaimCheckMessage}
	}
	switch mode {
	case ModeManual:
		return Decision{NeedsOwner: true}
	case ModeYolo:
		return Decision{Allowed: true, ApprovedBy: "policy:yolo", MatchedRule: "hard_checks"}
	case ModeAuto:
		if in.SiteQuality < rules.MinSiteQuality {
			return Decision{NeedsOwner: true, Failures: []string{"below_min_site_quality"}}
		}
		if len(rules.AllowedNiches) > 0 && !contains(rules.AllowedNiches, in.Niche) {
			return Decision{NeedsOwner: true, Failures: []string{"niche_not_allowed"}}
		}
		rule := "auto_defaults"
		if len(rules.AllowedNiches) > 0 {
			rule = "allow_niche:" + in.Niche
		}
		if rules.MinSiteQuality > 0 && rule == "auto_defaults" {
			rule = "min_site_quality"
		}
		return Decision{Allowed: true, ApprovedBy: "policy:auto", MatchedRule: rule}
	default:
		return Decision{NeedsOwner: true, Failures: []string{"unknown_mode"}}
	}
}

// HardCheck is one public failure. HardCheckCatalog is the only list of
// check ids and the text a 422 returns with them.
type HardCheck struct {
	ID   string
	Text string
}

var HardCheckCatalog = []HardCheck{
	{ID: "disclosure", Text: "Add a disclosure."},
	{ID: "rel_sponsored", Text: "Add rel=sponsored."},
	{ID: "claims_from_offer", Text: "Use only claims from the offer."},
	{ID: "copy_length", Text: "Keep the copy to 140 characters."},
	{ID: "one_per_post", Text: "Keep one box on this post."},
	{ID: "relevance_min", Text: "Relevance is below the minimum."},
	{ID: "offer_live", Text: "The offer is not live."},
	{ID: "revoke_path", Text: "The revoke path is not available."},
	{ID: "cap", Text: "The placement cap is full."},
}

// HardChecks are the ids from HardCheckCatalog, in that order.
func HardChecks() []string {
	out := make([]string, len(HardCheckCatalog))
	for i, check := range HardCheckCatalog {
		out[i] = check.ID
	}
	return out
}

// HardCheckLabels are the texts from HardCheckCatalog, in that order.
func HardCheckLabels() []string {
	out := make([]string, len(HardCheckCatalog))
	for i, check := range HardCheckCatalog {
		out[i] = check.Text
	}
	return out
}

// FirstCheck returns the earliest catalog entry among the failure ids.
func FirstCheck(failures []string) (HardCheck, bool) {
	present := map[string]bool{}
	for _, id := range failures {
		present[id] = true
	}
	for _, check := range HardCheckCatalog {
		if present[check.ID] {
			return check, true
		}
	}
	return HardCheck{}, false
}

// HardCheckDoc is the hard-check section for /llms.txt and the mode docs.
func HardCheckDoc() string {
	var b strings.Builder
	b.WriteString("## Hard checks\n\n")
	b.WriteString("`GET /v1/policy/hard-checks` is the single source the UI renders. The response `checks` list is this catalog, in this order. The UI does not keep a second copy.\n\n")
	b.WriteString("A 422 returns `check_id` and `check_text`:\n\n")
	for _, check := range HardCheckCatalog {
		b.WriteString("- `")
		b.WriteString(check.ID)
		b.WriteString("`: ")
		b.WriteString(check.Text)
		b.WriteString("\n")
	}
	return b.String()
}

// RestoreRefusal is one public 409 reason. RestoreRefusals is the only list.
// Every 409 returns reason and message from this list.
type RestoreRefusal struct {
	Reason string
	Text   string
}

var RestoreRefusals = []RestoreRefusal{
	{Reason: "window_expired", Text: "The restore window has expired."},
	{Reason: "paused", Text: "Pause or the kill switch was turned on after the undo."},
	{Reason: "not_failed", Text: "The placement is not failed."},
	{Reason: "device_used", Text: "That device code was already used."},
	{Reason: "account_exists", Text: "An account already exists for this email."},
	{Reason: "not_approvable", Text: "The placement cannot be approved."},
	{Reason: "not_undone", Text: "The placement was not undone."},
	{Reason: "not_committed", Text: "The placement is not committed."},
	{Reason: "nothing_to_ack", Text: "There is nothing to acknowledge."},
	{Reason: "offer_ineligible", Text: "The offer is not eligible for this content."},
	{Reason: "offer_unavailable", Text: "The offer is unavailable."},
	{Reason: "placement_unavailable", Text: "The placement is unavailable, expired, or a self-referral."},
	{Reason: "receipt_mismatch", Text: "The published text must include the approved copy, the disclosure, and the exact tracked link."},
	{Reason: "receipt_recorded", Text: "The receipt was already recorded or the placement is unavailable."},
	{Reason: "publisher_missing", Text: "The publisher was not found."},
	{Reason: "restore_refused", Text: "The restore was refused."},
	{Reason: "one_per_post", Text: "Keep one box on this post."},
}

// RestoreRefusalByReason returns the catalog entry for one reason.
func RestoreRefusalByReason(reason string) (RestoreRefusal, bool) {
	for _, refusal := range RestoreRefusals {
		if refusal.Reason == reason {
			return refusal, true
		}
	}
	return RestoreRefusal{}, false
}

// RestoreRefusalDoc is the 409-reason section for /llms.txt and the mode docs.
func RestoreRefusalDoc() string {
	var b strings.Builder
	b.WriteString("## 409 reasons\n\n")
	b.WriteString("A 409 returns `reason` and `message`. `reason` is one of these values. `message` is the plain-English text:\n\n")
	for _, refusal := range RestoreRefusals {
		b.WriteString("- `")
		b.WriteString(refusal.Reason)
		b.WriteString("`: ")
		b.WriteString(refusal.Text)
		b.WriteString("\n")
	}
	return b.String()
}

func placementHardFailures(in PlacementFacts, rules AutoRules, claims ClaimChecker) []string {
	var failures []string
	if strings.TrimSpace(in.Disclosure) == "" {
		failures = append(failures, "disclosure")
	}
	if !in.RelSponsored {
		failures = append(failures, "rel_sponsored")
	}
	if err := claims.CheckClaims(in.Copy, in.OfferData); err != nil && (claims.Real() || !errors.Is(err, ErrClaimCheckUnavailable)) {
		failures = append(failures, "claims_from_offer")
	}
	if len([]rune(in.Copy)) > 140 {
		failures = append(failures, "copy_length")
	}
	if in.ExistingBoxes > 0 {
		failures = append(failures, "one_per_post")
	}
	if in.Relevance < MinRelevanceLimit(rules) {
		failures = append(failures, "relevance_min")
	}
	if !in.OfferLive {
		failures = append(failures, "offer_live")
	}
	if !in.RevokeWorks {
		failures = append(failures, "revoke_path")
	}
	if in.PlacementsIn30Days >= CapLimit(rules) {
		failures = append(failures, "cap")
	}
	return failures
}

// RelSponsoredInPost is true only when the plugin's re-read of the post
// carries rel=sponsored. Disclosure words do not satisfy it.
func RelSponsoredInPost(html string) bool {
	blob := strings.ToLower(html)
	return strings.Contains(blob, `rel="sponsored"`) || strings.Contains(blob, "rel='sponsored'") || strings.Contains(blob, "rel=sponsored")
}

// RelSponsoredFromMarkup is true when our disclosure or copy carries
// rel=sponsored, or states the commercial relationship in words.
func RelSponsoredFromMarkup(disclosure, copyText string) bool {
	blob := strings.ToLower(disclosure + "\n" + copyText)
	if strings.Contains(blob, `rel="sponsored"`) || strings.Contains(blob, "rel='sponsored'") || strings.Contains(blob, "rel=sponsored") {
		return true
	}
	for _, word := range []string{"sponsor", "affiliate", "commission"} {
		if strings.Contains(blob, word) {
			return true
		}
	}
	return false
}

func matchPlacement(rules AutoRules, in PlacementFacts) (string, bool, string) {
	if contains(rules.DenyMakers, in.MakerID) {
		return "", false, "deny_maker"
	}
	if in.Category != "" && containsFold(rules.DenyCategories, in.Category) {
		return "", false, "deny_category"
	}
	if len(rules.AllowMakers) > 0 && !contains(rules.AllowMakers, in.MakerID) {
		return "", false, "maker_not_allowed"
	}
	if len(rules.AllowCategories) > 0 && !containsFold(rules.AllowCategories, in.Category) {
		return "", false, "category_not_allowed"
	}
	minRel := rules.MinRelevance
	if minRel == 0 {
		minRel = 40
	}
	if in.Relevance < minRel {
		return "", false, "below_min_relevance"
	}
	if len(rules.AllowedPositions) > 0 && !contains(rules.AllowedPositions, in.Position) {
		return "", false, "position_not_allowed"
	}
	cap := rules.MaxPlacementsPer30Days
	if cap == 0 {
		cap = 3
	}
	if in.PlacementsIn30Days >= cap {
		return "", false, "placement_cap"
	}
	if in.CopyEdited && !rules.CopyEditsAllowed {
		return "", false, "copy_edits_denied"
	}
	parts := []string{}
	if len(rules.AllowMakers) > 0 {
		parts = append(parts, "allow_maker")
	}
	if len(rules.AllowCategories) > 0 {
		parts = append(parts, "allow_category:"+in.Category)
	}
	if len(rules.AllowedPositions) > 0 {
		parts = append(parts, "position:"+in.Position)
	}
	if len(parts) == 0 {
		return "auto_defaults", true, ""
	}
	return strings.Join(parts, "+"), true, ""
}

// LooserThan reports whether next widens what an agent is allowed to pass.
// An empty allow-list means every value is allowed.
func (next AutoRules) LooserThan(prev AutoRules) bool {
	prevMin, nextMin := prev.MinRelevance, next.MinRelevance
	if prevMin == 0 {
		prevMin = 40
	}
	if nextMin == 0 {
		nextMin = 40
	}
	if nextMin < prevMin {
		return true
	}
	prevCap, nextCap := prev.MaxPlacementsPer30Days, next.MaxPlacementsPer30Days
	if prevCap == 0 {
		prevCap = 3
	}
	if nextCap == 0 {
		nextCap = 3
	}
	if nextCap > prevCap {
		return true
	}
	if next.CopyEditsAllowed && !prev.CopyEditsAllowed {
		return true
	}
	if next.MinSiteQuality < prev.MinSiteQuality {
		return true
	}
	if allowLooser(prev.AllowMakers, next.AllowMakers) || allowLooser(prev.AllowCategories, next.AllowCategories) {
		return true
	}
	if allowLooser(prev.AllowedPositions, next.AllowedPositions) || allowLooser(prev.AllowedNiches, next.AllowedNiches) {
		return true
	}
	if denyLooser(prev.DenyMakers, next.DenyMakers) || denyLooser(prev.DenyCategories, next.DenyCategories) {
		return true
	}
	return false
}

func allowLooser(prev, next []string) bool {
	if len(prev) == 0 {
		return false
	}
	if len(next) == 0 {
		return true
	}
	return addsAny(next, prev)
}

func denyLooser(prev, next []string) bool {
	return removesAny(prev, next)
}

func addsAny(next, prev []string) bool {
	have := map[string]bool{}
	for _, item := range prev {
		have[strings.ToLower(item)] = true
	}
	for _, item := range next {
		if !have[strings.ToLower(item)] {
			return true
		}
	}
	return false
}

func removesAny(prev, next []string) bool {
	keep := map[string]bool{}
	for _, item := range next {
		keep[strings.ToLower(item)] = true
	}
	for _, item := range prev {
		if !keep[strings.ToLower(item)] {
			return true
		}
	}
	return false
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func containsFold(list []string, value string) bool {
	for _, item := range list {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}
