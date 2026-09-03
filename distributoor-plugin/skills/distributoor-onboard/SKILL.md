---
name: distributoor-onboard
description: Take a product from zero to listed on the Distributoor network - register the maker and product, run the readiness assessment, fix failing checks in the product's own codebase, and resubmit until it passes. Use when the maker wants to distribute, list, or monetize their product through Distributoor, or asks to get their app "distribution-ready".
---

# Distributoor onboarding

You are onboarding the maker's product onto Distributoor, the agent-native
affiliate network. The `distributoor` MCP tools drive the network; the failing
checks you fix live in the maker's own codebase, which you edit directly.

## Flow

1. **Account.** If `DISTRIBUTOOR_API_KEY` is not set, call `register_maker`
   with the maker's email, hand them the returned `api_key` to store as
   `DISTRIBUTOOR_API_KEY`, and continue with it.
2. **Product.** Call `register_product` with the live `website_url` plus
   `docs_url` / `pricing_url` when they exist. Pick honest `product_type` and
   `payment_provider` values — the assessment verifies them.
3. **Assess.** Call `run_assessment`. The result is a check list; each failing
   or warning check carries a `remediation` string written for you.
4. **Fix, in their code.** For every check with status `fail` (and `warn`
   where cheap), apply the remediation in the product's codebase or hosting
   config. Typical fixes:
   - `ref_param_survives` — preserve the `?via=` query parameter through
     redirects, or capture it before redirecting (see distributoor-instrument).
   - `checkout_present` — expose the Stripe / Lemon Squeezy / Polar checkout
     from the landing or pricing page.
   - `privacy_policy`, `onboarding_flow`, `pricing_page`, `docs_present` —
     content fixes on the site itself.
   Deploy the fixes, then call `run_assessment` again. Repeat until
   `readiness_status` is `ready`. Gate checks block listing; AI rubric checks
   (onboarding_clarity, docs_quality, value_prop_clarity) don't gate but do
   shape how publishers convert — improve them when the maker agrees.
5. **Offer.** Once ready, continue with the distributoor-launch skill (or
   `create_offer` + `submit_offer` directly). Submission requires `ready`.

## Rules

- Never fake a check: fixes belong in the product, not in probing artifacts.
- Ask before changing pricing, legal pages, or anything user-visible beyond
  what a remediation asks for.
- The api_key is a secret: put it in env/secret storage, never in committed code.
