---
name: distributoor-launch
description: Define and submit an Offer Rails offer - commission terms, cookie and refund windows, offer copy - and track it after listing. Use when a ready product needs an offer created, submitted for review, paused, or when the maker asks how their offers are earning.
---

# Offer Rails offer launch

An offer is the monetizable unit publishers see: commission terms plus the
landing page. It rides the network's rails into publisher content once listed.

## Terms that convert

- `commission_type`: the Stripe pilot supports `percentage` or `flat` on the
  first paid Checkout. Recurring commissions are outside this pilot.
- `commission_value`: use the maker's agreed amount. Do not infer a financial
  commitment from the product's price or industry averages.
- `cookie_duration_days` (default 30): how long after a click a purchase still
  attributes. Longer is more attractive to publishers.
- `refund_window_days` (default 30): commissions stay pending this long before
  clearing for payout. Match it to the product's actual refund policy.
- `terms`: one short paragraph a publisher reads before joining — what the
  product is, who converts, any exclusions. No hype.

## Flow

1. Product must be `ready` (`get_product`); otherwise run distributoor-onboard.
2. `create_offer` with the terms agreed with the maker — confirm the
   commission before submitting, it is a financial commitment.
3. `submit_offer`. A human reviewer approves listings; rejection returns the
   offer to draft with a note — fix and resubmit.
4. After listing: `get_earnings` for clicks / sales / refunds / commission per
   offer. `update_offer` with `status: "paused"` stops new attribution without
   breaking published links; `"retired"` is permanent — confirm with the maker
   first.

## FTC note

Publisher-side disclosure is enforced by the network. Never suggest terms that
reward removing or hiding disclosure — such offers are rejected.
