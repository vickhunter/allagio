import assert from 'node:assert/strict'
import test from 'node:test'
import { DRAFT_2020_12, loadSchemas, validateWithSchema } from './validator.mjs'

const registry = loadSchemas()

const SAMPLE_TERMS_HASH = 'sha256:4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a'
const SAMPLE_CONTENT_HASH = 'sha256:7d865e959b2466918c9863afca942d0fb89d7c9ac0c99bafc3749504ded97730'

const SAMPLE_TERMS = {
  space_id: 'spc_nl_typescript_weekly',
  space_kind: 'newsletter_slot',
  settlement: {
    mode: 'space_swap',
    initiator_space_id: 'spc_x_codelens_feed',
    responder_space_id: 'spc_nl_typescript_weekly',
    sync_window_seconds: 86400,
    min_live_seconds: 604800,
  },
  creative: {
    headline: 'CodeLens AI — Schema-checked MCP & ORP debugging',
    body: 'Inspect and replay agent tool calls locally with zero config.',
    target_url: 'https://codelens.example.com',
    disclosure_text: 'Sponsored partner swap',
    rel_attributes: ['sponsored'],
  },
  reciprocal_creative: {
    headline: 'TypeScript Weekly Dispatch',
    body: 'Deep-dive TypeScript & agent engineering tutorials every Thursday.',
    target_url: 'https://tsweekly.example.com',
    disclosure_text: '#ad #partner',
  },
  window: {
    execute_after: '2026-10-22T08:00:00Z',
    execute_before: '2026-10-25T20:00:00Z',
    min_live_seconds: 604800,
  },
}

export const VALID_MESSAGES = {
  discover: {
    orp_version: '0.3.0',
    message_id: 'msg_01jc9001discover',
    message_type: 'discover',
    sender: 'orp:party:maker_codelens',
    timestamp: '2026-10-21T09:30:00Z',
    payload: {
      target_type: 'space',
      query: 'TypeScript and AI engineering newsletters or X accounts',
      space_kinds: ['newsletter_slot', 'social_repost', 'content_inline'],
      settlement_modes: ['flat_fee', 'space_swap', 'cpa_commission', 'repost_bounty'],
      pools: ['public', 'network:indie-ai-founders'],
      min_reputation_karma: 20,
      max_results: 5,
    },
  },
  offer: {
    orp_version: '0.3.0',
    message_id: 'msg_01jc9002offer',
    message_type: 'offer',
    deal_id: 'deal_01jc9002swap',
    sender: 'orp:party:maker_codelens',
    recipient: 'orp:party:pub_tsweekly',
    timestamp: '2026-10-21T09:35:00Z',
    idempotency_key: 'idem_offer_01jc9002swap',
    payload: {
      seq: 1,
      terms: SAMPLE_TERMS,
      terms_hash: SAMPLE_TERMS_HASH,
      expires_at: '2026-10-22T09:35:00Z',
    },
  },
  counter: {
    orp_version: '0.3.0',
    message_id: 'msg_01jc9003counter',
    message_type: 'counter',
    deal_id: 'deal_01jc9002swap',
    sender: 'orp:party:pub_tsweekly',
    recipient: 'orp:party:maker_codelens',
    timestamp: '2026-10-21T09:42:00Z',
    idempotency_key: 'idem_counter_01jc9003',
    payload: {
      seq: 2,
      prior_message_id: 'msg_01jc9002offer',
      reason: 'Adding $150 topper to balance newsletter subscriber reach.',
      terms: {
        ...SAMPLE_TERMS,
        settlement: {
          ...SAMPLE_TERMS.settlement,
          topper_amount_minor: 15000,
          currency: 'USD',
        },
      },
      terms_hash: SAMPLE_TERMS_HASH,
      expires_at: '2026-10-22T09:42:00Z',
    },
  },
  accept: {
    orp_version: '0.3.0',
    message_id: 'msg_01jc9004accept',
    message_type: 'accept',
    deal_id: 'deal_01jc9002swap',
    sender: 'orp:party:maker_codelens',
    recipient: 'orp:party:pub_tsweekly',
    timestamp: '2026-10-21T09:45:00Z',
    idempotency_key: 'idem_accept_01jc9004',
    payload: {
      accepted_message_id: 'msg_01jc9003counter',
      accepted_seq: 2,
      terms_hash: SAMPLE_TERMS_HASH,
    },
  },
  consent: {
    orp_version: '0.3.0',
    message_id: 'msg_01jc9005consent',
    message_type: 'consent',
    deal_id: 'deal_01jc9002swap',
    sender: 'orp:party:pub_tsweekly',
    timestamp: '2026-10-21T10:02:00Z',
    idempotency_key: 'idem_consent_01jc9005',
    payload: {
      consent_id: 'cns_01jc9005owner',
      principal_id: 'orp:party:pub_tsweekly',
      step: 'space_execute',
      mode_at_evaluation: 'ask',
      escalation_reasons: ['exceeds_approval_threshold', 'step_ask'],
      decision: 'approve',
      terms_hash: SAMPLE_TERMS_HASH,
      mandate_type: 'explicit_owner_approval',
      verification_proof: {
        method: 'jws_ed25519',
        kid: 'orp:party:pub_tsweekly#key-1',
        signed_at: '2026-10-21T10:01:58Z',
        jws: 'eyJhbGciOiJFZERTQSJ9..detachedSignatureBytes',
      },
      expires_at: '2026-10-28T10:02:00Z',
    },
  },
  execute: {
    orp_version: '0.3.0',
    message_id: 'msg_01jc9006execute',
    message_type: 'execute',
    deal_id: 'deal_01jc9002swap',
    sender: 'orp:party:pub_tsweekly',
    timestamp: '2026-10-22T12:00:00Z',
    idempotency_key: 'idem_exec_01jc9006',
    payload: {
      space_id: 'spc_nl_typescript_weekly',
      consent_id: 'cns_01jc9005owner',
      terms_hash: SAMPLE_TERMS_HASH,
      tracking_url: 'https://offer-rails.com/v1/go/trk_01jc9002swap',
      tracking_token: 'orp_tok_01jc9002swap',
      dry_run: false,
    },
  },
  receipt: {
    orp_version: '0.3.0',
    message_id: 'msg_01jc9007receipt',
    message_type: 'receipt',
    deal_id: 'deal_01jc9002swap',
    sender: 'orp:party:pub_tsweekly',
    timestamp: '2026-10-22T12:01:15Z',
    idempotency_key: 'idem_rcpt_01jc9007',
    payload: {
      receipt_id: 'rcpt_01jc9007nl',
      space_id: 'spc_nl_typescript_weekly',
      verification_mode: 'public_check',
      proof_url: 'https://tsweekly.example.com/issues/149',
      location_url: 'https://tsweekly.example.com/issues/149',
      tracking_token_found: true,
      verified_at: '2026-10-22T12:01:14Z',
      disclosure_verified: true,
      observed_disclosure: 'Sponsored partner: CodeLens AI (rel="sponsored")',
      content_hash: SAMPLE_CONTENT_HASH,
      hold_until: '2026-10-29T12:01:14Z',
      public_check_result: {
        http_status: 200,
        matched_target: 'https://offer-rails.com/v1/go/trk_01jc9002swap',
        checked_at: '2026-10-22T12:01:14Z',
      },
    },
  },
  revoke: {
    orp_version: '0.3.0',
    message_id: 'msg_01jc9008revoke',
    message_type: 'revoke',
    deal_id: 'deal_01jc9002swap',
    sender: 'orp:party:pub_tsweekly',
    timestamp: '2026-10-23T15:10:00Z',
    idempotency_key: 'idem_rev_01jc9008',
    payload: {
      revoked_by: 'orp:party:pub_tsweekly',
      reason_code: 'owner_kill_switch',
      reason: 'Landing page changed pricing claims after publication.',
      adapter_cleanup_status: 'removed',
      redirect_disabled: true,
    },
  },
  dispute: {
    orp_version: '0.3.0',
    message_id: 'msg_01jc9009dispute',
    message_type: 'dispute',
    deal_id: 'deal_01jc9002swap',
    sender: 'orp:party:pub_tsweekly',
    timestamp: '2026-10-24T08:00:00Z',
    idempotency_key: 'idem_disp_01jc9009',
    payload: {
      dispute_id: 'dsp_01jc9009earlydel',
      reason_code: 'premature_removal',
      description: 'Counterparty deleted reciprocal X repost after 6 hours instead of 7-day min_live_seconds.',
      evidence: [
        {
          type: 'public_check_failure',
          observed_at: '2026-10-24T07:55:00Z',
          target_uri: 'https://x.com/codelens/status/1848999999999',
          http_status: 404,
        },
      ],
      requested_remedy: 'refund_escrow_and_slash_karma',
    },
  },
  reputation: {
    orp_version: '0.3.0',
    message_id: 'msg_01jc9010rep',
    message_type: 'reputation',
    sender: 'orp:party:relay_offer_rails',
    timestamp: '2026-10-29T12:05:00Z',
    payload: {
      principal_id: 'orp:party:pub_tsweekly',
      calculated_at: '2026-10-29T12:05:00Z',
      total_karma: 148,
      dimensions: {
        fulfillment_karma: 95,
        conversion_karma: 42,
        compliance_karma: 16,
        dispute_penalty: -5,
      },
      stats: {
        verified_deals_completed: 20,
        unique_counterparties: 16,
        fulfillment_rate_bps: 9850,
        disputes_lost: 0,
        median_hold_seconds: 604800,
      },
      recent_attestations: [
        {
          deal_id: 'deal_01jc9002swap',
          counterparty_id: 'orp:party:maker_codelens',
          space_kind: 'newsletter_slot',
          settlement_mode: 'space_swap',
          receipt_id: 'rcpt_01jc9007nl',
          pair_deal_index: 1,
          pair_decay_weight: 1.0,
          karma_delta: 6,
          settled_at: '2026-10-29T12:02:00Z',
        },
      ],
    },
  },
}

test('all 12 schema files declare JSON Schema Draft 2020-12 and valid $id', () => {
  const expectedFiles = [
    'common.schema.json',
    'discover.schema.json',
    'offer.schema.json',
    'counter.schema.json',
    'accept.schema.json',
    'consent.schema.json',
    'execute.schema.json',
    'receipt.schema.json',
    'revoke.schema.json',
    'dispute.schema.json',
    'reputation.schema.json',
    'orp-message.schema.json',
  ]
  for (const file of expectedFiles) {
    const s = registry.get(file)
    assert.ok(s, `Missing schema file ${file}`)
    assert.equal(s.$schema, DRAFT_2020_12, `${file} must use Draft 2020-12`)
    assert.ok(s.$id && s.$id.startsWith('https://offer-rails.com/schema/v0.3/'), `${file} must declare canonical $id`)
  }
})

test('every canonical message validates against its dedicated schema and the union orp-message.schema.json', () => {
  for (const [type, msg] of Object.entries(VALID_MESSAGES)) {
    const schemaFile = `${type}.schema.json`
    const res = validateWithSchema(msg, schemaFile, registry)
    assert.equal(res.valid, true, `${type} failed ${schemaFile}: ${res.errors.join(', ')}`)

    const unionRes = validateWithSchema(msg, 'orp-message.schema.json', registry)
    assert.equal(unionRes.valid, true, `${type} failed union schema: ${unionRes.errors.join(', ')}`)
  }
})

test('ask message validates against counter.schema.json and union schema', () => {
  const askMsg = {
    ...VALID_MESSAGES.counter,
    message_id: 'msg_01jc9003ask',
    message_type: 'ask',
    payload: {
      seq: 1,
      reason: 'Booking open for next Thursday primary newsletter slot.',
      terms: SAMPLE_TERMS,
      terms_hash: SAMPLE_TERMS_HASH,
      expires_at: '2026-10-22T09:42:00Z',
    },
  }
  assert.equal(validateWithSchema(askMsg, 'counter.schema.json', registry).valid, true)
  assert.equal(validateWithSchema(askMsg, 'orp-message.schema.json', registry).valid, true)
})

test('zero-click receipt with counterparty_attestation validates without any platform adapter_id', () => {
  const zeroClickReceipt = {
    ...VALID_MESSAGES.receipt,
    payload: {
      receipt_id: 'rcpt_repost_01',
      space_id: 'spc_x_creator42',
      verification_mode: 'counterparty_attestation',
      proof_url: 'https://x.com/creator42/status/1849111222333',
      location_url: 'https://x.com/creator42/status/1849111222333',
      verified_at: '2026-10-22T15:00:00Z',
      disclosure_verified: true,
      observed_disclosure: '#ad',
      content_hash: SAMPLE_CONTENT_HASH,
      counterparty_attestation: {
        attested_by: 'orp:party:maker_codelens',
        attested_at: '2026-10-22T15:00:00Z',
        statement: 'Verified quote-post is live with #ad disclosure.',
      },
    },
  }
  const res = validateWithSchema(zeroClickReceipt, 'receipt.schema.json', registry)
  assert.equal(res.valid, true, res.errors.join(', '))
})

test('rejects privacy violations, PII in party IDs, k_anonymity < 50, and missing disclosures', () => {
  // 1. Party ID containing an email address (@) must be rejected
  const emailParty = {
    ...VALID_MESSAGES.discover,
    sender: 'alice@example.com',
  }
  assert.equal(validateWithSchema(emailParty, 'discover.schema.json', registry).valid, false)

  // 2. AudienceCard with k_anonymity_threshold < 50 or raw PII field must be rejected
  const spaceSchema = {
    $ref: 'common.schema.json#/$defs/Space',
  }
  const badSpaceLowK = {
    space_id: 'spc_bad_k',
    principal_id: 'orp:party:pub_1',
    platform: 'newsletter',
    kind: 'newsletter_slot',
    uri: 'https://newsletter.example.com',
    title: 'Newsletter',
    verification: { status: 'verified', method: 'public_check' },
    accepted_settlement_modes: ['flat_fee'],
    audience_card: {
      k_anonymity_threshold: 10,
      reach_bucket: '1k_10k',
      provenance: 'self_declared',
    },
  }
  assert.equal(validateWithSchema(badSpaceLowK, spaceSchema, registry).valid, false)

  const badSpacePII = {
    ...badSpaceLowK,
    audience_card: {
      k_anonymity_threshold: 50,
      reach_bucket: '1k_10k',
      provenance: 'self_declared',
      subscriber_emails: ['reader@example.com'],
    },
  }
  assert.equal(validateWithSchema(badSpacePII, spaceSchema, registry).valid, false)

  // 3. Receipt with disclosure_verified: false must be rejected
  const undisclosedReceipt = {
    ...VALID_MESSAGES.receipt,
    payload: {
      ...VALID_MESSAGES.receipt.payload,
      disclosure_verified: false,
    },
  }
  assert.equal(validateWithSchema(undisclosedReceipt, 'receipt.schema.json', registry).valid, false)

  // 4. Forbidden chat_completion_injection space kind must be rejected
  const forbiddenSpaceKind = {
    ...VALID_MESSAGES.offer,
    payload: {
      ...VALID_MESSAGES.offer.payload,
      terms: {
        ...SAMPLE_TERMS,
        space_kind: 'chat_completion_injection',
      },
    },
  }
  assert.equal(validateWithSchema(forbiddenSpaceKind, 'offer.schema.json', registry).valid, false)
})
