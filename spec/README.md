# Offer Rails Protocol (ORP) Specification — v0.3.0

**Status:** Draft Standard (`v0.3.0`, Protocol Revision `2026-10-21`)  
**Supersedes:** Distribution Context Protocol (`DCP v0.1`, `DCP v0.2`)  
**License:** MIT / Open Specification

---

## 1. Overview & The Four Spine Principles

The **Offer Rails Protocol (ORP)** is an open, lightweight, versioned protocol that enables AI agents to **discover, negotiate, consent to, execute, receipt, and revoke deals over any digital space**—including affiliate programs, traffic swaps, space-for-space exchanges, incentivized reposts, comment spaces, newsletter slots, partnerships, data exchanges, and cross-platform growth.

Just as Anthropic's [Model Context Protocol (MCP)](https://modelcontextprotocol.io) standardized how AI applications connect to external tools and context, Offer Rails Protocol standardizes how agents negotiate and verify distribution deals across the open web and social platforms.

### 1.1 The Spine of Offer Rails Protocol (21 Oct Core Principles)

Every design choice in ORP v0.3 is anchored on four non-negotiable principles:

#### Principle 1: The Tracked Link / Token Is the Universal Adapter
Like `bit.ly` or an affiliate link, a deal's **tracked link (`/v1/go/:token`) and tracking token (`orp_tok_...`) must work wherever they are placed**—email newsletters, X posts, Facebook, Instagram bios/stories, YouTube descriptions, blog posts, podcast show notes, or Discord channels—**with zero platform integration required**.
- **Platform adapters (`adapters/`) are optional helpers** for automated discovery and auto-posting. They are **never required** for link tracking or deal validity.
- **For zero-click actions** (reposts, quote-posts, pinned comments, space-for-space swaps, host-read shoutouts), ORP defines a **three-mode verifiable receipt**:
  1. **`proof_url`:** A public URL where the executed action can be observed (e.g., `https://x.com/handle/status/12345`).
  2. **`public_check`:** An unauthenticated HTTP / oEmbed / public page check confirming the target link, token, or mention + disclosure exists at `proof_url`.
  3. **`counterparty_attestation`:** An authenticated confirmation from the counterparty agent or owner for private/non-scrapable surfaces (e.g., private email sends, gated communities, or podcast audio).

#### Principle 2: Autonomous Agent-to-Agent Negotiation Inside Human Constraints
Agents find and negotiate opportunities **autonomously** with other agents inside **human-set constraints** (`budget`, `categories`, `brand_rules`, `approval_thresholds`), and **escalate to the human only when an action falls outside them**.
- Each owner configures per-step permissions (`auto` | `ask` | `off`) plus a **Standing Constraint Policy** (maximum deal spend, minimum CPA/floor price, allowed/blocked categories, brand safety rules, minimum counterparty reputation).
- When a deal fits inside the owner's constraints and step mode is `auto`, the agent completes negotiation and execution autonomously (`mandate_type: "constraint_auto"`).
- When a deal exceeds a budget threshold, touches a restricted category, alters brand rules, or hits an `ask` step, the protocol deterministically pauses (`status: "awaiting_consent"`) and escalates to the human owner for a verifiable `consent` signature.

#### Principle 3: Strict Privacy by Default
**No personal or sensitive data is ever exchanged between agents.**
- **Pseudonymous Party IDs:** Parties are identified by pseudonymous URIs (`orp:party:pub_01jc8x9...` or `orp:principal:pub_01jc8x9...`)—never personal emails, legal names, raw IP addresses, or credentials.
- **Aggregate Stats Only ($k \ge 50$):** Audience cards carry only coarse reach buckets (`10k_50k`), engagement rates in basis points, and $k$-anonymous ($k \ge 50$) topic/country aggregates.
- **Minimal Progressive Disclosure:** Parties disclose only the minimum metadata required for the current negotiation stage.

#### Principle 4: Lightweight "Afternoon Core"
ORP v0.3 is engineered as the **MVP of a standard**: a minimal JSON core that any developer can implement in an afternoon using standard HTTP+JSON or MCP tool calls, without requiring custom cryptography libraries, blockchain nodes, or heavy platform SDKs to get started.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                   THE UNIVERSAL ADAPTER (ZERO INTEGRATION)                  │
│  Deal Consented ──► Tracked Link (/v1/go/:token) + Token (orp_tok_...)      │
│    Works anywhere: Email · X · YouTube · Blog · Instagram · Podcast · Chat  │
│  Zero-Click Actions ──► Verifiable Receipt:                                 │
│    (1) proof_url   (2) public_check (HTTP/oEmbed)   (3) counterparty_attest │
└─────────────────────────────────────────────────────────────────────────────┘
                                      ▲
                                      │ Optional helpers for discovery & auto-post
                         ┌────────────┴────────────┐
                         │ Platform SpaceAdapters  │
                         │ (WP, X, Newsletter...)  │
                         └─────────────────────────┘
```

---

## 2. Explicit Design Decisions & Disagreements with Earlier Analyses

During internal reviews on 18–21 October 2026, three conflicting arguments were raised in prior documents (`offer-rails-redteam.md` and `offer-rails-vision-gap.md`). ORP v0.3 takes an explicit position on each:

### 2.1 Against the Red-Team's "Drop Protocol & Restrict Exclusively to CPA Affiliate Links"
In `offer-rails-redteam.md` (Objections #2 and #4), the red-team argued that Offer Rails should drop the "protocol" framing and commit exclusively to CPA affiliate links because mixing ad models creates regulatory ambiguity and CPC/CPM invites agent click-farming.
- **Why we disagree:** Restricting the protocol to CPA affiliate links makes Offer Rails useless for 80%+ of creator and indie distribution on X, newsletters, YouTube, Reddit, and Discord—where deals are **space-for-space swaps, traffic swaps, incentivized reposts, comment plugs, and flat-fee newsletter slots** that have no Stripe checkout webhook at the end of a click.
- **What we kept from the red-team:** The red-team was right that unverified CPC/CPM click-bounties invite bot farms and that covert ad injection into AI chat answers violates user trust. ORP v0.3 therefore supports **8 settlement modes** (including `space_swap`, `traffic_swap`, `repost_bounty`, `flat_fee`, `cpa_commission`), bans raw CPC/CPM impression billing, bans covert `chat_completion_injection`, and uses the **3-mode verifiable receipt** (`proof_url`, `public_check`, `counterparty_attestation`) for non-click deals.

### 2.2 Against Making Platform Plugins a Hard Prerequisite for Tracking
In `affilio-api` today (`placements.go` + `redirects.go`), `/v1/go/<placement_id>` refuses to track clicks until a WordPress plugin sends `POST /v1/placements/:id/plugin-ack` with `X-Plugin-Key`, and `surface_url` is rejected unless it matches the single domain registered at signup.
- **Why we disagree:** Requiring a native platform plugin before a tracked link works makes "works on any platform" impossible on X, YouTube, Instagram, Substack, and podcasts where third-party server plugins cannot be installed.
- **ORP v0.3 resolution:** The tracked link (`/v1/go/:token`) is the **universal adapter** and activates as soon as the deal is consented and a receipt (`proof_url` / `public_check` / `counterparty_attestation` / `adapter_ack`) is recorded—no platform plugin required.

### 2.3 Against the Oct 18 Vision-Gap Advice to "Delay Writing a Formal Spec"
Without a single normative schema and conformance suite, the codebase drifted into **three incompatible MCP surfaces** (`POST /v1/mcp`, `POST /mcp`, and `cli/lib/mcp.mjs`) and stale `/llms.txt` documentation claiming `/mcp` still required Google sign-in. A lightweight, schema-tested spec (`spec/` + `schema/` + `conformance/`) fixes that drift permanently.

---

## 3. Implement in an Afternoon: The Minimal Core

An agent or server can achieve **ORP Level 1/2 Core Conformance** in a single afternoon by implementing:
1. **Discovery (`/.well-known/orp.json` + `discover`):** Return or query pseudonymous `Space` / `Offer` cards with aggregate stats ($k \ge 50$).
2. **Bilateral Terms (`offer` → `counter` → `accept`):** Exchange canonical JSON `DealTerms` (`space_kind`, `settlement`, `creative`, `window`) bound by `terms_hash` (`sha256` of sorted-key JSON).
3. **Constraint Gate (`consent`):** Auto-approve if `DealTerms` fall within the owner's `MandateConstraints` (`budget`, `categories`, `brand_rules`, `approval_thresholds`); otherwise return `status: "awaiting_consent"` (`hold_reason: "outside_constraints"` or `"step_ask"`) and wait for human approval.
4. **Universal Tracked Link & 3-Mode Receipt (`execute` → `receipt` → `revoke`):** Issue a universal `tracking_url` (`/v1/go/:token`) and verify fulfillment via `proof_url`, `public_check`, or `counterparty_attestation`.

---

## 4. Specification Index

| Document | Contents |
|---|---|
| [`spec/README.md`](./README.md) | Overview, 4 Spine Principles, disagreements with prior analyses, "Afternoon Core" guide |
| [`spec/architecture.md`](./architecture.md) | Roles (`Party`/`Principal`, `Agent`, Universal Tracked Link vs. optional `SpaceAdapter`, `Relay`), HTTP+JSON and MCP bindings |
| [`spec/lifecycle.md`](./lifecycle.md) | Capability negotiation, deal state machine, Constraint-Bounded Autonomy & escalation rules, `auto`/`ask`/`off`, versioning |
| [`spec/spaces.md`](./spaces.md) | The `Space` abstraction, 10 space kinds, 8 settlement modes, Universal Link vs. Zero-Click Receipts, $k$-anonymous Audience Cards |
| [`spec/messages.md`](./messages.md) | Normative wire format and JSON examples for all 10 messages (`discover`, `offer`, `ask`/`counter`, `accept`, `consent`, `execute`, `receipt`, `revoke`, `dispute`, `reputation`) |
| [`spec/security-and-privacy.md`](./security-and-privacy.md) | Pseudonymous party IDs, minimal disclosure, domain/public verification, JWS consent records, Proof-of-Outcome Karma math |
| [`spec/conformance.md`](./conformance.md) | Conformance levels (Level 1, Level 2, Level 3) and test suite mapping |

---

## 5. Normative Conventions

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in BCP 14 ([RFC 2119](https://datatracker.ietf.org/doc/html/rfc2119)) ([RFC 8174](https://datatracker.ietf.org/doc/html/rfc8174)).

All JSON payloads MUST be UTF-8 encoded. Timestamps MUST use UTC [RFC 3339](https://datatracker.ietf.org/doc/html/rfc3339) format (`2026-10-21T09:30:00Z`). Monetary amounts MUST use integer minor units (`4900` = `$49.00`) paired with a 3-letter ISO 4217 currency code, or basis points (`3000` = `30.00%`).
