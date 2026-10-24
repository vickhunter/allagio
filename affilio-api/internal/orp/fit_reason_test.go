package orp

import "strings"
import "testing"

func TestFitReasonCitesOnlyDraftTerms(t *testing.T) {
	draft := "These invoices help a solo founder track notes."
	reason := FitReason(draft, "Ahrefs Affiliate Program", "SEO suite")
	if reason != NeutralFitReason || strings.Contains(reason, "Ahrefs") || strings.Contains(reason, "mentions") {
		t.Fatalf("reason %q", reason)
	}
	matched := FitReason(draft, "Ahrefs Affiliate Program", "invoices for founders")
	if !strings.Contains(matched, "invoices") || strings.Contains(matched, "Ahrefs") {
		t.Fatalf("matched %q", matched)
	}
	if !strings.Contains(strings.ToLower(draft), "invoices") {
		t.Fatal("draft lost invoices")
	}
}

func TestDraftSurfacesMatch(t *testing.T) {
	for _, value := range DraftSurfaces {
		if !DraftSurfaceAllowed(value) {
			t.Fatalf("allowed %s", value)
		}
		req, problem, ok := ParseSuggest(map[string]any{"text": "a short draft about tools", "platform": value})
		if !ok || problem.Code != "" || req.Text == "" {
			t.Fatalf("%s %#v %v", value, problem, ok)
		}
	}
	for _, value := range []string{"tiktok", "youtube", "other", "blog.example"} {
		_, problem, ok := ParseSuggest(map[string]any{"text": "a short draft about tools", "platform": value})
		if ok || problem.Code != "invalid_message" || problem.Field != "/platform" {
			t.Fatalf("%s %#v %v", value, problem, ok)
		}
		for _, allowed := range DraftSurfaces {
			if !strings.Contains(problem.Message, allowed) {
				t.Fatalf("%s message %q missing %s", value, problem.Message, allowed)
			}
		}
	}
}

func TestAcceptHashShapeDefersMismatch(t *testing.T) {
	problem, bad := checkAcceptBody(map[string]any{"terms_hash": "not-a-hash", "offer": "ofr_testoffer01"})
	if bad || problem.Code != "" {
		t.Fatalf("malformed hash should reach the host compare: %#v %v", problem, bad)
	}
	problem, bad = checkAcceptBody(map[string]any{"offer": "ofr_testoffer01"})
	if !bad || problem.Code != "invalid_message" || problem.Field != "/body/terms_hash" || !strings.Contains(problem.Message, "sha256-<b64url43>") {
		t.Fatalf("missing %#v %v", problem, bad)
	}
}
