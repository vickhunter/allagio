# DCP v0.1 — Attribution semantics

These rules make "a conversion" mean the same thing on every DCP network, so
agents can reason about earnings across networks. They hold independent of any
payment provider.

## The relay

```
publisher post ── GET /v1/go/{tracking_code} ──▶ 302 landing_url?{ref_param}={click_id}
                        │ click logged                        │
                        │                       product persists the value (client-side)
                        │                                     │
                        │                       checkout metadata carries it
                        │                                     ▼
network ◀── POST /v1/webhooks/{provider} ── provider webhook (signed)
   │ conversion joined to click via click_id
   ▼
ledger entry: commission, pending, clears_at = occurred_at + refund_window_days
```

## Rules

1. **The click id is the attribution token.** The tracked redirect logs a
   click and stamps its id into the offer's `ref_param`. The product must
   deliver that exact value into provider checkout metadata (Stripe
   `client_reference_id`/`metadata.dtr`, Lemon Squeezy
   `checkout[custom][dtr]`, Polar `metadata.dtr`).
2. **Readers never see an error page for a tracked link.** `/v1/go/{tracking_code}`
   stays supported permanently: saved posts, feeds, and caches keep that URL,
   and nothing rewrites them. `?placement=` follows the placement rules. Without
   it, a click is logged only when a placement on that campaign's site counts
   as live. A revoked code, a paused or inactive campaign, and every other
   clicks-off state returns `302` to the offer's plain URL with no ref and no
   click. When the offer is not listed, paused, or the stored landing URL is
   not `https`, a bare code returns `302` to the site home. `?placement=`
   returns `302` to the referring post, or the site home when that post is
   missing. Link-preview clients and `HEAD` get the same redirect and record
   no click. A deleted or unknown tracking code returns `302` to the Allagio
   home page. A rate limit is that untracked `302`, never a 429. Readers never
   see a 4xx or a 5xx. Every go-link response sends `Cache-Control: no-store`
   and `X-Robots-Tag: noindex, nofollow`.
3. **Webhook authenticity.** Every provider event is verified with the
   provider's signature scheme before it touches storage. Unverifiable events
   are rejected, not stored.
4. **Idempotency.** Raw events are stored uniquely on
   `(provider, provider_event_id)`; redelivery is acknowledged as `duplicate`
   and never double-books.
5. **Honest unattribution.** A sale whose ref is missing or matches no click
   is acknowledged (`unattributed`) and recorded in the raw event log — never
   guessed onto a campaign, never silently dropped.
6. **Commission.** `percentage`/`recurring`: `amount × commission_value / 100`;
   `flat`: `commission_value`. Rounded to 2 decimals.
7. **Holds.** Commissions book as `pending` and clear only after
   `refund_window_days`. A refund inside the window reverses the pending entry
   in place; after clearing, it books a negative `clawback` entry. Refunds join
   to their sale via `provider_object_id`.
8. **Payout safety.** Only `cleared` entries without a `payout_id` are
   payable; a payout stamps the entries it covers so they can never pay twice.
