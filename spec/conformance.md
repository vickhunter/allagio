# Offer Rails Protocol — Conformance Levels

**Protocol Revision:** `2026-10-21` (`v0.3.0`)

---

## 1. Conformance Overview

Offer Rails Protocol v0.3 is structured so that any agent or server can implement the **"Afternoon Core" (Level 1 & Level 2)** in a few hours using standard JSON and universal tracked links, while full relays and multi-platform networks implement **Level 3**.

| Level | Name | What It Requires | Platform Integration Needed? |
|---|---|---|---|
| **Level 1** | **Universal Tracked Link, Discovery & Privacy** | Universal tracked link (`/v1/go/:token`) working on any surface (email, X, blog) with no adapter, `/.well-known/orp.json` (or `dcp.json`), JSON Schema 2020-12 validation, pseudonymous party IDs, $k \ge 50$ aggregate Audience Cards, secret/PII redaction | **None** (Tracked link is the universal adapter; adapters are optional extras) |
| **Level 2** | **Constraint-Bounded Deals & 3-Mode Receipt** | Level 1 + `offer`, `accept`, constraint-bounded `consent` (`auto` inside constraints, escalate to `ask` outside constraints), 3-mode `receipt` (`proof_url`, `public_check`, `counterparty_attestation`), and instant `revoke` | **None** |
| **Level 3** | **Bilateral Negotiation, Swaps, Optional Adapters & Reputation** | Level 2 + multi-turn `ask`/`counter`, all 10 `Space` kinds & 8 settlement modes (`space_swap`, `traffic_swap`, `repost_bounty`), `dispute` handling, Proof-of-Outcome `reputation` decay, and optional platform `SpaceAdapter` helpers | **Optional** (`SpaceAdapter` helpers for X, Newsletter, WP, etc.) |

---

## 2. Normative Conformance Test Matrix (`/conformance/`)

### Level 1 — Universal Link (No Adapter), Core Discovery & Privacy (`L1-*`)
- **`C1-UNIVERSAL-LINK-NO-ADAPTER` (Test #1 — Spine Invariant):** Any tracked link (`/v1/go/:token`) works on any surface (email, X, blog) with **no adapter** (`adapter_id` omitted). Clicking the link from an email client, an X post, or a blog post records attribution and redirects to the offer destination, and `revoke` immediately stops redirect/counting. Platform adapters are strictly optional extras.
- **`L1-MANIFEST`:** `GET /.well-known/orp.json` or `GET /.well-known/dcp.json` returns `200 OK` with `offer-rails` (not legacy `distributoor-mcp`) and valid MCP/HTTP transport metadata.
- **`L1-LLMS-TXT`:** `GET /llms.txt` returns `200 OK` (`text/plain`) and accurately documents bearer-token `/mcp` authentication without claiming `/mcp` requires Google sign-in.
- **`L1-SCHEMA-2020-12`:** All 10 ORP message types (`discover`, `offer`, `counter`, `accept`, `consent`, `execute`, `receipt`, `revoke`, `dispute`, `reputation`) validate against Draft 2020-12 schemas in `/schema/`.
- **`L1-PRIVACY-PSEUDONYMOUS`:** Rejects party identifiers containing email addresses (`@`) or PII in `audience_card`, and enforces `k_anonymity_threshold >= 50`.
- **`L1-LISTING-REDACTION`:** Public listing cards strip internal `source` paths, credentials, and raw owner emails.

### Level 2 — Constraint Gate, 3-Mode Receipts & MCP (`L2-*`)
- **`L2-CONSTRAINT-AUTO-VS-ESCALATE`:** Deals inside `MandateConstraints` auto-approve when `step_mode == "auto"`; deals exceeding `require_human_above_minor`, touching restricted categories, or hitting `step_mode == "ask"` escalate to `awaiting_consent` / `awaiting_approval`.
- **`L2-RECEIPT-THREE-MODES`:** Accepts and validates receipts using `proof_url`, `public_check`, and `counterparty_attestation` (including zero-click actions), and rejects receipts where `disclosure_verified != true`.
- **`L2-MCP-TRANSPORT`:** MCP endpoint (`POST /v1/mcp` or `/mcp`) supports JSON-RPC `initialize`, `tools/list`, and authenticated tool execution using an `/v1/agent-accounts` key.

### Level 3 — Bilateral Swaps, Negotiation, Reputation & Adapters (`L3-*`)
- **`L3-COUNTER-NEGOTIATION`:** Multi-turn `offer` → `counter` → `accept` preserves monotonic `seq` and canonical `terms_hash`.
- **`L3-ZERO-CLICK-AND-SWAPS`:** Validates zero-click `social_repost`, `social_comment`, `space_swap`, and `traffic_swap` deals across non-blog platforms (`x`, `newsletter`, `youtube`, `discord`).
- **`L3-REPUTATION-ANTI-GAMING`:** Enforces pairwise exponential decay ($2^{-(n-1)}$), counterparty diversity weighting, and $-15$ dispute slashing.
- **`L3-OPTIONAL-ADAPTERS`:** Verifies that the X (`adapters/x`) and Newsletter (`adapters/newsletter`) optional adapters implement `SpaceAdapter` and degrade gracefully to universal tracked links + `public_check` receipts when platform API credentials are absent.
