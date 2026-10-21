import type { PrincipalUri, SettlementMode, SpaceKind } from './types.ts'

export interface CompletedDealOutcome {
  deal_id: string
  principal_id: PrincipalUri
  counterparty_id: PrincipalUri
  space_kind: SpaceKind
  settlement_mode: SettlementMode
  receipt_id: string
  receipt_verified: boolean
  held_full_duration: boolean
  conversion_or_swap_settled: boolean
  disclosure_compliant: boolean
  dispute_lost: boolean
  counterparty_unique_partners: number
  settled_at: string
}

/**
 * Pairwise exponential decay weight w(n) = 2^{-(n-1)} for the n-th deal (1-indexed)
 * between the same two Principals within a 180-day window.
 * Caps total wash-traded karma between any colluding pair at < 2x a single deal.
 */
export function pairwiseDecayWeight(pairDealIndex: number): number {
  if (pairDealIndex <= 1) return 1.0
  return Math.pow(2, -(pairDealIndex - 1))
}

/**
 * Counterparty diversity trust weight tau in [0.1, 1.0].
 */
export function counterpartyTrustFactor(uniquePartners: number): number {
  const clamped = Math.max(0, uniquePartners)
  return Math.min(1.0, 0.1 + 0.9 * (clamped / 10))
}

/**
 * Computes dimensional Proof-of-Outcome Karma for a principal from deal outcomes.
 */
export function computeReputationProfile(
  principalId: PrincipalUri,
  outcomes: CompletedDealOutcome[],
  calculatedAt = new Date().toISOString(),
) {
  let fulfillmentKarma = 0
  let conversionKarma = 0
  let complianceKarma = 0
  let disputePenalty = 0
  let verifiedDealsCompleted = 0
  let disputesLost = 0

  const pairCounts = new Map<string, number>()
  const uniqueCounterparties = new Set<string>()
  const recentAttestations: Array<{
    deal_id: string
    counterparty_id: PrincipalUri
    space_kind: SpaceKind
    settlement_mode: SettlementMode
    receipt_id: string
    pair_deal_index: number
    pair_decay_weight: number
    karma_delta: number
    settled_at: string
  }> = []

  for (const outcome of outcomes) {
    if (outcome.principal_id !== principalId) continue

    if (outcome.dispute_lost) {
      disputePenalty -= 15
      disputesLost += 1
      recentAttestations.push({
        deal_id: outcome.deal_id,
        counterparty_id: outcome.counterparty_id,
        space_kind: outcome.space_kind,
        settlement_mode: outcome.settlement_mode,
        receipt_id: outcome.receipt_id,
        pair_deal_index: (pairCounts.get(outcome.counterparty_id) ?? 0) + 1,
        pair_decay_weight: 1.0,
        karma_delta: -15,
        settled_at: outcome.settled_at,
      })
      continue
    }

    // Gate 1: Karma is never minted without a verified receipt and full hold duration
    if (!outcome.receipt_verified || !outcome.held_full_duration) {
      continue
    }

    const pairIndex = (pairCounts.get(outcome.counterparty_id) ?? 0) + 1
    pairCounts.set(outcome.counterparty_id, pairIndex)
    uniqueCounterparties.add(outcome.counterparty_id)
    verifiedDealsCompleted += 1

    const decay = pairwiseDecayWeight(pairIndex)
    const trust = counterpartyTrustFactor(outcome.counterparty_unique_partners)
    const weight = decay * trust

    const fDelta = Math.round(5 * weight)
    const cDelta = outcome.conversion_or_swap_settled ? Math.round(3 * weight) : 0
    const dDelta = outcome.disclosure_compliant ? Math.round(1 * weight) : 0

    fulfillmentKarma += fDelta
    conversionKarma += cDelta
    complianceKarma += dDelta

    recentAttestations.push({
      deal_id: outcome.deal_id,
      counterparty_id: outcome.counterparty_id,
      space_kind: outcome.space_kind,
      settlement_mode: outcome.settlement_mode,
      receipt_id: outcome.receipt_id,
      pair_deal_index: pairIndex,
      pair_decay_weight: Number(decay.toFixed(6)),
      karma_delta: fDelta + cDelta + dDelta,
      settled_at: outcome.settled_at,
    })
  }

  const totalAttempts = verifiedDealsCompleted + disputesLost
  const fulfillmentRateBps =
    totalAttempts === 0 ? 10000 : Math.round((verifiedDealsCompleted / totalAttempts) * 10000)

  return {
    principal_id: principalId,
    calculated_at: calculatedAt,
    total_karma: fulfillmentKarma + conversionKarma + complianceKarma + disputePenalty,
    dimensions: {
      fulfillment_karma: fulfillmentKarma,
      conversion_karma: conversionKarma,
      compliance_karma: complianceKarma,
      dispute_penalty: disputePenalty,
    },
    stats: {
      verified_deals_completed: verifiedDealsCompleted,
      unique_counterparties: uniqueCounterparties.size,
      fulfillment_rate_bps: fulfillmentRateBps,
      disputes_lost: disputesLost,
    },
    recent_attestations: recentAttestations,
  }
}
