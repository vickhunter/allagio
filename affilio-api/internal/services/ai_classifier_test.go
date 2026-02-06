package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAIClassifier_ClassifyAffiliateLinks_MergesResults(t *testing.T) {
	t.Parallel()
	respBody, _ := json.Marshal(map[string]interface{}{
		"content": []map[string]string{{
			"type": "text",
			"text": "[{\"url\":\"https://example.com/?ref=1\",\"is_affiliate\":true,\"confidence\":0.77,\"program_guess\":\"Example\"}]",
		}},
	})
	classifier := &AIClassifier{
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
	detections := []LinkDetection{{
		URL:          "https://example.com/?ref=1",
		Domain:       "example.com",
		IsAffiliate:  false,
		Confidence:   0.1,
		Method:       "rules",
		ProgramGuess: "Old",
	}}

	got, err := classifier.ClassifyAffiliateLinks(context.Background(), detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(got))
	}
	if !got[0].IsAffiliate || got[0].Confidence != 0.77 || got[0].ProgramGuess != "Example" {
		t.Fatalf("unexpected merge result: %#v", got[0])
	}
	if got[0].Method != "claude" {
		t.Fatalf("expected method claude, got %s", got[0].Method)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAIClassifier_ClassifyAffiliateLinks_FallbackWithoutKey(t *testing.T) {
	t.Parallel()
	classifier := &AIClassifier{Model: "test-model"}
	detections := []LinkDetection{{URL: "https://example.com"}}
	got, err := classifier.ClassifyAffiliateLinks(context.Background(), detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].URL != "https://example.com" {
		t.Fatalf("expected fallback detections, got %#v", got)
	}
}
