# Offer Rails Protocol (v0.3.0) — Conformance Report

**Protocol Revision:** `2026-10-21` (`v0.3.0`)  
**First Conformance Invariant (`C1-UNIVERSAL-LINK-NO-ADAPTER`):** Any tracked link (`/v1/go/:token`) works on any surface with no adapter (email, X, blog). Platform adapters are optional extras.

---

## Executive Summary

- **Reference SDK & Schemas (`sdk/typescript`, `schema/`, `adapters/`):** **10 / 10 PASS** (Level 1, Level 2, Level 3).
- **Current Go Server (`affilio-api`):** **5 PASS, 3 PARTIAL, 1 FAIL (`GAP-01`), 1 MISSING** (Full details, protocol friction log, and root causes in [`../GAPS.md`](../GAPS.md)).

---

## Live Conformance Matrix

| # | Check ID | Level | Requirement | Reference SDK (v0.3) | Live Go Server (`affilio-api`) | Live Observation |
|---|---|---|---|---|---|---|
| 1 | `C1-UNIVERSAL-LINK-NO-ADAPTER` | Level 1 (Spine #1) | Any tracked link works on any surface with no adapter (email, X, blog). Adapters are optional extras. | **PASS** | **FAIL (GAP-01)** | In v0.3 SDK/spec, any tracked link (/v1/go/:token) attributes clicks on email, X, and blog as soon as a receipt is recorded with no adapter. On current Go server, after commit + receipt (without X-Plugin-Key plugin-ack), GET /v1/go/:code returns 302 to plain landing URL without ?via= on email/X/blog because placement_counts_as_live still requires plugin_wait='placed' (logged as GAP-01 in GAPS.md). |
| 2 | `L1-MANIFEST` | Level 1 | Discovery manifest (/.well-known/dcp.json & /.well-known/orp.json) uses offer-rails v0.3.0 (not distributoor-mcp) | **PASS** | **PASS** | GET /.well-known/dcp.json and /.well-known/orp.json returned 200 with name="offer-rails", orp_version="0.3.0", stdio.command="offer-rails-mcp". |
| 3 | `L1-LLMS-TXT` | Level 1 | GET /llms.txt documents bearer token auth on /mcp and no longer claims /mcp needs Google sign-in | **PASS** | **PASS** | GET /llms.txt returned 200 (text/plain; charset=utf-8); states keys from /v1/agent-accounts work as Bearer tokens on /mcp and on the REST API. |
| 4 | `L1-SCHEMA-2020-12` | Level 1 | All 10 ORP v0.3 message types validate against Draft 2020-12 JSON Schemas (/schema/) | **PASS** | **PARTIAL** | All 12 schema files in /schema/ and SDK validator pass; Go server currently exposes REST/MCP payloads for offers/placements rather than the unified 10-message ORP v0.3 JSON envelope. |
| 5 | `L1-PRIVACY-PSEUDONYMOUS` | Level 1 (Spine #3) | No personal or sensitive data exchanged between agents; pseudonymous party IDs and aggregate stats only (k >= 50) | **PASS** | **PASS** | Public offer listings strip maker emails (verified); ORP v0.3 schema enforces pseudonymous party URIs and k_anonymity_threshold >= 50. |
| 6 | `L2-CONSTRAINT-AUTO-VS-ESCALATE` | Level 2 (Spine #2) | Agents negotiate autonomously inside human-set constraints and escalate to the human only when outside them | **PASS** | **PARTIAL** | SDK evaluates budget, category, brand_rules, and approval_thresholds autonomously. Go server holds new-site placements with 201 status="awaiting_approval", hold_reason="domain_unverified", but lacks multi-dimensional budget/category/karma constraint evaluation on bilateral deals (GAP-03). |
| 7 | `L2-RECEIPT-THREE-MODES` | Level 2 (Spine #1) | Verifiable receipts for clicked and zero-click actions via proof_url, public_check, or counterparty_attestation | **PASS** | **PARTIAL** | Go server supports POST /v1/placements/:id/receipt with surface_url + published_text + evidence, but requires tracked link in published_text (blocking zero-click reposts/swaps) and still waits for plugin-ack before counting clicks (GAP-01, GAP-02). |
| 8 | `L2-REVOKE-KILL-SWITCH` | Level 2 | Either party can revoke a deal at any time and immediately disable tracked link attribution | **PASS** | **PASS** | POST /v1/placements/:id/revoke returned HTTP 200 and immediately disabled attribution on the placement. |
| 9 | `L2-MCP-TRANSPORT` | Level 2 | MCP JSON-RPC transport (initialize, tools/list, tools/call) works with /v1/agent-accounts key without Google sign-in | **PASS** | **PASS** | POST /v1/mcp initialize (200), tools/list (29 tools), and find_opportunities with X-API-Key (200, isError=false) succeeded without browser OAuth. |
| 10 | `L3-NEGOTIATION-SWAPS-REPUTATION` | Level 3 | Bilateral counter-offers, zero-click swaps (space_swap, traffic_swap, repost_bounty), Proof-of-Outcome Karma, and optional SpaceAdapters | **PASS** | **MISSING** | Reference SDK + schemas + optional X/Newsletter SpaceAdapters pass all tests; Go server currently implements fixed-price CPA/flat offers without multi-turn counter, bilateral swaps, or Karma decay endpoints (GAP-02, GAP-04, GAP-06). |

---

## Key Finding from Test #1 (`C1-UNIVERSAL-LINK-NO-ADAPTER`)

1. **Why Test #1 is the Spine of v0.3:** A deal's tracked link (`/v1/go/:token`) must work on any surface (email, X, Facebook, Instagram, YouTube, blog, podcast notes) with **no platform integration**.
2. **What Live Conformance Testing Revealed in `affilio-api` (`GAP-01`):**
   - In `affilio-api/internal/handlers/redirects.go` and `affilio-api/internal/database/attribution.go`, once a placement is committed (`POST /v1/placements/:id/commit`), `pp.status` becomes `'published'` and `pp.plugin_wait` becomes `'waiting'`.
   - Both `GET /v1/go/<placement_id>` (`LookupPlacementRedirect`) and `GET /v1/go/<campaign_code>` (`LivePlacementForCampaign`) check `EXISTS (SELECT 1 FROM placement_counts_as_live live WHERE live.id = pp.id)`, which requires `pp.plugin_wait = 'placed'` — a state only set by `POST /v1/placements/:id/plugin-ack` with `X-Plugin-Key`!
   - Even after an agent submits a valid `POST /v1/placements/:id/receipt` with the live `surface_url` and `published_text`, `GET /v1/go/:code` redirects **without** counting the click or appending `?via=<click_id>` on email, X, or non-WordPress blogs unless `plugin-ack` is called.
3. **v0.3 Resolution:** In ORP v0.3, a verified `receipt` (`proof_url`, `public_check`, or `counterparty_attestation`) immediately marks the placement live so `/v1/go/:token` counts clicks and appends `?via=<click_id>` on every surface with **no adapter**.

---

## Reproducing This Report

```bash
# Run unit & schema conformance tests standalone
node --experimental-strip-types --test ./conformance/conformance.test.mjs

# Run full live-server conformance suite against affilio-api + PostgreSQL
node --experimental-strip-types ./conformance/run.mjs
```
