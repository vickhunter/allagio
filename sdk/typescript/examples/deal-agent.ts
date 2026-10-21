import {
  computeReputationProfile,
  computeSha256Hash,
  computeTermsHash,
  OrpAgent,
  verifyPublicContentReceipt,
  type ConsentMessage,
  type DealTerms,
  type ReceiptMessage,
} from '../src/index.ts'

// 1. Maker Agent (@maker_codelens) and Publisher Agent (@pub_tsweekly)
//    Both use pseudonymous party IDs and human-set constraints.
const makerAgent = new OrpAgent({
  principalId: 'orp:party:maker_codelens',
  relayBaseUrl: 'https://offer-rails.com',
  stepModes: {
    deal_offer: 'auto',
    deal_accept: 'auto',
    space_execute: 'auto',
  },
  constraints: {
    max_deal_amount_minor: 50000,
    require_human_above_minor: 10000, // Auto-approve up to $100; escalate above $100
    currency: 'USD',
    allowed_settlement_modes: ['space_swap', 'flat_fee', 'cpa_commission', 'repost_bounty'],
    allowed_categories: ['typescript', 'devtools', 'ai-tools'],
    blocked_categories: ['gambling', 'adult'],
    brand_rules: {
      require_verified_counterparty: true,
      min_counterparty_karma: 10,
      disallow_superlatives: true,
    },
  },
})

const publisherAgent = new OrpAgent({
  principalId: 'orp:party:pub_tsweekly',
  relayBaseUrl: 'https://offer-rails.com',
  stepModes: {
    deal_offer: 'auto',
    deal_accept: 'auto',
    space_execute: 'ask', // New space owner default: ask before publishing
  },
})

// 2. Maker proposes a space-for-space swap (Newsletter slot <-> X repost)
const initialTerms: DealTerms = {
  space_id: 'spc_nl_typescript_weekly',
  space_kind: 'newsletter_slot',
  settlement: {
    mode: 'space_swap',
    initiator_space_id: 'spc_x_codelens',
    responder_space_id: 'spc_nl_typescript_weekly',
    sync_window_seconds: 86400,
    min_live_seconds: 604800,
  },
  creative: {
    headline: 'CodeLens AI — Local MCP & ORP Debugger',
    body: 'Trace and replay agent protocol messages in VS Code.',
    target_url: 'https://codelens.example.com',
    disclosure_text: 'Sponsored partner swap',
    rel_attributes: ['sponsored'],
  },
  reciprocal_creative: {
    headline: 'TypeScript Weekly Dispatch',
    body: '25,000+ engineers reading practical AI agent deep dives.',
    target_url: 'https://tsweekly.example.com',
    disclosure_text: '#ad #partner',
  },
  window: {
    min_live_seconds: 604800,
  },
}

const offer = makerAgent.createOffer({
  dealId: 'deal_demo_01',
  recipient: publisherAgent.principalId,
  terms: initialTerms,
  rationale: 'High TypeScript developer overlap (k>=50 aggregate card).',
  expiresAt: '2026-10-22T12:00:00Z',
})

// 3. Publisher Agent counters with a $50 topper (5000 minor units <= Maker's $100 auto threshold!)
const counterUnderThresholdTerms: DealTerms = {
  ...initialTerms,
  settlement: {
    ...initialTerms.settlement,
    topper_amount_minor: 5000,
    currency: 'USD',
  },
}

const counter1 = publisherAgent.createCounter({
  dealId: offer.deal_id,
  recipient: makerAgent.principalId,
  seq: 2,
  priorMessageId: offer.message_id,
  reason: 'Add $50 topper to balance subscriber reach.',
  terms: counterUnderThresholdTerms,
  expiresAt: '2026-10-22T12:00:00Z',
})

// 4. Maker Agent evaluates Counter #1 ($50 <= $100 threshold) -> AUTONOMOUS APPROVAL!
const autoDecision = makerAgent.acceptAndEvaluateConsent({
  dealId: offer.deal_id,
  counterparty: publisherAgent.principalId,
  acceptedMessageId: counter1.message_id,
  acceptedSeq: counter1.payload.seq,
  terms: counter1.payload.terms,
  dealCategories: ['typescript', 'devtools'],
  counterpartyVerified: true,
  counterpartyKarma: 85,
})

// 5. Execution mints the Universal Tracked Link (/v1/go/:token) — works anywhere with ZERO plugin!
const execution = makerAgent.createExecutionWithUniversalLink({
  dealId: offer.deal_id,
  spaceId: counter1.payload.terms.space_id,
  consentId: autoDecision.consent!.payload.consent_id,
  terms: counter1.payload.terms,
})

// 6. Verify receipt via unauthenticated `public_check` on the published newsletter archive HTML
const publishedHtml = `
  <article>
    <span class="badge">Sponsored partner swap</span>
    <h4>CodeLens AI — Local MCP & ORP Debugger</h4>
    <p>Trace and replay agent protocol messages in VS Code.
      <a href="${execution.payload.tracking_url}" rel="sponsored">Try CodeLens</a>
    </p>
  </article>
`
const check = verifyPublicContentReceipt({
  observedContent: publishedHtml,
  expectedTargets: [execution.payload.tracking_url!, execution.payload.tracking_token!],
  requiredDisclosureMarkers: ['Sponsored', 'rel="sponsored"'],
})

const receipt: ReceiptMessage = {
  orp_version: '0.3.0',
  message_id: 'msg_rcpt_demo_01',
  message_type: 'receipt',
  deal_id: offer.deal_id,
  sender: publisherAgent.principalId,
  timestamp: new Date().toISOString(),
  idempotency_key: 'idem_rcpt_demo_01',
  payload: {
    receipt_id: 'rcpt_demo_01',
    space_id: 'spc_nl_typescript_weekly',
    verification_mode: 'public_check',
    proof_url: 'https://tsweekly.example.com/issues/149',
    location_url: 'https://tsweekly.example.com/issues/149',
    tracking_token_found: check.targetMatched !== null,
    verified_at: new Date().toISOString(),
    disclosure_verified: true,
    observed_disclosure: check.observedDisclosure!,
    content_hash: check.contentHash,
    public_check_result: {
      http_status: 200,
      matched_target: check.targetMatched!,
      checked_at: new Date().toISOString(),
    },
  },
}
publisherAgent.assertValid(receipt)

// 7. Compute Proof-of-Outcome Karma after completion
const rep = computeReputationProfile(publisherAgent.principalId, [
  {
    deal_id: offer.deal_id,
    principal_id: publisherAgent.principalId,
    counterparty_id: makerAgent.principalId,
    space_kind: 'newsletter_slot',
    settlement_mode: 'space_swap',
    receipt_id: receipt.payload.receipt_id,
    receipt_verified: true,
    held_full_duration: true,
    conversion_or_swap_settled: true,
    disclosure_compliant: true,
    dispute_lost: false,
    counterparty_unique_partners: 10,
    settled_at: new Date().toISOString(),
  },
])

console.log(
  JSON.stringify(
    {
      deal_id: offer.deal_id,
      terms_hash: computeTermsHash(counter1.payload.terms),
      autonomous_decision: autoDecision.status,
      universal_tracking_url: execution.payload.tracking_url,
      universal_tracking_token: execution.payload.tracking_token,
      receipt_verification_mode: receipt.payload.verification_mode,
      karma_after_deal: rep.total_karma,
    },
    null,
    2,
  ),
)
