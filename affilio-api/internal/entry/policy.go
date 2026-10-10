package entry

import "strings"

// ClaimChecker is the D-028 claim-check hook. The stub accepts every claim.
// D-028 replaces it with the deterministic checker.
type ClaimChecker interface {
	CheckClaims(copyText, offerData string) error
}

// StubClaimChecker does not reject claims.
type StubClaimChecker struct{}

func (StubClaimChecker) CheckClaims(string, string) error { return nil }

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

func EvaluatePlacement(mode string, rules AutoRules, in PlacementFacts, claims ClaimChecker) Decision {
	if claims == nil {
		claims = StubClaimChecker{}
	}
	failures := placementHardFailures(in, claims)
	if len(failures) > 0 {
		return Decision{Failures: failures}
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

func EvaluatePublisher(mode string, rules AutoRules, in PublisherFacts) Decision {
	if !in.OfferLive {
		return Decision{Failures: []string{"offer_not_live"}}
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

// HardChecks are the failure codes that run in every mode. The dashboard
// shows the matching labels. D-028 replaces the claim-check stub.
func HardChecks() []string {
	return []string{
		"disclosure_and_rel_sponsored",
		"claim_check",
		"copy_over_140",
		"one_box_per_post",
		"relevance_below_25",
		"offer_not_live",
		"revoke_unavailable",
	}
}

// HardCheckLabels are the plain words the dashboard shows for HardChecks.
func HardCheckLabels() []string {
	return []string{
		"Disclosure label and rel=sponsored",
		"Claim check",
		"140-character cap",
		"One box per post",
		"Relevance at least 25",
		"Live offer",
		"Working revoke",
	}
}

func placementHardFailures(in PlacementFacts, claims ClaimChecker) []string {
	var failures []string
	if strings.TrimSpace(in.Disclosure) == "" || !in.RelSponsored {
		failures = append(failures, "disclosure_and_rel_sponsored")
	}
	if err := claims.CheckClaims(in.Copy, in.OfferData); err != nil {
		failures = append(failures, "claim_check")
	}
	if len([]rune(in.Copy)) > 140 {
		failures = append(failures, "copy_over_140")
	}
	if in.ExistingBoxes > 0 {
		failures = append(failures, "one_box_per_post")
	}
	if in.Relevance < 25 {
		failures = append(failures, "relevance_below_25")
	}
	if !in.OfferLive {
		failures = append(failures, "offer_not_live")
	}
	if !in.RevokeWorks {
		failures = append(failures, "revoke_unavailable")
	}
	return failures
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
