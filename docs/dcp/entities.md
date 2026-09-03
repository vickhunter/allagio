# DCP v0.1 — Entities

Six entities, serialized as JSON objects. Timestamps are RFC 3339. Money is a
decimal number in currency units with a 3-letter uppercase `currency` code.

## Maker
The supply-side account. `{maker_id, email, name, api_key}` — the `api_key`
authenticates every maker verb and MUST support rotation with a previous-key
grace window.

## Product
The thing being distributed.
`{id, name, tagline, website_url, docs_url, pricing_url, product_type,
payment_provider, primary_category, tags[], keywords[], readiness_score,
readiness_status, created_at, updated_at}`
- `product_type`: `saas | game | shopify_app | dev_tool | other`
- `payment_provider`: `stripe | lemonsqueezy | polar | none`
- `readiness_status`: `unassessed | failing | ready` — only `ready` products
  can submit offers.

## Assessment
One run of the readiness engine.
`{id, product_id, status, score, checks[], ai_summary, created_at, finished_at}`
Each check: `{key, kind: probe|ai, gate, status: pass|warn|fail, score,
evidence, remediation}`. A failing `gate` check blocks listing. `remediation`
is written for a coding agent to act on.

## Offer
The monetizable unit.
`{id, product_id, status, commission_type, commission_value, commission_unit,
commission_currency, cookie_duration_days, refund_window_days, landing_url,
ref_param, terms, program_id, created_at, updated_at}`
- `commission_type`: `percentage | flat | recurring`
- `status` lifecycle: `draft → submitted → listed ↔ paused → retired`, with
  `submitted → draft` on rejection/withdrawal. Listing is a network decision
  (human or automated review), never a maker self-transition.

## Campaign
An offer × publisher relationship. `{id, offer_id, site_id, status,
tracking_code, created_at}` — `tracking_code` is stable for the campaign's
life and addresses the tracked redirect link.

## Conversion & LedgerEntry
`Conversion: {id, offer_id, campaign_id, click_id, provider,
provider_event_id, provider_object_id, kind: sale|renewal|refund, amount,
currency, commission_amount, occurred_at}` — unique on
`(provider, provider_event_id)`.

`LedgerEntry: {id, site_id, conversion_id, kind:
commission|clawback|adjustment, amount, currency, status:
pending|cleared|reversed, clears_at, payout_id}` — see
[attribution.md](attribution.md) for the state rules.
