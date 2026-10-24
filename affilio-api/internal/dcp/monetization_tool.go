package dcp

import "net/http"

// MonetizationTools is the drafting suggestion. It uses the bearer from
// POST /v1/agent-tokens with an empty body. It does not publish.
var MonetizationTools = []Tool{
	{
		Name:        "suggest_monetization",
		Description: "While drafting, send the draft text and get 0-3 offers the person could monetize. Each offer has offer_id, title, why_match, tracked_link, and disclosure_line. human_prompt is the sentence to show the person. Emails, @handles, and phone numbers are stripped and are not stored. A placement_publish or offer_match mode of off returns no offers. ask marks needs_human_confirm. Nothing is published. Use the bearer from POST /v1/agent-tokens with {}.",
		InputSchema: schema([]string{"text"}, map[string]interface{}{
			"text":     str("Draft text, 1 to 4000 characters. The same field as an ORP suggest request."),
			"surface":  map[string]interface{}{"type": "string", "enum": []string{"x", "blog", "newsletter", "email"}, "description": "Where the draft will be published."},
			"language": str("Language tag, such as en or it. The same values as suggest lang."),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.DoSite(http.MethodPost, "/orp/monetize", args)
		},
	},
}
