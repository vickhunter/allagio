# Offer Rails Protocol — Message Types & Wire Examples

**Protocol Revision:** `2026-10-21` (`v0.3.0`)

---

## 1. Common Message Envelope

Every ORP v0.3 message uses a lightweight JSON envelope with **pseudonymous party identifiers** (`orp:party:<id>` or `orp:principal:<id>`). **No personal names, email addresses, website signups, or raw credentials ever appear in protocol messages.**

| Field | Type | Required | Description |
|---|---|---|---|
| `orp_version` | `string` | Yes | Protocol version (`"0.3.0"`) |
| `message_id` | `string` | Yes | Unique message ID (`msg_...`) |
| `message_type` | `string` | Yes | `discover` \| `offer` \| `ask` \| `counter` \| `accept` \| `consent` \| `execute` \| `receipt` \| `revoke` \| `dispute` \| `reputation` |
| `deal_id` | `string` | Conditional | Required on all deal lifecycle messages (`offer` through `dispute`) |
| `sender` | `string` | Yes | Pseudonymous Party URI (`orp:party:...`, `orp:party:ephem_...`, or `orp:principal:...`) |
| `recipient` | `string` | No | Counterparty Party URI or Relay URI |
| `timestamp` | `string` | Yes | RFC 3339 UTC timestamp |
| `idempotency_key` | `string` | Conditional | Required on state-mutating messages |
| `signature` | `object` | No | Optional JWS signature (`alg`, `kid`, `jws`) |
| `payload` | `object` | Yes | Message body validated against `/schema/<type>.schema.json` |

---

## 2. `discover` — Mode 1: Mid-Task Inline Suggestion (1 Step, No Account, No Email, No Website Signup)

Any AI writing content for someone (blog, social post, newsletter) can call `discover` with `mode: "inline_suggest"` in **one step with no account, email, or website signup**:

### 2.1 Request (`discover` with `mode: "inline_suggest"`)

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9000inline",
  "message_type": "discover",
  "sender": "orp:party:ephem_writer_01jc9",
  "timestamp": "2026-10-21T09:29:00Z",
  "payload": {
    "mode": "inline_suggest",
    "target_type": "offer",
    "draft_text": "We migrated our CI pipeline to ephemeral Postgres branches on every pull request and cut integration test times by 45%.",
    "surface_kinds": ["content_inline", "newsletter_slot", "social_post"],
    "growth_goals": ["monetization", "traffic"],
    "deal_types": ["affiliate", "traffic_swap", "link_exchange"],
    "max_results": 3
  }
}
```

### 2.2 Response (1-Step Actionable Suggestions + Provisional Universal Tracked Link)

```json
{
  "orp_version": "0.3.0",
  "ephemeral_party_id": "orp:party:ephem_writer_01jc9",
  "ephemeral_agent_key": "orp_ek_98f1a2b3c4d5",
  "inline_prompt_suggestion": "You can monetize this post (25% CPA on CloudBranch) or swap traffic with Postgres Weekly — want to test it?",
  "suggestions": [
    {
      "suggestion_id": "sug_01jc9_aff",
      "deal_type": "affiliate",
      "growth_goal": "monetization",
      "offer_id": "off_cloudbranch_25",
      "counterparty_id": "orp:party:maker_cloudbranch",
      "headline": "CloudBranch — Ephemeral Postgres branching for CI",
      "match_rationale": "Exact match on Postgres branching for CI pull requests (score: 92/100).",
      "settlement_summary": "25% CPA commission (30-day attribution)",
      "provisional_tracking_url": "https://offer-rails.com/v1/go/trk_01jc9aff01",
      "provisional_tracking_token": "orp_tok_01jc9aff01",
      "ready_to_paste_snippet": "Sponsored: Try CloudBranch for instant Postgres branches in CI (https://offer-rails.com/v1/go/trk_01jc9aff01).",
      "one_step_claim_token": "orp_claim_01jc9aff01"
    },
    {
      "suggestion_id": "sug_01jc9_swap",
      "deal_type": "traffic_swap",
      "growth_goal": "traffic",
      "space_id": "spc_nl_postgres_weekly",
      "counterparty_id": "orp:party:pub_pgweekly",
      "headline": "1:1 Traffic Swap with Postgres Weekly (10k–50k readers)",
      "match_rationale": "High overlap on Postgres CI & developer tooling.",
      "settlement_summary": "Reciprocal 250 verified reader visits",
      "provisional_tracking_url": "https://offer-rails.com/v1/go/trk_01jc9swap02",
      "provisional_tracking_token": "orp_tok_01jc9swap02",
      "ready_to_paste_snippet": "Partner pick: Read Postgres Weekly for deep-dive DB engineering (https://offer-rails.com/v1/go/trk_01jc9swap02) #partner",
      "one_step_claim_token": "orp_claim_01jc9swap02"
    }
  ]
}
```

---

## 3. `discover` — Mode 2: Agent-to-Agent Network Discovery

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9001discover",
  "message_type": "discover",
  "sender": "orp:party:maker_codelens",
  "timestamp": "2026-10-21T09:30:00Z",
  "payload": {
    "target_type": "space",
    "query": "TypeScript and AI engineering newsletters or X accounts",
    "deal_types": ["affiliate", "cross_sell_upsell", "link_exchange", "traffic_swap", "repost_retweet"],
    "space_kinds": ["newsletter_slot", "social_repost", "content_inline"],
    "settlement_modes": ["flat_fee", "space_swap", "traffic_swap", "cpa_commission", "repost_bounty"],
    "pools": ["public", "network:indie-ai-founders"],
    "min_reputation_karma": 20,
    "max_results": 5
  }
}
```

---

## 4. `offer` — Propose Any Growth Deal Type Over Any Space

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9002offer",
  "message_type": "offer",
  "deal_id": "deal_01jc9002swap",
  "sender": "orp:party:maker_codelens",
  "recipient": "orp:party:pub_tsweekly",
  "timestamp": "2026-10-21T09:35:00Z",
  "idempotency_key": "idem_offer_01jc9002swap",
  "payload": {
    "seq": 1,
    "terms": {
      "deal_type": "link_exchange",
      "space_id": "spc_nl_typescript_weekly",
      "space_kind": "newsletter_slot",
      "settlement": {
        "mode": "space_swap",
        "initiator_space_id": "spc_x_codelens_feed",
        "responder_space_id": "spc_nl_typescript_weekly",
        "sync_window_seconds": 86400,
        "min_live_seconds": 604800
      },
      "creative": {
        "headline": "CodeLens AI — Schema-checked MCP & ORP debugging",
        "body": "Inspect and replay agent tool calls locally with zero config.",
        "target_url": "https://codelens.example.com",
        "disclosure_text": "Sponsored partner swap",
        "rel_attributes": ["sponsored"]
      },
      "reciprocal_creative": {
        "headline": "TypeScript Weekly Dispatch",
        "body": "Deep-dive TypeScript & agent engineering tutorials every Thursday.",
        "target_url": "https://tsweekly.example.com",
        "disclosure_text": "#ad #partner"
      },
      "window": {
        "execute_after": "2026-10-22T08:00:00Z",
        "execute_before": "2026-10-25T20:00:00Z",
        "min_live_seconds": 604800
      }
    },
    "terms_hash": "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
    "expires_at": "2026-10-22T09:35:00Z"
  }
}
```

---

## 5. `ask` / `counter` — Direct Agent-to-Agent Negotiation Turn

Agents negotiate directly with other agents inside their owners' standing constraints, with zero cold emails.

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9003counter",
  "message_type": "counter",
  "deal_id": "deal_01jc9002swap",
  "sender": "orp:party:pub_tsweekly",
  "recipient": "orp:party:maker_codelens",
  "timestamp": "2026-10-21T09:42:00Z",
  "idempotency_key": "idem_counter_01jc9003",
  "payload": {
    "seq": 2,
    "prior_message_id": "msg_01jc9002offer",
    "reason": "Adding $150 topper to balance newsletter subscriber reach.",
    "terms": {
      "deal_type": "link_exchange",
      "space_id": "spc_nl_typescript_weekly",
      "space_kind": "newsletter_slot",
      "settlement": {
        "mode": "space_swap",
        "initiator_space_id": "spc_x_codelens_feed",
        "responder_space_id": "spc_nl_typescript_weekly",
        "topper_amount_minor": 15000,
        "currency": "USD",
        "sync_window_seconds": 86400,
        "min_live_seconds": 604800
      },
      "creative": {
        "headline": "CodeLens AI — Schema-checked MCP & ORP debugging",
        "body": "Inspect and replay agent tool calls locally with zero config.",
        "target_url": "https://codelens.example.com",
        "disclosure_text": "Sponsored partner: CodeLens AI",
        "rel_attributes": ["sponsored"]
      },
      "reciprocal_creative": {
        "headline": "TypeScript Weekly Dispatch",
        "body": "Deep-dive TypeScript & agent engineering tutorials every Thursday.",
        "target_url": "https://tsweekly.example.com",
        "disclosure_text": "#ad #partner"
      },
      "window": {
        "execute_after": "2026-10-22T08:00:00Z",
        "execute_before": "2026-10-25T20:00:00Z",
        "min_live_seconds": 604800
      }
    },
    "terms_hash": "sha256:4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a",
    "expires_at": "2026-10-22T09:42:00Z"
  }
}
```

---

## 6. `accept` — Bind Deal Terms

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9004accept",
  "message_type": "accept",
  "deal_id": "deal_01jc9002swap",
  "sender": "orp:party:maker_codelens",
  "recipient": "orp:party:pub_tsweekly",
  "timestamp": "2026-10-21T09:45:00Z",
  "idempotency_key": "idem_accept_01jc9004",
  "payload": {
    "accepted_message_id": "msg_01jc9003counter",
    "accepted_seq": 2,
    "terms_hash": "sha256:4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a"
  }
}
```

---

## 7. `consent` — Constraint Auto-Approval or Escalated Human Consent

When a deal is within human constraints (`mandate_type: "constraint_auto"`), consent is recorded automatically. When it exceeds a constraint (here `topper_amount_minor: 15000` > `require_human_above_minor: 10000`) or hits an `ask` step, the human approves and emits `mandate_type: "explicit_owner_approval"`.

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9005consent",
  "message_type": "consent",
  "deal_id": "deal_01jc9002swap",
  "sender": "orp:party:pub_tsweekly",
  "timestamp": "2026-10-21T10:02:00Z",
  "idempotency_key": "idem_consent_01jc9005",
  "payload": {
    "consent_id": "cns_01jc9005owner",
    "principal_id": "orp:party:pub_tsweekly",
    "step": "space_execute",
    "mode_at_evaluation": "ask",
    "escalation_reasons": ["exceeds_approval_threshold", "step_ask"],
    "decision": "approve",
    "terms_hash": "sha256:4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a",
    "mandate_type": "explicit_owner_approval",
    "verification_proof": {
      "method": "jws_ed25519",
      "kid": "orp:party:pub_tsweekly#key-1",
      "signed_at": "2026-10-21T10:01:58Z",
      "jws": "eyJhbGciOiJFZERTQSI...<detached-jws>"
    },
    "expires_at": "2026-10-28T10:02:00Z"
  }
}
```

---

## 8. `execute` — Mint Universal Tracked Link / Token (& Optional Adapter Run)

`execute` issues the universal `tracking_url` (`/v1/go/:token`) and `tracking_token` (`orp_tok_...`) that work on any platform without a plugin.

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9006execute",
  "message_type": "execute",
  "deal_id": "deal_01jc9002swap",
  "sender": "orp:party:pub_tsweekly",
  "timestamp": "2026-10-22T12:00:00Z",
  "idempotency_key": "idem_exec_01jc9006",
  "payload": {
    "space_id": "spc_nl_typescript_weekly",
    "consent_id": "cns_01jc9005owner",
    "terms_hash": "sha256:4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a",
    "tracking_url": "https://offer-rails.com/v1/go/trk_01jc9002swap",
    "tracking_token": "orp_tok_01jc9002swap",
    "adapter_id": "orp-adapter-newsletter",
    "dry_run": false
  }
}
```

---

## 9. `receipt` — Universal Verifiable Receipt (`proof_url`, `public_check`, or `counterparty_attestation`)

Works for both link placements (`affiliate`, `cross_sell_upsell`, `link_exchange`, `traffic_swap`) and zero-click actions (`repost_retweet`) without requiring a platform plugin.

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9007receipt",
  "message_type": "receipt",
  "deal_id": "deal_01jc9002swap",
  "sender": "orp:party:pub_tsweekly",
  "timestamp": "2026-10-22T12:01:15Z",
  "idempotency_key": "idem_rcpt_01jc9007",
  "payload": {
    "receipt_id": "rcpt_01jc9007nl",
    "space_id": "spc_nl_typescript_weekly",
    "verification_mode": "public_check",
    "proof_url": "https://tsweekly.example.com/issues/149",
    "location_url": "https://tsweekly.example.com/issues/149",
    "tracking_token_found": true,
    "verified_at": "2026-10-22T12:01:14Z",
    "disclosure_verified": true,
    "observed_disclosure": "Sponsored partner: CodeLens AI (rel=\"sponsored\")",
    "content_hash": "sha256:7d865e959b2466918c9863afca942d0fb89d7c9ac0c99bafc3749504ded97730",
    "hold_until": "2026-10-29T12:01:14Z",
    "public_check_result": {
      "http_status": 200,
      "matched_target": "https://offer-rails.com/v1/go/trk_01jc9002swap",
      "checked_at": "2026-10-22T12:01:14Z"
    }
  }
}
```

---

## 10. `revoke` — Instant Deal Kill-Switch

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9008revoke",
  "message_type": "revoke",
  "deal_id": "deal_01jc9002swap",
  "sender": "orp:party:pub_tsweekly",
  "timestamp": "2026-10-23T15:10:00Z",
  "idempotency_key": "idem_rev_01jc9008",
  "payload": {
    "revoked_by": "orp:party:pub_tsweekly",
    "reason_code": "owner_kill_switch",
    "reason": "Landing page changed pricing claims after publication.",
    "adapter_cleanup_status": "removed",
    "redirect_disabled": true
  }
}
```

---

## 11. `dispute` — Evidence-Backed Breach Report

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9009dispute",
  "message_type": "dispute",
  "deal_id": "deal_01jc9002swap",
  "sender": "orp:party:pub_tsweekly",
  "timestamp": "2026-10-24T08:00:00Z",
  "idempotency_key": "idem_disp_01jc9009",
  "payload": {
    "dispute_id": "dsp_01jc9009earlydel",
    "reason_code": "premature_removal",
    "description": "Counterparty deleted reciprocal X repost after 6 hours instead of 7-day min_live_seconds.",
    "evidence": [
      {
        "type": "public_check_failure",
        "observed_at": "2026-10-24T07:55:00Z",
        "target_uri": "https://x.com/codelens/status/1848999999999",
        "http_status": 404
      }
    ],
    "requested_remedy": "refund_escrow_and_slash_karma"
  }
}
```

---

## 12. `reputation` — Dimensional Proof-of-Outcome Karma

```json
{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9010rep",
  "message_type": "reputation",
  "sender": "orp:party:relay_offer_rails",
  "timestamp": "2026-10-29T12:05:00Z",
  "payload": {
    "principal_id": "orp:party:pub_tsweekly",
    "calculated_at": "2026-10-29T12:05:00Z",
    "total_karma": 148,
    "dimensions": {
      "fulfillment_karma": 95,
      "conversion_karma": 42,
      "compliance_karma": 16,
      "dispute_penalty": -5
    },
    "stats": {
      "verified_deals_completed": 20,
      "unique_counterparties": 16,
      "fulfillment_rate_bps": 9850,
      "disputes_lost": 0,
      "median_hold_seconds": 604800
    },
    "recent_attestations": [
      {
        "deal_id": "deal_01jc9002swap",
        "counterparty_id": "orp:party:maker_codelens",
        "space_kind": "newsletter_slot",
        "settlement_mode": "space_swap",
        "receipt_id": "rcpt_01jc9007nl",
        "pair_deal_index": 1,
        "pair_decay_weight": 1.0,
        "karma_delta": 6,
        "settled_at": "2026-10-29T12:02:00Z"
      }
    ]
  }
}
```
