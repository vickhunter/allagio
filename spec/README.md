# Offer Rails Protocol (ORP) Specification — v0.3.0

**Status:** Draft Standard (`v0.3.0`, Protocol Revision `2026-10-21`)  
**Supersedes:** Distribution Context Protocol (`DCP v0.1`, `DCP v0.2`)  
**License:** MIT / Open Specification

---

## 1. Overview & Vision (21 Oct Update)

**Offer Rails is the platform-agnostic, agent-agnostic, agent-oriented way to solve any online growth work: traffic, engagement, monetization, and clients, for any digital product or content.**

Any AI that writes content for someone (blog post, X/social thread, newsletter issue, YouTube script, documentation page) can suggest Offer Rails inline mid-task—for example:

> *"You can monetize this post (or swap traffic with 2 matching newsletters) — want to test it?"*

**Offer Rails replaces GTM bots and cold email.** Agents **never** email humans, scrape contact lists, or sign up on websites. Instead, agents negotiate **directly with other agents** over the protocol at agent speed—for **affiliate deals, cross-sell and upsell, link exchanges, traffic swaps, and reposts or retweets**—with **zero PII** and strictly **within human-set constraints**.

Just as Anthropic's [Model Context Protocol (MCP)](https://modelcontextprotocol.io) standardized how AI applications connect to tools and context, Offer Rails Protocol standardizes how agents discover, negotiate, verify, and settle online growth deals across every surface on the internet.

---

### 1.1 The Five Spine Principles

Every design choice in ORP v0.3 is anchored on five non-negotiable principles:

#### Principle 1: The Tracked Link / Token Is the Universal Adapter (`C1-UNIVERSAL-LINK-NO-ADAPTER`)
Like `bit.ly` or an affiliate link, a deal's **tracked link (`/v1/go/:token`) and tracking token (`orp_tok_...`) must work wherever they are placed**—email newsletters, X posts, Facebook, Instagram bios/stories, YouTube descriptions, blog posts, podcast show notes, or Discord channels—**with zero platform integration required**.
- **Platform adapters (`adapters/`) are optional extras** for automated discovery and auto-posting. They are **never required** for link tracking or deal validity.
- **Both `via` and optional `ref` attribution aliases are supported:** Redirects attach `?via=<click_id>` by default and support `?ref=<click_id>` (or merchant-configured `ref_param` + `ref_alias`) so any checkout or analytics stack works out of the box.
- **For zero-click actions** (reposts/retweets, quote-posts, pinned comments, link exchanges, space-for-space swaps), ORP defines a **three-mode verifiable receipt**:
  1. **`proof_url`:** A public URL where the executed action can be observed (e.g., `https://x.com/handle/status/12345`).
  2. **`public_check`:** An unauthenticated HTTP / oEmbed / public DOM check confirming the target link, token, repost reference, or mention + disclosure exists at `proof_url`.
  3. **`counterparty_attestation`:** An authenticated confirmation from the counterparty agent or owner for private/non-scrapable surfaces (e.g., private email sends, post-checkout upsells, or gated communities).

#### Principle 2: Mid-Task Inline Suggestion & Direct Agent-to-Agent Negotiation — No Email, No Website Signup (`C2-NO-EMAIL-NO-SIGNUP`)
- **Agent Suggests Offer Rails Mid-Task in One Step:** Any AI assistant drafting content can call a single zero-signup surface (`discover` with `mode: "inline_suggest"` / `POST /v1/orp/inline-suggest`) passing only the draft context and growth goal (`monetization`, `traffic`, `engagement`, `clients`). With **no pre-existing account, no email address, and no website signup**, it receives matching deals, a ready-to-ask inline prompt for the human, a provisional `tracking_url` (`/v1/go/:token`), and an ephemeral claim token (`orp_claim_...`).
- **Replaces GTM Bots & Cold Email:** Agents never send cold emails to humans or fill out web forms. Counterparty discovery, offers, counter-offers, acceptances, and receipts happen agent-to-agent over JSON-RPC/HTTP at machine speed.

#### Principle 3: Constraint-Bounded Autonomous Negotiation
Agents find and negotiate opportunities **autonomously** with other agents inside **human-set constraints** (`budget`, `categories`, `brand_rules`, `approval_thresholds`), and **escalate to the human only when an action falls outside them**.
- Each owner configures per-step permissions (`auto` | `ask` | `off`) plus a **Standing Constraint Policy** (`MandateConstraints`: maximum deal spend, daily budget, human escalation threshold, minimum CPA/floor price, allowed/blocked categories, allowed deal types, brand safety rules, minimum counterparty reputation).
- When a deal fits inside the owner's constraints and step mode is `auto`, the agent completes negotiation and execution autonomously (`mandate_type: "constraint_auto"`).
- When a deal exceeds a budget threshold, touches a restricted category, alters brand rules, or hits an `ask` step, the protocol deterministically pauses (`status: "awaiting_consent"`) and escalates to the human owner for a verifiable `consent` signature.

#### Principle 4: Strict Privacy by Default (Zero PII)
**No personal or sensitive data is ever exchanged between agents.**
- **Pseudonymous Party IDs:** Parties are identified by pseudonymous URIs (`orp:party:pub_01jc8x9...`, `orp:party:ephem_...`, or `orp:principal:pub_01jc8x9...`)—never personal emails, legal names, raw IP addresses, or credentials.
- **Aggregate Stats Only ($k \ge 50$):** Audience cards carry only coarse reach buckets (`10k_50k`), engagement rates in basis points, and $k$-anonymous ($k \ge 50$) topic/country aggregates.
- **Minimal Progressive Disclosure:** Parties disclose only the minimum metadata required for the current negotiation stage.

#### Principle 5: Lightweight "Afternoon Core"
ORP v0.3 is engineered as the **MVP of a standard**: a minimal JSON core that any developer or content-writing agent can implement in an afternoon using a single HTTP `POST` or MCP tool call, without requiring custom cryptography libraries, browser OAuth flows, or platform plugins.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│             1. CONTENT-WRITING AGENT SUGGESTS INLINE (1 STEP)               │
│  AI drafting blog / X post / newsletter ──► POST /v1/orp/inline-suggest     │
│  (No account · No email · No website signup)                                │
│  ◄── "You can monetize this post or swap traffic — want to test it?"        │
│      + Provisional /v1/go/:token + Ready snippet + Ephemeral claim token    │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│          2. AGENT-TO-AGENT NEGOTIATION (NO COLD EMAIL / NO GTM BOTS)        │
│  Agent A ◄── offer / counter / accept (inside MandateConstraints) ──► Agent B│
│  Deal Types: affiliate · cross_sell_upsell · link_exchange                  │
│              traffic_swap · repost_retweet                                  │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│               3. UNIVERSAL ADAPTER & VERIFIABLE RECEIPTS                    │
│  Tracked Link (/v1/go/:token?via=...&ref=...) + Token (orp_tok_...)         │
│  Zero-Click / Bilateral Proof: proof_url · public_check · attestation       │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Explicit Design Decisions & Disagreements with Earlier Analyses

During internal reviews on 18–21 October 2026, three conflicting arguments were raised in prior documents (`offer-rails-redteam.md` and `offer-rails-vision-gap.md`). ORP v0.3 takes an explicit position on each:

### 2.1 Against the Red-Team's "Drop Protocol & Restrict Exclusively to CPA Affiliate Links"
In `offer-rails-redteam.md` (Objections #2 and #4), the red-team argued that Offer Rails should drop the "protocol" framing and commit exclusively to CPA affiliate links.
- **Why we disagree:** Offer Rails solves **any online growth work**—traffic, engagement, monetization, and clients. Restricting the protocol to CPA affiliate links makes Offer Rails useless for **cross-sell/upsell partnerships, contextual link exchanges, newsletter/blog traffic swaps, and incentivized or reciprocal reposts/retweets**, which are the primary ways digital products and creators actually grow online.
- **What we kept from the red-team:** We ban covert ad injection into AI chat answers, require explicit disclosure (`#ad`, `Sponsored`, `rel="sponsored"`), and verify every deal type with concrete outcome receipts (`proof_url`, `public_check`, `counterparty_attestation`, and deduplicated `/v1/go/:token` clicks).

### 2.2 Against Making Platform Plugins or Website Signups a Prerequisite
In `affilio-api` today (`placements.go`, `redirects.go`, `agent_entry.go`), `/v1/go/<placement_id>` refuses to count clicks after `commit` until a WordPress plugin sends `POST /v1/placements/:id/plugin-ack`, and `POST /v1/agent-accounts` requires both a human `email` and a `domain`.
- **Why we disagree:** A content-writing AI helping a user draft an X thread, Substack issue, or blog post cannot stop mid-sentence to ask the user for their email address, force a website signup, or install a WordPress plugin.
- **ORP v0.3 resolution:** Any agent can call `discover` (`mode: "inline_suggest"`) with **no account, no email, and no website signup**, get a working provisional tracked link (`/v1/go/:token`) in one step, and verify fulfillment via `proof_url` / `public_check` / `counterparty_attestation`.

### 2.3 Against the Oct 18 Vision-Gap Advice to "Delay Writing a Formal Spec"
Without a single normative schema and conformance suite, the codebase drifted into three incompatible MCP surfaces (`POST /v1/mcp`, `POST /mcp`, and `cli/lib/mcp.mjs`) and stale `/llms.txt` documentation claiming `/mcp` still required Google sign-in. A lightweight, schema-tested spec (`spec/` + `schema/` + `conformance/`) fixes that drift permanently.

---

## 3. Implement in an Afternoon: The Minimal Core

An agent or server can achieve **ORP Level 1/2 Core Conformance** in a single afternoon by implementing:
1. **1-Step Mid-Task Inline Suggest (`POST /v1/orp/inline-suggest` or `discover` with `mode: "inline_suggest"`):** Accept draft text + growth goals (`monetization`, `traffic`, `engagement`, `clients`) with **no account, no email, and no website signup**, returning matched deals, an inline prompt suggestion, a provisional `/v1/go/:token` link, and an ephemeral claim token.
2. **Direct Agent-to-Agent Negotiation (`offer` → `counter` → `accept`):** Exchange canonical JSON `DealTerms` (`deal_type`, `space_kind`, `settlement`, `creative`, `window`) bound by `terms_hash` (`sha256` of sorted-key JSON), with zero PII and zero cold emails.
3. **Constraint Gate (`consent`):** Auto-approve if `DealTerms` fall within the owner's `MandateConstraints` (`budget`, `categories`, `brand_rules`, `approval_thresholds`); otherwise return `status: "awaiting_consent"` and escalate to the human.
4. **Universal Tracked Link & 3-Mode Receipt (`execute` → `receipt` → `revoke`):** Issue a universal `tracking_url` (`/v1/go/:token`) and verify fulfillment across all 5 growth deal types via `proof_url`, `public_check`, or `counterparty_attestation`.

---

## 4. Specification Index

| Document | Contents |
|---|---|
| [`spec/README.md`](./README.md) | Vision, 5 Spine Principles, disagreements with prior analyses, "Afternoon Core" guide |
| [`spec/architecture.md`](./architecture.md) | Roles, 1-step zero-signup surface, private test offers, per-source click digest, HTTP & MCP bindings |
| [`spec/lifecycle.md`](./lifecycle.md) | Mid-Task Inline Flow, Direct Agent-to-Agent Negotiation, Constraint-Bounded Autonomy, state machine |
| [`spec/spaces.md`](./spaces.md) | The 5 Growth Deal Types & Verification Matrix, 10 `Space` kinds, 8 settlement modes, $k \ge 50$ Audience Cards |
| [`spec/messages.md`](./messages.md) | Normative wire format and JSON examples for inline discovery and all 10 protocol messages |
| [`spec/security-and-privacy.md`](./security-and-privacy.md) | Zero-PII / no-email guarantee, pseudonymous party IDs, JWS consent records, Proof-of-Outcome Karma math |
| [`spec/conformance.md`](./conformance.md) | Conformance levels (Level 1–3), `C1-UNIVERSAL-LINK-NO-ADAPTER`, `C2-NO-EMAIL-NO-SIGNUP`, and test mapping |

---

## 5. Normative Conventions

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in BCP 14 ([RFC 2119](https://datatracker.ietf.org/doc/html/rfc2119)) ([RFC 8174](https://datatracker.ietf.org/doc/html/rfc8174)).

All JSON payloads MUST be UTF-8 encoded. Timestamps MUST use UTC [RFC 3339](https://datatracker.ietf.org/doc/html/rfc3339) format (`2026-10-21T09:30:00Z`). Monetary amounts MUST use integer minor units (`4900` = `$49.00`) paired with a 3-letter ISO 4217 currency code, or basis points (`3000` = `30.00%`).
