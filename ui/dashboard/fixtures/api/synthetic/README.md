Synthetic bodies: shapes the server does not send today, used to test UI fallbacks.
- account-digest-in-review-*.json (v9): offer items with `status: submitted`, `review_reason`, `message`. At 2d3aed7 these fields are on
  maker routes (`GET /v1/listings`, maker offers, listing cards), not on the owner digest. The UI reads them when present (optional).
- plugin-key-40x-legacy.json (v9): pre-2d3aed7 bodies with only `error`, to prove the status-code fallback still works.
