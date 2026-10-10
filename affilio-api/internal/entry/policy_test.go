package entry

import (
	"errors"
	"strings"
	"testing"
)

func passingFacts() PlacementFacts {
	return PlacementFacts{
		Disclosure:   "(link sponsorizzato)",
		RelSponsored: true,
		Copy:         "A short honest note about the product.",
		OfferData:    "A short honest note about the product.",
		Relevance:    55,
		OfferLive:    true,
		RevokeWorks:  true,
		Position:     "end",
		MakerID:      "maker-1",
		Category:     "devtools",
	}
}

func TestHardChecksAlwaysRun(t *testing.T) {
	rules := AutoRules{MinRelevance: 40, MaxPlacementsPer30Days: 3}
	base := passingFacts()
	cases := []struct {
		name    string
		mutate  func(*PlacementFacts)
		failure string
	}{
		{"missing disclosure", func(in *PlacementFacts) { in.Disclosure = " " }, "disclosure"},
		{"missing rel", func(in *PlacementFacts) { in.RelSponsored = false }, "rel_sponsored"},
		{"over 140", func(in *PlacementFacts) { in.Copy = strings.Repeat("a", 141) }, "copy_length"},
		{"second box", func(in *PlacementFacts) { in.ExistingBoxes = 1 }, "one_per_post"},
		{"relevance 24", func(in *PlacementFacts) { in.Relevance = 24 }, "relevance_min"},
		{"relevance 30", func(in *PlacementFacts) { in.Relevance = 30 }, "relevance_min"},
		{"paused offer", func(in *PlacementFacts) { in.OfferLive = false }, "offer_live"},
		{"revoke broken", func(in *PlacementFacts) { in.RevokeWorks = false }, "revoke_path"},
		{"cap full", func(in *PlacementFacts) { in.PlacementsIn30Days = 3 }, "cap"},
	}
	for _, mode := range []string{ModeManual, ModeAuto, ModeYolo} {
		for _, tc := range cases {
			in := base
			tc.mutate(&in)
			got := EvaluatePlacement(mode, rules, in, nil)
			if got.Allowed || !hasFailure(got, tc.failure) {
				t.Fatalf("%s/%s: %+v", mode, tc.name, got)
			}
		}
	}
}

type rejectClaims struct{}

func (rejectClaims) CheckClaims(string, string) error { return errors.New("number not in offer") }
func (rejectClaims) Real() bool                       { return true }

func TestClaimCheckHookBlocks(t *testing.T) {
	got := EvaluatePlacement(ModeYolo, AutoRules{}, passingFacts(), rejectClaims{})
	if got.Allowed || !hasFailure(got, "claims_from_offer") {
		t.Fatalf("%+v", got)
	}
	ok := EvaluatePlacement(ModeYolo, AutoRules{}, passingFacts(), LiveClaimChecker{})
	if !ok.Allowed || ok.ApprovedBy != "policy:yolo" || ok.MatchedRule != "hard_checks" {
		t.Fatalf("live checker should pass yolo: %+v", ok)
	}
	stub := EvaluatePlacement(ModeYolo, AutoRules{}, passingFacts(), StubClaimChecker{})
	if stub.Allowed || !stub.NeedsOwner || stub.Reason != ClaimCheckReason || stub.Message != ClaimCheckMessage {
		t.Fatalf("stub should hold automatic go-live: %+v", stub)
	}
}

func TestAutoPolicyAndYolo(t *testing.T) {
	rules := AutoRules{
		AllowMakers:            []string{"maker-1"},
		DenyCategories:         []string{"gambling"},
		MinRelevance:           40,
		AllowedPositions:       []string{"end"},
		MaxPlacementsPer30Days: 3,
	}
	live := LiveClaimChecker{}
	in := passingFacts()
	got := EvaluatePlacement(ModeAuto, rules, in, live)
	if !got.Allowed || got.ApprovedBy != "policy:auto" || !strings.Contains(got.MatchedRule, "allow_maker") {
		t.Fatalf("auto match: %+v", got)
	}
	in.MakerID = "maker-2"
	if got = EvaluatePlacement(ModeAuto, rules, in, live); got.Allowed || !got.NeedsOwner {
		t.Fatalf("outside allow list should wait for the owner: %+v", got)
	}
	in = passingFacts()
	in.Relevance = 30
	if got = EvaluatePlacement(ModeAuto, rules, in, live); got.Allowed || got.NeedsOwner || !hasFailure(got, "relevance_min") {
		t.Fatalf("below policy relevance: %+v", got)
	}
	in = passingFacts()
	in.Relevance = 25
	yolo := EvaluatePlacement(ModeYolo, rules, in, live)
	if yolo.Allowed || !hasFailure(yolo, "relevance_min") {
		t.Fatalf("yolo enforces min relevance: %+v", yolo)
	}
	floor := rules
	floor.MinRelevance = 25
	if yolo = EvaluatePlacement(ModeYolo, floor, in, live); !yolo.Allowed || yolo.ApprovedBy != "policy:yolo" {
		t.Fatalf("yolo at the saved minimum: %+v", yolo)
	}
	in = passingFacts()
	in.PlacementsIn30Days = 3
	if got = EvaluatePlacement(ModeYolo, rules, in, live); got.Allowed || !hasFailure(got, "cap") {
		t.Fatalf("yolo cap: %+v", got)
	}
	in = passingFacts()
	in.CopyEdited = true
	if got = EvaluatePlacement(ModeAuto, rules, in, live); !hasFailure(got, "copy_edits_denied") {
		t.Fatalf("copy edits: %+v", got)
	}
	manual := EvaluatePlacement(ModeManual, rules, passingFacts(), nil)
	if manual.Allowed || !manual.NeedsOwner || manual.ApprovedBy != "" {
		t.Fatalf("manual: %+v", manual)
	}
}

func TestPresentRule(t *testing.T) {
	id, text := PresentRule("hard_checks")
	if id != "hard_checks_passed" || text != "Passed every hard check." {
		t.Fatalf("hard checks: %s %s", id, text)
	}
	id, text = PresentRule("allow_category:invoicing")
	if id != "category:invoicing" || text != "The category is invoicing." {
		t.Fatalf("category: %s %s", id, text)
	}
	id, text = PresentRule("allow_maker+allow_category:invoicing+position:end")
	if id != "category:invoicing" {
		t.Fatalf("combined: %s", id)
	}
}

func TestPublisherMirror(t *testing.T) {
	rules := AutoRules{MinSiteQuality: 60, AllowedNiches: []string{"devtools"}}
	got := EvaluatePublisher(ModeAuto, rules, PublisherFacts{OfferLive: true, SiteQuality: 70, Niche: "devtools"}, LiveClaimChecker{})
	if !got.Allowed || got.ApprovedBy != "policy:auto" || got.MatchedRule != "allow_niche:devtools" {
		t.Fatalf("%+v", got)
	}
	low := EvaluatePublisher(ModeAuto, rules, PublisherFacts{OfferLive: true, SiteQuality: 10, Niche: "devtools"}, LiveClaimChecker{})
	if low.Allowed || !low.NeedsOwner {
		t.Fatalf("quality: %+v", low)
	}
	if got := EvaluatePublisher(ModeYolo, rules, PublisherFacts{OfferLive: false, SiteQuality: 100, Niche: "devtools"}, LiveClaimChecker{}); got.Allowed || !hasFailure(got, "offer_live") {
		t.Fatalf("dead offer: %+v", got)
	}
}

func TestMinRelevanceFloor(t *testing.T) {
	if v, err := NormalizeMinRelevance(0, false); err != nil || v != 40 {
		t.Fatalf("default: %d %v", v, err)
	}
	if _, err := NormalizeMinRelevance(24, true); err == nil {
		t.Fatal("24 was accepted")
	}
	if v, err := NormalizeMinRelevance(25, true); err != nil || v != 25 {
		t.Fatalf("floor: %d %v", v, err)
	}
}

func TestAgentCannotLoosenRules(t *testing.T) {
	prev := AutoRules{MinRelevance: 40, MaxPlacementsPer30Days: 3, AllowMakers: []string{"a"}, DenyMakers: []string{"b"}, AllowedPositions: []string{"end"}}
	if (AutoRules{MinRelevance: 25, MaxPlacementsPer30Days: 3, AllowMakers: []string{"a"}, DenyMakers: []string{"b"}, AllowedPositions: []string{"end"}}).LooserThan(prev) != true {
		t.Fatal("lower relevance should be looser")
	}
	if (AutoRules{MinRelevance: 50, MaxPlacementsPer30Days: 1, AllowMakers: []string{"a"}, DenyMakers: []string{"b", "c"}, AllowedPositions: []string{"end"}}).LooserThan(prev) {
		t.Fatal("tighter rules were called looser")
	}
	if !(AutoRules{MinRelevance: 40, MaxPlacementsPer30Days: 3, DenyMakers: []string{"b"}}).LooserThan(prev) {
		t.Fatal("dropping the allow list should be looser")
	}
}

func TestHardCheckCatalogIsTheOnlyList(t *testing.T) {
	want := []string{"disclosure", "rel_sponsored", "claims_from_offer", "copy_length", "one_per_post", "relevance_min", "offer_live", "revoke_path", "cap"}
	if len(HardCheckCatalog) != len(want) {
		t.Fatalf("catalog: %+v", HardCheckCatalog)
	}
	for i, id := range want {
		if HardCheckCatalog[i].ID != id || HardCheckCatalog[i].Text == "" {
			t.Fatalf("catalog[%d]: %+v", i, HardCheckCatalog[i])
		}
		if HardChecks()[i] != id || HardCheckLabels()[i] != HardCheckCatalog[i].Text {
			t.Fatal("derived lists drifted from the catalog")
		}
	}
	first, ok := FirstCheck([]string{"cap", "disclosure", "nope"})
	if !ok || first.ID != "disclosure" || first.Text != HardCheckCatalog[0].Text {
		t.Fatalf("first check: %+v %v", first, ok)
	}
	doc := HardCheckDoc()
	for _, check := range HardCheckCatalog {
		if !strings.Contains(doc, "`"+check.ID+"`") || !strings.Contains(doc, check.Text) {
			t.Fatalf("doc missing %s", check.ID)
		}
	}
}

func TestRestoreRefusalsAreTheOnlyList(t *testing.T) {
	want := []string{
		"window_expired", "paused", "not_failed", "device_used", "account_exists",
		"not_approvable", "not_undone", "nothing_to_ack", "offer_ineligible", "offer_unavailable",
		"placement_unavailable", "receipt_mismatch", "receipt_recorded", "publisher_missing", "restore_refused",
	}
	if len(RestoreRefusals) != len(want) {
		t.Fatalf("refusals: %+v", RestoreRefusals)
	}
	doc := RestoreRefusalDoc()
	for i, reason := range want {
		if RestoreRefusals[i].Reason != reason || RestoreRefusals[i].Text == "" {
			t.Fatalf("refusal[%d]: %+v", i, RestoreRefusals[i])
		}
		got, ok := RestoreRefusalByReason(reason)
		if !ok || got != RestoreRefusals[i] {
			t.Fatalf("lookup %s: %+v %v", reason, got, ok)
		}
		if !strings.Contains(doc, "`"+reason+"`") || !strings.Contains(doc, RestoreRefusals[i].Text) {
			t.Fatalf("doc missing %s", reason)
		}
	}
}

func hasFailure(d Decision, name string) bool {
	for _, failure := range d.Failures {
		if failure == name {
			return true
		}
	}
	return false
}
