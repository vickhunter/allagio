package orp

import (
	"strings"
	"testing"
)

func TestDealTitleOptIn(t *testing.T) {
	if !TestDeal(map[string]any{"title": "TEST"}) || !TestDeal(map[string]any{"title": "TEST shout-out swap"}) {
		t.Fatal("TEST titles are the fixture opt-in")
	}
	if TestDeal(map[string]any{"title": "Testing"}) || TestDeal(map[string]any{"title": "Live swap"}) || TestDeal(map[string]any{"title": "test shout-out swap"}) {
		t.Fatal("only TEST or a TEST prefix opts in")
	}
}

func TestMintTrackingBothPlaceSides(t *testing.T) {
	terms := map[string]any{"obligations": []any{
		map[string]any{"id": "place_a", "action": "place", "destination": "https://example.org/"},
		map[string]any{"id": "place_b", "action": "place", "destination": "https://example.com/"},
		map[string]any{"id": "pay", "action": "pay"},
	}}
	tracks := MintTracking("http://127.0.0.1:8080", "deal_1", terms, nil)
	if len(tracks) != 2 || tracks[0].Obligation != "place_a" || tracks[1].Obligation != "place_b" {
		t.Fatalf("tracks %+v", tracks)
	}
	if tracks[0].Token == "" || tracks[0].Token == tracks[1].Token {
		t.Fatalf("tokens %+v", tracks)
	}
	if tracks[0].Destination != "https://example.org/" || tracks[1].Destination != "https://example.com/" {
		t.Fatalf("destinations %+v", tracks)
	}
	again := MintTracking("http://127.0.0.1:8080", "deal_1", terms, tracks)
	if len(again) != 2 || again[0].Token != tracks[0].Token {
		t.Fatalf("remint %+v", again)
	}
}

func TestMintTrackingKeepsLinkAndRepostFallbacks(t *testing.T) {
	linked := map[string]any{"obligations": []any{
		map[string]any{"id": "place", "action": "place", "destination": "https://example.com/notes", "content": map[string]any{"link": true}},
	}}
	one := MintTracking("http://127.0.0.1:8080", "deal_2", linked, nil)
	if len(one) != 1 || one[0].Obligation != "place" || one[0].Destination != "https://example.com/notes" {
		t.Fatalf("linked %+v", one)
	}
	repost := map[string]any{"obligations": []any{
		map[string]any{"id": "repost", "action": "repost", "target": "https://example.com/launch"},
	}}
	fall := MintTracking("http://127.0.0.1:8080", "deal_3", repost, nil)
	if len(fall) != 1 || fall[0].Obligation != "repost" {
		t.Fatalf("repost fallback %+v", fall)
	}
}

func TestProofPageNamesEachObligation(t *testing.T) {
	page := ProofPage("Affiliate link", "active", false, nil, []ProofObligation{
		{ID: "place_a", State: "verified", Receipt: "m_receipt_a", URL: "https://host.example/orp/proof/deal_1/test/place_a"},
		{ID: "place_b", State: "claimed", Receipt: "", URL: ""},
	})
	for _, want := range []string{
		"Allagio does not hold money",
		"The deal is active.",
		"obligation place_a state verified receipt m_receipt_a url https://host.example/orp/proof/deal_1/test/place_a",
		"obligation place_b state claimed receipt  url ",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("missing %q in %s", want, page)
		}
	}
}

func TestFixturePageRendersRequiredElements(t *testing.T) {
	page := TestFixturePage(FixtureInput{
		Obligation:  "place_a",
		Disclosure:  "Affiliate link",
		Destination: "https://example.org/",
		Token:       "tk_abc",
		Direct:      true,
		Target:      "https://example.com/launch",
		Copy:        "say this",
		MustInclude: []string{"must phrase"},
	})
	for _, want := range []string{
		"TEST",
		"does not count toward reputation or earnings",
		"Affiliate link",
		`href="/go/tk_abc"`,
		`href="https://example.org/"`,
		`rel="sponsored nofollow"`,
		"https://example.com/launch",
		"say this",
		"must phrase",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("missing %q in %s", want, page)
		}
	}
}

func TestOwnTestFixture(t *testing.T) {
	origin := "http://127.0.0.1:8080"
	raw := TestFixtureURL(origin, "deal_1", "place_a")
	if !OwnTestFixture(origin, raw, "deal_1", "place_a") {
		t.Fatal("own fixture")
	}
	if OwnTestFixture(origin, raw, "deal_1", "place_b") || OwnTestFixture(origin, ProofURL(origin, "deal_1"), "deal_1", "place_a") {
		t.Fatal("public proof page is not the fixture")
	}
	if OwnTestFixture("https://127.0.0.1:8080", raw, "deal_1", "place_a") {
		t.Fatal("scheme must match")
	}
	host, id, ob, ok := SplitTestFixture(raw + "?x=1")
	if !ok || host != "127.0.0.1:8080" || id != "deal_1" || ob != "place_a" {
		t.Fatalf("split %s %s %s %v", host, id, ob, ok)
	}
}

func TestPlanPayoutsSkipsTestFixture(t *testing.T) {
	obs := []map[string]any{
		{"id": "place_a", "action": "place"},
		{"id": "place_b", "action": "place"},
		{"id": "pay", "action": "pay", "by": "offerer", "per": "placement", "amount": map[string]any{"amount_minor": 1500, "currency": "EUR"}},
	}
	states := []ObligationState{
		{ID: "place_a", State: "verified", Method: "proof_url", TestFixture: true},
		{ID: "place_b", State: "verified", Method: "proof_url", TestFixture: true},
	}
	if got := PlanPayouts(obs, states, "off", "acc"); len(got) != 0 {
		t.Fatalf("fixture payouts %+v", got)
	}
	states[0].TestFixture = false
	states[1].TestFixture = false
	if got := PlanPayouts(obs, states, "off", "acc"); len(got) != 2 {
		t.Fatalf("real payouts %+v", got)
	}
}
