package dcp

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var PreviewTools = []Tool{{
	Name:        "preview_revenue",
	Description: "Get up to five relevant native offers for content before registration. Preview only: no writes, publication or financial obligations.",
	InputSchema: schema([]string{"context"}, map[string]interface{}{"context": str("Content context, 10–12000 characters")}),
	call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
		return c.Do(http.MethodPost, "/v1/revenue/preview", args)
	},
}}

var RevenueTools = []Tool{
	{
		Name:        "find_opportunities",
		Description: "Find up to five relevant native offers for supplied content. Returns no_match when fit is weak. A configured connection is not proof that checkout tracking has passed a live test.",
		InputSchema: schema([]string{"context"}, map[string]interface{}{"context": str("Article or tutorial context, 10–12000 characters")}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.DoSite(http.MethodPost, "/v1/revenue/opportunities", args)
		},
	},
	{
		Name:        "prepare_placement",
		Description: "Prepare exact copy and mandatory disclosure for a publisher's HTTPS article. Creates an expiring proposal. placement_publish is ask on a new site, so the call waits and does not publish. placement_publish auto commits in this call only when the domain is verified and claim checks are live. While claim checks are not live, a stored auto still waits. A site key can set that publish step to auto. A held proposal is HTTP 201 with status awaiting_approval and hold_reason, and no error key.",
		InputSchema: schema([]string{"offer_id", "surface_url", "context", "copy"}, map[string]interface{}{
			"offer_id": str("Eligible offer ID"), "surface_url": str("HTTPS article on the publisher's registered domain"), "context": str("Article content used to check relevance"), "copy": str("Proposed recommendation, without invented product claims")}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.DoSite(http.MethodPost, "/v1/placements", args)
		},
	},
	{
		Name:        "list_agent_tasks",
		Description: "Poll this publisher's durable placement inbox. Pending means owner review; approved means commit once. IDs and receipts persist across agent restarts.",
		InputSchema: schema(nil, map[string]interface{}{}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.DoSite(http.MethodGet, "/v1/placements", nil)
		},
	},
	placementTool("get_placement", "Read an owned proposal, its frozen terms, approval status and receipt.", http.MethodGet, "", nil),
	placementTool("commit_placement", "Idempotently commit an unexpired placement. placement_publish ask waits until a person approves it. placement_publish auto is committed by prepare when the domain is verified and claim checks are live. A site key can set that publish step to auto. Returns an attribution link and the text. Does not publish. confirm=true cannot replace owner approval.", http.MethodPost, "/commit", nil),
	placementTool("record_placement_receipt", "Record publisher-reported publication evidence after publishing through the owner's existing tool. Requires exact approved copy, disclosure and tracked link. This is not independent publication verification.", http.MethodPost, "/receipt", map[string]interface{}{
		"surface_url": str("Approved article URL"), "published_text": str("Published text including exact copy, disclosure and tracked link"), "evidence": str("Evidence reference or local artifact description")}),
	{
		Name:        "undo_placement",
		Description: "Undo a placement immediately. Revokes it in this request with reason undo. There is no delay.",
		InputSchema: schema([]string{"placement_id"}, map[string]interface{}{"placement_id": str("Placement ID")}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			id, err := pathArg(args, "placement_id")
			if err != nil {
				return nil, err
			}
			return callPlacementIdentity(c, http.MethodPost, "/v1/placements/"+url.PathEscape(id)+"/revoke", map[string]interface{}{"reason": "undo"})
		},
	},
	{
		Name:        "restore_placement",
		Description: "Does not restore the placement. Decide and restore require a signed-in dashboard session. Returns a sign in again link to the owner dashboard.",
		InputSchema: schema([]string{"placement_id"}, map[string]interface{}{"placement_id": str("Placement ID that was just undone")}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			if _, err := pathArg(args, "placement_id"); err != nil {
				return nil, err
			}
			link := "/owner"
			if c.BaseURL != "" {
				link = strings.TrimRight(c.BaseURL, "/") + "/owner"
			}
			return map[string]interface{}{
				"message":     "Sign in again to restore this placement: " + link,
				"sign_in_url": link,
			}, nil
		},
	},
}

// callPlacementIdentity calls the placement route with the key that is present.
// A site key or bearer wins. A maker key is used when that is the only credential.
// The call returns as soon as the API responds.
func callPlacementIdentity(c *Client, method, path string, payload interface{}) (map[string]interface{}, error) {
	if c.SiteKey != "" || c.Bearer != "" {
		return c.DoSite(method, path, payload)
	}
	if c.MakerKey != "" {
		return c.Do(method, path, payload)
	}
	return nil, fmt.Errorf("publisher verbs need a site key: set %s (the Offer Rails site api key)", EnvSiteKey)
}

func placementTool(name, description, method, suffix string, props map[string]interface{}) Tool {
	fields := map[string]interface{}{"placement_id": str("Owned placement ID")}
	required := []string{"placement_id"}
	for key, value := range props {
		fields[key] = value
		required = append(required, key)
	}
	return Tool{Name: name, Description: description, InputSchema: schema(required, fields), call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
		id, err := pathArg(args, "placement_id")
		if err != nil {
			return nil, err
		}
		return c.DoSite(method, "/v1/placements/"+url.PathEscape(id)+suffix, bodyWithout(args, "placement_id"))
	}}
}
