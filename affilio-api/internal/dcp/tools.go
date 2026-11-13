package dcp

import (
	"fmt"
	"net/http"
	"strings"
)

// Tool is one DCP verb exposed to agents over MCP.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	call        func(c *Client, args map[string]interface{}) (map[string]interface{}, error)
}

// Title is the human label directories show for the verb ("check_deals" →
// "Check deals"). The Claude connectors directory requires one per tool.
func (t Tool) Title() string {
	s := strings.ReplaceAll(t.Name, "_", " ")
	if s == "" {
		return t.Name
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func schema(required []string, props map[string]interface{}) map[string]interface{} {
	s := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

func num(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "number", "description": desc}
}

func strList(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": desc}
}

func pathArg(args map[string]interface{}, key string) (string, error) {
	value, _ := args[key].(string)
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

// bodyWithout returns the args minus the keys already consumed as path params.
func bodyWithout(args map[string]interface{}, keys ...string) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range args {
		out[k] = v
	}
	for _, k := range keys {
		delete(out, k)
	}
	return out
}

// Tools is DCP v0.1's maker verb set. Order is presentation order in agents.
var Tools = []Tool{
	{
		Name:        "get_started",
		Description: "Start here. Returns the ordered maker recipe (register_maker → register_product → run_assessment → create_offer → submit_offer), which steps are already done for this key, and next_step with the exact tool and arguments to call. Call it again after each step to stay on the guided path instead of reading documentation.",
		InputSchema: schema(nil, map[string]interface{}{}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.Do(http.MethodGet, "/v1/maker/getting-started", nil)
		},
	},
	{
		Name:        "register_maker",
		Description: "Register a new maker account on Allagio. Returns the maker_id and the api_key to use for every other tool (set it as " + EnvAPIKey + ").",
		InputSchema: schema([]string{"email"}, map[string]interface{}{
			"email": str("Maker's email address (unique)"),
			"name":  str("Display name"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.Do(http.MethodPost, "/v1/makers/register", args)
		},
	},
	{
		Name:        "register_product",
		Description: "Register a product to distribute. website_url must be the live landing page; the readiness assessment probes it.",
		InputSchema: schema([]string{"name", "website_url"}, map[string]interface{}{
			"name":             str("Product name"),
			"tagline":          str("One-line description"),
			"website_url":      str("Live landing page URL (https only)"),
			"docs_url":         str("Documentation URL"),
			"pricing_url":      str("Pricing page URL"),
			"product_type":     str("One of: saas, game, shopify_app, dev_tool, other"),
			"payment_provider": str("Optional. One of: stripe, lemonsqueezy, polar, none. Readiness does not require a known provider."),
			"primary_category": str("Lowercase category used for matching"),
			"tags":             strList("Tags used for matching"),
			"keywords":         strList("Keywords used for matching"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.Do(http.MethodPost, "/v1/maker/products", args)
		},
	},
	{
		Name:        "list_products",
		Description: "List the maker's registered products with their readiness status.",
		InputSchema: schema(nil, map[string]interface{}{}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.Do(http.MethodGet, "/v1/maker/products", nil)
		},
	},
	{
		Name:        "get_product",
		Description: "Get one product by id.",
		InputSchema: schema([]string{"product_id"}, map[string]interface{}{
			"product_id": str("Product id"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			id, err := pathArg(args, "product_id")
			if err != nil {
				return nil, err
			}
			return c.Do(http.MethodGet, "/v1/maker/products/"+id, nil)
		},
	},
	{
		Name:        "update_product",
		Description: "Update product fields (same fields as register_product).",
		InputSchema: schema([]string{"product_id"}, map[string]interface{}{
			"product_id":       str("Product id"),
			"name":             str("Product name"),
			"tagline":          str("One-line description"),
			"website_url":      str("Live landing page URL (https only)"),
			"docs_url":         str("Documentation URL"),
			"pricing_url":      str("Pricing page URL"),
			"product_type":     str("One of: saas, game, shopify_app, dev_tool, other"),
			"payment_provider": str("Optional. One of: stripe, lemonsqueezy, polar, none. Readiness does not require a known provider."),
			"primary_category": str("Lowercase category used for matching"),
			"tags":             strList("Tags used for matching"),
			"keywords":         strList("Keywords used for matching"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			id, err := pathArg(args, "product_id")
			if err != nil {
				return nil, err
			}
			return c.Do(http.MethodPatch, "/v1/maker/products/"+id, bodyWithout(args, "product_id"))
		},
	},
	{
		Name:        "run_assessment",
		Description: "Run the distribution-readiness assessment against the product's live pages. Checkout is provider-agnostic: a buy, subscribe, checkout, start, or sign-up link passes, and no known payment provider is only a warning. Reachable site, pricing, docs, privacy, and terms are gates. Fix every failing gate using each check's remediation, then run again. Offers can be submitted only while the product is ready, with or without a connected payment provider.",
		InputSchema: schema([]string{"product_id"}, map[string]interface{}{
			"product_id":   str("Product id"),
			"ref_param":    str("Query parameter the product preserves for attribution (default: via)"),
			"checkout_url": str("Optional declared checkout URL. A known provider is not required."),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			id, err := pathArg(args, "product_id")
			if err != nil {
				return nil, err
			}
			return c.Do(http.MethodPost, "/v1/maker/products/"+id+"/assess", bodyWithout(args, "product_id"))
		},
	},
	{
		Name:        "list_assessments",
		Description: "List the product's stored readiness assessments, newest first.",
		InputSchema: schema([]string{"product_id"}, map[string]interface{}{
			"product_id": str("Product id"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			id, err := pathArg(args, "product_id")
			if err != nil {
				return nil, err
			}
			return c.Do(http.MethodGet, "/v1/maker/products/"+id+"/assessments", nil)
		},
	},
	{
		Name:        "create_offer",
		Description: "Create a draft offer for a product. The revenue POC uses percentage or flat commission on the first paid checkout.",
		InputSchema: schema([]string{"product_id", "commission_type", "commission_value"}, map[string]interface{}{
			"product_id":           str("Product id"),
			"commission_type":      str("One of: percentage, flat, recurring"),
			"commission_value":     num("Commission value (percent for percentage/recurring, currency units for flat)"),
			"commission_currency":  str("3-letter currency code (default USD)"),
			"cookie_duration_days": num("Attribution window in days (default 30)"),
			"refund_window_days":   num("Hold period before commissions clear (default 30)"),
			"landing_url":          str("Offer landing URL, https only (defaults to the product website)"),
			"ref_param":            str("Query parameter carrying the click id (default via)"),
			"terms":                str("Human-readable offer terms"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.Do(http.MethodPost, "/v1/maker/offers", args)
		},
	},
	{
		Name:        "list_offers",
		Description: "List the maker's offers with their statuses. Each offer includes review_reason (claims, head, relevance, or caps) and message while it is in review. Both are null when the offer is listed.",
		InputSchema: schema(nil, map[string]interface{}{}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.Do(http.MethodGet, "/v1/maker/offers", nil)
		},
	},
	{
		Name:        "list_listings",
		Description: "List this maker's listing cards. review_reason is claims, head, relevance, or caps while the offer is in review, with a human message. Both are null when the offer is listed.",
		InputSchema: schema(nil, map[string]interface{}{}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.Do(http.MethodGet, "/v1/listings", nil)
		},
	},
	{
		Name:        "get_offer",
		Description: "Get one offer by id.",
		InputSchema: schema([]string{"offer_id"}, map[string]interface{}{
			"offer_id": str("Offer id"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			id, err := pathArg(args, "offer_id")
			if err != nil {
				return nil, err
			}
			return c.Do(http.MethodGet, "/v1/maker/offers/"+id, nil)
		},
	},
	{
		Name:        "update_offer",
		Description: "Update a draft offer's terms, or change status: listed offers can be 'paused'/'retired', paused ones re-'listed'.",
		InputSchema: schema([]string{"offer_id"}, map[string]interface{}{
			"offer_id":             str("Offer id"),
			"status":               str("Status transition: submitted->draft, listed->paused|retired, paused->listed|retired"),
			"commission_type":      str("One of: percentage, flat, recurring"),
			"commission_value":     num("Commission value"),
			"commission_currency":  str("3-letter currency code"),
			"cookie_duration_days": num("Attribution window in days"),
			"refund_window_days":   num("Hold period in days"),
			"landing_url":          str("Offer landing URL (https only)"),
			"ref_param":            str("Attribution query parameter"),
			"terms":                str("Human-readable offer terms"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			id, err := pathArg(args, "offer_id")
			if err != nil {
				return nil, err
			}
			return c.Do(http.MethodPatch, "/v1/maker/offers/"+id, bodyWithout(args, "offer_id"))
		},
	},
	{
		Name:        "submit_offer",
		Description: "Submit a draft offer. Requires the product's readiness to be 'ready'. A connected payment provider is not required. On a hosted server the claims check defaults to stub and stays there: hosted refuses real. Local real compares the listing card with the merchant page. Local off-local lists when the claims check, the cached HEAD check, minimum relevance, and the cap all pass. Trusted makers list directly. A readiness failure returns reason and message.",
		InputSchema: schema([]string{"offer_id"}, map[string]interface{}{
			"offer_id": str("Offer id"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			id, err := pathArg(args, "offer_id")
			if err != nil {
				return nil, err
			}
			return c.Do(http.MethodPost, "/v1/maker/offers/"+id+"/submit", nil)
		},
	},
	{
		Name:        "get_earnings",
		Description: "Per-offer clicks, sales, refunds, commissionable revenue and publisher commission, separated by currency and payment mode. Native Stripe revenue excludes Checkout tax and shipping.",
		InputSchema: schema(nil, map[string]interface{}{}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.Do(http.MethodGet, "/v1/maker/earnings", nil)
		},
	},
}

// PublisherTools is the publisher-side DCP verb set, authenticated by a site
// key. Bloggers keep the WordPress plugin as their surface; these tools give
// a publisher's agent the same verbs.
var PublisherTools = []Tool{
	{
		Name:        "list_marketplace_offers",
		Description: "List the offers this publisher site can join, ranked by how well each fits the site's content (relevance 0-100). Joined offers include their tracked link.",
		InputSchema: schema(nil, map[string]interface{}{}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.DoSite(http.MethodGet, "/v1/marketplace/offers", nil)
		},
	},
	{
		Name:        "join_offer",
		Description: "Join a listed offer for this publisher site. Returns the campaign and the stable tracked link to publish.",
		InputSchema: schema([]string{"offer_id"}, map[string]interface{}{
			"offer_id": str("Offer id from list_marketplace_offers"),
		}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			id, err := pathArg(args, "offer_id")
			if err != nil {
				return nil, err
			}
			return c.DoSite(http.MethodPost, "/v1/marketplace/offers/"+id+"/join", nil)
		},
	},
	{
		Name:        "get_balance",
		Description: "Publisher balances by currency and test/live/legacy mode: pending, cleared_unpaid, reserved, payment_reported and paid. Paid means owner-confirmed receipt; this is not independent bank verification.",
		InputSchema: schema(nil, map[string]interface{}{}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.DoSite(http.MethodGet, "/v1/payouts/balance", nil)
		},
	},
	{
		Name:        "get_payout_history",
		Description: "The publisher site's payouts and recent commission ledger entries.",
		InputSchema: schema(nil, map[string]interface{}{}),
		call: func(c *Client, args map[string]interface{}) (map[string]interface{}, error) {
			return c.DoSite(http.MethodGet, "/v1/payouts/history", nil)
		},
	},
}

func AllTools() []Tool {
	out := make([]Tool, 0, len(Tools)+len(PublisherTools))
	out = append(out, Tools...)
	out = append(out, PublisherTools...)
	out = append(out, RevenueTools...)
	out = append(out, PreviewTools...)
	out = append(out, MonetizationTools...)
	return append(out, ORPTools...)
}

func CallTool(c *Client, name string, args map[string]interface{}) (map[string]interface{}, error) {
	if args == nil {
		args = map[string]interface{}{}
	}
	for _, tool := range AllTools() {
		if tool.Name == name {
			return tool.call(c, args)
		}
	}
	return nil, fmt.Errorf("unknown tool %q", name)
}
