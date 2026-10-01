## October 1 — landing page and setup UX

- Added an original Three.js distribution illustration, editorial ivory/moss styling and clearer builder/WordPress entry points. The scene loads only on desktop when visible; mobile, reduced-motion, data-saving, no-JavaScript and unavailable-WebGL paths retain an original SVG illustration.
- Added accessible pause/play controls and copy-command actions in the landing page and owner workspace, with a manual selection fallback when clipboard access is unavailable.
- Self-hosted, pinned Three.js and esbuild; Docker builds the optional scene and includes its MIT notice. The WordPress ZIP excludes hosted-page assets and the UI toolchain.
- Verified Chromium workspace flow, new enhancement/fallback states and command copying; built the release Docker image. Production activation still requires the Google OAuth credentials documented in `docs/MVP-DELIVERY.md`.

# Affilio — Project Status

## Distributoor MVP implementation · 2026-10-01

Added the working owner workspace: product creation/editing, readiness checks, offer creation/submission, placement review, earnings, agent revocation, publisher payment instructions, merchant payment reports and separately confirmed receipt. The configured operator gets offer review and live commission batching. UI sources live in `affilio-wp/admin/build/`.

WordPress Distribution links an installed site to its authenticated owner through a ten-minute, single-use token, then finds native offers, prepares placements and inserts only approved recommendations. Existing content is preserved, disclosure appears once, and publication recording can retry without duplicate insertion. Agent credentials cannot claim site ownership.

`Dockerfile.mvp` bundles the API, UI, portable installer and origin-configured plugin download. Opt-in automatic migrations apply 001–015 to a fresh/history-tracked database with locking and checksum checks. Public readiness probes validate redirects and dial validated public addresses.

Provisioned isolated Railway project `distributoor-mvp`, PostgreSQL 15 with persistent storage, and application domain `distributoor-production.up.railway.app`. The image built, but startup is blocked by missing Google OAuth client credentials. **The hosted MVP is not usable yet.** Existing Affilio production is untouched. Exact activation and shipping instructions: [MVP-DELIVERY.md](docs/MVP-DELIVERY.md). The previous October 2 presentation plan is superseded; product/article recruitment is not a software release prerequisite.

Verification: full Go suite, handler/database/migration race checks, vet, seven installer tests, PHP setup/distribution suites and Docker/archive checks pass. Chromium exercises real owner backend forms, product/offer management, placement approval/commit and separate merchant/publisher payment confirmations. WordPress 6.6/PHP 8.2 browser checks also pass installed-site handoff, matching, owner approval, real post insertion and backend publication recording on an isolated local fixture. Real Google login, a real merchant checkout and real commission receipt remain unverified.

## One-entry setup · 2026-09-30

- Added `packages/distributoor-setup`: portable Node CLI, manifest detection, hosted OAuth preflight, Codex agent handoff and Cursor/Claude MCP + skill installation. Configuration and custom skills are preserved on conflicts. The npm package is prepared, not published.
- Hosted explicit maker OAuth and `/owner?setup=maker` create a new owned maker without an API key; existing email/account conflicts still require proof of ownership.
- Fresh WordPress activation opens setup. One Connect action provisions a site credential and scans published posts in resumable batches; settings show progress and retry. The backend accepts hashed, idempotent installation registration. New plugin needs the updated backend; no deployment occurred.
- Fixed missing editor proxy that prevented activation, with post access and recommendation ownership checks.
- WordPress cron drives scans. Native WordPress connection remains separate from hosted publisher-agent identity. Builder code wiring, Stripe connection and real purchase verification remain distinct from installing files.
- See `docs/ONE-STEP-SETUP.md` for release gates and `packages/distributoor-setup/README.md` for runnable local commands.
- Verification: full Go suite, handler/database race tests and vet passed; seven installer tests and the portable npm archive ran successfully. WordPress 6.6/PHP 8.2 activated and completed keyless Connect → scan → dashboard in Playwright (3 passed, 1 optional legacy-key test skipped). PHP setup tests cover lost responses, permissions, unsupported backends, batch failure and resume. The local browser fixture used an explicit cron run.

## Hosted pilot · 2026-09-30

Implemented hosted Streamable HTTP MCP with OIDC owner sign-in, OAuth discovery/DCR/S256 PKCE, hashed single-use codes, scoped access tokens, rotating refresh tokens and owner revocation. Browser approval now records verified issuer/subject authority for the exact placement; agent tokens cannot approve or confirm receipt of money. New accounts need no API key, and OAuth maker accounts have no legacy key stored. Legacy ownership requires an owner key once, never an email match.

The owner workspace supports account setup, placement review, connected-agent revocation, live payment receipt confirmation and Stripe configuration checks. A restricted merchant key is used once for read-only account/one-time-price/webhook verification, then discarded; signing secrets remain product-bound and encrypted. Hosted matching requires verified configuration. Rotation preserves account, price, currency and mode.

Migrations `013_hosted_identity.sql` and `014_verified_stripe_connections.sql` are required. Go remains 1.23. The official SDK HTTP client and installed Codex CLI tested the local hosted flow with fixture owner sessions. Real identity-provider login, public HTTPS, Cursor, Stripe-delivered checkout/refund, live commissions and recruitment still require activation and participant details. Nothing has been deployed to Railway production. See [HOSTED-PILOT.md](docs/HOSTED-PILOT.md) and `affilio-api/hosted-pilot.env.example`.

## Agent revenue POC · 2026-09-30

Implemented an agent-native workflow on the existing Go 1.23 service. Run `./scripts/run-agent-poc.sh` and open <http://127.0.0.1:8086/pilot>. Details and endpoint contracts: [AGENT-REVENUE-POC.md](docs/AGENT-REVENUE-POC.md).

- Signup-free public preview; up to five relevant native offers or `no_match`.
- Exact placement proposals, frozen terms, separate operator-recorded owner approval, expiry/revocation and stable commit receipts.
- Scoped, hashed publisher-agent keys; agents cannot approve themselves or confirm bank receipt.
- Portable revenue skill plus seven new MCP tools. Two actual local MCP processes exercise the flow.
- Product-scoped encrypted Stripe connections; paid-state, mode, currency, product and attribution-window checks; async success and payment-level dedupe.
- Durable event retries and recovery; cumulative partial/full refunds; pending batch cancellation; explicit late clawbacks.
- Separate test/live/legacy balances and maker-specific payout batches. Reserved, payment-reported and owner-confirmed receipt are distinct.
- WordPress Earnings shows mode labels and all five balance states; payment reports and cancelled batches no longer appear as received payments.
- Migration `012_agent_revenue_poc.sql`, regression coverage and a runnable local proof report in the compiled admin assets directory.

Scope: synthetic signed Stripe events, simulated owner approval and local publication artifact. Real checkout/publishing/client integrations, SSO, external-program reconciliation, funding/disputes and service-fee invoicing remain pilot launch work. The global Stripe webhook route now requires migration to a product-specific connection; renewals are outside this POC.

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
  - Migrations: `001_initial.sql`, `002_seed_programs.sql` (seed list), `003_add_user_program_id.sql`, `004_discovery_reviews.sql`, `005_api_key_rotation.sql`, `006_recommendation_status_guardrails.sql`, `007_distributoor_core.sql`, `008_attribution_payouts.sql`, `009_scale_fraud_trust.sql`, `010_audit_events.sql`, `011_hash_api_keys.sql`.
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
- **API key security**:
  - Keys stored as SHA-256 hashes with 8-char prefix for indexed lookup; plaintext never persisted.
  - Admin-only endpoint to rotate site keys with audit trail and configurable grace period (`KEY_ROTATION_GRACE_HOURS`, default 24h).
  - Previous key expires automatically after the grace window.
- **Audit logging**:
  - `audit_events` table records auth failures, admin actions, key rotations, site deletions, and discovery operations.
  - Async non-blocking audit logger with buffered channel.
  - Admin endpoints for viewing audit events (`/audit/events`) and alerts (`/audit/alerts`).
  - Alerts surface IPs with repeated auth failures (>5 in 15 minutes).
  - Admin key auth uses constant-time comparison.
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
- `affilio-api/internal/services/audit.go`
- `affilio-api/internal/services/keyhash.go`
- `affilio-api/internal/handlers/audit.go`

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
- Payment configuration: `CONNECTION_ENCRYPTION_KEY` for product-scoped Stripe credentials; `LEMONSQUEEZY_WEBHOOK_SECRET` and `POLAR_WEBHOOK_SECRET` for the legacy adapters. The old global Stripe secret no longer enables ingestion.
- **Agentic layer (P4)**: `distributoor` maker CLI (`cmd/distributoor`: init/assess/offer/snippet/status), stdio MCP server (`cmd/distributoor-mcp`) exposing 13 DCP maker verbs, Agent Plugins 1.0.0 package (`distributoor-plugin/` with plugin.json + mcp.json + 3 skills), DCP v0.1 spec extracted to `docs/dcp/`, and `GET /.well-known/dcp.json` discovery manifest.
- **Hosted MCP + publisher tools (P5, part 2)**: `POST /v1/mcp` streamable‑HTTP MCP transport (maker auth via `X-Maker-Key`/Bearer, publisher via `X-API-Key`, per‑request loopback client, rate‑limited), plus 4 publisher‑side MCP tools (`list_marketplace_offers`, `join_offer`, `get_balance`, `get_payout_history`) behind `DISTRIBUTOOR_SITE_KEY` on the stdio server; discovery manifest and `docs/dcp/` updated.
- **WP plugin Distributoor screens**: new Marketplace submenu (offers ranked by fit with join/copy‑link flows, `admin/build/marketplace.js`) and Earnings submenu (pending/cleared/paid balance cards, commission ledger, payout history, `admin/build/earnings.js`), both bilingual EN/IT and matching existing plugin CSS patterns.
- **Scale (P5, part 1)** (`009_scale_fraud_trust.sql`): site‑profile matching — publisher marketplace ranked by topical fit (profile from scanned article titles + recommendation categories, reusing the matcher tokenizer) and maker‑side `GET /v1/maker/offers/:id/matches` (publishers worth inviting); fraud v1 — conversions flagged on implausibly fast click‑to‑purchase and per‑campaign velocity cap with campaign auto‑pause, flagged commissions held from clearing until admin releases them (`GET /offer-review/flagged-conversions`, `POST /offer-review/conversions/:id/clear-flag`); trusted makers (`POST /offer-review/makers/:id/trust`) list submissions without the review queue.
- Not yet implemented: Stripe Connect payouts (needs a real Connect account/KYC) and a first non‑WordPress DCP surface.

## What’s Missing / Stubbed
- **Matching quality**: lightweight semantic overlap exists; no true intent or LLM semantic analysis.
- **Program discovery pipeline**: enrichment automation exists, but full quality scoring/verification workflow is still lightweight.
- **Tests**: browser-level E2E now covers onboarding/connected/disconnected states; still missing full insert + tracking + analytics end-to-end assertions.

## Known Issues / Constraints
- `go mod tidy` requires outbound access to fetch modules from `proxy.golang.org` when cache is cold.
- `rg` is not available in shell; use `grep` or `find`.

## How to Run (local)
### Quick Start
1. **Start DB + API**
   - `cd affilio-api`
   - `docker-compose up -d`
   - Apply migrations in order: `001_initial.sql`, `002_seed_programs.sql`, `003_add_user_program_id.sql`, `004_discovery_reviews.sql`, `005_api_key_rotation.sql`, `006_recommendation_status_guardrails.sql`, `007_audit_events.sql`, `008_hash_api_keys.sql`
   - Run API: `go run ./cmd/server`
2. **Install Plugin**
   - Copy `affilio-wp/` into `wp-content/plugins/affilio/`
   - Activate “Affilio” plugin in WP admin
   - Open **Affilio → Settings** and click **Connect and find opportunities**. This requires the updated backend; manual keys remain available under the settings accordion.
3. **Test Flow**
   - Review the automatic first scan; **Scan All Posts** in the dashboard refreshes it later.
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
- 2026-08-30: Added WP plugin Marketplace + Earnings screens (submenus, build JS, CSS) wired to the marketplace and payout endpoints; added hosted streamable-HTTP MCP transport at POST /v1/mcp and publisher-side MCP tools; updated DCP discovery manifest + docs.
- 2026-08-30: Implemented Distributoor P5 part 1 (migration 009): site-profile offer ranking in the marketplace + maker publisher-match endpoint, conversion fraud flags (fast click-to-purchase, campaign velocity auto-pause) with admin flag review holding commissions from clearing, and trusted-maker auto-listing.
- 2026-08-30: Implemented Distributoor P4 — agentic layer: `distributoor` maker CLI, `distributoor-mcp` stdio MCP server exposing the 13 DCP maker verbs, Agent Plugins 1.0.0 package under `distributoor-plugin/` (plugin.json + mcp.json + onboard/instrument/launch skills), DCP v0.1 spec extracted to `docs/dcp/`, and `/.well-known/dcp.json` discovery endpoint. `internal/dcp` client/tool-registry/JSON-RPC tests added and wired into ci-test.

- 2026-03-28: Implemented API key hashing (SHA-256 + prefix indexed lookup), previous-key expiry window with configurable grace period, audit logging for auth failures and admin actions, admin audit events/alerts endpoints, and fixed AdminKeyAuth to use constant-time comparison.

## Next Suggested Tasks
1. Improve semantic matching quality (LLM/intent).
2. Extend browser E2E to full insert + tracking + analytics assertions on staging.
3. Add external alerting (Slack/email webhook) for critical audit events.
