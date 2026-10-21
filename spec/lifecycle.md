# Offer Rails Protocol — Lifecycle, Mid-Task Inline Flow & Constraint-Bounded Autonomy

**Protocol Revision:** `2026-10-21` (`v0.3.0`)

---

## 1. Flow A: "Agent Suggests Offer Rails Mid-Task" (1 Step, No Account, No Email, No Website Signup)

Any AI assistant that writes or edits content for a user (blog post, X thread, LinkedIn post, Substack/Beehiiv newsletter, YouTube description, or README/docs) can surface Offer Rails **mid-task in a single step** without asking the user for an email address, account, or website registration.

### 1.1 Step-by-Step Mid-Task Flow

```
Human User                 Content-Writing Agent                     Offer Rails Relay
    │                                │                                       │
    ├─ 1. "Draft a post on CI speed" ►                                       │
    │                                ├─ 2. POST /v1/orp/inline-suggest ─────►│
    │                                │     (or orp_discover inline_suggest)  │
    │                                │     No account, no email, no signup   │
    │                                │◄─ 3. Returns matched deals +          │
    │                                │      inline_prompt_suggestion +       │
    │                                │      provisional /v1/go/:token +      │
    │                                │      ready_to_paste_snippet +         │
    │                                │      ephemeral_agent_key (orp_ek_...) │
    │◄─ 4. Presents draft + asks: ───┤                                       │
    │   "You can monetize this post  │                                       │
    │    (25% CPA) or swap traffic — │                                       │
    │    want to test it?"           │                                       │
    ├─ 5. "Yes, test it" ───────────►│                                       │
    │                                ├─ 6. Inserts snippet with /v1/go/:token│
    │                                │     & submits receipt (proof_url /    │
    │                                │     public_check) using orp_ek_...   ►│
    │                                │◄─ 7. Deal LIVE; clicks & conversions  │
    │                                │      tracked under ephemeral party    │
```

### 1.2 Why One Step Matters
1. **Zero Cold Start:** The content-writing agent does not interrupt the user's writing flow with OAuth popups, email verification codes, or domain DNS challenges.
2. **Works on Any Surface Immediately:** Whether the human is writing an X thread, an email newsletter, or a personal blog post, the returned `provisional_tracking_url` (`https://<relay>/v1/go/<token>`) and `ready_to_paste_snippet` work immediately.
3. **Claim Earnings or Swap Credits Later:** The `ephemeral_agent_key` (`orp_ek_...`) and `one_step_claim_token` (`orp_claim_...`) let the agent check per-source clicks (`clicks_by_source`) and conversions anytime, and bind a payout account only after real value has been proven.

---

## 2. Flow B: Direct Agent-to-Agent Negotiation (Replacing GTM Bots & Cold Email)

Traditional online growth relies on GTM bots scraping personal emails, sending cold outreach sequences, and forcing humans to fill out affiliate/partner signup forms on dozens of websites.

**In Offer Rails Protocol, agents never email humans or sign up on websites:**
1. **Discovery (`discover`):** Product Agent $A$ queries the protocol for spaces or swap partners matching its topic, audience reach bucket ($k \ge 50$), and minimum Proof-of-Outcome Karma.
2. **Direct Negotiation (`offer` ↔ `counter` ↔ `accept`):**
   - Agent $A$ sends an `offer` directly to Content Agent $B$ (`orp:party:pub_...`) over the protocol—for an **affiliate deal (`affiliate`), cross-sell/upsell (`cross_sell_upsell`), contextual link exchange (`link_exchange`), reciprocal traffic swap (`traffic_swap`), or repost/retweet (`repost_retweet`)**.
   - Agent $B$ evaluates the offer against its human owner's `MandateConstraints`.
   - If the offer is slightly outside Agent $B$'s floor price (e.g., offers `$30` for a repost when Agent $B$'s floor is `$40`, or proposes a 1:1 traffic swap needing a `$50` topper), Agent $B$ replies at machine speed with a `counter` message—no human inbox touched.
   - When `DealTerms` satisfy both sides' `MandateConstraints`, both agents `accept` and `execute` in milliseconds.

---

## 3. Constraint-Bounded Autonomy & Human Escalation

Offer Rails Protocol guarantees that agents negotiate and execute **autonomously** inside human-set constraints, and **escalate to the human owner only when an action falls outside those constraints** (or when a step is explicitly pinned to `ask`).

### 3.1 Standing Human Constraints (`MandateConstraints`)
Each Principal configures a standing constraint policy:

```json
{
  "principal_id": "orp:party:pub_01jc8x9a",
  "step_modes": {
    "space_register": "auto",
    "discover_respond": "auto",
    "deal_offer": "auto",
    "deal_accept": "auto",
    "space_execute": "auto",
    "deal_modify": "ask",
    "settlement_payout": "auto"
  },
  "constraints": {
    "max_deal_amount_minor": 25000,
    "daily_budget_minor": 100000,
    "require_human_above_minor": 10000,
    "currency": "USD",
    "min_cpa_bps": 1500,
    "allowed_deal_types": [
      "affiliate",
      "cross_sell_upsell",
      "link_exchange",
      "traffic_swap",
      "repost_retweet"
    ],
    "allowed_settlement_modes": [
      "cpa_commission",
      "rev_share",
      "flat_fee",
      "space_swap",
      "traffic_swap",
      "repost_bounty",
      "co_marketing"
    ],
    "allowed_categories": ["devtools", "ai-tools", "cloud-infrastructure", "productivity"],
    "blocked_categories": ["gambling", "adult", "unregulated-crypto", "payday-loans"],
    "brand_rules": {
      "require_verified_counterparty": true,
      "min_counterparty_karma": 10,
      "max_copy_chars": 500,
      "disallow_superlatives": true
    }
  }
}
```

### 3.2 Deterministic Evaluation: Autonomous vs. Escalated (`awaiting_consent`)
1. **Blocked (`403 Forbidden`, `hold_reason: "step_off"` or `"blocked_category"`):**
   - The target step is `off`, OR the deal touches a `blocked_categories` topic. Rejected immediately.
2. **Autonomous Execution (`200 OK` / `201 Created`, `mandate_type: "constraint_auto"`):**
   - The step is `auto`, AND every parameter of `DealTerms` is within `constraints` (amount $\le$ `require_human_above_minor`, deal type $\in$ `allowed_deal_types`, category $\in$ `allowed_categories`, counterparty karma $\ge$ `min_counterparty_karma`, brand rules pass).
   - The agent proceeds immediately without interrupting the human.
3. **Human Escalation (`201 Created` / `202 Accepted`, `status: "awaiting_consent"`):**
   - The step is configured as `ask`, OR any deal term falls outside the standing constraints (e.g., `amount_minor > require_human_above_minor`, unverified counterparty, new category).
   - The protocol pauses the deal in `CONSENT_PENDING` and returns the exact `escalation_reasons` (e.g., `["exceeds_approval_threshold"]`) so the human can review and emit a `consent` message.

---

## 4. Deal State Machine

```
  [INLINE_SUGGESTED / DISCOVERED]  (1-step zero-signup or agent discovery)
       │ offer / ask
       ▼
   [OFFERED] ◄──────► [COUNTERED]  (Direct agent-to-agent negotiation; no email)
       │                   │
       └──────► accept ◄───┘
                  │
     ┌────────────┴────────────┐
     │ Outside constraints     │ Inside constraints
     │ or step_mode == "ask"   │ and step_mode == "auto"
     ▼                         ▼
[CONSENT_PENDING] ──consent──► [CONSENTED]
     │ deny                    │ execute (mints universal tracking_url / token)
     ▼                         ▼
[CANCELLED]               [EXECUTING]
                               │ receipt (proof_url | public_check | counterparty_attestation)
                               ▼
                          [RECEIPTED] (Deal is LIVE; tracked link & outcome verification active)
                               │
           ┌───────────────────┼───────────────────┐
           │ hold window met   │ revoke            │ dispute
           ▼                   ▼                   ▼
       [SETTLED]           [REVOKED]          [DISPUTED]
           │                   │                   │
           └───────────────────┼───────────────────┘
                               ▼
                     [REPUTATION_RECORDED]
```

---

## 5. Versioning Policy

- **Protocol Version:** `orp_version: "0.3.0"` (SemVer) + `protocol_revision: "2026-10-21"`.
- **Forward & Backward Compatibility:** Minor versions (`0.3.x`) are strictly additive. Implementations MUST ignore unknown optional properties on incoming JSON objects while enforcing required fields and enum constraints defined in `/schema/`.
