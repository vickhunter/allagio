package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	ProgramName      string
	Company          string
	Reason           string
	InsertionType    string
	ProgramType      string
	ArticleTitle     string
	ArticleURL       string
	ArticleContext   string
	PreferredTone    string
	SiteLanguage     string
	ArticleContent   string
	ArticleHeadings  []string
	ExistingLinks    []string
	ArticleWordCount int
	ArticleCategory  string
}

type GeneratedSnippetPlan struct {
	Blurb                   string
	SuggestedPosition       string
	PlacementReason         string
	TransitionSentence      string
	SuggestedParagraphIndex int
	ConfidenceScore         float64
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
	Blurb                   string  `json:"blurb"`
	SuggestedPosition       string  `json:"suggested_position"`
	PlacementReason         string  `json:"placement_reason"`
	TransitionSentence      string  `json:"transition_sentence"`
	SuggestedParagraphIndex int     `json:"suggested_paragraph_index"`
	ConfidenceScore         float64 `json:"confidence_score"`
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
		//allagio:outbound operator-config CLAUDE_API_URL, default api.anthropic.com
		client = &http.Client{Timeout: 20 * time.Second}
	}
	payload := claudeGenRequest{
		Model:       g.Model,
		MaxTokens:   600,
		Temperature: 0.4,
		System:      "You are an expert content writer who seamlessly weaves affiliate recommendations into existing articles. You match the article's voice, tone, and language exactly. Output JSON only. No markdown, no commentary.",
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
			Blurb:                   strings.TrimSpace(out.Blurb),
			SuggestedPosition:       strings.TrimSpace(out.SuggestedPosition),
			PlacementReason:         strings.TrimSpace(out.PlacementReason),
			TransitionSentence:      strings.TrimSpace(out.TransitionSentence),
			SuggestedParagraphIndex: out.SuggestedParagraphIndex,
			ConfidenceScore:         out.ConfidenceScore,
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
	articleContext := strings.TrimSpace(input.ArticleContext)
	if articleContext == "" {
		articleContext = "n/a"
	}

	// Use the richer ArticleContent if available, capped at 3000 chars
	articleContent := strings.TrimSpace(input.ArticleContent)
	if articleContent == "" {
		articleContent = articleContext
	}
	if len(articleContent) > 3000 {
		articleContent = articleContent[:3000]
	}

	headings := "none"
	if len(input.ArticleHeadings) > 0 {
		headings = strings.Join(input.ArticleHeadings, " | ")
	}

	existingLinks := "none"
	if len(input.ExistingLinks) > 0 {
		existingLinks = strings.Join(input.ExistingLinks, ", ")
	}

	wordCount := ""
	if input.ArticleWordCount > 0 {
		wordCount = fmt.Sprintf("approximately %d words", input.ArticleWordCount)
	}

	category := strings.TrimSpace(input.ArticleCategory)

	prompt := "Write a natural affiliate recommendation that reads as if the original article author wrote it. " +
		"Write 2-3 sentences that flow naturally within the article. No hype, no disclosure text, no markdown, no emojis. " +
		"CRITICAL: Write in the SAME LANGUAGE as the article. If the article is in Italian, write in Italian. If in English, write in English. " +
		"Also write a short transition sentence (1 sentence) that bridges from the surrounding article content to the recommendation.\n\n" +
		"Choose suggested_position from: after_intro, mid_article, before_conclusion, end_of_article.\n" +
		"Provide suggested_paragraph_index (0-based integer) for where to insert within the article.\n" +
		"Provide confidence_score (0.0-1.0) indicating how confident you are this recommendation fits naturally.\n\n" +
		"Return JSON only: {\"blurb\":\"...\",\"transition_sentence\":\"...\",\"suggested_position\":\"...\",\"suggested_paragraph_index\":N,\"confidence_score\":N.N,\"placement_reason\":\"...\"}.\n\n" +
		"Article title: " + title + "\n" +
		"Article URL: " + url + "\n"

	if wordCount != "" {
		prompt += "Article length: " + wordCount + "\n"
	}
	if category != "" {
		prompt += "Article category: " + category + "\n"
	}

	prompt += "Article headings: " + headings + "\n" +
		"Existing affiliate links in article: " + existingLinks + "\n" +
		"Article content:\n" + articleContent + "\n\n" +
		"Preferred tone: " + tone + "\n" +
		"Target language: " + language + "\n" +
		"Product: " + input.ProgramName + company + "\n" +
		"INTERNAL FIT NOTE (not reader-facing): " + reason + "\n" +
		"The internal fit note is scoring context for you only. It must not appear in blurb, transition_sentence, or placement_reason. " +
		"Do not quote it, paraphrase it, or include \"strong contextual fit\", \"Performance-adjusted\", historical outcome counts, or score adjustments.\n" +
		"Insertion type: " + strings.TrimSpace(input.InsertionType) + "\n" +
		"Program type: " + strings.TrimSpace(input.ProgramType)

	return prompt
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
