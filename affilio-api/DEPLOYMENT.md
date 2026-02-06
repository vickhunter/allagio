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
- `API_KEY_SECRET` (required for site registration workflows)

## Optional Environment Variables
- `CLAUDE_API_KEY` (enable AI classification/generation)
- `CLAUDE_MODEL` (default in config)
- `CLAUDE_API_URL` (defaults to Anthropic endpoint)
- `CLAUDE_API_VERSION` (defaults to `2023-06-01`)
- `ALLOWED_ORIGINS` (if CORS restrictions are required)

## Database Migration Order
Run migrations in this exact order:
1. `internal/migrations/001_initial.sql`
2. `internal/migrations/002_seed_programs.sql`
3. `internal/migrations/003_add_user_program_id.sql`
4. `internal/migrations/004_discovery_reviews.sql`
5. `internal/migrations/005_api_key_rotation.sql`

Notes:
- `002_seed_programs.sql` uses upsert semantics on `slug`.
- `005_api_key_rotation.sql` adds key rotation support and audit table.

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
  - Admin routes via `X-Admin-Key` only (`/v1/program-discovery/*`).
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
