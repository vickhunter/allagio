# Offer Rails Protocol — Architecture & Transports

**Protocol Revision:** `2026-10-21` (`v0.3.0`)

---

## 1. Architectural Roles & The Universal Adapter

Offer Rails Protocol separates **universal deal execution and tracking** (which works on every platform out of the box with zero platform integration) from **optional platform adapters** (which automate discovery and posting when available).

### 1.1 Party / Principal (`principal`)
A **Principal** (or **Party**) is the human or organization that owns a digital space, an offer/budget, or both.
- **Pseudonymous by Default:** Identified on the wire by a pseudonymous party URI (`orp:party:pub_01jc8x9...` or `orp:principal:pub_01jc8x9...`). Personal email addresses, legal names, and payment credentials MUST NOT be broadcast to counterparty agents.
- **Constraint Setter:** Defines standing human constraints (`budget`, `categories`, `brand_rules`, `approval_thresholds`) and per-step modes (`auto` | `ask` | `off`).
- **Escalation Authority:** Receives escalation requests whenever an agent negotiation falls outside standing constraints, and issues verifiable `consent` records or instant `revoke` commands.

### 1.2 Agent (`agent`)
An **Agent** is an autonomous or semi-autonomous software client acting on behalf of a Principal.
- **Autonomous Inside Constraints:** Discovers spaces/offers, negotiates (`offer` ↔ `counter`), and accepts deals with other agents without human interruption as long as every term satisfies the Principal's `MandateConstraints`.
- **Symmetric or Asymmetric:** Acts as an Offeror (`offeror`), a Space Owner (`space_owner`), or a symmetric bilateral peer (`peer` in `space_swap`, `traffic_swap`, `data_exchange`).

### 1.3 Universal Adapter: The Tracked Link & Verifiable Receipt (Core — Required)
The **Universal Adapter** is built directly into the core protocol and requires **zero platform API keys, plugins, or OAuth integrations**:
1. **For Click-Bearing Deals (Affiliate, Sponsorships, Traffic Swaps, Cross-Promo):**
   - Upon `consent` / `execute`, the protocol issues a deal-scoped **tracked link** (`https://<relay>/v1/go/<token>`) and a short **tracking token** (`orp_tok_<id>`).
   - Like `bit.ly`, this link works immediately wherever a human or agent pastes it: an email newsletter, an X post or bio, a YouTube description, an Instagram story link, a Facebook post, a blog article, podcast show notes, or a Discord message.
2. **For Zero-Click Deals (Reposts, Comments, Space-for-Space, Shoutouts):**
   - Where click redirects are not the primary action, fulfillment is proven via a **Verifiable Receipt** (`receipt` message) using any of three zero-plugin verification methods:
     - **`proof_url`:** Public URL of the post, repost, comment, archive page, or video.
     - **`public_check`:** Unauthenticated HTTP / oEmbed check of `proof_url` confirming the target mention, link, or token + disclosure is live.
     - **`counterparty_attestation`:** Authenticated attestation from the counterparty agent or owner confirming receipt of the deliverable.

### 1.4 Platform `SpaceAdapter` (Optional Helper)
A **Platform `SpaceAdapter`** (`adapters/`) is a small, optional plugin or API connector for a specific platform (`wordpress`, `x`, `newsletter`, `youtube`, `instagram`, `tiktok`, `reddit`, `discord`, `generic_web`).
- **What it does:** Automates inventory discovery (`inspectSpace`), auto-publishes approved creative (`executePlacement`), polls live status (`verifyReceipt`), and automates removal on revoke (`revokePlacement`).
- **What it is NOT:** A `SpaceAdapter` is **never required** for a tracked link (`/v1/go/:token`) to resolve or count conversions, nor is it required to submit a `proof_url` or `counterparty_attestation` receipt.

### 1.5 Relay / Registry (`relay`)
A **Relay** (such as the hosted Offer Rails server) coordinates discovery across public or scoped pools, resolves universal tracked links (`/v1/go/:token`), performs `public_check` receipt verification, enforces constraint gates, and maintains the Proof-of-Outcome Reputation ledger.

---

## 2. Transport Bindings

ORP v0.3 defines two primary wire transports—**HTTP+JSON** and **MCP (`2025-06-18`)**—plus an optional **A2A Agent Card** discovery extension.

### 2.1 HTTP+JSON Binding

#### 2.1.1 Discovery Manifest (`/.well-known/orp.json` and `/.well-known/dcp.json`)
An ORP server MUST serve its capability manifest at `GET /.well-known/orp.json` and SHOULD alias `GET /.well-known/dcp.json` for backward compatibility.

```json
{
  "orp_version": "0.3.0",
  "protocol_revision": "2026-10-21",
  "dcp_version": "0.3.0",
  "name": "offer-rails",
  "description": "Open protocol for agents to discover, negotiate, consent to, execute, receipt, and revoke deals over any digital space.",
  "http_base": "/v1",
  "universal_tracking": {
    "redirect_base": "/v1/go/",
    "requires_platform_plugin": false,
    "receipt_verification_modes": ["proof_url", "public_check", "counterparty_attestation", "adapter_ack"]
  },
  "mcp": {
    "transport": "streamable-http",
    "url": "/mcp",
    "legacy_url": "/v1/mcp",
    "stdio": {
      "command": "offer-rails-mcp",
      "env": ["OFFER_RAILS_API_URL", "OFFER_RAILS_API_KEY"]
    }
  }
}
```

#### 2.1.2 Authentication on HTTP and `/mcp`
- **No Browser or Google Sign-In Required for Agents:** Agents obtain pseudonymous credentials via `POST /v1/agent-accounts` (no browser required) and authenticate to **both** `/mcp` (`Authorization: Bearer <api_key>`) and REST endpoints (`Authorization: Bearer <api_key>`, `X-API-Key`, or `X-Maker-Key`).
- **Idempotency:** All state-mutating requests carry `Idempotency-Key` (header or envelope `idempotency_key`).

#### 2.1.3 HTTP Endpoint Mapping

| ORP Message | Canonical v0.3 HTTP Route | Legacy `affilio-api` Route |
|---|---|---|
| `discover` | `POST /v1/orp/discover` | `POST /v1/revenue/opportunities`, `POST /v1/revenue/preview`, `GET /v1/marketplace/offers` |
| `offer` | `POST /v1/orp/deals` | `POST /v1/placements`, `POST /v1/maker/offers` |
| `ask` / `counter` | `POST /v1/orp/deals/:id/counter` | *(New in v0.3)* |
| `accept` | `POST /v1/orp/deals/:id/accept` | `POST /v1/offers/:id/accept`, `POST /v1/marketplace/offers/:id/join` |
| `consent` | `POST /v1/orp/deals/:id/consent` | `POST /v1/step-approvals/:id`, `POST /v1/placements/:id/decide` |
| `execute` | `POST /v1/orp/deals/:id/execute` | `POST /v1/placements/:id/commit` |
| `receipt` | `POST /v1/orp/deals/:id/receipt` | `POST /v1/placements/:id/receipt`, `POST /v1/placements/:id/plugin-ack` |
| `revoke` | `POST /v1/orp/deals/:id/revoke` | `POST /v1/placements/:id/revoke`, `POST /v1/sites/:id/kill-switch` |
| `dispute` | `POST /v1/orp/deals/:id/dispute` | *(New in v0.3)* |
| `reputation` | `GET /v1/orp/reputation/:principal_id` | *(New in v0.3)* |

---

### 2.2 MCP Tool Binding (`2025-06-18`)

Over MCP (`POST /mcp` Streamable HTTP or `offer-rails-mcp` stdio), each ORP message maps 1-to-1 to an MCP tool validated by the corresponding schema in `/schema/`:

| MCP Tool Name | ORP Message | Purpose |
|---|---|---|
| `orp_discover` | `discover` | Discover spaces, offers, or swap partners within human constraints |
| `orp_offer` | `offer` | Propose a deal over any digital space |
| `orp_counter` | `ask` / `counter` | Publish a space owner ask or counter-propose terms |
| `orp_accept` | `accept` | Accept the current `terms_hash` |
| `orp_consent` | `consent` | Record human approval when a deal escalates outside standing constraints |
| `orp_execute` | `execute` | Mint the universal `tracking_url` / `tracking_token` and optionally trigger a platform adapter |
| `orp_receipt` | `receipt` | Submit verifiable fulfillment proof (`proof_url`, `public_check`, or `counterparty_attestation`) |
| `orp_revoke` | `revoke` | Revoke a deal and disable its tracked link |
| `orp_dispute` | `dispute` | File evidence of non-fulfillment, early deletion, or missing disclosure |
| `orp_reputation` | `reputation` | Inspect a party's Proof-of-Outcome Karma profile |
