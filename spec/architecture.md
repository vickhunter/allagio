# Offer Rails Protocol — Architecture, Zero-Signup Surface & Transports

**Protocol Revision:** `2026-10-21` (`v0.3.0`)

---

## 1. Architectural Roles & The Anti-Cold-Email Model

Offer Rails Protocol is the platform-agnostic, agent-agnostic, agent-oriented way to execute online growth work (**traffic, engagement, monetization, clients**). It replaces GTM outreach bots and cold email by connecting content-writing and product agents directly over a structured protocol.

### 1.1 Party / Principal (`principal`)
A **Principal** (or **Party**) is the human or organization that owns content/distribution spaces, a digital product/budget, or both.
- **Pseudonymous & Zero-Email by Default:** Identified on the wire by a pseudonymous party URI (`orp:party:pub_01jc8x9...`, `orp:party:ephem_01jc8x9...`, or `orp:principal:pub_01jc8x9...`). Personal email addresses, legal names, and payment credentials MUST NOT be required to use the protocol and MUST NOT be broadcast to counterparty agents.
- **Constraint Setter:** Defines standing human constraints (`budget`, `categories`, `brand_rules`, `approval_thresholds`) and per-step modes (`auto` | `ask` | `off`).
- **Escalation Authority:** Approves or denies only when an agent action steps outside standing constraints, and retains an instant `revoke` kill-switch.

### 1.2 Content-Writing or Growth Agent (`agent`)
An **Agent** is any AI system writing content (blogs, social posts, newsletters, docs, scripts) or managing growth for a digital product.
- **Suggests Offer Rails Mid-Task:** While helping a user write a post, newsletter, or launch announcement, the agent can query Offer Rails in **one step with no account, email, or website signup** and suggest growth actions inline (*"You can monetize this post, or swap traffic with 2 developer newsletters — want to test it?"*).
- **Never Emails Humans or Signs Up on Websites:** Agents never send cold emails or fill out web registration forms. They negotiate directly with counterparty agents over ORP (`discover` → `offer` → `counter` → `accept` → `execute` → `receipt`) at agent speed.

### 1.3 Universal Adapter: The Tracked Link & Verifiable Receipt (Core — Required)
The **Universal Adapter** is built directly into the core protocol and requires **zero platform API keys, plugins, or OAuth integrations**:
1. **Universal Tracked Link (`/v1/go/:token`) with `via` and Optional `ref` Alias:**
   - Upon inline suggestion (`provisional`) or deal `execute`, the protocol issues a deal-scoped **tracked link** (`https://<relay>/v1/go/<token>`) and **tracking token** (`orp_tok_<id>`).
   - Works immediately on any surface: email, X, LinkedIn, YouTube, Instagram, Facebook, blog, podcast notes, or chat.
   - When redirecting to the destination URL, the relay attaches the click ID using `ref_param` (default `via`, e.g. `?via=<click_id>`) and optionally mirrors or aliases `ref` (`?via=<click_id>&ref=<click_id>` or `?ref=<click_id>` when `ref_alias: true` or `ref_param: "ref"` is set), and accepts `?via=plugin` or `?ref=plugin` on forwarded hits.
2. **Three-Mode Verifiable Receipt for Link & Zero-Click Deals:**
   - Fulfillment across all 5 deal types (`affiliate`, `cross_sell_upsell`, `link_exchange`, `traffic_swap`, `repost_retweet`) is verified via `proof_url`, `public_check` (unauthenticated HTTP/oEmbed check), or `counterparty_attestation`.

### 1.4 Platform `SpaceAdapter` (Optional Extra)
A **Platform `SpaceAdapter`** (`adapters/`) is an optional connector for a specific surface (`wordpress`, `x`, `newsletter`, `youtube`, etc.). It is **never required** for tracked links or receipts to work.

---

## 2. The 1-Step Zero-Signup Surface for Content-Writing Agents

To allow **any AI that writes content for someone** to suggest and test Offer Rails inline without friction, an ORP relay MUST expose a **1-step zero-signup discovery and offer surface**:

### 2.1 One-Step Inline Suggest (`POST /v1/orp/inline-suggest` or `orp_discover` with `mode: "inline_suggest"`)
- **Authentication:** None required (unauthenticated or ephemeral self-generated `orp:party:ephem_<random>`).
- **Email / Website Signup Required:** **None.** Neither `email` nor `domain` is required.
- **Single-Call Input:**
  - `draft_text`: The content currently being written (blog post draft, X thread, newsletter issue, product launch blurb).
  - `surface_kind`: Optional surface hint (`content_inline`, `social_post`, `social_repost`, `newsletter_slot`, `video_segment`, `cross_promo_slot`).
  - `growth_goals`: Subset of `["monetization", "traffic", "engagement", "clients"]`.
  - `deal_types`: Optional filter over `["affiliate", "cross_sell_upsell", "link_exchange", "traffic_swap", "repost_retweet"]`.
  - `constraints`: Optional inline `MandateConstraints` (e.g., `blocked_categories`, `min_cpa_bps`).
- **Single-Call Output (Everything Needed to Ask the Human & Act Immediately):**
  - `ephemeral_party_id`: `"orp:party:ephem_01jc9..."`
  - `ephemeral_agent_key`: Short-lived bearer token (`orp_ek_...`) scoped to this pseudonymous party so the agent can immediately negotiate, execute, or read click digests without a separate registration call.
  - `inline_prompt_suggestion`: Human-ready one-liner, e.g. `"You can monetize this post (25% CPA on CloudBranch) or run a 1:1 traffic swap — want to test it?"`
  - `suggestions`: Ranked array of ready-to-test deals, each containing:
    - `deal_type`: `"affiliate" | "cross_sell_upsell" | "link_exchange" | "traffic_swap" | "repost_retweet"`
    - `growth_goal`: `"monetization" | "traffic" | "engagement" | "clients"`
    - `offer_or_space_id`, `counterparty_id` (`orp:party:...`), `headline`, `match_rationale`, `settlement_summary`
    - `provisional_tracking_url`: `"https://<relay>/v1/go/trk_..."` (active immediately once the agent sends `receipt` or `one_step_claim_token`)
    - `provisional_tracking_token`: `"orp_tok_..."`
    - `ready_to_paste_snippet`: Pre-formatted inline copy + disclosure (`#ad` or `Sponsored`) ready to insert into the user's draft.
    - `one_step_claim_token`: `"orp_claim_..."`

### 2.2 Sanctioned Private Test Offers (Without a Public `https` Site)
Agents and developers frequently test Offer Rails in local, staging, or pre-launch environments before a public `https://` website is live:
- An agent MAY create or submit an offer with `visibility_pools: ["private_test"]` (or `test_mode: true` on `POST /v1/maker/offers`).
- **Sanctioned Private Test Rules:**
  - Private test offers list immediately (`status: "listed"`, `visibility: "private_test"`) for the owning agent and explicitly invited test counterparties **without** requiring a public `https://` website readiness probe (`site_reachable`) or admin queue approval.
  - Clicks and conversions on `private_test` offers are tagged `click_class: "test"` / `test_mode: true` so they can be verified end-to-end without polluting public marketplace rankings or live payout ledgers.

### 2.3 Per-Source Click & Outcome Digest Readable with an Agent Key
Both maker and publisher agents need to inspect which surfaces and referral sources (`x.com`, `substack.com`, `direct`, newsletter archive, etc.) are driving traffic and conversions:
- `GET /v1/digest`, `GET /v1/maker/digest`, and `GET /v1/account/digest` MUST accept standard agent keys (`Authorization: Bearer <api_key>`, `X-API-Key`, `X-Maker-Key`, or `orp_ek_...` ephemeral keys)—never requiring a browser dashboard login session—and MUST include `clicks_by_source: [{"source": "x.com", "clicks": 14, "paid_clicks": 12}, ...]` alongside per-deal click and conversion totals.

---

## 3. Transport Bindings

### 3.1 Discovery Manifest (`/.well-known/orp.json` and `/.well-known/dcp.json`)

```json
{
  "orp_version": "0.3.0",
  "protocol_revision": "2026-10-21",
  "dcp_version": "0.3.0",
  "name": "offer-rails",
  "description": "Platform-agnostic, agent-oriented protocol for online growth: traffic, engagement, monetization, and clients.",
  "http_base": "/v1",
  "inline_suggest": {
    "url": "/v1/orp/inline-suggest",
    "legacy_preview_url": "/v1/revenue/preview",
    "requires_account": false,
    "requires_email": false,
    "requires_website_signup": false
  },
  "universal_tracking": {
    "redirect_base": "/v1/go/",
    "requires_platform_plugin": false,
    "attribution_params": ["via", "ref"],
    "receipt_verification_modes": ["proof_url", "public_check", "counterparty_attestation", "adapter_ack"]
  },
  "deal_types": [
    "affiliate",
    "cross_sell_upsell",
    "link_exchange",
    "traffic_swap",
    "repost_retweet"
  ],
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

### 3.2 HTTP Endpoint Mapping

| ORP Operation | Canonical v0.3 HTTP Route | Current `affilio-api` Route & Status |
|---|---|---|
| **1-Step Inline Suggest (Zero Signup)** | `POST /v1/orp/inline-suggest` | `POST /v1/revenue/preview` (Preview only; lacks provisional link & ephemeral key) |
| `discover` | `POST /v1/orp/discover` | `POST /v1/revenue/opportunities`, `GET /v1/marketplace/offers` |
| `offer` | `POST /v1/orp/deals` | `POST /v1/placements`, `POST /v1/maker/offers`, `POST /v1/deals/propose` |
| `ask` / `counter` | `POST /v1/orp/deals/:id/counter` | *(Missing in `affilio-api`)* |
| `accept` | `POST /v1/orp/deals/:id/accept` | `POST /v1/offers/:id/accept`, `POST /v1/marketplace/offers/:id/join` |
| `consent` | `POST /v1/orp/deals/:id/consent` | `POST /v1/step-approvals/:id`, `POST /v1/placements/:id/decide` |
| `execute` | `POST /v1/orp/deals/:id/execute` | `POST /v1/placements/:id/commit` |
| `receipt` | `POST /v1/orp/deals/:id/receipt` | `POST /v1/placements/:id/receipt`, `POST /v1/placements/:id/plugin-ack` |
| `revoke` | `POST /v1/orp/deals/:id/revoke` | `POST /v1/placements/:id/revoke`, `POST /v1/sites/:id/kill-switch` |
| `dispute` | `POST /v1/orp/deals/:id/dispute` | *(Missing in `affilio-api`)* |
| `reputation` | `GET /v1/orp/reputation/:principal_id` | *(Missing in `affilio-api`)* |
| **Per-Source Click Digest** | `GET /v1/orp/digest` | `GET /v1/account/digest` (Currently blocked for agent keys; `GET /v1/digest` omits `clicks_by_source`) |
