# DCP v0.1 — Verbs

## Maker verbs (MCP tools)

Exposed by the network's MCP server; authenticated by the maker api key
(env `DISTRIBUTOOR_API_KEY` for the reference server). Input fields marked *
are required.

| Tool | Input | Result |
|---|---|---|
| `register_maker` | email*, name | `{maker_id, api_key}` |
| `register_product` | name*, website_url*, tagline, docs_url, pricing_url, product_type, payment_provider, primary_category, tags[], keywords[] | Product |
| `list_products` | — | `{products: Product[]}` |
| `get_product` | product_id* | Product |
| `update_product` | product_id* + any register_product field | Product |
| `run_assessment` | product_id*, ref_param | `{assessment_id, readiness_status, readiness_score, checks[], ai_summary}` |
| `list_assessments` | product_id* | `{assessments: Assessment[]}` |
| `create_offer` | product_id*, commission_type*, commission_value*, commission_currency, cookie_duration_days, refund_window_days, landing_url, ref_param, terms | Offer |
| `list_offers` | — | `{offers: Offer[]}` |
| `get_offer` | offer_id* | Offer |
| `update_offer` | offer_id* + fields, or status transition | Offer |
| `submit_offer` | offer_id* | `{id, status: "submitted"}` — requires product `ready` |
| `get_earnings` | — | `{offers: [{offer_id, product_name, status, clicks, sales, refunds, revenue, commission, currency}], totals[]}` |

Errors are returned as MCP tool results with `isError: true` and the reason as
text (e.g. the readiness gate: *"product has not passed readiness
assessment"*), so agents can react without protocol-level failures.

## Publisher verbs (HTTP-bound in v0.1)

The demand side is served by publisher platforms (WordPress plugin today), so
v0.1 binds these to HTTP under the publisher's site key rather than MCP:

- `list_offers` — `GET /v1/marketplace/offers` → listed offers with terms and
  join state for this site.
- `join_offer` — `POST /v1/marketplace/offers/{id}/join` → `{campaign_id,
  tracking_code, tracked_link}`.
- `get_balance` — `GET /v1/payouts/balance` → per-currency
  `{pending, cleared_unpaid, paid}`; `GET /v1/payouts/history` for the trail.

A publisher-facing MCP binding of the same verbs is planned but not part of v0.1.
