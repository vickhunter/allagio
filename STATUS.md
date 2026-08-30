# Affilio — Project Status

## Goal
Build **Affilio**, an AI affiliate copilot for WordPress (Yoast‑style) that scans posts, detects existing affiliate links, recommends curated + user‑specific programs, and supports one‑click insertion with a clear monetization score + potential indicator. The MVP proves revenue‑lift with minimal risk and a tight scope.

## MVP Plan (condensed)
1. **Scan & Analyze**: Full site scan + single post scan. Detect existing affiliate links and store them. Compute Monetization Score + Potential.
2. **Recommendations**: Prioritize user‑specific programs, then curated catalog. Provide reasons + relevance. Max 5 recs per article.
3. **Insert**: Approve recommendation → auto insert inline mention. Manual insert for other blocks. Add FTC disclosure once.
4. **Tracking**: Basic click tracking.
5. **Discovery Queue**: Unknown affiliate domains go to a review queue for catalog growth.

## Repo Structure
- `affilio-api/` Go backend API
- `affilio-wp/` WordPress plugin

## What’s Implemented

### Backend (Go)
- **API framework**: Gin, API key auth, admin key auth.
- **Database schema**:
  - Core tables: `affiliate_programs`, `sites`, `articles`, `recommendations`, `clicks`, `site_affiliate_accounts`.
  - New MVP tables: `user_programs`, `user_program_links`, `link_detections`, `program_discovery_queue`, `scan_jobs`, `program_discovery_reviews`.
  - Migrations: `001_initial.sql`, `002_seed_programs.sql` (seed list), `003_add_user_program_id.sql`, `004_discovery_reviews.sql`, `005_api_key_rotation.sql`, `006_recommendation_status_guardrails.sql`.
- **Curated catalog**:
  - Seed data deduped and upserts on `slug` for safe re-runs.
- **Scan pipeline**:
  - Upserts articles by site + post_id.
  - Detects affiliate links (rule‑based) and stores in `link_detections`.
  - Optional Claude reclassification (when `CLAUDE_API_KEY` is set) for higher‑confidence affiliate detection. Supports `CLAUDE_API_URL` + `CLAUDE_API_VERSION`.
  - Auto‑imports user programs from detected links.
  - Enqueues unknown domains to `program_discovery_queue`.
  - Computes monetization score + potential (heuristic).
  - Generates recommendations for user programs + curated programs.
- **Recommendations**:
  - `user_program_id` or `program_id` populated.
  - Reasons provided by matcher.
  - Explainable recommendation annotations in API (`reason_summary`, `expected_impact`, `placement_why`) for clearer operator decisions.
  - Max 5 per article, sorted by relevance.
  - Dedupe logic prioritizes user programs over curated duplicates.
  - Lightweight semantic token overlap for better matches.
  - Status transitions hardened in API + DB guardrails (`pending -> approved/dismissed`, `approved -> inserted/dismissed`).
- **Content generation**:
  - `GenerateContent` uses Claude when configured; falls back to a safe template.
  - Includes per-article context (title + URL) in the prompt.
- **Discovery queue**:
  - Admin endpoint includes suggested name + logo URL + review history.
  - Automated enrichment suggestions (name/company/slug/category/description/URLs + confidence) from queue domain/sample URL.
  - Metadata-assisted enrichment during promote flow to auto-fill curated program fields when payload is missing.
  - Review validation (`reviewed`/`rejected`) + audit log.
  - Pagination supported via `limit` + `offset`.
  - Promote flow to create curated programs from queue entries.
- **API key rotation**:
  - Admin-only endpoint to rotate site keys with audit trail and previous-key fallback auth.
- **Site lifecycle hardening**:
  - `POST /v1/sites/register` now validates domain input and applies per-IP rate limiting.
  - `DELETE /v1/sites` no longer accepts destructive deletes by `X-API-Key`; it requires `X-Admin-Key` plus explicit `site_id` and confirmation.
- **Tracking analytics**:
  - Endpoints for click counts by article and recommendation (`/track/articles`, `/track/recommendations`).
  - Optional `days` query param for time-range analytics.
  - Summary + daily trend endpoints (`/track/summary`, `/track/trends`).
  - Attribution endpoint with estimated revenue model (`/track/attribution`).
  - Funnel endpoint for inserted -> clicks -> dismissed metrics (`/track/funnel`).
  - Export endpoint for JSON/CSV analytics snapshots (`/track/export`).
  - Multi-touch attribution model options: `last_click`, `first_click`, `linear_session`.
  - Cohort endpoint by article publish date (`/track/cohorts`).
  - Recommendation quality endpoint with accept/dismiss/insert outcome rates (`/track/recommendation-quality`).
  - Retention/churn cohorts by article cohort week (`/track/retention`).
  - Recommendation tuning profiles endpoint (`/track/recommendation-tuning`).
  - Learning-insights payload for tuning (`insights` with boosted/penalized/neutral counts, high-confidence profiles, top movers, and notes).
  - Click anti-abuse controls on `/track/click`: per-IP short-window rate limiting and duplicate-click suppression heuristics.
  - Closed-loop recommendation tuning in scan flow using outcome-based multipliers.
- **Tests**:
  - Unit tests for services (Claude classifier + generator + matcher dedupe).
  - Unit tests for discovery enricher heuristics/metadata parsing.
  - Handler integration tests for scan, generate, detect, recommendations, and tracking analytics (requires test DB).
  - Browser-level Playwright E2E tests for WP admin onboarding/disconnected/connected flows (`affilio-wp/e2e`).
  - Test DB compose + `make test` target.
  - CI workflow for API tests via GitHub Actions (`.github/workflows/affilio-api-tests.yml`).
  - Coverage quality gate enforced in CI via `make ci-test` (`COVERAGE_MIN`, default `25`).

### WordPress Plugin
- **Admin Dashboard**:
  - Connection-aware dashboard state (shows setup message when site is not connected).
  - Lists articles with score + potential.
  - Click row → “Reasons” panel shows recommendations.
  - Approve & Insert button per rec.
  - Dismiss button per rec.
  - Analytics tables for top articles + recommendations by click count with sorting + top-N filters.
  - Time-range filter (All time / 7 days / 30 days).
  - Analytics KPI cards and daily click trend visualization.
  - Program attribution table with click-share and estimated revenue.
  - Attribution model selector (last-click, first-click, linear-session).
  - Funnel cards (inserted, clicks, dismissed, rates).
  - Cohort table by article publish date (articles/inserted/clicks/clicks per article).
  - Retention/churn table with week-based retention and churn rates.
  - Recommendation quality panel (approval/dismiss/insert/click outcome rates).
  - Recommendation tuning table (approval/insert/click rates + multiplier by program).
  - Export buttons for JSON and CSV analytics downloads.
  - Toast feedback for insert/dismiss.
  - “My Affiliate Links” list + manual add form.
- **Editor Sidebar**:
  - Connection-aware sidebar state with settings CTA when not connected.
  - Sidebar no longer sends/stores raw backend API key in browser calls; editor actions use nonce-protected WP REST proxy routes.
  - Analyze button; shows score + potential.
  - Shows detected affiliate link count.
  - Shows recommendations with “Approve & Insert.”
- **Onboarding + Settings UX**:
  - Default API URL is built-in; normal setup requires only API key.
  - API connection status checks (connected, missing key, invalid key, API URL error, network/API errors).
  - Admin banners shown on Affilio/editor screens when not connected.
  - Account creation CTA and guided setup messaging in settings.
  - Advanced mode (`WP_DEBUG` or `AFFILIO_ADVANCED_MODE`) exposes API URL override and Admin API key.
- **Discovery Queue UI**:
  - Gated behind advanced mode and admin-key presence.
  - Preview links open in new tab.
  - Filter by domain.
  - Expand history row to see review log.
  - Review/Reject actions.
  - Promote action to create curated programs.
- **Insertion**:
  - `affilio/v1/insert` REST route inserts HTML and adds FTC disclosure once.
- **Tracking**:
  - Front‑end click beacon to WP REST → backend.
  - Analytics endpoints for click counts by article and recommendation.

## Key Files
### Backend
- `affilio-api/cmd/server/main.go`
- `affilio-api/internal/database/queries.go`
- `affilio-api/internal/handlers/scan.go`
- `affilio-api/internal/handlers/recommendations.go`
- `affilio-api/internal/handlers/program_discovery.go`
- `affilio-api/internal/handlers/generate.go`
- `affilio-api/internal/services/link_detector.go`
- `affilio-api/internal/services/matcher.go`
- `affilio-api/internal/services/scorer.go`

### WordPress Plugin
- `affilio-wp/affilio.php`
- `affilio-wp/includes/class-affilio-admin.php`
- `affilio-wp/includes/class-affilio-inserter.php`
- `affilio-wp/admin/build/dashboard.js`
- `affilio-wp/admin/build/sidebar.js`
- `affilio-wp/admin/build/discovery.js`

### Distributoor (supply side) — P0–P3 implemented
See `DISTRIBUTOOR_PLAN.md` for the full architecture. Implemented so far:
- **Makers & products** (`007_distributoor_core.sql`): maker registration (rate‑limited) + `X-Maker-Key` auth with rotation, product/offer CRUD with status guardrails (`draft → submitted → listed` via admin approve only; `listed ↔ paused → retired`).
- **Readiness engine**: deterministic probes (reachability, Stripe/Lemon Squeezy/Polar checkout markers, `ref_param_survives` redirect probe, pricing/docs/privacy/onboarding) + Claude rubric with no‑key fallback; AI extraction fills empty matcher taxonomy fields; agent‑readable remediation per check; private hosts blocked. Offer submit is gated on `readiness_status='ready'`.
- **Offer rails**: admin review queue; approve promotes the offer into `affiliate_programs` (`source='distributoor'`, `offer_id` linked) so the existing matcher/recommendation/insertion pipeline serves it unchanged.
- **Attribution** (`008_attribution_payouts.sql`): publisher marketplace (`GET /v1/marketplace/offers`, `POST /v1/marketplace/offers/:id/join`), tracked redirect `GET /v1/go/:code` (click + ref param carrying click id), `POST /v1/webhooks/:provider` for Stripe/Lemon Squeezy/Polar behind a `ProviderAdapter` interface with per‑provider HMAC verification, idempotent raw event storage, conversions joined to clicks, commissions booked as pending ledger entries clearing after the offer's refund window, refunds reversing/clawing back.
- **Payouts**: publisher `GET /v1/payouts/balance|history`, admin `POST /v1/payout-batches/run` (manual payouts per site+currency over a minimum) + `:id/mark-paid`, maker `GET /v1/maker/earnings`.
- New env vars (all optional, degrade off): `STRIPE_WEBHOOK_SECRET`, `LEMONSQUEEZY_WEBHOOK_SECRET`, `POLAR_WEBHOOK_SECRET`.
- **Agentic layer (P4)**: `distributoor` maker CLI (`cmd/distributoor`: init/assess/offer/snippet/status), stdio MCP server (`cmd/distributoor-mcp`) exposing 13 DCP maker verbs, Agent Plugins 1.0.0 package (`distributoor-plugin/` with plugin.json + mcp.json + 3 skills), DCP v0.1 spec extracted to `docs/dcp/`, and `GET /.well-known/dcp.json` discovery manifest.
- Not yet implemented: P5 scale items (site‑profile matching, Stripe Connect payouts, fraud scoring, hosted MCP transport, publisher‑side agent plugin).

## What’s Missing / Stubbed
- **Matching quality**: lightweight semantic overlap exists; no true intent or LLM semantic analysis.
- **Program discovery pipeline**: enrichment automation exists, but full quality scoring/verification workflow is still lightweight.
- **Auth hardening**: API keys are stored as plain strings; rotation now supported but no auto-revoke window.
- **Tests**: browser-level E2E now covers onboarding/connected/disconnected states; still missing full insert + tracking + analytics end-to-end assertions.

## Known Issues / Constraints
- `go mod tidy` requires outbound access to fetch modules from `proxy.golang.org` when cache is cold.
- `rg` is not available in shell; use `grep` or `find`.

## How to Run (local)
### Quick Start
1. **Start DB + API**
   - `cd affilio-api`
   - `docker-compose up -d`
   - Apply migrations in order: `001_initial.sql`, `002_seed_programs.sql`, `003_add_user_program_id.sql`, `004_discovery_reviews.sql`, `005_api_key_rotation.sql`, `006_recommendation_status_guardrails.sql`
   - Run API: `go run ./cmd/server`
2. **Install Plugin**
   - Copy `affilio-wp/` into `wp-content/plugins/affilio/`
   - Activate “Affilio” plugin in WP admin
   - Open **Affilio → Settings**, create an account if needed, and set **API Key**
3. **Test Flow**
   - Click **Scan All Posts** in **Affilio** dashboard
   - Open a post → sidebar → **Analyze**
   - Approve & insert a recommendation

## Project Tracking
- `MVP_CHECKLIST.md` is the authoritative checklist for MVP completion.
- `scripts/mvp_status.sh` prints a quick summary and pointers to key docs.
- Production runbook: `affilio-api/DEPLOYMENT.md`.
- Branch protection setup script: `scripts/setup_branch_protection.sh` (`make branch-protect`).

## Changelog
- 2026-02-05: Added MVP checklist + status script, analytics dashboard sorting/top-N, seed dedupe with upsert, new analytics endpoints, Claude integration for detection + content generation, handler tests + test DB compose, make targets.
- 2026-02-06: Added recommendation quality analytics, retention/churn cohorts endpoint + dashboard panel, and tracking handler coverage updates.
- 2026-02-06: Implemented recommendation quality model tuning loop with outcome-based multipliers and analytics visibility.
- 2026-02-06: Improved WP onboarding/connection UX with built-in API URL default, connection status banners, and advanced-mode gating for discovery/admin key settings.
- 2026-02-06: Added program discovery enrichment automation and Playwright browser E2E tests for core WP admin flows.
- 2026-02-06: Hardened recommendation status transitions in handler logic and DB migration guardrails with tests.
- 2026-02-06: Cleaned up WP admin UX for dashboard/settings (removed duplicate notices on Affilio pages, clearer connection status hierarchy, panelized layout, responsive form spacing, improved onboarding CTA presentation).
- 2026-02-06: Set plugin production API default to Railway backend (`AFFILIO_API_URL`) and improved Gutenberg sidebar UX (clearer sections, connection CTA, insert confirmation feedback).
- 2026-02-06: Refined dashboard analytics UX with class-based cards/sections/filters, responsive split panels, and reduced inline styling for easier maintenance.
- 2026-02-06: Fixed plugin connection status freshness after saving API key (reads latest option value on each check/request) and added backend CORS middleware with `ALLOWED_ORIGINS` enforcement + preflight support for browser requests.
- 2026-02-06: Added recommendation insert preview flow (generated copy + planned placement + rationale), structured generation response fields, smarter insertion placement handling in WP route, and per-post capability enforcement for insert endpoint.
- 2026-02-06: Added human-in-the-loop insertion controls (editable generated text, position selector, regenerate action, and live preview/context updates before insert) with backend `link_url` support in generation response.
- 2026-02-06: Added opportunity-first dashboard UX v1 (“Quick Wins” cards ranked by score + actionable recommendations), resilient data loading for dashboard essentials, and one-click jump-to-article review flow.
- 2026-02-06: Added localized UX copy defaults (Italian/English) for connection states, settings onboarding, dashboard core actions, recommendation preview flow, and Gutenberg sidebar labels/messages.
- 2026-02-06: Hardened site lifecycle endpoints with register domain validation + rate limiting and admin-key-only site deletion by `site_id` confirmation.
- 2026-02-06: Added explainable recommendation UX data (`reason_summary`, `expected_impact`, `placement_why`) and surfaced it in the WP dashboard recommendation cards.
- 2026-02-06: Added recommendation tuning learning-insights UX in analytics (top boosted/penalized programs, confidence counters, and operator-facing notes) plus program-name enriched tuning profiles.
- 2026-02-06: Added click anti-abuse protection for tracking (basic per-IP rate limiting + duplicate suppression window) with handler tests.
- 2026-02-06: Reduced API key exposure in Gutenberg editor/sidebar by replacing direct backend key calls with WP nonce + server-side proxy routes (`/affilio/v1/editor/*`) and capability checks.
- 2026-08-30: Added `DISTRIBUTOOR_PLAN.md` (v2) — architecture plan for Distributoor, the supply-side extension of Affilio: DCP (Distribution Context Protocol) layer on top of MCP, maker onboarding, readiness assessment, offer rails into the curated catalog, conversion attribution + payout ledger, and an agentic layer shipped as an Agent Plugins 1.0.0 package (plugin.json + skills/ + mcp.json). Documentation only; no code changes.
- 2026-08-30: Implemented Distributoor P0–P3 (migrations 007–008): maker/product/offer supply side with admin offer review promoting into `affiliate_programs`, readiness engine with listing gate, publisher marketplace + tracked redirect links, Stripe/Lemon Squeezy/Polar webhook ingestion with conversion attribution and refund clawbacks, commission ledger with refund-window holds, and manual payout batches + maker earnings endpoints. Handler/services/provider tests added; ci-test now covers the providers package.
- 2026-08-30: Implemented Distributoor P4 — agentic layer: `distributoor` maker CLI, `distributoor-mcp` stdio MCP server exposing the 13 DCP maker verbs, Agent Plugins 1.0.0 package under `distributoor-plugin/` (plugin.json + mcp.json + onboard/instrument/launch skills), DCP v0.1 spec extracted to `docs/dcp/`, and `/.well-known/dcp.json` discovery endpoint. `internal/dcp` client/tool-registry/JSON-RPC tests added and wired into ci-test.

## Next Suggested Tasks
1. Complete security launch blockers listed in `MVP_CHECKLIST.md` (`P0` items first).
2. Improve semantic matching quality (LLM/intent).
3. Extend browser E2E to full insert + tracking + analytics assertions on staging.
