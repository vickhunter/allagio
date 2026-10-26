# DCP v0.1 — Verbs

## Maker verbs (MCP tools)

Exposed by the network's MCP server; authenticated by the maker api key
(env `OFFERRAILS_API_KEY` for the reference server). Input fields marked *
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
| `list_offers` | — | `{offers: Offer[]}` — each offer includes `review_reason` and `message`, null when listed |
| `list_listings` | — | `{listings: Listing[]}` — `GET /v1/listings`; `review_reason` and `message` are null when listed |
| `get_offer` | offer_id* | Offer |
| `update_offer` | offer_id* + fields, or status transition | Offer |
| `submit_offer` | offer_id* | On a hosted server (`GIN_MODE=release`) the claims check defaults to `stub` and stays there: hosted refuses `real`. Local `real` compares the listing card with the merchant page. Local `off-local` lists when claims, the cached HEAD check, minimum relevance, and the cap pass. A failed check stays in review: `{id, status: "submitted", review_reason, message}`. A readiness failure is `{error, reason, message}`. Trusted makers list directly. |
| `get_earnings` | — | `{offers: [{offer_id, product_name, status, clicks, sales, refunds, revenue, commission, currency}], totals[], conversions[]}` |
| `report_sale` | `POST /v1/maker/sales` | Records a seller-reported sale. Trust is `seller_reported`. A client `trust_level` is ignored. |
| `report_refund` | `POST /v1/maker/refunds` | Records a seller-reported refund for this seller's sale. It applies only when the sale is `seller_reported`. A less trusted refund stays `pending_review`. An amount of zero or less, including `-1`, is `rejected` with `invalid_refund_amount`. Omitting `amount_refunded_minor`, or setting `full` to true with no amount, records a full refund. Another seller's event id is a separate refund. |

Errors are returned as MCP tool results with `isError: true` and the reason as
text (e.g. the readiness gate: *"product has not passed readiness
assessment"*), so agents can react without protocol-level failures. That
readiness error also includes `reason` (`claims`, `head`, or `relevance`) and
`message`.

## Publisher verbs (MCP tools + HTTP)

The demand side is served by publisher platforms (WordPress plugin today) over
HTTP under the publisher's site key, and the same verbs are exposed as MCP
tools (site key via `OFFERRAILS_SITE_KEY` on the stdio server, or the
`X-API-Key` header on the hosted transport):

| Tool | HTTP binding | Result |
|---|---|---|
| `list_marketplace_offers` | `GET /v1/marketplace/offers` | Listed offers ranked by fit for this site, with terms, `relevance`, and join state |
| `join_offer` | `POST /v1/marketplace/offers/{id}/join` | `{campaign_id, tracking_code, tracked_link}` |
| `get_balance` | `GET /v1/payouts/balance` | Per-currency `{pending, cleared_unpaid, paid}` |
| `get_payout_history` | `GET /v1/payouts/history` | Payouts and recent ledger entries |

## Transports

Both verb sets are served two ways: a **stdio** MCP server whose command and
reported name are `offer-rails-mcp` (`affilio-api/cmd/offer-rails-mcp`;
`affilio-api/cmd/distributoor-mcp` is the same process under the previous
binary name; env `OFFERRAILS_API_URL`, `OFFERRAILS_API_KEY`,
`OFFERRAILS_SITE_KEY`, with the previous `DISTRIBUTOOR_*` names still read
when the new name is blank) and a **hosted
streamable-HTTP** endpoint at `POST /mcp` when a public base is set, or
`POST /v1/mcp` otherwise (one JSON-RPC message per POST;
maker auth via `X-Maker-Key` or `Authorization: Bearer`, publisher auth via
`X-API-Key`; notifications are acknowledged with 202). The npm package
`offer-rails` is not published yet. Use the hosted MCP URL.
