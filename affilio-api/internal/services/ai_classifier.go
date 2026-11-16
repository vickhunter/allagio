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

type AIClassifier struct {
	APIKey     string
	Model      string
	APIURL     string
	APIVersion string
	Client     *http.Client
}

// ClassifyAffiliateLinks reclassifies rule-based detections using Claude when configured.
// If Claude isn't configured or returns an error, it falls back to the original detections.
func (c *AIClassifier) ClassifyAffiliateLinks(ctx context.Context, detections []LinkDetection) ([]LinkDetection, error) {
	if c == nil || c.APIKey == "" || c.Model == "" {
		return detections, nil
	}
	results, err := c.classifyWithClaude(ctx, detections)
	if err != nil {
		return detections, err
	}
	if len(results) == 0 {
		return detections, nil
	}
	merged := mergeClassifiedDetections(detections, results)
	if len(merged) == 0 {
		return detections, nil
	}
	return merged, nil
}

type claudeMessageRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
	System      string          `json:"system"`
	Messages    []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeMessageResponse struct {
	Content []claudeContentBlock `json:"content"`
}

type claudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeLinkResult struct {
	URL          string  `json:"url"`
	IsAffiliate  bool    `json:"is_affiliate"`
	Confidence   float64 `json:"confidence"`
	ProgramGuess string  `json:"program_guess"`
}

func (c *AIClassifier) classifyWithClaude(ctx context.Context, detections []LinkDetection) ([]claudeLinkResult, error) {
	if len(detections) == 0 {
		return nil, nil
	}
	apiURL := strings.TrimSpace(c.APIURL)
	if apiURL == "" {
		apiURL = "https://api.anthropic.com/v1/messages"
	}
	apiVersion := strings.TrimSpace(c.APIVersion)
	if apiVersion == "" {
		apiVersion = "2023-06-01"
	}
	client := c.Client
	if client == nil {
		//allagio:outbound fixed-host Anthropic API
		client = &http.Client{Timeout: 12 * time.Second}
	}
	payload := claudeMessageRequest{
		Model:       c.Model,
		MaxTokens:   800,
		Temperature: 0,
		System:      "You classify whether a URL is an affiliate link. Return JSON only. Do not add any extra text.",
		Messages: []claudeMessage{
			{
				Role:    "user",
				Content: buildClaudePrompt(detections),
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
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", apiVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("claude request failed")
	}
	var parsed claudeMessageResponse
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
	var results []claudeLinkResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		return nil, err
	}
	return results, nil
}

func buildClaudePrompt(detections []LinkDetection) string {
	type input struct {
		URL          string `json:"url"`
		Domain       string `json:"domain"`
		RuleGuess    bool   `json:"rule_guess"`
		ProgramGuess string `json:"program_guess"`
	}
	items := make([]input, 0, len(detections))
	for _, d := range detections {
		items = append(items, input{
			URL:          d.URL,
			Domain:       d.Domain,
			RuleGuess:    d.IsAffiliate,
			ProgramGuess: d.ProgramGuess,
		})
	}
	b, _ := json.Marshal(items)
	return "Classify these links. Return a JSON array of {url, is_affiliate, confidence, program_guess}. Only JSON.\n\n" + string(b)
}

func mergeClassifiedDetections(original []LinkDetection, classified []claudeLinkResult) []LinkDetection {
	if len(classified) == 0 {
		return original
	}
	byURL := map[string]claudeLinkResult{}
	for _, r := range classified {
		if r.URL == "" {
			continue
		}
		byURL[r.URL] = r
	}
	out := make([]LinkDetection, 0, len(original))
	for _, d := range original {
		r, ok := byURL[d.URL]
		if !ok {
			out = append(out, d)
			continue
		}
		nd := d
		nd.IsAffiliate = r.IsAffiliate
		if r.Confidence >= 0 && r.Confidence <= 1 {
			nd.Confidence = r.Confidence
		}
		if r.ProgramGuess != "" {
			nd.ProgramGuess = r.ProgramGuess
		}
		nd.Method = "claude"
		out = append(out, nd)
	}
	return out
}
