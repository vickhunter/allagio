package services

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DiscoveryEnrichment struct {
	SuggestedName        string `json:"suggested_name"`
	SuggestedCompany     string `json:"suggested_company"`
	SuggestedSlug        string `json:"suggested_slug"`
	SuggestedWebsiteURL  string `json:"suggested_website_url"`
	SuggestedSignupURL   string `json:"suggested_signup_url"`
	SuggestedCategory    string `json:"suggested_primary_category"`
	SuggestedDescription string `json:"suggested_description"`
	Confidence           int    `json:"confidence"`
}

func EnrichDiscovery(domain, sampleURL string) DiscoveryEnrichment {
	return enrichDiscoveryInternal(context.Background(), domain, sampleURL, nil, false)
}

func EnrichDiscoveryWithMetadata(ctx context.Context, domain, sampleURL string, client *http.Client) DiscoveryEnrichment {
	return enrichDiscoveryInternal(ctx, domain, sampleURL, client, true)
}

func enrichDiscoveryInternal(ctx context.Context, domain, sampleURL string, client *http.Client, fetchRemote bool) DiscoveryEnrichment {
	cleanDomain := normalizeDomain(domain)
	website := ensureURL(cleanDomain)
	signup := strings.TrimSpace(sampleURL)
	if signup == "" {
		signup = website
	}
	slug := slugFromDomain(cleanDomain)
	name := titleFromDomain(cleanDomain)
	description := "Affiliate program discovered from site traffic."
	category := inferCategory(cleanDomain + " " + sampleURL + " " + name)
	confidence := 40

	if fetchRemote {
		title, metaDesc := fetchMetadata(ctx, chooseMetaURL(sampleURL, website), client)
		if title != "" {
			name = titleToProgramName(title, name)
			confidence += 25
		}
		if metaDesc != "" {
			description = trimDesc(metaDesc)
			confidence += 20
		}
		category = inferCategory(cleanDomain + " " + sampleURL + " " + title + " " + metaDesc)
	}

	if category == "" {
		category = "other"
	}
	if confidence > 95 {
		confidence = 95
	}

	return DiscoveryEnrichment{
		SuggestedName:        name,
		SuggestedCompany:     name,
		SuggestedSlug:        slug,
		SuggestedWebsiteURL:  website,
		SuggestedSignupURL:   signup,
		SuggestedCategory:    category,
		SuggestedDescription: description,
		Confidence:           confidence,
	}
}

func chooseMetaURL(sampleURL, website string) string {
	if strings.TrimSpace(sampleURL) != "" {
		return sampleURL
	}
	return website
}

func fetchMetadata(ctx context.Context, rawURL string, client *http.Client) (string, string) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return "", ""
	}
	if client == nil {
		client = &http.Client{Timeout: 4 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	if err != nil {
		return "", ""
	}
	text := string(body)
	title := extractTag(text, "<title>", "</title>")
	metaDesc := extractMetaDescription(text)
	return strings.TrimSpace(htmlDecode(title)), strings.TrimSpace(htmlDecode(metaDesc))
}

func extractTag(s, start, end string) string {
	lo := strings.ToLower(s)
	a := strings.Index(lo, strings.ToLower(start))
	if a < 0 {
		return ""
	}
	b := strings.Index(lo[a+len(start):], strings.ToLower(end))
	if b < 0 {
		return ""
	}
	return s[a+len(start) : a+len(start)+b]
}

func extractMetaDescription(s string) string {
	lo := strings.ToLower(s)
	idx := strings.Index(lo, `name="description"`)
	if idx < 0 {
		idx = strings.Index(lo, `name='description'`)
	}
	if idx < 0 {
		return ""
	}
	chunk := s[idx:]
	for _, key := range []string{`content="`, `content='`} {
		k := strings.Index(strings.ToLower(chunk), key)
		if k < 0 {
			continue
		}
		rest := chunk[k+len(key):]
		quote := key[len(key)-1:]
		end := strings.Index(rest, quote)
		if end > 0 {
			return rest[:end]
		}
	}
	return ""
}

func htmlDecode(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&quot;", "\"", "&#39;", "'", "&lt;", "<", "&gt;", ">")
	return r.Replace(s)
}

func normalizeDomain(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	input = strings.TrimPrefix(input, "http://")
	input = strings.TrimPrefix(input, "https://")
	input = strings.TrimPrefix(input, "www.")
	input = strings.Split(input, "/")[0]
	return input
}

func slugFromDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) == 0 {
		return ""
	}
	return strings.ReplaceAll(parts[0], "_", "-")
}

func titleFromDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) == 0 || parts[0] == "" {
		return "Unknown Program"
	}
	return strings.Title(strings.ReplaceAll(parts[0], "-", " "))
}

func ensureURL(domain string) string {
	if domain == "" {
		return ""
	}
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		return domain
	}
	return "https://" + domain
}

func titleToProgramName(title, fallback string) string {
	raw := strings.TrimSpace(title)
	if raw == "" {
		return fallback
	}
	for _, sep := range []string{"|", "-", ":", "·"} {
		if strings.Contains(raw, sep) {
			raw = strings.TrimSpace(strings.Split(raw, sep)[0])
			break
		}
	}
	if raw == "" {
		return fallback
	}
	return raw
}

func inferCategory(input string) string {
	s := strings.ToLower(input)
	switch {
	case strings.Contains(s, "hosting"):
		return "hosting"
	case strings.Contains(s, "seo"), strings.Contains(s, "keyword"), strings.Contains(s, "backlink"):
		return "marketing"
	case strings.Contains(s, "email"), strings.Contains(s, "newsletter"):
		return "email"
	case strings.Contains(s, "shop"), strings.Contains(s, "ecommerce"), strings.Contains(s, "store"):
		return "ecommerce"
	case strings.Contains(s, "vpn"), strings.Contains(s, "security"), strings.Contains(s, "privacy"):
		return "security"
	case strings.Contains(s, "ai"), strings.Contains(s, "assistant"):
		return "ai"
	default:
		return ""
	}
}

func trimDesc(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return "Affiliate program discovered from site traffic."
	}
	if len(s) > 220 {
		return strings.TrimSpace(s[:217]) + "..."
	}
	return s
}
