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
