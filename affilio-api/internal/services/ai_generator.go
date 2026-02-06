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
	ProgramName   string
	Company       string
	Reason        string
	InsertionType string
	ProgramType   string
	ArticleTitle  string
	ArticleURL    string
}

func (g *AIContentGenerator) GenerateSnippet(ctx context.Context, input ContentRequest) (string, error) {
	if g == nil || g.APIKey == "" || g.Model == "" {
		return "", nil
	}
	text, err := g.generateWithClaude(ctx, input)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
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
	Blurb string `json:"blurb"`
}

func (g *AIContentGenerator) generateWithClaude(ctx context.Context, input ContentRequest) (string, error) {
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
		System:      "You write one short, neutral sentence recommending a product. Output JSON only, no extra text.",
		Messages: []claudeMessage{
			{
				Role:    "user",
				Content: buildGenPrompt(input),
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.APIKey)
	req.Header.Set("anthropic-version", apiVersion)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New("claude request failed")
	}
	var parsed claudeGenResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	text := ""
	for _, block := range parsed.Content {
		if block.Type == "text" && block.Text != "" {
			text = block.Text
			break
		}
	}
	if text == "" {
		return "", errors.New("claude empty response")
	}
	var out blurbResponse
	if err := json.Unmarshal([]byte(text), &out); err == nil && strings.TrimSpace(out.Blurb) != "" {
		return out.Blurb, nil
	}
	return text, nil
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
	return "Write exactly one sentence (max 22 words), neutral tone, no hype, no markdown, no emojis, no disclosure text. " +
		"Return JSON only: {\"blurb\":\"...\"}. Use the article context to keep it relevant.\n\n" +
		"Article title: " + title + "\n" +
		"Article URL: " + url + "\n" +
		"Product: " + input.ProgramName + company + "\n" +
		"Reason: " + reason + "\n" +
		"Insertion type: " + strings.TrimSpace(input.InsertionType) + "\n" +
		"Program type: " + strings.TrimSpace(input.ProgramType)
}
