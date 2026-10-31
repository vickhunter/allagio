---
name: distributoor-onboard
description: Set up product distribution end to end in the builder's existing coding agent. Detect the stack and checkout, connect hosted MCP, reuse or register the product, wire attribution, test it, and report verified readiness. Use when a builder asks to monetize, distribute, or make a product distribution-ready.
---

# Allagio setup

Carry the integration through in this repository. Use the user's current agent,
tools and model account. The user should not have to assemble SDK snippets,
copy API keys or provide another model API key.

## 1. Detect and connect

Inspect dependency manifests, routes and existing checkout code. Infer the
framework, payment provider, product name and deployed URL from reliable project
configuration and documentation. Installer detection is a hint, not proof.
Ask only for information the repository cannot establish. Never read or upload
`.env` files, private keys or payment secrets to discover the stack.

Use the configured `distributoor` hosted MCP server. Authenticate with **maker**
access using the client's browser OAuth flow. Verified sign-in creates a new
maker automatically when no existing account conflicts. An existing maker
requires its owner's account connection; matching an email never proves access.
For clients that initially request publisher access, open
`<hosted-api>/owner?setup=maker` to set up maker access, then reconnect the client.
If no server is configured, use the installation's API URL and the client's
native MCP configuration. Never invent a production endpoint. A connection that
cannot authenticate is pending, even if configuration files were installed.

The local stdio server is a legacy alternative. `register_maker` and API-key
setup are for explicit legacy development use, not the hosted onboarding flow.

## 2. Reuse the product

Call `list_products` first. Reuse the product whose deployed website matches;
do not create a duplicate on reruns. Record its ID in a non-secret local setup
report. Otherwise call `register_product` with verified name, `website_url`,
`product_type`, `payment_provider`, and actual pricing/docs URLs when present.

## 3. Instrument the actual checkout

Use distributoor-instrument. Preserve the `via` click parameter across redirects,
capture it on the landing page and propagate it to the server's existing Stripe
Checkout creation. Merge metadata and preserve existing checkout behavior.
The server must set `dtr_product_id` to the registered product ID.
Use the project's existing libraries and test conventions; never generate a
second checkout implementation merely to pass a probe.

In this pilot, paid attribution supports Stripe first paid Checkout purchases
with a verified per-product connection. If the product uses another provider,
report that limitation instead of claiming paid attribution is configured.
Open `<hosted-api>/owner/products/<product_id>/connection` for the owner to
connect Stripe. Credentials belong in that owner form, never in chat or skills.
The agent can prepare the integration while this owner action is pending.

## 4. Verify and fix

Run the relevant project checks and `run_assessment`. Apply concrete remediations
in the product repository, then reassess after the deployed changes are actually
reachable. Preserve pricing, legal policies and payment behavior. Follow the
user's existing deployment authorization; otherwise leave a reviewable diff.
Do not retry assessments endlessly against an undeployed change.

Verify a referral through the real checkout creation in tests, including metadata
preservation and a purchase without a referral. A passing readiness probe does
not prove a paid conversion. A Stripe-delivered test purchase and refund provide
the separate end-to-end proof; confirm their results through `get_earnings`.
Use existing test-mode fixtures where available. Ask for authorization before
creating a real live charge or changing financial terms.

## 5. Finish with evidence

Write `distributoor-setup-report.md` with the product ID and these separate
states: agent connected, attribution code tested, deployed assessment passed,
Stripe connection verified, paid test purchase/refund verified, offer listed.
For each state give observed evidence or a concrete pending action. File
installation alone never means the product is earning or live.

Continue with distributoor-launch when the maker has supplied commission terms.
If terms are missing, prepare everything independent of them and ask for the
commission and refund window. Never invent a financial commitment or claim an
offer is listed before the network confirms it.
