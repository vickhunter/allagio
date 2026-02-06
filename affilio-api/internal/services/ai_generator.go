package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type AIContentGenerator struct {
	APIKey     string
	Model      string
	APIURL     string
	APIVersion string
	Client     *http.Client
}

type ContentRequest struct {
	ProgramName    string
	Company        string
	Reason         string
	InsertionType  string
	ProgramType    string
	ArticleTitle   string
	ArticleURL     string
	ArticleContext string
	PreferredTone  string
	SiteLanguage   string
}

type GeneratedSnippetPlan struct {
	Blurb             string
	SuggestedPosition string
	PlacementReason   string
}

func (g *AIContentGenerator) GenerateSnippet(ctx context.Context, input ContentRequest) (string, error) {
	plan, err := g.GeneratePlan(ctx, input)
	if err != nil {
		return "", err
	}
	if plan == nil {
		return "", nil
	}
	return strings.TrimSpace(plan.Blurb), nil
}

func (g *AIContentGenerator) GeneratePlan(ctx context.Context, input ContentRequest) (*GeneratedSnippetPlan, error) {
	if g == nil || g.APIKey == "" || g.Model == "" {
		return nil, nil
	}
	plan, err := g.generateWithClaude(ctx, input)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, nil
	}
	plan.Blurb = strings.TrimSpace(plan.Blurb)
	if plan.Blurb == "" {
		return nil, nil
	}
	plan.SuggestedPosition = normalizeSuggestedPosition(plan.SuggestedPosition)
	if plan.SuggestedPosition == "" {
		plan.SuggestedPosition = "mid_article"
	}
	if strings.TrimSpace(plan.PlacementReason) == "" {
		plan.PlacementReason = defaultPlacementReason(plan.SuggestedPosition)
	}
	return plan, nil
}

type claudeGenRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
	System      string          `json:"system"`
	Messages    []claudeMessage `json:"messages"`
}

type claudeGenResponse struct {
	Content []claudeContentBlock `json:"content"`
}

type blurbResponse struct {
	Blurb             string `json:"blurb"`
	SuggestedPosition string `json:"suggested_position"`
	PlacementReason   string `json:"placement_reason"`
}

func (g *AIContentGenerator) generateWithClaude(ctx context.Context, input ContentRequest) (*GeneratedSnippetPlan, error) {
	apiURL := strings.TrimSpace(g.APIURL)
	if apiURL == "" {
		apiURL = "https://api.anthropic.com/v1/messages"
	}
	apiVersion := strings.TrimSpace(g.APIVersion)
	if apiVersion == "" {
		apiVersion = "2023-06-01"
	}
	client := g.Client
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	payload := claudeGenRequest{
		Model:       g.Model,
		MaxTokens:   220,
		Temperature: 0.3,
		System:      "You are a conversion-focused affiliate writing assistant. Output JSON only. No markdown.",
		Messages: []claudeMessage{
			{
				Role:    "user",
				Content: buildGenPrompt(input),
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.APIKey)
	req.Header.Set("anthropic-version", apiVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("claude request failed")
	}
	var parsed claudeGenResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	text := ""
	for _, block := range parsed.Content {
		if block.Type == "text" && block.Text != "" {
			text = block.Text
			break
		}
	}
	if text == "" {
		return nil, errors.New("claude empty response")
	}
	var out blurbResponse
	if err := json.Unmarshal([]byte(text), &out); err == nil && strings.TrimSpace(out.Blurb) != "" {
		return &GeneratedSnippetPlan{
			Blurb:             strings.TrimSpace(out.Blurb),
			SuggestedPosition: strings.TrimSpace(out.SuggestedPosition),
			PlacementReason:   strings.TrimSpace(out.PlacementReason),
		}, nil
	}
	return &GeneratedSnippetPlan{Blurb: text}, nil
}

func buildGenPrompt(input ContentRequest) string {
	if strings.TrimSpace(input.ProgramName) == "" {
		input.ProgramName = "this product"
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		reason = "it is a solid fit"
	}
	company := strings.TrimSpace(input.Company)
	if company != "" {
		company = " (" + company + ")"
	}
	title := strings.TrimSpace(input.ArticleTitle)
	if title == "" {
		title = "this article"
	}
	url := strings.TrimSpace(input.ArticleURL)
	if url == "" {
		url = "n/a"
	}
	tone := strings.TrimSpace(input.PreferredTone)
	if tone == "" {
		tone = "match the article tone"
	}
	language := strings.TrimSpace(input.SiteLanguage)
	if language == "" {
		language = "auto-detect from article context"
	}
	context := strings.TrimSpace(input.ArticleContext)
	if context == "" {
		context = "n/a"
	}
	return "Write one concise affiliate recommendation snippet that feels native to the article voice and language. " +
		"Max 34 words, no hype, no disclosure text, no markdown, no emojis. " +
		"Choose one suggested_position from: after_intro, mid_article, before_conclusion, end_of_article. " +
		"Return JSON only: {\"blurb\":\"...\",\"suggested_position\":\"...\",\"placement_reason\":\"...\"}. placement_reason max 16 words.\n\n" +
		"Article title: " + title + "\n" +
		"Article URL: " + url + "\n" +
		"Article context excerpt: " + context + "\n" +
		"Preferred tone: " + tone + "\n" +
		"Target language: " + language + "\n" +
		"Product: " + input.ProgramName + company + "\n" +
		"Reason: " + reason + "\n" +
		"Insertion type: " + strings.TrimSpace(input.InsertionType) + "\n" +
		"Program type: " + strings.TrimSpace(input.ProgramType)
}

func normalizeSuggestedPosition(position string) string {
	switch strings.TrimSpace(strings.ToLower(position)) {
	case "after_intro":
		return "after_intro"
	case "mid_article":
		return "mid_article"
	case "before_conclusion":
		return "before_conclusion"
	case "end_of_article":
		return "end_of_article"
	default:
		return ""
	}
}

func defaultPlacementReason(position string) string {
	switch normalizeSuggestedPosition(position) {
	case "after_intro":
		return "Placed early where reader attention is highest."
	case "before_conclusion":
		return "Placed near decision phase before the final takeaway."
	case "end_of_article":
		return "Placed at the end as a low-disruption fallback."
	default:
		return "Placed mid-article near relevant explanatory content."
	}
}
