export type OrpVersion = `0.3.${number}`

/** Pseudonymous party URI (`orp:party:...` or `orp:principal:...` or `https://...`). Never an email. */
export type PrincipalUri = string
export type Sha256Hash = `sha256:${string}`
export type CurrencyCode = string

export type StepMode = 'auto' | 'ask' | 'off'

export type PermissionStep =
  | 'space_register'
  | 'discover_respond'
  | 'deal_offer'
  | 'deal_accept'
  | 'space_execute'
  | 'deal_modify'
  | 'settlement_payout'

export type SpaceKind =
  | 'content_inline'
  | 'content_box'
  | 'social_post'
  | 'social_repost'
  | 'social_comment'
  | 'newsletter_slot'
  | 'video_segment'
  | 'community_pin'
  | 'cross_promo_slot'
  | 'data_insight_room'

export type PlatformId =
  | 'wordpress'
  | 'x'
  | 'newsletter'
  | 'youtube'
  | 'instagram'
  | 'tiktok'
  | 'reddit'
  | 'discord'
  | 'facebook'
  | 'podcast'
  | 'generic_web'

export type SettlementMode =
  | 'cpa_commission'
  | 'rev_share'
  | 'flat_fee'
  | 'space_swap'
  | 'traffic_swap'
  | 'repost_bounty'
  | 'co_marketing'
  | 'data_exchange'

export type DealType =
  | 'affiliate'
  | 'cross_sell_upsell'
  | 'link_exchange'
  | 'traffic_swap'
  | 'repost_retweet'

export type GrowthGoal =
  | 'traffic'
  | 'engagement'
  | 'monetization'
  | 'clients'

export type ReceiptVerificationMode =
  | 'proof_url'
  | 'public_check'
  | 'counterparty_attestation'
  | 'adapter_ack'

export type EscalationReason =
  | 'step_ask'
  | 'exceeds_approval_threshold'
  | 'exceeds_daily_budget'
  | 'category_not_allowed'
  | 'counterparty_unverified'
  | 'counterparty_karma_below_min'
  | 'claim_check_unavailable'
  | 'settlement_mode_restricted'

export interface Signature {
  alg: 'EdDSA' | 'ES256'
  kid: string
  jws: string
}

export interface MandateConstraints {
  max_deal_amount_minor?: number
  daily_budget_minor?: number
  require_human_above_minor?: number
  currency?: CurrencyCode
  min_cpa_bps?: number
  allowed_deal_types?: DealType[]
  allowed_settlement_modes?: SettlementMode[]
  allowed_categories?: string[]
  blocked_categories?: string[]
  brand_rules?: {
    require_verified_counterparty?: boolean
    min_counterparty_karma?: number
    max_copy_chars?: number
    disallow_superlatives?: boolean
  }
}

export interface AudienceCard {
  k_anonymity_threshold: number
  reach_bucket: 'under_1k' | '1k_10k' | '10k_50k' | '50k_250k' | '250k_1m' | 'over_1m'
  engagement_rate_bps?: number
  top_geos?: string[]
  top_interests?: string[]
  provenance: 'self_declared' | 'public_check' | 'adapter_verified' | 'relay_measured'
}

export interface Space {
  space_id: string
  principal_id: PrincipalUri
  platform: PlatformId
  kind: SpaceKind
  uri: string
  handle_or_domain?: string
  title: string
  description?: string
  categories?: string[]
  locale?: string
  verification: {
    status: 'verified' | 'pending' | 'unverified'
    method: 'dns_txt' | 'http_well_known' | 'public_check' | 'oauth2_handle' | 'signed_challenge' | 'plugin_key'
    verified_at?: string
    adapter_id?: string
  }
  accepted_settlement_modes: SettlementMode[]
  constraints?: {
    max_chars?: number
    media_allowed?: boolean
    min_live_seconds?: number
    required_disclosure_markers?: string[]
    forbidden_categories?: string[]
  }
  floor_pricing?: {
    currency?: CurrencyCode
    flat_fee_minor?: number
    cpa_min_bps?: number
  }
  audience_card?: AudienceCard
  visibility_pools?: string[]
}

export interface Creative {
  headline?: string
  body: string
  target_url: string
  target_post_uri?: string
  disclosure_text: string
  rel_attributes?: Array<'sponsored' | 'nofollow' | 'ugc'>
}

export interface Settlement {
  mode: SettlementMode
  currency?: CurrencyCode
  amount_minor?: number
  topper_amount_minor?: number
  commission_type?: 'percentage' | 'flat'
  commission_bps?: number
  flat_amount_minor?: number
  share_bps?: number
  duration_months?: number
  attribution_window_days?: number
  initiator_space_id?: string
  responder_space_id?: string
  sync_window_seconds?: number
  min_live_seconds?: number
  target_verified_visits?: number
  exchange_ratio_bps?: number
  target_post_uri?: string
  required_action?: 'repost' | 'quote' | 'comment'
  dataset_schema_uri?: string
  min_k_anonymity?: number
  escrow_required?: boolean
}

export interface DealTerms {
  deal_type?: DealType
  space_id: string
  space_kind: SpaceKind
  settlement: Settlement
  creative: Creative
  reciprocal_creative?: Creative
  window: {
    execute_after?: string
    execute_before?: string
    min_live_seconds: number
  }
}

export interface InlineSuggestionItem {
  suggestion_id: string
  deal_type: DealType
  growth_goal: GrowthGoal
  offer_or_space_id: string
  counterparty_id: PrincipalUri
  headline: string
  match_rationale: string
  settlement_summary: string
  provisional_tracking_url: string
  provisional_tracking_token: string
  ready_to_paste_snippet: string
  one_step_claim_token: string
}

export interface InlineSuggestResponse {
  orp_version: OrpVersion
  ephemeral_party_id: PrincipalUri
  ephemeral_agent_key: string
  inline_prompt_suggestion: string
  suggestions: InlineSuggestionItem[]
}

export interface EnvelopeBase<TType extends string, TPayload> {
  orp_version: OrpVersion
  message_id: string
  message_type: TType
  sender: PrincipalUri
  recipient?: PrincipalUri
  timestamp: string
  signature?: Signature
  payload: TPayload
}

export interface DiscoverMessage
  extends EnvelopeBase<
    'discover',
    {
      mode?: 'inline_suggest' | 'network'
      target_type: 'space' | 'offer' | 'swap_partner'
      query?: string
      draft_text?: string
      growth_goals?: GrowthGoal[]
      deal_types?: DealType[]
      surface_kinds?: SpaceKind[]
      space_kinds?: SpaceKind[]
      platforms?: PlatformId[]
      settlement_modes?: SettlementMode[]
      pools?: string[]
      min_reputation_karma?: number
      max_results?: number
    }
  > {}

export interface OfferMessage
  extends EnvelopeBase<
    'offer',
    {
      seq: 1
      terms: DealTerms
      terms_hash: Sha256Hash
      rationale?: string
      expires_at: string
    }
  > {
  deal_id: string
  recipient: PrincipalUri
  idempotency_key: string
}

export interface CounterMessage
  extends EnvelopeBase<
    'ask' | 'counter',
    {
      seq: number
      prior_message_id?: string
      reason?: string
      terms: DealTerms
      terms_hash: Sha256Hash
      expires_at: string
    }
  > {
  deal_id: string
  idempotency_key: string
}

export interface AcceptMessage
  extends EnvelopeBase<
    'accept',
    {
      accepted_message_id: string
      accepted_seq: number
      terms_hash: Sha256Hash
    }
  > {
  deal_id: string
  recipient: PrincipalUri
  idempotency_key: string
}

export interface ConsentMessage
  extends EnvelopeBase<
    'consent',
    {
      consent_id: string
      principal_id: PrincipalUri
      step: PermissionStep
      mode_at_evaluation: StepMode
      escalation_reasons?: EscalationReason[]
      decision: 'approve' | 'deny'
      terms_hash: Sha256Hash
      mandate_type: 'constraint_auto' | 'explicit_owner_approval' | 'standing_policy'
      constraints_snapshot?: MandateConstraints
      verification_proof: {
        method: 'jws_ed25519' | 'jws_es256' | 'relay_owner_session' | 'constraint_policy_engine'
        kid?: string
        signed_at: string
        jws?: string
        session_attestation_id?: string
      }
      expires_at: string
    }
  > {
  deal_id: string
  idempotency_key: string
}

export interface ExecuteMessage
  extends EnvelopeBase<
    'execute',
    {
      space_id: string
      consent_id: string
      terms_hash: Sha256Hash
      tracking_url?: string
      tracking_token?: string
      adapter_id?: string
      dry_run?: boolean
    }
  > {
  deal_id: string
  idempotency_key: string
}

export interface ReceiptMessage
  extends EnvelopeBase<
    'receipt',
    {
      receipt_id: string
      space_id: string
      verification_mode: ReceiptVerificationMode
      proof_url?: string
      location_url: string
      tracking_token_found?: boolean
      adapter_id?: string
      platform_object_id?: string
      verified_at: string
      disclosure_verified: true
      observed_disclosure: string
      content_hash: Sha256Hash
      hold_until?: string
      public_check_result?: {
        http_status: number
        matched_target?: string
        checked_at: string
      }
      counterparty_attestation?: {
        attested_by: PrincipalUri
        attested_at: string
        statement: string
        signature?: Signature
      }
      adapter_attestation?: {
        method: string
        status_code?: number
        [key: string]: unknown
      }
    }
  > {
  deal_id: string
  idempotency_key: string
}

export interface RevokeMessage
  extends EnvelopeBase<
    'revoke',
    {
      revoked_by: PrincipalUri
      reason_code:
        | 'owner_kill_switch'
        | 'landing_page_changed'
        | 'counterparty_breach'
        | 'policy_violation'
        | 'expired_or_completed'
      reason: string
      adapter_cleanup_status: 'removed' | 'pending_adapter' | 'manual_required'
      redirect_disabled: boolean
    }
  > {
  deal_id: string
  idempotency_key: string
}

export interface DisputeMessage
  extends EnvelopeBase<
    'dispute',
    {
      dispute_id: string
      reason_code:
        | 'premature_removal'
        | 'missing_disclosure'
        | 'unfulfilled_swap'
        | 'landing_page_deception'
        | 'invalid_attribution'
      description: string
      evidence: Array<{
        type:
          | 'public_check_failure'
          | 'adapter_recheck_failure'
          | 'disclosure_missing_snapshot'
          | 'webhook_discrepancy'
          | 'swap_timeout'
        observed_at: string
        target_uri?: string
        http_status?: number
        details?: string
      }>
      requested_remedy:
        | 'refund_escrow_and_slash_karma'
        | 'require_republication'
        | 'cancel_reciprocal_obligation'
        | 'slash_karma_only'
    }
  > {
  deal_id: string
  idempotency_key: string
}

export interface ReputationMessage
  extends EnvelopeBase<
    'reputation',
    {
      principal_id: PrincipalUri
      calculated_at: string
      total_karma: number
      dimensions: {
        fulfillment_karma: number
        conversion_karma: number
        compliance_karma: number
        dispute_penalty: number
      }
      stats: {
        verified_deals_completed: number
        unique_counterparties: number
        fulfillment_rate_bps: number
        disputes_lost: number
        median_hold_seconds?: number
      }
      recent_attestations?: Array<{
        deal_id: string
        counterparty_id: PrincipalUri
        space_kind: SpaceKind
        settlement_mode: SettlementMode
        receipt_id: string
        pair_deal_index: number
        pair_decay_weight: number
        karma_delta: number
        settled_at: string
      }>
    }
  > {}

export type OrpMessage =
  | DiscoverMessage
  | OfferMessage
  | CounterMessage
  | AcceptMessage
  | ConsentMessage
  | ExecuteMessage
  | ReceiptMessage
  | RevokeMessage
  | DisputeMessage
  | ReputationMessage

export function assertNever(value: never): never {
  throw new Error(`Unhandled discriminated union variant: ${JSON.stringify(value)}`)
}
