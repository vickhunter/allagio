// Pitches are what a writing agent shows its person mid-task, as is: what they
// would do and what they would get. They are built from the terms alone, so
// they cannot carry anything personal.
import { isPay } from './constraints.ts';
import type { DealKind, Money, Obligation, PlaceObligation, RepostObligation, Role, Space, Terms } from './types.ts';

export type Want = 'earn' | 'grow';

export const EARN_KINDS: readonly DealKind[] = ['affiliate', 'sponsored', 'cross_sell'];
export const GROW_KINDS: readonly DealKind[] = ['swap', 'traffic_swap', 'link_exchange', 'repost'];

const MAX_PITCH = 280;

export const isPlace = (o: Obligation): o is PlaceObligation => o.action === 'place';
export const isRepost = (o: Obligation): o is RepostObligation => o.action === 'repost';

/** What the acceptor of these terms gets: paid (earn), or readers and reach (grow). */
export function benefits(terms: Terms, role: Role = 'acceptor'): Set<Want> {
  const out = new Set<Want>();
  for (const o of terms.obligations) {
    if (o.by === role) continue;
    if (o.action === 'pay') out.add('earn');
    else if (o.action === 'place' || o.action === 'repost' || o.action === 'comment') out.add('grow');
  }
  return out;
}

export function formatMoney(money: Money, lang = 'en'): string {
  const digits = new Intl.NumberFormat('en', { style: 'currency', currency: money.currency }).resolvedOptions().maximumFractionDigits ?? 2;
  const value = money.amount_minor / 10 ** digits;
  return new Intl.NumberFormat(lang, { style: 'currency', currency: money.currency, maximumFractionDigits: Number.isInteger(value) ? 0 : digits }).format(value);
}

const percent = (bps: number) => `${Number((bps / 100).toFixed(2))}%`;
const hostOf = (url: string) => new URL(url).hostname.replace(/^www\./, '');

function doing(o: Obligation, label: string): string | undefined {
  const labelled = label ? ` labelled '${label}'` : '';
  if (isPlace(o)) {
    if (o.content?.direct && o.destination) return `link to ${hostOf(o.destination)}${labelled}`;
    const count = o.count && o.count > 1 ? `${o.count} links` : 'one link';
    return `add ${count}${labelled}${o.clicks ? ` until it sends ${o.clicks} readers` : ''}`;
  }
  if (isRepost(o)) return o.quote ? `quote their post with your take${labelled}` : 'repost their post';
  if (o.action === 'comment') return 'comment on their post';
  if (isPay(o)) return o.amount ? `pay ${formatMoney(o.amount)}${o.per === 'deal' || o.per === 'placement' ? '' : ` per ${o.per}`}` : undefined;
  if (o.action === 'share') return 'share aggregate stats';
  return undefined;
}

function getting(o: Obligation): string | undefined {
  if (isPay(o)) {
    if (o.rate_bps) return `earn ${percent(o.rate_bps)} of what readers spend`;
    if (!o.amount) return undefined;
    if (o.per === 'conversion') return `earn ${formatMoney(o.amount)} per sale`;
    if (o.per === 'click') return `earn ${formatMoney(o.amount)} per reader click`;
    return `get ${formatMoney(o.amount)}`;
  }
  if (isPlace(o)) return o.clicks ? `get ${o.clicks} readers back` : 'get a link back';
  if (isRepost(o)) return 'get a repost';
  if (o.action === 'comment') return 'get a comment';
  return undefined;
}

function list(parts: string[]): string {
  return parts.length <= 1 ? (parts[0] ?? '') : `${parts.slice(0, -1).join(', ')} and ${parts.at(-1)}`;
}

function fit(build: (title: string) => string, title: string): string {
  const full = build(title);
  if (full.length <= MAX_PITCH) return full;
  const room = Math.max(10, title.length - (full.length - MAX_PITCH) - 1);
  return build(`${title.slice(0, room).trimEnd()}\u2026`).slice(0, MAX_PITCH);
}

/** In plain words, what the party in this role would do and what it gets. */
export function sides(terms: Terms, role: Role): { you: string[]; get: string[] } {
  const label = terms.disclosure?.label ?? '';
  return {
    you: terms.obligations.filter((o) => o.by === role).flatMap((o) => doing(o, label) ?? []),
    get: terms.obligations.filter((o) => o.by !== role).flatMap((o) => getting(o) ?? []),
  };
}

/** The pitch for taking a standing offer as its acceptor. */
export function pitchOffer(terms: Terms): string {
  const { you, get } = sides(terms, 'acceptor');
  const lead = benefits(terms).has('earn') ? 'You can monetize this post' : 'You can get more readers for this post';
  const deal = [list(you), list(get)].filter(Boolean).join(' and ');
  return fit((title) => `${lead}: ${title}.${deal ? ` You'd ${deal}.` : ''} Want to test it?`, terms.title);
}

const TRADES: Partial<Record<DealKind, [plural: string, one: string]>> = {
  swap: ['swaps', 'a swap'],
  traffic_swap: ['traffic swaps', 'a traffic swap'],
  link_exchange: ['link exchanges', 'a link exchange'],
  repost: ['reposts', 'a repost swap'],
};

/** The pitch for proposing a deal to an open space. */
export function pitchSpace(space: Space): string {
  const grow = (space.accepts ?? []).filter((kind) => GROW_KINDS.includes(kind));
  const ways = list(grow.map((kind) => TRADES[kind]?.[0] ?? kind));
  const first = grow[0] ? (TRADES[grow[0]]?.[1] ?? 'one') : 'one';
  const size = space.audience?.size ? ` (${space.audience.size} readers)` : '';
  return fit((title) => `Want more readers? ${title}${size} trades ${ways || 'mentions'} with posts like this. Want me to propose ${first}?`, space.title ?? space.kind);
}
