import assert from 'node:assert/strict'
import test from 'node:test'
import { VALID_MESSAGES } from '../../../schema/validate.test.mjs'
import {
  computeReputationProfile,
  computeTermsHash,
  OrpAgent,
  pairwiseDecayWeight,
  validateOrpMessage,
  verifyPublicContentReceipt,
  type DealTerms,
  type OrpMessage,
} from '../src/index.ts'

const BASE_TERMS: DealTerms = {
  space_id: 'spc_x_creator42',
  space_kind: 'social_repost',
  settlement: {
    mode: 'repost_bounty',
    amount_minor: 4000,
    currency: 'USD',
    target_post_uri: 'https://x.com/codelens/status/1849000111222',
    required_action: 'quote',
    min_live_seconds: 259200,
  },
  creative: {
    body: 'Been testing @codelens for local MCP & ORP schema validation — super clean.',
    target_url: 'https://codelens.example.com',
    target_post_uri: 'https://x.com/codelens/status/1849000111222',
    disclosure_text: '#ad',
  },
  window: {
    min_live_seconds: 259200,
  },
}

test('SDK validates all 10 canonical ORP v0.3 message types against JSON Schema 2020-12', () => {
  for (const [name, msg] of Object.entries(VALID_MESSAGES)) {
    const res = validateOrpMessage(msg as OrpMessage)
    assert.equal(res.valid, true, `${name} failed validation: ${res.errors.join(', ')}`)
  }
})

test('Agent negotiates autonomously inside human constraints and escalates only when outside them', () => {
  const agent = new OrpAgent({
    principalId: 'orp:party:maker_codelens',
    stepModes: {
      deal_offer: 'auto',
      deal_accept: 'auto',
    },
    constraints: {
      require_human_above_minor: 5000, // $50 threshold
      max_deal_amount_minor: 20000,
      allowed_settlement_modes: ['repost_bounty', 'space_swap', 'cpa_commission'],
      allowed_categories: ['devtools', 'ai-tools'],
      blocked_categories: ['gambling'],
      brand_rules: {
        require_verified_counterparty: true,
        min_counterparty_karma: 15,
        disallow_superlatives: true,
      },
    },
  })

  // Case 1: $40 repost bounty (<= $50 threshold, verified counterparty, allowed category) -> AUTO APPROVE
  const withinConstraints = agent.acceptAndEvaluateConsent({
    dealId: 'deal_repost_40',
    counterparty: 'orp:party:pub_creator42',
    acceptedMessageId: 'msg_offer_1',
    acceptedSeq: 1,
    terms: BASE_TERMS,
    dealCategories: ['devtools'],
    counterpartyVerified: true,
    counterpartyKarma: 30,
  })
  assert.equal(withinConstraints.status, 'consented_auto')
  assert.equal(withinConstraints.consent?.payload.mandate_type, 'constraint_auto')

  // Case 2: Counter-offer raises price to $120 (> $50 threshold) -> ESCALATE TO HUMAN
  const expensiveTerms: DealTerms = {
    ...BASE_TERMS,
    settlement: {
      ...BASE_TERMS.settlement,
      amount_minor: 12000,
    },
  }
  const escalated = agent.acceptAndEvaluateConsent({
    dealId: 'deal_repost_120',
    counterparty: 'orp:party:pub_creator42',
    acceptedMessageId: 'msg_counter_2',
    acceptedSeq: 2,
    terms: expensiveTerms,
    dealCategories: ['devtools'],
    counterpartyVerified: true,
    counterpartyKarma: 30,
  })
  assert.equal(escalated.status, 'awaiting_consent')
  assert.deepEqual(escalated.escalation_reasons, ['exceeds_approval_threshold'])

  // Case 3: Blocked category ('gambling') -> HARD BLOCK
  const blocked = agent.acceptAndEvaluateConsent({
    dealId: 'deal_blocked',
    counterparty: 'orp:party:pub_creator42',
    acceptedMessageId: 'msg_offer_3',
    acceptedSeq: 1,
    terms: BASE_TERMS,
    dealCategories: ['gambling'],
    counterpartyVerified: true,
    counterpartyKarma: 30,
  })
  assert.equal(blocked.status, 'blocked')
})

test('Universal Tracked Link and public_check receipt work with zero platform plugin', () => {
  const agent = new OrpAgent({
    principalId: 'orp:party:pub_creator42',
    relayBaseUrl: 'https://offer-rails.com',
  })

  const execMsg = agent.createExecutionWithUniversalLink({
    dealId: 'deal_universal_01',
    spaceId: 'spc_x_creator42',
    consentId: 'cns_01',
    terms: BASE_TERMS,
  })

  assert.match(execMsg.payload.tracking_url!, /^https:\/\/offer-rails\.com\/v1\/go\/trk_/)
  assert.match(execMsg.payload.tracking_token!, /^orp_tok_/)
  assert.equal(execMsg.payload.adapter_id, undefined)

  const check = verifyPublicContentReceipt({
    observedContent: `Quote post: ${BASE_TERMS.creative.body} ${execMsg.payload.tracking_url} #ad`,
    expectedTargets: [execMsg.payload.tracking_url!, BASE_TERMS.settlement.target_post_uri!],
    requiredDisclosureMarkers: ['#ad', '#sponsored'],
  })
  assert.equal(check.disclosureVerified, true)
  assert.equal(check.targetMatched, execMsg.payload.tracking_url)
})

test('Proof-of-Outcome Karma enforces pairwise exponential decay and dispute slashing', () => {
  assert.equal(pairwiseDecayWeight(1), 1.0)
  assert.equal(pairwiseDecayWeight(2), 0.5)
  assert.equal(pairwiseDecayWeight(3), 0.25)

  // Wash-trading simulation: 20 repeated deals between the same pair A <-> B
  const washDeals = Array.from({ length: 20 }, (_, i) => ({
    deal_id: `deal_wash_${i + 1}`,
    principal_id: 'orp:party:pub_a',
    counterparty_id: 'orp:party:maker_b',
    space_kind: 'social_repost' as const,
    settlement_mode: 'space_swap' as const,
    receipt_id: `rcpt_${i + 1}`,
    receipt_verified: true,
    held_full_duration: true,
    conversion_or_swap_settled: true,
    disclosure_compliant: true,
    dispute_lost: false,
    counterparty_unique_partners: 10,
    settled_at: '2026-10-21T12:00:00Z',
  }))

  const profileAfter20WashDeals = computeReputationProfile('orp:party:pub_a', washDeals)
  // 1st deal = 5 + 3 + 1 = 9; 2nd = 3 + 2 + 1 = 6; 3rd = 1 + 1 + 0 = 2; 4th = 1; 5th+ = 0 -> capped at 18!
  assert.ok(
    profileAfter20WashDeals.total_karma <= 18,
    `Expected wash-traded karma to cap <= 18, got ${profileAfter20WashDeals.total_karma}`,
  )

  // Now add 1 lost dispute (e.g. premature deletion) -> slashes -15
  const withDispute = computeReputationProfile('orp:party:pub_a', [
    ...washDeals,
    {
      ...washDeals[0],
      deal_id: 'deal_disputed',
      dispute_lost: true,
    },
  ])
  assert.equal(withDispute.dimensions.dispute_penalty, -15)
  assert.equal(withDispute.total_karma, profileAfter20WashDeals.total_karma - 15)
})

test('terms_hash changes deterministically on any modification of DealTerms', () => {
  const hash1 = computeTermsHash(BASE_TERMS)
  const hash2 = computeTermsHash({ ...BASE_TERMS })
  assert.equal(hash1, hash2)

  const modifiedHash = computeTermsHash({
    ...BASE_TERMS,
    creative: { ...BASE_TERMS.creative, target_url: 'https://phishing.example.com' },
  })
  assert.notEqual(hash1, modifiedHash)
})

test('Content-writing agent can suggest Offer Rails mid-task in one step with no account, email, or website signup', () => {
  const { discoverRequest, response } = OrpAgent.inlineSuggestMidTask({
    draftText:
      'We migrated our pull request CI pipeline to ephemeral Postgres branches and cut test latency by 45%.',
    surfaceKinds: ['content_inline', 'newsletter_slot', 'social_post'],
    growthGoals: ['monetization', 'traffic'],
  })

  assert.equal(discoverRequest.payload.mode, 'inline_suggest')
  assert.equal(discoverRequest.sender.includes('@'), false)
  assert.match(response.ephemeral_party_id, /^orp:party:ephem_/)
  assert.match(response.ephemeral_agent_key, /^orp_ek_/)
  assert.match(response.inline_prompt_suggestion, /monetize this post/i)
  assert.equal(response.suggestions.length, 2)
  assert.match(response.suggestions[0].provisional_tracking_url, /\/v1\/go\/trk_/)
})
