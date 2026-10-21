// Bounded autonomy: decide whether a step is auto (the agent acts alone), ask
// (a person decides) or off (refused). Hard rules refuse; allowances escalate.
// Anything the owner did not set falls to the safe side.
import type { Constraints, EscalationReason, Money, Obligation, PartyId, PayObligation, Role, Step, StepMode, Terms } from './types.ts';

export const isPay = (obligation: Obligation): obligation is PayObligation => obligation.action === 'pay';

export const DEFAULT_STEPS: Record<Step, StepMode> = {
  list: 'ask',
  negotiate: 'ask',
  accept: 'ask',
  execute: 'ask',
  amend: 'ask',
  settle: 'ask',
  share: 'ask',
  revoke: 'auto',
};

export interface Facts {
  step: Step;
  role?: Role;
  terms?: Terms;
  counterparty?: PartyId;
  counterpartyReputation?: number | null;
  firstDeal?: boolean;
  spaces?: string[];
  spent?: { day?: Money; month?: Money };
  now?: Date;
}

export type Decision =
  | { mode: 'auto' }
  | { mode: 'ask'; reasons: EscalationReason[] }
  | { mode: 'off'; code: 'step_off' | 'brand_rule' | 'forbidden'; detail: string };

export function durationMs(value: string): number {
  const match = value.match(/^P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$/);
  if (!match) throw new RangeError(`not an ISO 8601 duration: ${value}`);
  const [y, mo, w, d, h, mi, s] = match.slice(1).map((part) => Number(part ?? 0)) as [number, number, number, number, number, number, number];
  return (((y * 365 + mo * 30 + w * 7 + d) * 24 + h) * 60 + mi) * 60_000 + s * 1000;
}

const under = (category: string, prefix: string) => category === prefix || category.startsWith(`${prefix}.`);

// Cash the given role commits to pay. Infinity when a fixed amount per click or
// conversion has no cap. A pure share of the sale is paid out of the sale, so
// it is checked against budget.max_rate_bps instead (see highestRate).
export function committed(terms: Terms, role: Role, currency: string): number {
  let total = 0;
  for (const pay of terms.obligations) {
    if (!isPay(pay) || pay.by !== role) continue;
    if (pay.amount === undefined && (pay.per === 'click' || pay.per === 'conversion')) continue;
    const money = pay.per === 'deal' || pay.per === 'placement' ? pay.amount : pay.cap;
    if (!money) return Infinity;
    if (money.currency !== currency) return Infinity;
    const placements = pay.per === 'placement' ? Math.max(1, ...terms.obligations.map((o) => (o.action === 'place' && 'count' in o && typeof o.count === 'number' ? o.count : 1))) : 1;
    total += money.amount_minor * placements;
  }
  return total;
}

export function highestRate(terms: Terms, role: Role): number {
  return Math.max(0, ...terms.obligations.filter(isPay).filter((pay) => pay.by === role).map((pay) => pay.rate_bps ?? 0));
}

function textOf(terms: Terms): string {
  const parts = [terms.title];
  for (const obligation of terms.obligations) {
    const content = 'content' in obligation ? (obligation.content as { brief?: string; copy?: string; must_include?: string[] } | undefined) : undefined;
    if (content?.brief) parts.push(content.brief);
    if (content?.copy) parts.push(content.copy);
    if (content?.must_include) parts.push(...content.must_include);
  }
  return parts.join('\n').toLowerCase();
}

export function decide(constraints: Constraints | undefined, facts: Facts): Decision {
  const now = facts.now ?? new Date();
  const mode = constraints?.steps?.[facts.step] ?? DEFAULT_STEPS[facts.step];
  if (mode === 'off') return { mode: 'off', code: 'step_off', detail: `the owner turned off ${facts.step}` };

  const counterparties = constraints?.counterparties;
  if (facts.counterparty && counterparties?.deny?.includes(facts.counterparty)) {
    return { mode: 'off', code: 'forbidden', detail: 'the owner does not deal with this counterparty' };
  }
  if (facts.firstDeal && counterparties?.new === 'off') {
    return { mode: 'off', code: 'forbidden', detail: 'the owner does not take on new counterparties' };
  }

  const terms = facts.terms;
  const reasons = new Set<EscalationReason>();
  if (mode === 'ask') reasons.add('step_ask');
  if (constraints?.expires && Date.parse(constraints.expires) <= now.getTime()) reasons.add('mandate_expired');

  if (terms) {
    const denied = (terms.categories ?? []).find((category) => constraints?.categories?.deny?.some((prefix) => under(category, prefix)));
    if (denied) return { mode: 'off', code: 'brand_rule', detail: `category ${denied} is never allowed` };
    const text = textOf(terms);
    const banned = constraints?.brand?.must_not_include?.find((phrase) => text.includes(phrase.toLowerCase()));
    if (banned) return { mode: 'off', code: 'brand_rule', detail: `the terms contain "${banned}"` };
    if (constraints?.brand?.disclosure && !terms.disclosure) return { mode: 'off', code: 'brand_rule', detail: 'the owner requires a disclosure label' };

    const allow = constraints?.categories?.allow;
    if (allow && (terms.categories ?? []).some((category) => !allow.some((prefix) => under(category, prefix)))) reasons.add('category_outside');
    if (constraints?.kinds && (!terms.kind || !constraints.kinds.includes(terms.kind))) reasons.add('kind_outside');

    const role = facts.role ?? 'acceptor';
    const mine = terms.obligations.filter((obligation) => obligation.by === role);
    const allowedKinds = constraints?.spaces?.kinds;
    if (allowedKinds && mine.some((o) => 'space' in o && (o.space as { kinds?: string[] } | undefined)?.kinds?.some((kind) => !allowedKinds.includes(kind)))) reasons.add('space_outside');

    const budget = constraints?.budget;
    const currency = budget?.per_deal?.currency ?? budget?.per_day?.currency ?? budget?.per_month?.currency ?? constraints?.approve_over?.currency ?? 'EUR';
    const spend = committed(terms, role, currency);
    if (spend > 0) {
      const caps: Array<[Money | undefined, Money | undefined]> = [[budget?.per_deal, undefined], [budget?.per_day, facts.spent?.day], [budget?.per_month, facts.spent?.month]];
      const configured = caps.filter(([cap]) => cap);
      if (configured.length === 0 || configured.some(([cap, used]) => spend + (used?.amount_minor ?? 0) > cap!.amount_minor)) reasons.add('over_budget');
    }
    const rate = highestRate(terms, role);
    if (rate > 0 && rate > (budget?.max_rate_bps ?? 0)) reasons.add('over_budget');
    const threshold = constraints?.approve_over;
    if (threshold) {
      const other: Role = role === 'offerer' ? 'acceptor' : 'offerer';
      const incoming = terms.obligations
        .filter(isPay)
        .filter((o) => o.by === other && (o.per === 'deal' || o.per === 'placement') && o.amount?.currency === threshold.currency)
        .reduce((sum, o) => sum + (o.amount?.amount_minor ?? 0), 0);
      if (spend + incoming > threshold.amount_minor) reasons.add('over_threshold');
    }

    if (constraints?.max_duration) {
      const limit = durationMs(constraints.max_duration);
      const span = terms.ends ? Date.parse(terms.ends) - (terms.starts ? Date.parse(terms.starts) : now.getTime()) : 0;
      const keeps = mine.map((o) => ('keep' in o && typeof o.keep === 'string' ? durationMs(o.keep) : 0));
      if (span > limit || keeps.some((keep) => keep > limit)) reasons.add('duration_over');
    }
  }

  if (constraints?.spaces?.allow && facts.spaces?.some((space) => !constraints.spaces!.allow!.includes(space))) reasons.add('space_outside');
  if (facts.firstDeal && counterparties?.new === 'ask') reasons.add('new_counterparty');
  if (counterparties?.min_reputation !== undefined && facts.counterparty) {
    const score = facts.counterpartyReputation;
    if (score === null || score === undefined || score < counterparties.min_reputation) reasons.add('low_reputation');
  }

  return reasons.size ? { mode: 'ask', reasons: [...reasons] } : { mode: 'auto' };
}
