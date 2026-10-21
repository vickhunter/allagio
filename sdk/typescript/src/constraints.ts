import type {
  DealTerms,
  EscalationReason,
  MandateConstraints,
  PermissionStep,
  StepMode,
} from './types.ts'

export type ConstraintEvaluationResult =
  | {
      decision: 'auto_approve'
      mode_at_evaluation: 'auto'
      mandate_type: 'constraint_auto'
    }
  | {
      decision: 'escalate_to_human'
      mode_at_evaluation: StepMode
      escalation_reasons: EscalationReason[]
    }
  | {
      decision: 'block'
      reason: string
    }

const SUPERLATIVE_REGEX = /\b(#1|best in the world|guaranteed profit|100% risk-free)\b/i

/**
 * Evaluates whether an agent can execute a step autonomously inside human-set constraints
 * or must escalate to the human owner (`awaiting_consent`).
 */
export function evaluateDealConstraints(params: {
  step: PermissionStep
  stepModes: Partial<Record<PermissionStep, StepMode>>
  constraints: MandateConstraints
  terms: DealTerms
  dealCategories?: string[]
  counterpartyVerified?: boolean
  counterpartyKarma?: number
  dailySpendSoFarMinor?: number
}): ConstraintEvaluationResult {
  const {
    step,
    stepModes,
    constraints,
    terms,
    dealCategories = [],
    counterpartyVerified = true,
    counterpartyKarma = 0,
    dailySpendSoFarMinor = 0,
  } = params

  // Default `space_execute` to `ask` if not explicitly set, matching D-088 safety invariant.
  const effectiveMode: StepMode =
    stepModes[step] ?? (step === 'space_execute' ? 'ask' : 'auto')

  if (effectiveMode === 'off') {
    return {
      decision: 'block',
      reason: `Step '${step}' is set to 'off' by the owner.`,
    }
  }

  if (constraints.blocked_categories && constraints.blocked_categories.length > 0) {
    const blocked = dealCategories.find((cat) =>
      constraints.blocked_categories!.includes(cat),
    )
    if (blocked) {
      return {
        decision: 'block',
        reason: `Category '${blocked}' is in owner blocked_categories.`,
      }
    }
  }

  if (
    constraints.brand_rules?.disallow_superlatives &&
    SUPERLATIVE_REGEX.test(`${terms.creative.headline ?? ''} ${terms.creative.body}`)
  ) {
    return {
      decision: 'block',
      reason: 'Creative copy violates brand_rules.disallow_superlatives.',
    }
  }

  const escalationReasons: EscalationReason[] = []

  if (effectiveMode === 'ask') {
    escalationReasons.push('step_ask')
  }

  const spendMinor =
    (terms.settlement.amount_minor ?? 0) +
    (terms.settlement.topper_amount_minor ?? 0) +
    (terms.settlement.flat_amount_minor ?? 0)

  if (
    typeof constraints.require_human_above_minor === 'number' &&
    spendMinor > constraints.require_human_above_minor
  ) {
    escalationReasons.push('exceeds_approval_threshold')
  }

  if (
    typeof constraints.max_deal_amount_minor === 'number' &&
    spendMinor > constraints.max_deal_amount_minor
  ) {
    escalationReasons.push('exceeds_approval_threshold')
  }

  if (
    typeof constraints.daily_budget_minor === 'number' &&
    dailySpendSoFarMinor + spendMinor > constraints.daily_budget_minor
  ) {
    escalationReasons.push('exceeds_daily_budget')
  }

  if (
    constraints.allowed_settlement_modes &&
    !constraints.allowed_settlement_modes.includes(terms.settlement.mode)
  ) {
    escalationReasons.push('settlement_mode_restricted')
  }

  if (
    constraints.allowed_categories &&
    constraints.allowed_categories.length > 0 &&
    dealCategories.some((c) => !constraints.allowed_categories!.includes(c))
  ) {
    escalationReasons.push('category_not_allowed')
  }

  if (constraints.brand_rules?.require_verified_counterparty && !counterpartyVerified) {
    escalationReasons.push('counterparty_unverified')
  }

  if (
    typeof constraints.brand_rules?.min_counterparty_karma === 'number' &&
    counterpartyKarma < constraints.brand_rules.min_counterparty_karma
  ) {
    escalationReasons.push('counterparty_karma_below_min')
  }

  if (escalationReasons.length > 0) {
    return {
      decision: 'escalate_to_human',
      mode_at_evaluation: effectiveMode,
      escalation_reasons: Array.from(new Set(escalationReasons)),
    }
  }

  return {
    decision: 'auto_approve',
    mode_at_evaluation: 'auto',
    mandate_type: 'constraint_auto',
  }
}
