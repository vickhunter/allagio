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

Read the value back at checkout time and attach it:

| Provider | Where the click id goes |
|---|---|
| Stripe | Checkout Session `client_reference_id` (or `metadata.dtr`) |
| Lemon Squeezy | `checkout[custom][dtr]` custom data |
| Polar | checkout `metadata.dtr` |

## 3. Register webhooks

Point the provider's webhook at the network (each is verified with the
provider's signature scheme; the network operator configures the secrets):

- Stripe: `POST <api>/v1/webhooks/stripe` — events `checkout.session.completed`, `invoice.paid`, `charge.refunded`
- Lemon Squeezy: `POST <api>/v1/webhooks/lemonsqueezy` — events `order_created`, `order_refunded`
- Polar: `POST <api>/v1/webhooks/polar` — events `order.created`, `order.refunded`

## 4. Verify

Run `run_assessment` and confirm `ref_param_survives` passes. After a test
purchase, `get_earnings` should show the sale within a minute; an
"unattributed" webhook usually means step 2 isn't reaching the provider.
