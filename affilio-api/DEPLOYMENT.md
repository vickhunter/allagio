# Affilio API Production Deployment

## Scope
This guide covers production deployment for `affilio-api` and required integration points with the WordPress plugin.

## Prerequisites
- PostgreSQL 15+ reachable from API runtime.
- Container runtime (Docker/Kubernetes) or a host runtime for the Go binary.
- TLS termination in front of the API (reverse proxy or load balancer).
- Secure secrets storage for API keys and database credentials.

## Required Environment Variables
- `PORT` (default: `8080`)
- `DATABASE_URL` (required)
- `ADMIN_API_KEY` (required for discovery/admin routes)
- `API_KEY_SECRET` (required for site registration workflows; `GIN_MODE=release` refuses an empty value or `dev-secret`)
- `GIN_MODE` (`release` also refuses `ALLOWED_ORIGINS=*`)

## Optional Environment Variables
- `CLAUDE_API_KEY` (enable AI classification/generation)
- `CLAUDE_MODEL` (default in config)
- `CLAUDE_API_URL` (defaults to Anthropic endpoint)
- `CLAUDE_API_VERSION` (defaults to `2023-06-01`)
- `ALLOWED_ORIGINS` (if CORS restrictions are required)
- `TRUSTED_PROXIES` (comma-separated proxy IPs or CIDRs; empty ignores forwarded client IP headers). On Render, trust the Cloudflare ranges in front of the load balancer and add the observed Render proxy range if the client IP never changes. See [HOSTING.md](../docs/HOSTING.md). Do not set `0.0.0.0/0`.
- `CLAIMS_CHECK` (optional). Leave unset. Hosted is `GIN_MODE=release`, the same signal as the production startup guard, and that default is `stub`: an untrusted offer stays in review with the message "Claims are not checked yet." `off-local` keeps the four listing checks and is refused when `GIN_MODE=release`. `real` compares the listing card with the merchant page on local and dev, and a hosted server still refuses it and stays on `stub`. Do not set `real` or `off-local` on a hosted server.

## Database Migration Order
Run migrations in this exact order, before the new binary starts. `033_click_classes.sql` adds `clicks.click_class`. The new binary reads that column on every go-link hit and digest, so apply `033_click_classes.sql` before the new binary starts. `034_refund_trust.sql` adds `conversions.trust_level`, `conversions.refund_status`, and `conversions.refund_reason`. The new binary reads those columns on every sale, refund, earnings, and digest response, so apply `034_refund_trust.sql` before the new binary starts. `AUTO_MIGRATE` stays off unless it is set to `true`.
1. `internal/migrations/001_initial.sql`
2. `internal/migrations/002_seed_programs.sql`
3. `internal/migrations/003_add_user_program_id.sql`
4. `internal/migrations/004_discovery_reviews.sql`
5. `internal/migrations/005_api_key_rotation.sql`
6. `internal/migrations/006_recommendation_status_guardrails.sql`
7. `internal/migrations/007_distributoor_core.sql`
8. `internal/migrations/008_attribution_payouts.sql`
9. `internal/migrations/009_scale_fraud_trust.sql`
10. `internal/migrations/010_audit_events.sql`
11. `internal/migrations/011_hash_api_keys.sql`
12. `internal/migrations/012_agent_revenue_poc.sql`
13. `internal/migrations/013_hosted_identity.sql`
14. `internal/migrations/014_verified_stripe_connections.sql`
15. `internal/migrations/015_site_owner_connection.sql`
16. `internal/migrations/016_placement_removed_at.sql`
17. `internal/migrations/017_owner_attribution_source.sql`
18. `internal/migrations/018_d032_agent_entry.sql`
19. `internal/migrations/019_d032_account_keys.sql`
20. `internal/migrations/020_d032_kill_switch.sql`
21. `internal/migrations/021_d032_interface.sql`
22. `internal/migrations/022_d032_restore.sql`
23. `internal/migrations/023_d032_plugin_restore.sql`
24. `internal/migrations/024_d032_plugin_placed.sql`
25. `internal/migrations/025_d032_plugin_failed.sql`
26. `internal/migrations/026_d032_failed_action.sql`
27. `internal/migrations/027_d032_restore_unconfirmed.sql`
28. `internal/migrations/028_d032_placement_live.sql`
29. `internal/migrations/029_d032_post_url.sql`
30. `internal/migrations/030_d032_ui_wiring.sql`
31. `internal/migrations/031_d032_committed_live_plugin_keys.sql`
32. `internal/migrations/032_d032_box_identity.sql`
33. `internal/migrations/033_click_classes.sql`
34. `internal/migrations/034_refund_trust.sql`

Notes:
- `002_seed_programs.sql` uses upsert semantics on `slug`.
- `005_api_key_rotation.sql` adds key rotation support and audit table.
- `033_click_classes.sql` must run before the new binary. A binary that starts first will fail reads and writes that use `clicks.click_class`.
- `034_refund_trust.sql` must run before the new binary. A binary that starts first will fail reads and writes that use `conversions.trust_level` or `conversions.refund_status`.

## Build and Release
1. Build image:
   - `docker build -t affilio-api:<tag> .`
2. Push image to registry.
3. Deploy container with env vars above and network access to PostgreSQL.
4. Ensure health endpoint is reachable:
   - `GET /health` -> `{"status":"ok"}`

## Runtime Hardening
- Restrict inbound access to:
  - Public API routes via `X-API-Key`.
  - Admin routes via `X-Admin-Key` only (`/v1/program-discovery/*`, `DELETE /v1/sites`).
- Rotate site API keys periodically:
  - `POST /v1/program-discovery/sites/rotate-key`
- Store `ADMIN_API_KEY` outside source control.
- Enable request logging and central log collection.

## Branch Protection (Required Check)
Configure branch protection to require the CI test check:
1. Export variables:
   - `GITHUB_OWNER=<org-or-user>`
   - `GITHUB_REPO=<repo-name>`
   - Optional: `BRANCH_NAME=main` (defaults to `main`)
2. Run:
   - `cd affilio-api && make branch-protect`

This applies protection requiring the `affilio-api-tests` status check.

## WordPress Integration Steps
1. Deploy/upgrade API first.
2. In WordPress admin, set:
   - API URL
   - Site API key
3. Validate plugin flows:
   - Scan single post
   - Generate recommendation content
   - Insert and confirm
   - Click tracking + dashboard analytics

## Post-Deploy Validation Checklist
- `GET /health` returns 200.
- `POST /v1/scan/single` succeeds with valid `X-API-Key`.
- `GET /v1/track/summary?days=7` returns analytics payload.
- `GET /v1/track/trends?days=7` returns trend payload.
- Admin discovery endpoints require valid `X-Admin-Key`.

## Rollback
If deployment fails:
1. Roll back application image to last known good tag.
2. Keep schema changes in place unless a migration introduces breaking behavior.
3. Revert config changes (API keys, CLAUDE config, CORS) if they caused the issue.

If rollback requires DB rollback:
- Prefer forward-fix migration over destructive schema rollback.
- Snapshot database before any manual rollback actions.

## Operational Notes
- `go.sum` generation may require network in local environments.
- In restricted environments, set:
  - `GOTOOLCHAIN=local`
  - `GOCACHE=/tmp/go-cache`
  - `GOMODCACHE=/tmp/go-mod-cache`
