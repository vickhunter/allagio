export * from './types.ts'
export * from './crypto.ts'
export * from './constraints.ts'
export * from './reputation.ts'
export * from './validator.ts'

import { evaluateDealConstraints } from './constraints.ts'
import { computeTermsHash, mintUniversalTracking } from './crypto.ts'
import type {
  AcceptMessage,
  ConsentMessage,
  CounterMessage,
  DealTerms,
  DealType,
  DiscoverMessage,
  ExecuteMessage,
  GrowthGoal,
  InlineSuggestResponse,
  MandateConstraints,
  OfferMessage,
  OrpMessage,
  PermissionStep,
  PrincipalUri,
  ReceiptMessage,
  RevokeMessage,
  SpaceKind,
  StepMode,
} from './types.ts'
import { validateOrpMessage } from './validator.ts'

export interface OrpAgentOptions {
  principalId: PrincipalUri
  relayBaseUrl?: string
  stepModes?: Partial<Record<PermissionStep, StepMode>>
  constraints?: MandateConstraints
}

/**
 * Lightweight Offer Rails Protocol v0.3 Agent SDK.
 * Autonomous inside human constraints; escalates to the human only when outside them.
 */
export class OrpAgent {
  readonly principalId: PrincipalUri
  readonly relayBaseUrl: string
  readonly stepModes: Partial<Record<PermissionStep, StepMode>>
  readonly constraints: MandateConstraints

  constructor(options: OrpAgentOptions) {
    this.principalId = options.principalId
    this.relayBaseUrl = options.relayBaseUrl ?? 'https://offer-rails.com'
    this.stepModes = {
      space_register: 'auto',
      discover_respond: 'auto',
      deal_offer: 'auto',
      deal_accept: 'auto',
      space_execute: 'ask',
      deal_modify: 'ask',
      settlement_payout: 'auto',
      ...options.stepModes,
    }
    this.constraints = options.constraints ?? {}
  }

  /**
   * 1-Step Mid-Task Inline Discovery & Suggestion (`C2-NO-EMAIL-NO-SIGNUP`).
   * Any content-writing agent (blog, social, newsletter) can call this in a single step
   * with NO account, NO email address, and NO website signup.
   */
  static inlineSuggestMidTask(params: {
    draftText: string
    surfaceKinds?: SpaceKind[]
    growthGoals?: GrowthGoal[]
    dealTypes?: DealType[]
    relayBaseUrl?: string
    now?: string
  }): {
    discoverRequest: DiscoverMessage
    response: InlineSuggestResponse
  } {
    const now = params.now ?? new Date().toISOString()
    const relayBaseUrl = params.relayBaseUrl ?? 'https://offer-rails.com'
    const seed = computeTermsHash({ draftText: params.draftText, now }).slice(7, 17)
    const ephemeralPartyId: PrincipalUri = `orp:party:ephem_${seed}`
    const growthGoals: GrowthGoal[] = params.growthGoals ?? ['monetization', 'traffic']
    const dealTypes: DealType[] = params.dealTypes ?? [
      'affiliate',
      'cross_sell_upsell',
      'link_exchange',
      'traffic_swap',
      'repost_retweet',
    ]

    const discoverRequest: DiscoverMessage = {
      orp_version: '0.3.0',
      message_id: `msg_inline_${seed}`,
      message_type: 'discover',
      sender: ephemeralPartyId,
      timestamp: now,
      payload: {
        mode: 'inline_suggest',
        target_type: 'offer',
        draft_text: params.draftText,
        growth_goals: growthGoals,
        deal_types: dealTypes,
        ...(params.surfaceKinds ? { surface_kinds: params.surfaceKinds } : {}),
        max_results: 3,
      },
    }
    const validation = validateOrpMessage(discoverRequest)
    if (!validation.valid) {
      throw new Error(`Invalid inline discover message: ${validation.errors.join('; ')}`)
    }

    const affTrack = mintUniversalTracking(relayBaseUrl, `inline_aff_${seed}`)
    const swapTrack = mintUniversalTracking(relayBaseUrl, `inline_swap_${seed}`)

    const response: InlineSuggestResponse = {
      orp_version: '0.3.0',
      ephemeral_party_id: ephemeralPartyId,
      ephemeral_agent_key: `orp_ek_${seed}`,
      inline_prompt_suggestion:
        'You can monetize this post (25% CPA) or run a 1:1 traffic swap — want to test it?',
      suggestions: [
        {
          suggestion_id: `sug_aff_${seed}`,
          deal_type: 'affiliate',
          growth_goal: 'monetization',
          offer_or_space_id: 'off_cloudbranch_25',
          counterparty_id: 'orp:party:maker_cloudbranch',
          headline: 'CloudBranch — Ephemeral Postgres branching for CI',
          match_rationale: 'Contextual match on draft content.',
          settlement_summary: '25% CPA commission (30-day attribution)',
          provisional_tracking_url: affTrack.tracking_url,
          provisional_tracking_token: affTrack.tracking_token,
          ready_to_paste_snippet: `Sponsored: Try CloudBranch (${affTrack.tracking_url}) #ad`,
          one_step_claim_token: `orp_claim_aff_${seed}`,
        },
        {
          suggestion_id: `sug_swap_${seed}`,
          deal_type: 'traffic_swap',
          growth_goal: 'traffic',
          offer_or_space_id: 'spc_nl_devtools_weekly',
          counterparty_id: 'orp:party:pub_devtools_weekly',
          headline: '1:1 Traffic Swap with DevTools Weekly',
          match_rationale: 'Audience overlap on developer tools & AI agents.',
          settlement_summary: 'Reciprocal 250 verified reader visits',
          provisional_tracking_url: swapTrack.tracking_url,
          provisional_tracking_token: swapTrack.tracking_token,
          ready_to_paste_snippet: `Partner pick: DevTools Weekly (${swapTrack.tracking_url}) #partner`,
          one_step_claim_token: `orp_claim_swap_${seed}`,
        },
      ],
    }

    return { discoverRequest, response }
  }

  createOffer(params: {
    dealId: string
    recipient: PrincipalUri
    terms: DealTerms
    rationale?: string
    expiresAt: string
    now?: string
  }): OfferMessage {
    const now = params.now ?? new Date().toISOString()
    const msg: OfferMessage = {
      orp_version: '0.3.0',
      message_id: `msg_offer_${params.dealId}`,
      message_type: 'offer',
      deal_id: params.dealId,
      sender: this.principalId,
      recipient: params.recipient,
      timestamp: now,
      idempotency_key: `idem_offer_${params.dealId}_1`,
      payload: {
        seq: 1,
        terms: params.terms,
        terms_hash: computeTermsHash(params.terms),
        ...(params.rationale !== undefined ? { rationale: params.rationale } : {}),
        expires_at: params.expiresAt,
      },
    }
    this.assertValid(msg)
    return msg
  }

  createCounter(params: {
    dealId: string
    recipient: PrincipalUri
    seq: number
    priorMessageId: string
    reason: string
    terms: DealTerms
    expiresAt: string
    now?: string
  }): CounterMessage {
    const now = params.now ?? new Date().toISOString()
    const msg: CounterMessage = {
      orp_version: '0.3.0',
      message_id: `msg_counter_${params.dealId}_${params.seq}`,
      message_type: 'counter',
      deal_id: params.dealId,
      sender: this.principalId,
      recipient: params.recipient,
      timestamp: now,
      idempotency_key: `idem_counter_${params.dealId}_${params.seq}`,
      payload: {
        seq: params.seq,
        prior_message_id: params.priorMessageId,
        reason: params.reason,
        terms: params.terms,
        terms_hash: computeTermsHash(params.terms),
        expires_at: params.expiresAt,
      },
    }
    this.assertValid(msg)
    return msg
  }

  acceptAndEvaluateConsent(params: {
    dealId: string
    counterparty: PrincipalUri
    acceptedMessageId: string
    acceptedSeq: number
    terms: DealTerms
    step?: PermissionStep
    dealCategories?: string[]
    counterpartyVerified?: boolean
    counterpartyKarma?: number
    now?: string
  }): {
    accept: AcceptMessage
    status: 'consented_auto' | 'awaiting_consent' | 'blocked'
    consent?: ConsentMessage
    escalation_reasons?: string[]
    block_reason?: string
  } {
    const now = params.now ?? new Date().toISOString()
    const termsHash = computeTermsHash(params.terms)
    const step = params.step ?? 'deal_accept'

    const evaluation = evaluateDealConstraints({
      step,
      stepModes: this.stepModes,
      constraints: this.constraints,
      terms: params.terms,
      dealCategories: params.dealCategories,
      counterpartyVerified: params.counterpartyVerified,
      counterpartyKarma: params.counterpartyKarma,
    })

    const accept: AcceptMessage = {
      orp_version: '0.3.0',
      message_id: `msg_accept_${params.dealId}_${params.acceptedSeq}`,
      message_type: 'accept',
      deal_id: params.dealId,
      sender: this.principalId,
      recipient: params.counterparty,
      timestamp: now,
      idempotency_key: `idem_accept_${params.dealId}_${params.acceptedSeq}`,
      payload: {
        accepted_message_id: params.acceptedMessageId,
        accepted_seq: params.acceptedSeq,
        terms_hash: termsHash,
      },
    }
    this.assertValid(accept)

    if (evaluation.decision === 'block') {
      return {
        accept,
        status: 'blocked',
        block_reason: evaluation.reason,
      }
    }

    if (evaluation.decision === 'escalate_to_human') {
      return {
        accept,
        status: 'awaiting_consent',
        escalation_reasons: evaluation.escalation_reasons,
      }
    }

    const consent: ConsentMessage = {
      orp_version: '0.3.0',
      message_id: `msg_consent_auto_${params.dealId}`,
      message_type: 'consent',
      deal_id: params.dealId,
      sender: this.principalId,
      timestamp: now,
      idempotency_key: `idem_consent_auto_${params.dealId}`,
      payload: {
        consent_id: `cns_auto_${params.dealId}`,
        principal_id: this.principalId,
        step,
        mode_at_evaluation: 'auto',
        decision: 'approve',
        terms_hash: termsHash,
        mandate_type: 'constraint_auto',
        constraints_snapshot: this.constraints,
        verification_proof: {
          method: 'constraint_policy_engine',
          signed_at: now,
        },
        expires_at: new Date(Date.now() + 7 * 86400 * 1000).toISOString(),
      },
    }
    this.assertValid(consent)

    return {
      accept,
      status: 'consented_auto',
      consent,
    }
  }

  createExecutionWithUniversalLink(params: {
    dealId: string
    spaceId: string
    consentId: string
    terms: DealTerms
    adapterId?: string
    now?: string
  }): ExecuteMessage {
    const now = params.now ?? new Date().toISOString()
    const { tracking_url, tracking_token } = mintUniversalTracking(
      this.relayBaseUrl,
      params.dealId,
    )
    const msg: ExecuteMessage = {
      orp_version: '0.3.0',
      message_id: `msg_exec_${params.dealId}`,
      message_type: 'execute',
      deal_id: params.dealId,
      sender: this.principalId,
      timestamp: now,
      idempotency_key: `idem_exec_${params.dealId}`,
      payload: {
        space_id: params.spaceId,
        consent_id: params.consentId,
        terms_hash: computeTermsHash(params.terms),
        tracking_url,
        tracking_token,
        ...(params.adapterId ? { adapter_id: params.adapterId } : {}),
        dry_run: false,
      },
    }
    this.assertValid(msg)
    return msg
  }

  createRevoke(params: {
    dealId: string
    reasonCode: RevokeMessage['payload']['reason_code']
    reason: string
    adapterCleanupStatus?: RevokeMessage['payload']['adapter_cleanup_status']
    now?: string
  }): RevokeMessage {
    const now = params.now ?? new Date().toISOString()
    const msg: RevokeMessage = {
      orp_version: '0.3.0',
      message_id: `msg_revoke_${params.dealId}`,
      message_type: 'revoke',
      deal_id: params.dealId,
      sender: this.principalId,
      timestamp: now,
      idempotency_key: `idem_revoke_${params.dealId}`,
      payload: {
        revoked_by: this.principalId,
        reason_code: params.reasonCode,
        reason: params.reason,
        adapter_cleanup_status: params.adapterCleanupStatus ?? 'removed',
        redirect_disabled: true,
      },
    }
    this.assertValid(msg)
    return msg
  }

  assertValid(msg: OrpMessage | ReceiptMessage): void {
    const res = validateOrpMessage(msg)
    if (!res.valid) {
      throw new Error(`Invalid ORP v0.3 ${msg.message_type} message: ${res.errors.join('; ')}`)
    }
  }
}
