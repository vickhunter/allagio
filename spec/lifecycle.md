# Offer Rails Protocol — Lifecycle, Constraint-Bounded Autonomy & Versioning

**Protocol Revision:** `2026-10-21` (`v0.3.0`)

---

## 1. Constraint-Bounded Autonomy & Human Escalation

Offer Rails Protocol is built around **autonomous agent-to-agent negotiation inside human-set constraints**, escalating to the human owner **only when an action falls outside those constraints** (or when the owner has explicitly pinned a step to `ask`).

### 1.1 Standing Human Constraints (`MandateConstraints`)
Each Principal configures a standing constraint policy that governs what its agent may negotiate and execute without interrupting the human:

```json
{
  "principal_id": "orp:party:pub_01jc8x9a",
  "step_modes": {
    "space_register": "auto",
    "discover_respond": "auto",
    "deal_offer": "auto",
    "deal_accept": "auto",
    "space_execute": "ask",
    "deal_modify": "ask",
    "settlement_payout": "auto"
  },
  "constraints": {
    "max_deal_amount_minor": 25000,
    "daily_budget_minor": 100000,
    "require_human_above_minor": 10000,
    "currency": "USD",
    "min_cpa_bps": 1500,
    "allowed_settlement_modes": ["cpa_commission", "flat_fee", "space_swap", "repost_bounty"],
    "allowed_categories": ["devtools", "ai-tools", "cloud-infrastructure", "productivity"],
    "blocked_categories": ["gambling", "adult", "unregulated-crypto", "payday-loans"],
    "brand_rules": {
      "require_verified_counterparty": true,
      "min_counterparty_karma": 10,
      "max_copy_chars": 280,
      "disallow_superlatives": true
    }
  }
}
```

### 1.2 Deterministic Evaluation: Autonomous vs. Escalated (`awaiting_consent`)
Whenever an agent proposes, accepts, or executes a deal, the constraint engine evaluates the deal's `DealTerms` against the Principal's `step_modes` and `constraints`:

1. **Blocked (`403 Forbidden`, `hold_reason: "step_off"` or `"blocked_category"`):**
   - The target step is `off`, OR the deal touches a `blocked_categories` topic. The deal is rejected immediately.
2. **Autonomous Execution (`200 OK` / `201 Created`, `mandate_type: "constraint_auto"`):**
   - The step is `auto`, AND every parameter of `DealTerms` is within `constraints` (amount $\le$ `require_human_above_minor`, category $\in$ `allowed_categories`, counterparty karma $\ge$ `min_counterparty_karma`, hard claim/disclosure checks pass).
   - The agent proceeds immediately without waiting for human interaction.
3. **Human Escalation (`201 Created` / `202 Accepted`, `status: "awaiting_consent"`):**
   - The step is configured as `ask` (the default for `space_execute` on new spaces), OR any deal term falls outside the standing constraints (e.g., `amount_minor > require_human_above_minor`, unverified counterparty, new category, or `claim_check_unavailable`).
   - The protocol pauses the deal in `CONSENT_PENDING` and returns the exact `escalation_reasons` (e.g., `["exceeds_approval_threshold", "step_ask"]`) and `approval_url` so the human can review and emit a `consent` message.

---

## 2. Deal State Machine

```
  [DISCOVERED]
       │ offer / ask
       ▼
   [OFFERED] ◄──────► [COUNTERED]  (Autonomous agent-to-agent negotiation)
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
                          [RECEIPTED] (Deal is LIVE; tracked link counts conversions)
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

### 2.1 State Transition Rules

| State | Message | Next State | Notes |
|---|---|---|---|
| *(initial)* | `offer` / `ask` | `OFFERED` | `seq = 1`, includes `terms` and `terms_hash` |
| `OFFERED` / `COUNTERED` | `counter` | `COUNTERED` | `seq` increments by 1; agents negotiate autonomously within constraints |
| `OFFERED` / `COUNTERED` | `accept` | `CONSENT_PENDING` or `CONSENTED` | Evaluates `MandateConstraints`; escalates to human only if outside constraints or step is `ask` |
| `CONSENT_PENDING` | `consent` (`approve`) | `CONSENTED` | Verifiable human approval bound to `terms_hash` |
| `CONSENT_PENDING` | `consent` (`deny`) | `CANCELLED` | Terminal state |
| `CONSENTED` | `execute` | `EXECUTING` | Issues universal `tracking_url` (`/v1/go/:token`) and `tracking_token` |
| `EXECUTING` | `receipt` | `RECEIPTED` | Verified via `proof_url`, `public_check`, `counterparty_attestation`, or `adapter_ack` |
| `RECEIPTED` | `revoke` | `REVOKED` | Unconditional owner kill-switch; disables redirect immediately |
| `RECEIPTED` | `dispute` | `DISPUTED` | Triggered on premature deletion, missing disclosure, or broken swap |
| `RECEIPTED` / `SETTLED` / `DISPUTED` | `reputation` | `REPUTATION_RECORDED` | Updates dimensional Proof-of-Outcome Karma |

---

## 3. Versioning Policy

- **Protocol Version:** `orp_version: "0.3.0"` (SemVer) + `protocol_revision: "2026-10-21"`.
- **Forward & Backward Compatibility:** Minor versions (`0.3.x`) are strictly additive. Implementations MUST ignore unknown optional properties on incoming JSON objects while enforcing required fields and enum constraints defined in `/schema/`.
