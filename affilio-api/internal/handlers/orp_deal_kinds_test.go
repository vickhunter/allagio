package handlers

import (
	"net/http"
	"testing"
)

// Every deal type the landing page advertises must negotiate end to end:
// offer -> counterparties sign -> active. The kind label selects the
// commercial shape; the seat/signing machinery is the same for all of them.
func TestORP04EveryAdvertisedDealKind(t *testing.T) {
	cases := []struct {
		name      string
		kind      string
		terms     func(a, b party04) map[string]any
		trackWant int
	}{
		{
			name: "link swap",
			kind: "swap",
			// Two mutual placements: each party places a link for the other.
			terms: func(a, b party04) map[string]any {
				return map[string]any{
					"title": "Reciprocal link exchange", "kind": "swap",
					"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
					"obligations": []any{
						map[string]any{"id": "a-links-b", "by": a.party, "action": "place",
							"content": map[string]any{"link": true, "brief": "Mention B."},
							"destination": "https://b.example.com", "proof": []any{"link"}},
						map[string]any{"id": "b-links-a", "by": b.party, "action": "place",
							"content": map[string]any{"link": true, "brief": "Mention A."},
							"destination": "https://a.example.com", "proof": []any{"link"}},
					},
				}
			},
			trackWant: 2,
		},
		{
			name: "link exchange",
			kind: "link_exchange",
			terms: func(a, b party04) map[string]any {
				return map[string]any{
					"title": "Editorial link exchange", "kind": "link_exchange",
					"disclosure": map[string]any{"label": "Affiliate link", "lang": "en"},
					"obligations": []any{
						map[string]any{"id": "a-links-b", "by": a.party, "action": "place",
							"content": map[string]any{"link": true, "brief": "Mention B."},
							"destination": "https://b.example.com", "proof": []any{"link"}},
						map[string]any{"id": "b-links-a", "by": b.party, "action": "place",
							"content": map[string]any{"link": true, "brief": "Mention A."},
							"destination": "https://a.example.com", "proof": []any{"link"}},
					},
				}
			},
			trackWant: 2,
		},
		{
			name: "cross-sell",
			kind: "cross_sell",
			terms: func(a, b party04) map[string]any {
				return map[string]any{
					"title": "Bundle placement beside checkout", "kind": "cross_sell",
					"disclosure": map[string]any{"label": "Sponsored", "lang": "en"},
					"obligations": []any{map[string]any{"id": "place", "by": b.party, "action": "place",
						"content":     map[string]any{"link": true, "brief": "Recommend the add-on."},
						"destination": "https://a.example.com/addon", "proof": []any{"link"}}},
				}
			},
			trackWant: 1,
		},

		{
			name: "repost",
			kind: "repost",
			terms: func(a, b party04) map[string]any {
				return map[string]any{
					"title": "Repost the launch piece", "kind": "repost",
					"disclosure": map[string]any{"label": "Sponsored", "lang": "en"},
					"obligations": []any{map[string]any{"id": "repost", "by": b.party, "action": "repost",
						"target": "https://a.example.com/launch", "proof": []any{"link"}}},
				}
			},
			trackWant: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, _, r, cleanup := newORP04(t)
			defer cleanup()
			e := &orp04Env{t: t, r: r}
			offerer := joinAuto(t, db, r)
			acceptor := joinAuto(t, db, r)

			code, opened := e.send(offerer, "0.4", "offer", "", map[string]any{
				"terms": tc.terms(offerer, acceptor),
				"parties": []any{
					map[string]any{"id": offerer.party, "role": "offerer", "required": true},
					map[string]any{"id": acceptor.party, "role": "acceptor", "required": true},
				},
			})
			if code != http.StatusOK || opened["orp"] != "0.4" {
				t.Fatalf("open %d %s", code, mustJSON(opened))
			}
			storedTerms, _ := docBody(t, opened)["terms"].(map[string]any)
			if got := storedTerms["kind"]; got != tc.kind {
				t.Fatalf("kind %v stored, want %s", got, tc.kind)
			}
			code, done := acceptSeat(e, acceptor, opened)
			body := docBody(t, done)
			if code != http.StatusOK || body["state"] != "active" {
				t.Fatalf("active %d %s", code, mustJSON(done))
			}
			tracking, _ := body["tracking"].([]any)
			if len(tracking) != tc.trackWant {
				t.Fatalf("tracking %s", mustJSON(done))
			}
		})
	}
}
