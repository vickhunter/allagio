# DCP v0.1 — Discovery

A DCP endpoint advertises itself at `/.well-known/dcp.json` so agents can find
and qualify distribution networks the way MCP clients find servers.

```json
{
  "dcp_version": "0.1",
  "name": "distributoor",
  "description": "Agent-native distribution network: makers publish offers, publishers earn commissions, agents do the operational work.",
  "http_base": "/v1",
  "mcp": {
    "transport": "streamable-http",
    "url": "/v1/mcp",
    "stdio": {
      "command": "distributoor-mcp",
      "env": ["DISTRIBUTOOR_API_URL", "DISTRIBUTOOR_API_KEY", "DISTRIBUTOOR_SITE_KEY"]
    }
  },
  "verbs": {
    "maker": ["register_maker", "register_product", "list_products", "get_product",
              "update_product", "run_assessment", "list_assessments", "create_offer",
              "list_offers", "get_offer", "update_offer", "submit_offer", "get_earnings"],
    "publisher": ["list_marketplace_offers", "join_offer", "get_balance", "get_payout_history"]
  },
  "payout_methods": ["manual"]
}
```

Fields:

- `dcp_version` — the spec version the endpoint implements.
- `mcp` — how to reach the network's MCP server: the hosted `streamable-http`
  transport at `url` (relative to the endpoint's origin), with the `stdio`
  block describing the locally spawned server the Agent Plugin uses.
- `verbs` — the verb names actually served; agents SHOULD treat missing verbs
  as unsupported rather than failing the whole endpoint.
- `payout_methods` — how publishers get paid (`manual`, `stripe_connect`,
  `provider_native`), so agents can set maker/publisher expectations.
