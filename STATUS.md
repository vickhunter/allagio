## October 10 — agent entry backend (D-032 part A, D-031)

Device-code sign-in (`POST /v1/device/code`, `POST /v1/device/token`, `POST /v1/device/approve`) issues a project-scoped site or offer key after a signed-in human approves a single-use user code, and records `human_login` with the owner id and time. Accounts created through `POST /v1/agent-accounts` stay `agent_only` until that approval; `GET /v1/agent-signals` reports the two groups separately. `GET`/`PUT /v1/setup/settings` stores the maker and publisher answers, the mode shown, the chosen mode, the owner id, and the timestamp. The default mode is yolo for cli, cursor, connector, and grok, and manual for chatgpt_dir and claude_dir. Per-site and per-offer modes are manual, auto, and yolo. Auto policies cover allow and deny lists, a relevance floor that cannot drop below 25, positions, a 30-day cap, and copy edits. Hard checks always run. Approvals store `approved_by`, the policy version, and the matched rule. The kill switch returns a site to manual and pauses unpublished auto approvals. Agent tools can only tighten a mode. Offer copy, page content, and feedback cannot change a policy. Listing cards reject source-like and secret-like fields and store the payments check and offer-link health. Digest and dashboard opens, plus placement and offer undos, are events. `GET /v1/digest` lists what went live since the last digest open, with a one-tap undo token per item. `owners.source` now allows `cli` and `mcp`. `GET /from/<source>` redirects to the landing page and counts the hit; unknown values count as direct. `/llms.txt` and `/docs/:name` serve the agent-entry markdown. Migration `018_d032_agent_entry.sql`. Account-wide keys are created, listed, and revoked only from a signed-in dashboard session; an API key gets 403. A project key stays limited to one site or one offer. `GET /v1/account/digest` returns account totals and per-project went-live, clicks, test-mode earnings, and clicks by source, with an undo token on each item. Earnings are null when no test-mode commission is recorded. `GET /v1/account/projects` and `GET /v1/policy/hard-checks` cover the remaining dashboard numbers. Screen actions live in `internal/entry/parity.go`; `TestDocumentedScreenActionsHaveEndpoints` fails when one of them has no route. Migration `019_d032_account_keys.sql`. Four actions are session-only: loosening the approval mode, raising a cap or lowering minimum relevance, turning the kill switch off, and creating an account-wide key. A project key or an account key receives 403 for each of those. The same keys can tighten a mode, lower a cap, raise minimum relevance, pause unpublished approvals, and turn the kill switch on. The mode saved on the setup page after device-code sign-in is the human choosing, so a preselected yolo still applies for a CLI install. Migration `020_d032_kill_switch.sql`. Approvals also return `policy_rule_id` and `policy_rule_text`. Undo is `POST /v1/placements/:id/revoke` with `reason=undo`, allowed for the site owner or the offer's maker, and stored as `revoked_by_side` plus a `placement_undo` event. `POST /v1/account/pause` pauses every site and offer in scope `all`, `site`, or `offer` and accepts any key. `POST /v1/account/resume` is session-only. Digest earnings are null when no test-mode rows exist and 0 when those rows add up to zero. Setup and digest include `session.signed_in`. Setup also includes `human_login` and `mode_preselected`, derived from `install_path`. Migration `021_d032_interface.sql`. Undo takes effect in the same API and MCP request. `POST /v1/placements/:id/restore` is allowed for 6 seconds after that placement's server undo timestamp, and only for the same user and key identity. The server re-runs every hard check; a failure returns 422 and leaves the placement revoked. Success restores the pre-undo status, keeps `approved_by`, sets `restored_by`, and records a `placement_restore` event. Migration `022_d032_restore.sql`. A restore after the 6-second window returns 409 `reason: window_expired`. A pause or kill switch turned on for the site, offer, or account after that undo, including one that is still on, refuses restore with 409 `reason: paused`. The 409 reasons are `window_expired`, `paused`, and `not_failed`, defined in one enum and published in `/llms.txt` and the modes doc. The 30-day cap and minimum relevance are read at restore time. A 422 from restore, or from any hard-check failure, returns `check_id` and `check_text`. The ids live in one catalog: `disclosure`, `rel_sponsored`, `claims_from_offer`, `copy_length`, `one_per_post`, `relevance_min`, `offer_live`, `revoke_path`, and `cap`. `/llms.txt` and the modes doc render that catalog. A revoked `/v1/go/...` link returns 410 `reason: revoked` immediately, with no offer redirect and no click, and redirects again after a successful restore. There is no push channel. Clients poll `GET /v1/placements/:id`, a single-table read with no joins, at least once every 4 seconds per placement per session. Every placement's `plugin_state` is `pending`, `placed`, `removed`, `offline`, or `failed`, and it is never null. A brand-new placement stays `pending` until the site key's first `plugin-ack` with `action: restored`, which sets `placed`. After a restore it is `pending` again until that same acknowledgement. After a revoke it is `pending`, then `removed` on `action: removed`, or `offline` after `PLUGIN_OFFLINE_AFTER` (default 10 minutes) with no acknowledgement. `offline` is only that missing acknowledgement. `action: failed` with a `reason` stores `plugin_failed_reason` and `failed_action` (`removed` or `restored`) and sets `plugin_state` to `failed` when the plugin's post re-read fails after one retry. A later verified acknowledgement for that same action clears the failure. `POST /v1/placements/:id/retry` repeats only `failed_action`. The publisher session or site key may call it; the other side receives 403. Any other state returns 409 `not_failed`. A failed removal re-queues only the removal (`pending`, then `removed`), does not re-run hard checks, does not become `placed`, and leaves the go link stopped. A failed restore re-runs every hard check, returns 422 `{check_id, check_text}` when one fails, and on success re-queues the restore (`pending`, then `placed`). Placement reads and the digest include `plugin_failed_reason`. Migrations `023_d032_plugin_restore.sql`, `024_d032_plugin_placed.sql`, `025_d032_plugin_failed.sql`, and `026_d032_failed_action.sql`.

## October 9 — Walk A fixes (D-026)

The hosted Find empty state names the real reason. `below_relevance` includes the score and threshold, for example “relevance 18/100, needs 25”. A short topic (under 400 characters) asks for a longer excerpt or offers to fetch the article URL. `POST /owner/api/article-text` reads that URL only when it is on the registered site, and only over HTTP when the site is `127.0.0.1`, `::1`, `localhost`, or a `*.localhost` name. The payment sentence is shown only for `no_eligible_offers`. The published article URL is optional on Find when the about-text is filled; preparing a placement still requires it. `surface_url` on prepare follows the same HTTP rule and still requires the hostname to match the registered domain, ignoring the port. Plugin 0.1.18 wraps the block insert, the receipt-failure revert, and remove in a `wp_revisions_to_keep` filter that returns 0 at priority `PHP_INT_MAX` and is removed afterward, so those writes do not create a post revision and a later filter cannot turn revisions back on. The `add_filter` / `remove_filter` test stubs are not shipped in the plugin; they live in the test bootstrap. If a refused receipt's rollback leaves the `affilio-placement-<id>` block or its go link, the plugin retries the removal once. If the box is still there, it writes an error log entry with the placement id, post id, and reason, and shows an admin notice that the box is live without a receipt. Migration `017_owner_attribution_source.sql` adds `owners.source` (`NOT NULL DEFAULT 'direct'`) limited to chatgpt, claude, cursor, grok, x, community, and direct. Unknown values are stored as direct. `?source=` is carried through owner signup and the WordPress site-connect form. A later sign-in without a source leaves a stored non-direct source in place. Plugin 0.1.16 and 0.1.17 live on other pull requests; this branch was 0.1.14 and is now 0.1.18.

## October 8 — publisher leg (D-024)

Listed offers whose product has no payment provider stay eligible for matching. A Stripe product still needs an enabled connection in the offer currency, and a verified one when verification is required. Commission type stays limited to percentage and flat. `no_match` from `/revenue/opportunities` (and the `find_opportunities` tool, which calls that route) includes `reason`: `no_eligible_offers`, or `below_relevance` with `best_score` and `threshold` 25. The Distribution empty state shows that reason in Italian. The plugin and the receipt share one disclosure rule. After tags are stripped and the text is lowercased, a match is `commissione`, or `link` within 60 characters of `affiliat` in either order. The approved disclosure text also counts. Phrases such as "Leggi la nostra disclosure policy", "contenuto sponsorizzato", and "contiene link" do not. The setting "I miei articoli contengono già una disclosure" (`affilio_existing_disclosure`, stored only as `1` or empty, default off) does not suppress a disclosure by itself. An approved insert is a `wp:paragraph` block with the tracked `/v1/go/` link, `rel="sponsored nofollow"`, and the link text "Vedi il prodotto". Opportunities, prepare, and publish in the plugin return 403 "Collega prima il tuo account publisher." until `GET /sites/owner-connection` reports `connected`. The API prepare and commit routes now refuse with 403 when the site has no owner; they previously accepted any site API key. The publisher connection check is cached on the plugin object for the request. Plugin version is 0.1.14. A placement receipt accepts the approved disclosure or the same loose match. The approved copy and the exact tracked link are still required. Text with neither disclosure is still a 409. The plugin adds its disclosure in that case, and it includes the post's own disclosure in the receipt when that text sits outside the inserted block. The excerpt is cut with `mb_substr` on character boundaries. If the receipt is any non-2xx, the plugin restores the post content from before the insert and shows the API reason in Italian. That automatic revert does not call `POST /v1/placements/:id/removed`. The release ZIP stores every entry with deflate. Site key rotation stores the previous-key grace with `make_interval(hours => $4)`, so a 24 hour grace is one day rather than the interval text `24 hours hours`. Saving a product no longer clears `primary_category` or `keywords` when those fields arrive blank. The maker dashboard reads the category and topics from the visible inputs, falls back to the values rendered with the form, and leaves both out of the request when they are blank. A non-empty value still replaces the stored topics. `ApproveOwnerPlacement` still refuses a self-referral. An inserted placement is a core paragraph whose `className` is `affilio-placement-<id>` (and `affilio-disclosure-<id>` when the plugin adds the disclosure). The old `<!-- distributoor-placement:… -->` comment, including after an editor re-save pulls it inside the paragraph, is still found and removed. `Rimuovi inserimento` asks for confirmation, then edits the post and reports `POST /v1/placements/:id/removed` on a best-effort basis. `wp affilio revert` does the same. A failed report is queued and retried. Migration `016_placement_removed_at.sql` adds a nullable `removed_at`. A removed placement's `/v1/go/<code>` still redirects, records no click, and omits `via`. Owner and maker dashboards show `Rimosso` with the date.

## October 8 — demo landing page builder (D-025)

`/demo` now describes a demo landing page builder, with the tagline “Demo landing page builder: launch a landing page without code.” A DEMO label sits above the fold. Pricing and docs name templates, lead capture, and email signup, and pricing states there is no real price. Paths, language, viewport, titles, and `ref` / `via` on Get started are unchanged. Readiness gates are unchanged.

## October 8 — owner and demo follow-up (D-016c)

A live Stripe key (`sk_live_`, `rk_live_`, or `pk_live_`) is refused in the browser before the connection form is posted, and again on the server. The refusal is inline HTML next to “Verify and connect Stripe”. The submitted secret is not written to the response, the access log, or the database. At 390 the Stripe page does not scroll sideways; the webhook URL wraps. Register stays available from “Add product” after the first product exists. “Product registered. It is in your list.” sits next to the form, takes focus, and the list shows the new product immediately. Offer review is in the shared owner nav, including the Stripe page, for the operator. The rail step reads “Checkout (optional)” with an Optional flag and “no provider yet”. Empty offer fields explain the missing value in plain words. `/demo` answers HEAD, sets `lang` and a viewport, and gives each page its own title. Subpages link back. `/demo/start` links to sign-in and register. `ref` and `via` stay on Get started and pricing links when they match `[A-Za-z0-9_-]` and are at most 64 characters. A product saved with no provider stores an empty `payment_provider`. Empty and NULL are treated as none. Connecting Stripe is what sets the column to `stripe`. No migration was added: `products.payment_provider` is already `VARCHAR(50)` with no `NOT NULL` and no default in `007_distributoor_core.sql`.

## October 8 — owner walk fixes (D-016b)

Readiness `checkout_present` passes on a declared checkout URL or a buy, subscribe, checkout, start, or sign-up link. A known provider marker is reported when one is found. "No known payment provider" is a warning and does not gate. Reachable site, pricing, docs, privacy, and terms stay gates. Offer submit checks product readiness only. A maker with no payment connection can submit, and a trusted maker lists. Unready submit says the product has not passed readiness, with no API path in the response.

`/demo` serves a static product site (pricing, docs, privacy, terms, get-started). It does not show a checkout link. Readiness may probe only the configured `PUBLIC_BASE_URL` origin in addition to public hosts. `?via=` stays on `/demo`.

Registering a product shows "Product registered" and the product appears in the list without a reload. Offer submit errors next to the button are plain language. The offer station stays undone while readiness is pending. The Stripe test station is optional: with no provider connected it stays grey and reads "no provider yet", and Ready, Offer, and Submit proceed without it. Connecting Stripe test mode still marks that station done. The Stripe connection page says test mode only and rejects live keys before calling Stripe. `/owner/review` stays in the owner nav for `OPERATOR_OWNER_ID`.

## October 8 — plugin 0.1.11 (D-012c)

Cron onboarding and Scan All Posts send published posts in batches of 10. If a batch is rejected, each post in that batch is scanned on its own. A post that is still rejected is skipped and recorded as its id plus a short reason, with no title or body, and the scan continues. Onboarding finishes instead of stopping on `scan_error`. The setup screen shows `Analisi completata: N articoli analizzati, M saltati` and lists the skipped posts for administrators. Scan All uses the same batching and shows that result on the dashboard. Scan requests to `/v1/scan/full` and `/v1/scan/single` send `title_b64` and `content_b64` (`content_encoding: gzip+base64`, or `base64` when gzip is unavailable) so a WAF cannot match install snippets in the raw body. The API still accepts the previous raw `title` and `content` fields, rejects a field sent both ways, rejects unknown or malformed encodings with 400, and applies the same limits after decoding (title 512 bytes, content 1 MiB). `POST /sites/register` is still only `{site_url, plugin_version}`. Plugin admin REST routes return 403 for non-administrators. `/distribution/placements` returns an Italian sentence when the API cannot be reached, without the transport error, host, or port. Connection-status transients are indexed and older keys are deleted when the API URL or key changes. A health check that times out at 8 seconds is retried once at 20 seconds; if the service is still waking, the message is `Il servizio si sta avviando, riprova tra qualche secondo`.

## October 8 — plugin 0.1.10 (D-012b)

The WordPress package is folder `affilio-wp/` at version 0.1.10. Global functions and classes are guarded so a second copy cannot redeclare them. The public `/track` route and front-end tracker script are gone. The API base URL comes only from the `AFFILIO_API_URL` constant, the `affilio_api_url_override` option, or the `affilio_api_base_url` filter. With none of those set, admin shows “Not connected — set the API URL in Settings” and the plugin makes no outbound HTTP calls. Connect with an empty API URL tells the owner to set the API URL first and does not call the service. The release ZIP must be `affilio-wp/`, include every enqueued or required file (including `admin/build/analytics.js` and `readme.txt`), guard `affilio_init`, and contain no `railway.app` host. `GET /downloads/affilio.zip` serves that `affilio-wp/affilio.php` package and still passes `validate_plugin`. Settings never prints a stored site key or admin key. The fields render empty, with a “Key saved (abcd…)” placeholder that shows only the first four characters. An empty save keeps the stored key. Admin JavaScript, the status box, and notices use that same mask. No mode, including `WP_DEBUG`, shows the rest of the key. Dashboard, marketplace, earnings, analytics, and discovery call `POST /wp-json/affilio/v1/proxy`, which keeps the real key on the server. Scan All Posts requires a nonce and `manage_options`. Connection status is cached per API URL and key, so changing either applies immediately. `POST /sites/register` sends only `site_url` and `plugin_version`. The API accepts that body, derives the domain from the http(s) host, and still accepts the older `{domain}` body. `plugin_version` is stored only when `sites.plugin_version` already exists. Scans may send published titles, permalinks, and content. Emails, IP addresses, usernames, display names, comments, and unpublished posts stay on the site.

## October 8 — pre-deploy hardening follow-up

Tracked links that are paused, unlisted, or removed return `410` and record no click, including for preview bots and `HEAD`. A deleted tracking code stays `404`. Product `website_url` and offer `landing_url` must be `https` at create and update; `/v1/go` refuses a stored non-https destination with `400` and records no click. An active listed `https` browser `GET` still redirects with `via=`.

`scripts/mvp_release.py check --host generic --env-file` validates a non-Railway host without the Railway CLI. `verify --base-url` accepts any https origin. Both modes require `API_KEY_SECRET` (32+ characters, not `dev-secret`), `GIN_MODE=release`, and a `TRUSTED_PROXIES` list that is not open to every address. `Dockerfile.mvp` sets `GIN_MODE=release`. Hosted MCP rate limits use the trusted client IP. Render plus Neon (Aiven Free fallback) is documented in [HOSTING.md](docs/HOSTING.md), including the cold-start `GET /health` retry. The onboarding workflow also runs for API, script, Dockerfile, and docs changes. The hosted page `Content-Security-Policy` allows `font-src 'self' data:` so embedded OFL fonts load. The other directives are unchanged.

## October 8 — waking host and narrow nav (D-020)

- At 390 and 360 the workspace navigation wraps into two columns, so Setup & agents stays on screen without a sideways scroll. The desktop list is unchanged.
- While `/SKILL.md` is not served, the agent start line's copy control is a secondary button. The copied hint is "Copied. Paste it to your agent."
- If the hosted workspace is asleep (network error, 502, 503, 504, or a timeout), the page says it is waking the server and retries `GET /health` every 4 seconds for up to 60 seconds. A recovered check reloads the page data and keeps what the owner already typed. If it is still down, the page says so and offers Try again. No server code changed.

## October 8 — offer rail polish (D-019)

- Once a referral offer exists, Products & offers hides "Create a referral offer" behind a secondary "New offer" button. The form stays open when there are no offers, or when creating an offer is the rail's next step. Expanding it focuses the first field, and a rejected create still shows the inline error.
- The landing page and the owner workspace share one palette and one type scale from the custom properties on `:root` in `workspace.css`. The blue action color is only the single next step. The name on the page is still "distributoor".
- Setup & agents gives an agent one line to copy (`Read <origin>/SKILL.md and connect this site to Offer Rails`), says the skill file is coming soon while that URL 404s, states the current rail step in a sentence, and lists the steps that still need a person (Stripe connect, operator approval). No Go route was added.

## October 8 — offer rail in the owner workspace

- The owner products page shows an offer rail: product, readiness, Stripe test mode, offer, submitted, listed. Exactly one control is the next action. Done stations stay marked done. Readiness and submit errors sit on the check or button, take focus, and scroll into view at 390 and 1440.
- Stripe confirmation and a returned draft are read in the browser from the existing confirmation page and status changes. The owner JSON still has no connection or review-note fields. See `docs/design/D-011-design-rules.md`.
- WordPress Distribution, Marketplace, and Earnings keep one next action and show load or join errors inline. Marketplace no longer uses an alert dialog.
- UI files only. The Go host that serves `affilio-wp/admin/build/workspace.html` is unchanged.
- `affilio-wp/e2e/offer-rail.spec.js` runs in the GitHub Actions job `offer-rail` at 1440×900 and 390×844.

## October 1 — release gates

- Added a deployment preflight pinned to the isolated `distributoor-mvp` service: configuration, clean/pushed revision and required CI checks must pass before uploading. Variable values and CLI failure output are withheld.
- Added Docker release CI covering runtime assets, license notices, the installer command/skills and WordPress ZIP. A public smoke command checks downloads, embedded API origin, OAuth discovery and protected owner/MCP routes after Railway reports success.
- Extended the WordPress browser fixture to install the HTTP-served release ZIP and use its embedded API origin without an override, then verify the public article's referral redirect and persisted placement-attributed click. Real Google login and operator activation remain pending credentials.
- Verified the Docker bundle and packaged builder command, release-gate tests and Go vet. The packaged WordPress flow passed twice after fixing its sign-in autofocus race. No production deployment was requested while OAuth credentials are missing.

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
