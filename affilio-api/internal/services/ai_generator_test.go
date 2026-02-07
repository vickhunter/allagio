package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAIContentGenerator_GenerateSnippet_ParsesJSONBlurb(t *testing.T) {
	t.Parallel()
	respBody, _ := json.Marshal(map[string]interface{}{
		"content": []map[string]string{{
			"type": "text",
			"text": "{\"blurb\":\"A helpful option to consider.\"}",
		}},
	})
	gen := &AIContentGenerator{
		APIKey: "test-key",
		Model:  "test-model",
		APIURL: "https://example.invalid/v1/messages",
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(string(respBody))),
				}, nil
			}),
		},
	}
	out, err := gen.GenerateSnippet(context.Background(), ContentRequest{
		ProgramName: "Example",
		Reason:      "Matches the topic",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "A helpful option to consider." {
		t.Fatalf("unexpected blurb: %q", out)
	}
}

func TestAIContentGenerator_GeneratePlan_ParsesPlacementFields(t *testing.T) {
	t.Parallel()
	respBody, _ := json.Marshal(map[string]interface{}{
		"content": []map[string]string{{
			"type": "text",
			"text": "{\"blurb\":\"Ottimo per creator.\",\"suggested_position\":\"before_conclusion\",\"placement_reason\":\"High intent near final decision.\"}",
		}},
	})
	gen := &AIContentGenerator{
		APIKey: "test-key",
		Model:  "test-model",
		APIURL: "https://example.invalid/v1/messages",
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(string(respBody))),
				}, nil
			}),
		},
	}
	plan, err := gen.GeneratePlan(context.Background(), ContentRequest{
		ProgramName:  "Later",
		Reason:       "Match Instagram workflows",
		ArticleTitle: "Guida Instagram",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatalf("expected plan")
	}
	if plan.SuggestedPosition != "before_conclusion" {
		t.Fatalf("unexpected position: %q", plan.SuggestedPosition)
	}
	if plan.PlacementReason == "" {
		t.Fatalf("expected placement reason")
	}
}

func TestAIContentGenerator_GeneratePlan_ParsesEnrichedFields(t *testing.T) {
	t.Parallel()
	respBody, _ := json.Marshal(map[string]interface{}{
		"content": []map[string]string{{
			"type": "text",
			"text": `{"blurb":"Se stai cercando un modo per programmare i tuoi post, Later è una scelta eccellente. L'integrazione con Instagram è fluida e il calendario visuale rende la pianificazione intuitiva.","transition_sentence":"Parlando di strumenti per la gestione dei social media, vale la pena considerare anche le opzioni di programmazione.","suggested_position":"mid_article","suggested_paragraph_index":3,"confidence_score":0.85,"placement_reason":"Fits naturally after the social media tools discussion."}`,
		}},
	})
	gen := &AIContentGenerator{
		APIKey: "test-key",
		Model:  "test-model",
		APIURL: "https://example.invalid/v1/messages",
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(string(respBody))),
				}, nil
			}),
		},
	}
	plan, err := gen.GeneratePlan(context.Background(), ContentRequest{
		ProgramName:      "Later",
		Reason:           "Match Instagram workflows",
		ArticleTitle:     "Guida Instagram per Creator",
		ArticleContent:   "Questo è un articolo su come usare Instagram per far crescere il tuo brand personale...",
		ArticleHeadings:  []string{"Introduzione", "Strumenti Social Media", "Programmazione Post", "Conclusione"},
		ExistingLinks:    []string{"buffer.com", "hootsuite.com"},
		ArticleWordCount: 1500,
		ArticleCategory:  "social-media",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatalf("expected plan")
	}
	if plan.TransitionSentence == "" {
		t.Fatalf("expected transition sentence")
	}
	if plan.SuggestedParagraphIndex != 3 {
		t.Fatalf("expected paragraph index 3, got %d", plan.SuggestedParagraphIndex)
	}
	if plan.ConfidenceScore < 0.1 || plan.ConfidenceScore > 1.0 {
		t.Fatalf("unexpected confidence score: %f", plan.ConfidenceScore)
	}
}

func TestAIContentGenerator_GeneratePlan_BackwardCompat(t *testing.T) {
	t.Parallel()
	// Old-style response without new fields should still work
	respBody, _ := json.Marshal(map[string]interface{}{
		"content": []map[string]string{{
			"type": "text",
			"text": "{\"blurb\":\"A great tool for creators.\",\"suggested_position\":\"mid_article\",\"placement_reason\":\"Relevant context.\"}",
		}},
	})
	gen := &AIContentGenerator{
		APIKey: "test-key",
		Model:  "test-model",
		APIURL: "https://example.invalid/v1/messages",
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(string(respBody))),
				}, nil
			}),
		},
	}
	plan, err := gen.GeneratePlan(context.Background(), ContentRequest{
		ProgramName: "Example",
		Reason:      "Good fit",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatalf("expected plan")
	}
	if plan.Blurb != "A great tool for creators." {
		t.Fatalf("unexpected blurb: %q", plan.Blurb)
	}
	// New fields should be zero-valued when not present in response
	if plan.TransitionSentence != "" {
		t.Fatalf("expected empty transition sentence, got %q", plan.TransitionSentence)
	}
	if plan.SuggestedParagraphIndex != 0 {
		t.Fatalf("expected 0 paragraph index, got %d", plan.SuggestedParagraphIndex)
	}
	if plan.ConfidenceScore != 0 {
		t.Fatalf("expected 0 confidence score, got %f", plan.ConfidenceScore)
	}
}

func TestAIContentGenerator_GenerateSnippet_FallbackWithoutKey(t *testing.T) {
	t.Parallel()
	gen := &AIContentGenerator{Model: "test-model"}
	out, err := gen.GenerateSnippet(context.Background(), ContentRequest{ProgramName: "Example"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty output without key, got %q", out)
	}
}
