package services

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEnrichDiscovery_Basic(t *testing.T) {
	t.Parallel()
	out := EnrichDiscovery("www.example-seo.com", "https://example-seo.com/partners")
	if out.SuggestedName == "" || out.SuggestedSlug == "" {
		t.Fatalf("expected name/slug, got %#v", out)
	}
	if out.SuggestedCategory == "" {
		t.Fatalf("expected category inference, got %#v", out)
	}
	if out.SuggestedWebsiteURL != "https://example-seo.com" {
		t.Fatalf("unexpected website url: %s", out.SuggestedWebsiteURL)
	}
}

func TestEnrichDiscoveryWithMetadata_UsesTitleAndDescription(t *testing.T) {
	t.Parallel()
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`<html><head><title>Semrush Affiliate Program | Semrush</title><meta name="description" content="Promote Semrush and earn recurring commissions for marketers."></head></html>`,
				)),
				Header: make(http.Header),
			}, nil
		}),
	}

	out := EnrichDiscoveryWithMetadata(context.Background(), "semrush.com", "https://semrush.com/partners", client)
	if out.SuggestedName != "Semrush Affiliate Program" {
		t.Fatalf("expected title-derived name, got %q", out.SuggestedName)
	}
	if out.SuggestedDescription == "" || out.SuggestedDescription == "Affiliate program discovered from site traffic." {
		t.Fatalf("expected meta-derived description, got %q", out.SuggestedDescription)
	}
	if out.Confidence < 60 {
		t.Fatalf("expected confidence boost from metadata, got %d", out.Confidence)
	}
}
