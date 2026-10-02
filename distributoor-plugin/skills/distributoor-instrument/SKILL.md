---
name: distributoor-instrument
description: Wire Distributoor attribution into a product - the ref-capture snippet, passing the click id into Stripe / Lemon Squeezy / Polar checkout, and registering conversion webhooks. Use when a maker needs attribution set up, the ref_param_survives check fails, or conversions are arriving unattributed.
---

# Distributoor attribution instrumentation

Attribution is a relay: tracked link → `?via=<click_id>` on the landing page →
persisted client-side → passed into checkout metadata → returned to
Distributoor by the provider's webhook. You are wiring the middle of that
relay inside the maker's product.

## 1. Capture the ref parameter

Add to every landing page (the offer's `ref_param`, default `via`):

```html
<script>
(function () {
  var ref = new URLSearchParams(location.search).get("via");
  if (ref) {
    try { localStorage.setItem("dtr", ref); } catch (e) {}
    document.cookie = "dtr=" + ref + ";max-age=" + 90*24*3600 + ";path=/;SameSite=Lax";
  }
})();
</script>
```

If the site redirects visitors (locale, www, auth), the parameter must survive
to a page carrying this snippet — that is exactly what the
`ref_param_survives` readiness check probes.

## 2. Pass it into checkout

Read the value back at checkout time and attach it to the existing server-side
checkout request. Merge existing metadata. If `client_reference_id` already has
a business purpose, use `metadata.dtr` rather than overwriting it. Keep
`metadata.dtr_product_id` fixed by the server, not supplied by the browser.

| Provider | Where the click id goes |
|---|---|
| Stripe | Checkout Session `client_reference_id` (or `metadata.dtr`), plus `metadata.dtr_product_id` matching the connected product |
| Lemon Squeezy | `checkout[custom][dtr]` custom data |
| Polar | checkout `metadata.dtr` |

## 3. Register webhooks

The hosted pilot supports Stripe first paid Checkout purchases. The owner
connects it at `<api>/owner/products/<product_id>/connection`; the form verifies
the Stripe account, price, endpoint, mode and currency. The API key is used once
and discarded. The signing secret is encrypted. Never ask for either in chat.
Prepare the existing checkout integration and tests while this form is pending.

Provider endpoints:

- Stripe: `POST <api>/v1/webhooks/stripe/<product_id>` — events `checkout.session.completed`, `checkout.session.async_payment_succeeded`, `charge.refunded`. Fixed test/live mode and USD/EUR/GBP currency. Only positive paid first checkouts are eligible; renewals are outside this POC.
- Legacy Lemon Squeezy and Polar adapters exist, but do not have the verified hosted connection flow. Do not enable them or present their configuration as equivalent to the Stripe pilot.

## 4. Verify

Run `run_assessment` and confirm `ref_param_survives` passes. After a test
purchase, verify paid status, product metadata, mode, currency and the click's attribution window. `get_earnings` must show the actual eligible sale; an
"unattributed" webhook usually means step 2 isn't reaching the provider.
