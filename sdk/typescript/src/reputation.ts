// orp-rep-1: reputation from verified outcomes only, hard to farm.
//  - Only receipts and resolved disputes count. Claims, clicks and reviews do not.
//  - The k-th event with the same counterparty weighs 1/k, so a pair of
//    accounts trading with each other gains almost nothing.
//  - Events weigh by the counterparty's own standing (0.25 to 1), so a ring of
//    new accounts confirming each other stays near the floor.
//  - Stronger proofs weigh more than attestations.
//  - Events halve in weight every 180 days.
//  - The score is the Wilson lower bound of the weighted success share, so a
//    new party with two good deals does not outrank one with two hundred.
import type { PartyId, ProofMethod } from './types.ts';

export type RepOutcome = 'verified' | 'failed' | 'dispute_lost' | 'breach';

export interface RepEvent {
  counterparty: PartyId;
  counterpartyScore: number | null;
  outcome: RepOutcome;
  method?: ProofMethod;
  at: Date;
}

export interface RepResult {
  score: number | null;
  components: { verified: number; failed: number; disputes_lost: number; counterparties: number; evidence_weight: number };
}

export const HALF_LIFE_DAYS = 180;
export const MIN_EVIDENCE = 3;
const Z = 1.96;

const METHOD_WEIGHT: Record<string, number> = {
  provider: 1,
  platform_check: 1,
  link: 0.9,
  witness: 0.8,
  proof_url: 0.7,
  attestation: 0.5,
};

const PENALTY: Record<RepOutcome, number> = { verified: 0, failed: 1, breach: 1, dispute_lost: 2 };

export function wilsonLowerBound(successes: number, total: number): number {
  if (total <= 0) return 0;
  const p = successes / total;
  const z2 = Z * Z;
  return (p + z2 / (2 * total) - Z * Math.sqrt((p * (1 - p)) / total + z2 / (4 * total * total))) / (1 + z2 / total);
}

const round1 = (value: number) => Math.round(value * 10) / 10;

export function score(events: RepEvent[], now: Date = new Date()): RepResult {
  const ordered = [...events].sort((a, b) => a.at.getTime() - b.at.getTime());
  const seen = new Map<PartyId, number>();
  let positive = 0;
  let negative = 0;
  let disputesLost = 0;
  for (const event of ordered) {
    const k = (seen.get(event.counterparty) ?? 0) + 1;
    seen.set(event.counterparty, k);
    const ageDays = Math.max(0, (now.getTime() - event.at.getTime()) / 86_400_000);
    const decay = 0.5 ** (ageDays / HALF_LIFE_DAYS);
    const trust = 0.25 + 0.75 * Math.min(1, Math.max(0, (event.counterpartyScore ?? 0) / 1000));
    const weight = decay * (1 / k) * trust;
    if (event.outcome === 'verified') {
      positive += weight * (METHOD_WEIGHT[event.method ?? 'attestation'] ?? 0.5);
    } else {
      negative += weight * PENALTY[event.outcome];
      if (event.outcome === 'dispute_lost') disputesLost += 1;
    }
  }
  const evidence = positive + negative;
  const components = { verified: round1(positive), failed: round1(negative), disputes_lost: disputesLost, counterparties: seen.size, evidence_weight: round1(evidence) };
  if (evidence < MIN_EVIDENCE) return { score: null, components };
  return { score: Math.round(1000 * wilsonLowerBound(positive, evidence)), components };
}
