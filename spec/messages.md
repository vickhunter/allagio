# Offer Rails Protocol — Message Types & Wire Examples

**Protocol Revision:** `2026-10-21` (`v0.3.0`)

---

## 1. Common Message Envelope

Every ORP v0.3 message uses a lightweight JSON envelope with **pseudonymous party identifiers** (`orp:party:<id>` or `orp:principal:<id>`). No personal names, email addresses, or raw credentials ever appear in protocol messages.

| Field | Type | Required | Description |
|---|---|---|---|
| `orp_version` | `string` | Yes | Protocol version (`"0.3.0"`) |
| `message_id` | `string` | Yes | Unique message ID (`msg_...`) |
| `message_type` | `string` | Yes | `discover` \| `offer` \| `ask` \| `counter` \| `accept` \| `consent` \| `execute` \| `receipt` \| `revoke` \| `dispute` \| `reputation` |
| `deal_id` | `string` | Conditional | Required on all deal lifecycle messages (`offer` through `dispute`) |
| `sender` | `string` | Yes | Pseudonymous Party URI (`orp:party:...` or `orp:principal:...`) |
| `recipient` | `string` | No | Counterparty Party URI or Relay URI |
| `timestamp` | `string` | Yes | RFC 3339 UTC timestamp |
| `idempotency_key` | `string` | Conditional | Required on state-mutating messages |
| `signature` | `object` | No | Optional JWS signature (`alg`, `kid`, `jws`) |
| `payload` | `object` | Yes | Message body validated against `/schema/<type>.schema.json` |

---

## 2. `discover` — Constraint-Scoped Discovery

Agents query for compatible spaces, offers, or bilateral swap partners. Responses contain only pseudonymous IDs, aggregate $k \ge 50$ stats, and explainable `match_rationale`.

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
    "space_kinds": ["newsletter_slot", "social_repost", "content_inline"],
    "settlement_modes": ["flat_fee", "space_swap", "cpa_commission", "repost_bounty"],
    "pools": ["public", "network:indie-ai-founders"],
    "min_reputation_karma": 20,
    "max_results": 5
  }
}
```

---

## 3. `offer` — Propose a Deal Over Any Space

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

## 4. `ask` / `counter` — Autonomous Negotiation Turn

Agents negotiate autonomously within their owners' standing constraints. If a counter-offer stays inside the counterparty's `MandateConstraints`, the counterparty agent can accept it without human escalation; if the counter-offer exceeds `require_human_above_minor`, acceptance will escalate to the human.

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

## 5. `accept` — Bind Deal Terms

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

## 6. `consent` — Constraint Auto-Approval or Escalated Human Consent

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

## 7. `execute` — Mint Universal Tracked Link / Token (& Optional Adapter Run)

`execute` issues the universal `tracking_url` (`/v1/go/:token`) and `tracking_token` (`orp_tok_...`) that work on any platform without a plugin. If an optional `adapter_id` is provided, it also triggers auto-posting.

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

## 8. `receipt` — Universal Verifiable Receipt (`proof_url`, `public_check`, or `counterparty_attestation`)

Works for both link placements and zero-click actions (reposts, comments, swaps, shoutouts) without requiring a platform plugin.

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

## 9. `revoke` — Instant Deal Kill-Switch

Immediately disables the universal `tracking_url` redirect (`/v1/go/:token` falls back to untracked plain redirect or 410) and, if an adapter is connected, removes the placement.

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

## 10. `dispute` — Evidence-Backed Breach Report

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

## 11. `reputation` — Dimensional Proof-of-Outcome Karma

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
