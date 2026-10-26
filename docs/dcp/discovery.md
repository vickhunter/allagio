# DCP v0.1 — Discovery

A DCP endpoint advertises itself at `/.well-known/dcp.json` so agents can find
and qualify distribution networks the way MCP clients find servers.

```json
{
  "dcp_version": "0.1",
  "name": "offer-rails-mcp",
  "description": "Offer Rails. Makers publish offers, publishers earn commissions, agents do the operational work.",
  "http_base": "/v1",
  "mcp": {
    "transport": "streamable-http",
    "url": "/mcp",
    "stdio": {
      "command": "offer-rails-mcp",
      "env": ["OFFERRAILS_API_URL", "OFFERRAILS_API_KEY", "OFFERRAILS_SITE_KEY"]
    }
  },
  "verbs": {
    "maker": ["register_maker", "register_product", "list_products", "get_product",
              "update_product", "run_assessment", "list_assessments", "create_offer",
              "list_offers", "list_listings", "get_offer", "update_offer", "submit_offer", "get_earnings"],
    "publisher": ["list_marketplace_offers", "join_offer", "get_balance", "get_payout_history"]
  },
  "payout_methods": ["manual"]
}
```

Fields:

- `dcp_version` — the spec version the endpoint implements.
- `mcp` — how to reach the network's MCP server: the hosted `streamable-http`
  transport at `url` (relative to the endpoint's origin), with the `stdio`
  block describing the locally spawned server. The server name is
  `offer-rails-mcp`. The npm package `offer-rails` is not published yet. It is
  coming soon. Use the hosted MCP URL, or the installer archive on that same
  host: `npx --yes --package=<origin>/downloads/offer-rails-0.1.0.tgz offer-rails --api-url <origin>`.
- `verbs` — the verb names actually served; agents SHOULD treat missing verbs
  as unsupported rather than failing the whole endpoint.
- `payout_methods` — how publishers get paid (`manual`, `stripe_connect`,
  `provider_native`), so agents can set maker/publisher expectations.
