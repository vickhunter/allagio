package dcp

import (
	"fmt"
	"net/http"

	"affilio-api/internal/entry"
)

func init() {
	PublisherTools = append(PublisherTools, stepModeTools...)
}

var stepModeTools = []Tool{
	{
		Name:        "get_step_modes",
		Description: "Read this owner or site's step modes. Each of site_signup, offer_match, placement_publish, link_change, payout, share, and revoke is auto, ask, or off. " + entry.NewSitePublishDefault + " " + entry.MissingRowPublishAsk,
		InputSchema: schema(nil, map[string]interface{}{}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return callStepModes(c, http.MethodGet, nil)
		},
	},
	{
		Name:        "set_step_modes",
		Description: "Set step modes for this owner or site. Each value is auto, ask, or off. auto runs the step. ask waits on the approval queue. off never runs and the API returns the reason.",
		InputSchema: schema(nil, map[string]interface{}{
			"site_signup":       str("auto, ask, or off"),
			"offer_match":       str("auto, ask, or off"),
			"placement_publish": str("auto, ask, or off"),
			"link_change":       str("auto, ask, or off"),
			"payout":            str("auto, ask, or off"),
			"share":             str("auto, ask, or off"),
			"revoke":            str("auto, ask, or off"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return callStepModes(c, http.MethodPut, args)
		},
	},
}

func callStepModes(c *Client, method string, payload interface{}) (map[string]interface{}, error) {
	if c.SiteKey != "" || c.Bearer != "" {
		return c.DoSite(method, "/v1/step-modes", payload)
	}
	if c.MakerKey != "" {
		return c.Do(method, "/v1/step-modes", payload)
	}
	return nil, fmt.Errorf("step modes need a site key or a maker key")
}
