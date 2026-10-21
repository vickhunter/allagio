// A small in-memory ORP 0.3 host. It is the reference the conformance suite
// runs against, and a starting point for real hosts. It keeps everything in
// memory and its /orp-test harness must never be enabled in production.
import { randomBytes } from 'node:crypto';
import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http';
import type { JsonWebKey } from 'node:crypto';
import type {
  Authority,
  Capabilities,
  ClickCounts,
  Constraints,
  DealBody,
  DealState,
  Discovery,
  EscalationReason,
  Hash,
  InboxPage,
  JoinResponse,
  Message,
  Money,
  Obligation,
  ObligationId,
  OutcomeRow,
  PartyId,
  ProofMethod,
  ReceiptBody,
  Result,
  Role,
  Space,
  Step,
  SuggestResponse,
  Terms,
  Tracking,
  Trust,
  Waiting,
} from './types.ts';
import { decide, DEFAULT_STEPS, durationMs, isPay } from './constraints.ts';
import { hashJson, messageHash, termsHash } from './hash.ts';
import { MCP_PROTOCOL_VERSION, ORP_TOOLS, orpCapability } from './mcp.ts';
import { message, now as nowIso, ORP_VERSION, type CoreType, type MessageByType } from './message.ts';
import { benefits, GROW_KINDS, isPlace, isRepost, pitchOffer, pitchSpace, sides } from './pitch.ts';
import { score, type RepEvent } from './reputation.ts';
import { generateSigningKey, signMessage, verifyMessage, type SigningKey } from './sign.ts';
import { checkJoinRequest, checkMessage, checkSuggestRequest } from './validate.ts';

export interface HostOptions {
  origin: string;
  name?: string;
  harness?: boolean;
  sign?: boolean;
  web?: (url: string) => Promise<{ status: number; body: string }>;
  clock?: () => Date;
  /** Requests per minute per client address for the unauthenticated endpoints. */
  limits?: { suggest?: number; join?: number };
}

export interface Reply {
  status: number;
  body?: Message | InboxPage | Discovery;
}

type ObligationState = 'pending' | 'claimed' | 'verified' | 'failed' | 'revoked' | 'waived';
type Found = NonNullable<ReceiptBody['found']>;

interface PartyRecord {
  id: PartyId;
  token: string;
  constraints?: Constraints;
  spaces: Space[];
  jwk?: JsonWebKey;
  inbox: Message[];
  chain: Map<string, Hash>;
  sent: number[];
  events: RepEvent[];
  completed: number;
  /** A person has made the party theirs. Parties from join start unclaimed. */
  claimed: boolean;
  agent?: string;
}

interface Escalation {
  id: string;
  party: PartyId;
  step: Step;
  reasons: EscalationReason[];
  termsHash: Hash;
  since: string;
  deal?: string;
  offer?: string;
}

interface OfferRecord {
  id: string;
  offerer: PartyId;
  terms: Terms;
  hash: Hash;
  limit?: number;
  expires?: string;
  opened: number;
  listed: boolean;
  revoked: boolean;
  authority: Authority;
  waiting?: Escalation;
}

interface DealRecord {
  id: string;
  state: DealState;
  before?: DealState;
  offerer: PartyId;
  acceptor: PartyId;
  offer?: string;
  terms: Terms;
  hash: Hash;
  round: number;
  bound: Array<{ party: PartyId; authority: Authority; at: string }>;
  waiting?: Escalation;
  tracking: Tracking[];
  obligations: Array<{ id: ObligationId; state: ObligationState; due?: string; receipt?: string; proofUrl?: string; verifiedAt?: string }>;
  spaces: string[];
  activatedAt?: string;
  clicks: Map<ObligationId, Required<ClickCounts>>;
  conversions: Map<ObligationId, { count: number; refunds: number; revenue: number; currency: string }>;
  updated: string;
}

interface DisputeRecord {
  id: string;
  deal: string;
  obligation: ObligationId;
  opener: PartyId;
  state: 'open' | 'resolved' | 'withdrawn';
}

class OrpError extends Error {
  readonly code: string;
  readonly extra: Record<string, unknown>;

  constructor(code: string, message: string, extra: Record<string, unknown> = {}) {
    super(message);
    this.code = code;
    this.extra = extra;
  }
}

const STATUS: Record<string, number> = {
  invalid_message: 400,
  unsupported_version: 400,
  unsupported_type: 400,
  unsupported_extension: 400,
  stale: 400,
  unauthorized: 401,
  bad_signature: 401,
  forbidden: 403,
  step_off: 403,
  brand_rule: 403,
  not_found: 404,
  state_conflict: 409,
  terms_mismatch: 409,
  replay: 409,
  chain_broken: 409,
  rate_limited: 429,
  internal: 500,
};

const HOST_ONLY = new Set(['results', 'deal', 'outcome', 'consent', 'error']);
const PREVIEW_AGENTS = /twitterbot|facebookexternalhit|facebot|slackbot|linkedinbot|discordbot|telegrambot|whatsapp|skypeuripreview|applebot|redditbot|embedly|iframely|pinterest|vkshare|googleimageproxy|mastodon|bluesky|cardyb/i;
const BOT_AGENTS = /bot\b|crawler|spider|curl\/|wget\/|python-requests|go-http-client|headlesschrome|httpclient|okhttp|axios\/|node-fetch|^node$|^undici/i;
const MAX_SKEW_MS = 15 * 60_000;
const RATE_PER_MINUTE = 120;
const UNCLAIMED_OPEN_DEALS = 3;
const KEEP_RECHECK_MS = 30 * 86_400_000;
const PLACE_LIKE = new Set(['place', 'repost', 'comment']);
const OPEN_STATES: DealState[] = ['proposed', 'awaiting', 'active', 'disputed'];
// Spaces that hold something a person writes. Suggestions are for those.
const CONTENT_KINDS = new Set(['web.article', 'web.page', 'newsletter.issue', 'newsletter.dedicated', 'social.post', 'social.reply', 'social.repost', 'video.description', 'video.segment', 'video.comment', 'audio.segment', 'audio.notes', 'community.post', 'community.comment']);

const randomId = (prefix: string, bytes = 9) => `${prefix}_${randomBytes(bytes).toString('base64url')}`;
const emptyClicks = (): Required<ClickCounts> => ({ human: 0, bot: 0, preview: 0, test: 0 });

export function classifyClick(method: string, headers: Record<string, string | string[] | undefined>, query: URLSearchParams): keyof ClickCounts {
  if (query.get('orp_test') === '1') return 'test';
  const purpose = String(headers['sec-purpose'] ?? headers['purpose'] ?? '');
  if (method === 'HEAD' || /prefetch|prerender/i.test(purpose)) return 'preview';
  const agent = String(headers['user-agent'] ?? '').trim();
  if (!agent) return 'bot';
  if (PREVIEW_AGENTS.test(agent)) return 'preview';
  if (BOT_AGENTS.test(agent)) return 'bot';
  return 'human';
}

const STOP_WORDS = new Set('the and for with your you this that from are was but not all can our out get how what why who into about more than just have has its one two three four five six seven eight nine ten top best new use using will them they their there here also very most'.split(' '));

/** Lowercase words of three or more letters, plurals folded, common words dropped. */
export function wordsOf(text: string): Set<string> {
  const words = text.toLowerCase().match(/[a-z0-9]{3,}/g) ?? [];
  return new Set(words.filter((w) => !STOP_WORDS.has(w)).map((w) => (w.length > 3 && w.endsWith('s') && !w.endsWith('ss') ? w.slice(0, -1) : w)));
}

const matches = (words: Set<string>, text: string) => {
  const target = wordsOf(text);
  return [...words].filter((w) => target.has(w));
};

function offerText(terms: Terms): string {
  const briefs = terms.obligations.flatMap((o) => (isPlace(o) && o.content?.brief ? [o.content.brief] : []));
  return [terms.title, ...(terms.categories ?? []), ...briefs].join(' ');
}

function anchorsIn(html: string): Array<{ href: string; rel: string }> {
  const attr = (tag: string, name: string) => tag.match(new RegExp(`\\b${name}\\s*=\\s*(?:"([^"]*)"|'([^']*)'|([^\\s>]+))`, 'i'))?.slice(1).find((v) => v !== undefined) ?? '';
  return (html.match(/<a\b[^>]*>/gi) ?? []).map((tag) => ({ href: attr(tag, 'href'), rel: attr(tag, 'rel') }));
}

function sameTarget(href: string, destination: string): boolean {
  try {
    const [a, b] = [new URL(href), new URL(destination)];
    const key = (u: URL) => `${u.hostname.replace(/^www\./, '')}${u.pathname.replace(/\/$/, '')}`;
    return key(a) === key(b);
  } catch {
    return false;
  }
}

const escapeHtml = (text: string) => text.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);

function htmlPage(title: string, body: string): string {
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex"><title>${escapeHtml(title)}</title></head><body><main>${body}</main></body></html>`;
}

const STEP_WORDS: Record<Step, string> = {
  list: 'lists this offer',
  negotiate: 'sends this offer',
  accept: 'takes this deal',
  execute: 'does the work',
  amend: 'changes the terms',
  settle: 'settles payment',
  share: 'shares stats',
  revoke: 'ends the deal',
};

const REASON_WORDS: Record<EscalationReason, string> = {
  step_ask: 'this step needs your approval',
  over_budget: 'it is over the budget you set',
  over_threshold: 'it is worth more than your approval limit',
  new_counterparty: 'you have not dealt with them before',
  low_reputation: 'their reputation is below your minimum',
  category_outside: 'it is outside the categories you allow',
  space_outside: 'it uses a space you did not allow',
  kind_outside: 'it is a kind of deal you did not allow',
  duration_over: 'it runs longer than you allow',
  mandate_expired: 'your standing instructions have expired',
};

export class OrpHost {
  readonly origin: string;
  readonly hostId: PartyId;
  readonly endpoint: string;
  private readonly options: HostOptions;
  private readonly key?: SigningKey;
  private readonly parties = new Map<PartyId, PartyRecord>();
  private readonly tokens = new Map<string, PartyId>();
  private readonly offers = new Map<string, OfferRecord>();
  private readonly deals = new Map<string, DealRecord>();
  private readonly links = new Map<string, { deal: string; obligation: ObligationId; destination: string }>();
  private readonly escalations = new Map<string, Escalation>();
  private readonly disputes = new Map<string, DisputeRecord>();
  private readonly seen = new Map<string, { hash: Hash; reply: Reply }>();
  private readonly fakeWeb = new Map<string, { status: number; body: string }>();
  private readonly claims = new Map<string, PartyId>();
  private readonly sessions = new Map<string, string | undefined>();
  private readonly perAddress = new Map<string, number[]>();

  constructor(options: HostOptions) {
    this.options = options;
    this.origin = options.origin.replace(/\/$/, '');
    this.hostId = `orp:${new URL(this.origin).host}/host`;
    this.endpoint = `${this.origin}/orp`;
    if (options.sign) this.key = generateSigningKey(`host-${randomBytes(3).toString('hex')}`);
  }

  private clock(): Date {
    return this.options.clock?.() ?? new Date();
  }

  private ts(): string {
    return this.options.clock ? this.clock().toISOString().replace(/\.\d{3}Z$/, 'Z') : nowIso();
  }

  // ---- parties (account setup is host-specific; ORP does not standardise it)

  createParty(input: { constraints?: Constraints; spaces?: Space[]; jwk?: JsonWebKey; claimed?: boolean; agent?: string } = {}): { party: PartyId; token: string } {
    const id: PartyId = `orp:${new URL(this.origin).host}/${randomId('p', 12)}`;
    const token = randomBytes(24).toString('base64url');
    this.parties.set(id, {
      id, token, constraints: input.constraints, spaces: input.spaces ?? [], jwk: input.jwk, inbox: [], chain: new Map(), sent: [], events: [], completed: 0,
      claimed: input.claimed ?? true, ...(input.agent ? { agent: input.agent } : {}),
    });
    this.tokens.set(token, id);
    return { party: id, token };
  }

  setConstraints(party: PartyId, constraints: Constraints): void {
    this.party(party).constraints = constraints;
  }

  private party(id: PartyId): PartyRecord {
    const record = this.parties.get(id);
    if (!record) throw new OrpError('not_found', 'no such party');
    return record;
  }

  discovery(): Discovery {
    return {
      orp: [ORP_VERSION],
      host: this.hostId,
      name: this.options.name ?? 'ORP reference host',
      description: 'In-memory reference host from the ORP TypeScript SDK. Test use only.',
      endpoint: this.endpoint,
      suggest: `${this.endpoint}/suggest`,
      join: `${this.endpoint}/join`,
      mcp: { url: `${this.origin}/mcp`, transport: 'streamable-http', auth: ['bearer'], binding: true, tools: ORP_TOOLS.map((tool) => tool.name) },
      auth: { bearer: true },
      tracking: { links: true, codes: true, base: `${this.origin}/go/` },
      conformance: { levels: this.key ? ['L0', 'L1', 'L2', 'L3'] : ['L0', 'L1', 'L2'], profiles: this.options.harness ? ['test-harness'] : [] },
      capabilities: this.capabilities(),
      ...(this.key ? { keys: [{ kty: 'OKP' as const, crv: 'Ed25519', x: String(this.key.publicJwk.x), kid: this.key.kid, alg: 'EdDSA' as const, use: 'sig' as const }] } : {}),
    };
  }

  private capabilities(): Capabilities {
    return {
      messages: ['hello', 'discover', 'results', 'ask', 'offer', 'counter', 'accept', 'decline', 'deal', 'execute', 'receipt', 'outcome', 'revoke', 'consent', 'dispute', 'reputation', 'error'],
      proofs: ['link', 'proof_url', 'attestation'],
      tracking: { links: true, codes: true },
      signatures: { algs: ['EdDSA'], required: false },
      inbox: { poll: true },
    };
  }

  // ---- outgoing messages

  private hostMessage<T extends CoreType>(type: T, to: PartyId | undefined, body: MessageByType[T]['body'], extra: { deal?: string; re?: string } = {}): MessageByType[T] {
    const built = message(type, this.hostId, body, { ts: this.ts(), ...(to ? { to } : {}), ...extra });
    return this.key ? signMessage(built, this.key) : built;
  }

  private deliver(to: PartyId, msg: Message): void {
    this.parties.get(to)?.inbox.push(msg);
  }

  // ---- deal views

  private roleOf(deal: DealRecord, party: PartyId): Role | undefined {
    return deal.offerer === party ? 'offerer' : deal.acceptor === party ? 'acceptor' : undefined;
  }

  private obligor(deal: DealRecord, obligation: ObligationId): PartyId {
    const spec = deal.terms.obligations.find((o) => o.id === obligation);
    if (!spec) throw new OrpError('not_found', `no obligation ${obligation}`, { field: '/body/obligation' });
    return spec.by === 'offerer' ? deal.offerer : deal.acceptor;
  }

  private waitingView(waiting: Escalation, viewer: PartyId): Waiting {
    const shared: Waiting = { party: waiting.party, step: waiting.step, since: waiting.since };
    if (viewer !== waiting.party) return shared;
    return { ...shared, escalation: waiting.id, reasons: waiting.reasons, url: `${this.origin}/approve/${waiting.id}` };
  }

  private snapshot(deal: DealRecord, viewer: PartyId, re?: string): MessageByType['deal'] {
    const body: DealBody = {
      state: deal.state,
      offerer: deal.offerer,
      acceptor: deal.acceptor,
      ...(deal.offer ? { offer: deal.offer } : {}),
      terms: deal.terms,
      terms_hash: deal.hash,
      round: deal.round,
      bound: deal.bound,
      ...(deal.waiting ? { waiting: this.waitingView(deal.waiting, viewer) } : {}),
      ...(deal.tracking.length ? { tracking: deal.tracking } : {}),
      obligations: deal.obligations.map((o) => ({ id: o.id, state: o.state, ...(o.due ? { due: o.due } : {}), ...(o.receipt ? { receipt: o.receipt } : {}) })),
      updated: deal.updated,
    };
    const revealed = deal.state === 'active' || deal.state === 'completed' || deal.state === 'disputed';
    if (revealed && deal.spaces.length) {
      body.spaces = deal.spaces.flatMap((id) => [...this.parties.values()].flatMap((p) => p.spaces.filter((s) => s.id === id)));
    }
    return this.hostMessage('deal', viewer, body, { deal: deal.id, ...(re ? { re } : {}) });
  }

  private broadcast(deal: DealRecord, sender: PartyId, re: string): MessageByType['deal'] {
    deal.updated = this.ts();
    const other = deal.offerer === sender ? deal.acceptor : deal.offerer;
    // A proposal waiting for its own sender's person is not firm yet, so the
    // counterparty hears nothing until it is approved.
    const notFirmYet = deal.state === 'awaiting' && deal.waiting?.party === sender && deal.waiting.step === 'negotiate';
    if (!notFirmYet) this.deliver(other, this.snapshot(deal, other));
    const own = this.snapshot(deal, sender, re);
    this.deliver(sender, own);
    return own;
  }

  // ---- constraints

  private reputationOf(party: PartyId): number | null {
    return score(this.party(party).events, this.clock()).score;
  }

  private firstDeal(a: PartyId, b: PartyId): boolean {
    return ![...this.deals.values()].some((d) => d.state !== 'declined' && ((d.offerer === a && d.acceptor === b) || (d.offerer === b && d.acceptor === a)) && d.bound.length === 2);
  }

  private judge(party: PartyId, step: Step, role: Role, terms: Terms, counterparty?: PartyId, spaces: string[] = []) {
    const decision = decide(this.party(party).constraints, {
      step,
      role,
      terms,
      counterparty,
      counterpartyReputation: counterparty ? this.reputationOf(counterparty) : undefined,
      firstDeal: counterparty ? this.firstDeal(party, counterparty) : undefined,
      spaces,
      now: this.clock(),
    });
    if (decision.mode === 'off') throw new OrpError(decision.code, decision.detail, { step });
    return decision;
  }

  // execute and revoke happen after a person or mandate already bound the
  // deal, so only the step switch matters: off (or ask, which in 0.3 means
  // the owner does it in the host's own UI) refuses the agent.
  private requireStep(party: PartyId, step: 'execute' | 'revoke'): void {
    const mode = this.party(party).constraints?.steps?.[step] ?? DEFAULT_STEPS[step];
    if (step === 'execute' ? mode === 'off' : mode !== 'auto') throw new OrpError('step_off', `the owner does not let this agent ${step}`, { step });
  }

  private escalate(party: PartyId, step: Step, reasons: EscalationReason[], hash: Hash, link: { deal?: string; offer?: string }): Escalation {
    const escalation: Escalation = { id: randomId('esc', 12), party, step, reasons, termsHash: hash, since: this.ts(), ...link };
    this.escalations.set(escalation.id, escalation);
    return escalation;
  }

  private checkTerms(terms: Terms, claimed?: Hash): Hash {
    const hash = termsHash(terms);
    if (claimed && claimed !== hash) throw new OrpError('terms_mismatch', 'terms_hash does not match the terms', { field: '/body/terms_hash' });
    if (terms.obligations.some((o) => PLACE_LIKE.has(o.action)) && !terms.disclosure) {
      throw new OrpError('invalid_message', 'terms that place content need a disclosure label', { field: '/body/terms/disclosure' });
    }
    const ids = terms.obligations.map((o) => o.id);
    if (new Set(ids).size !== ids.length) throw new OrpError('invalid_message', 'obligation ids must be unique', { field: '/body/terms/obligations' });
    for (const o of terms.obligations) {
      if (!['place', 'repost', 'comment', 'pay', 'credit', 'share'].includes(o.action)) throw new OrpError('unsupported_type', `unknown obligation action ${o.action}`, { field: '/body/terms/obligations' });
      if (isRepost(o) && !o.quote && terms.obligations.some((pay) => isPay(pay) && pay.by !== o.by)) {
        throw new OrpError('invalid_message', 'a paid repost must be a quote, so it can carry the disclosure label', { field: '/body/terms/obligations' });
      }
    }
    return hash;
  }

  private checkOpenDeals(party: PartyRecord): void {
    if (party.claimed) return;
    const open = [...this.deals.values()].filter((d) => (d.offerer === party.id || d.acceptor === party.id) && OPEN_STATES.includes(d.state)).length;
    if (open >= UNCLAIMED_OPEN_DEALS) throw new OrpError('forbidden', `an unclaimed party can have ${UNCLAIMED_OPEN_DEALS} open deals; ask your person to claim it with the claim url from join`);
  }

  // ---- lifecycle

  private bind(deal: DealRecord, party: PartyId, authority: Authority): void {
    deal.bound = deal.bound.filter((b) => b.party !== party).concat({ party, authority, at: this.ts() });
    deal.waiting = undefined;
    if (deal.bound.length === 2) this.activate(deal);
    else deal.state = 'proposed';
  }

  private activate(deal: DealRecord): void {
    deal.state = 'active';
    deal.activatedAt = this.ts();
    const start = this.clock().getTime();
    deal.tracking = [];
    deal.obligations = deal.terms.obligations.map((o) => {
      const due = 'due' in o && typeof o.due === 'string' ? (o.due.startsWith('P') ? new Date(start + durationMs(o.due)).toISOString().replace(/\.\d{3}Z$/, 'Z') : o.due) : undefined;
      return { id: o.id, state: 'pending' as ObligationState, ...(due ? { due } : {}) };
    });
    for (const o of deal.terms.obligations) {
      const content = 'content' in o ? (o.content as { link?: boolean; code?: boolean; direct?: boolean } | undefined) : undefined;
      const destination = 'destination' in o && typeof o.destination === 'string' ? o.destination : undefined;
      if (!destination || content?.direct || (o.action !== 'place' && !(o.action === 'comment' && content?.link))) continue;
      const code = randomBytes(6).toString('base64url');
      this.links.set(code, { deal: deal.id, obligation: o.id, destination });
      deal.tracking.push({ obligation: o.id, link: `${this.origin}/go/${code}`, ...(content?.code ? { code } : {}), destination, status: 'live' });
      deal.clicks.set(o.id, emptyClicks());
    }
  }

  // A deal with pay per click or per conversion runs until it ends or is
  // revoked; any other deal completes when every obligation is done.
  private settleIfDone(deal: DealRecord): void {
    if (deal.state !== 'active') return;
    const ongoing = deal.terms.obligations.filter(isPay).some((o) => o.per === 'click' || o.per === 'conversion');
    if (ongoing) return;
    if (deal.obligations.every((o) => o.state === 'verified' || o.state === 'waived')) {
      deal.state = 'completed';
      this.party(deal.offerer).completed += 1;
      this.party(deal.acceptor).completed += 1;
    }
  }

  private recordEvent(subject: PartyId, counterparty: PartyId, outcome: RepEvent['outcome'], method?: ProofMethod): void {
    this.party(subject).events.push({ counterparty, counterpartyScore: this.reputationOf(counterparty), outcome, method, at: this.clock() });
  }

  // A verified obligation only changes again when a recheck finds the work
  // gone before keep ran out.
  private receipt(deal: DealRecord, obligation: ObligationId, body: Omit<ReceiptBody, 'obligation' | 'at'>, re?: string, recheck = false): void {
    const record = deal.obligations.find((o) => o.id === obligation);
    if (!record || record.state === 'revoked' || (record.state === 'verified' && !recheck)) return;
    const doer = this.obligor(deal, obligation);
    const other = doer === deal.offerer ? deal.acceptor : deal.offerer;
    const full: ReceiptBody = { obligation, at: this.ts(), ...body };
    let firstId = '';
    for (const to of [doer, other]) {
      const msg = this.hostMessage('receipt', to, full, { deal: deal.id, ...(re ? { re } : {}) });
      firstId ||= msg.id;
      this.deliver(to, msg);
    }
    record.receipt = firstId;
    if (body.result === 'verified') {
      record.state = 'verified';
      record.verifiedAt = full.at;
      this.recordEvent(doer, other, 'verified', body.method);
    } else if (body.result === 'failed') {
      record.state = 'failed';
      this.recordEvent(doer, other, 'failed', body.method);
    }
    this.settleIfDone(deal);
  }

  private async fetchPublic(url: string): Promise<{ status: number; body: string }> {
    const fake = this.fakeWeb.get(url);
    if (fake) return fake;
    if (this.options.web) return this.options.web(url);
    return { status: 0, body: '' };
  }

  // What a public page proves about one obligation. A direct link must carry
  // rel="sponsored" or rel="nofollow": exchanges are for readers, not rankings.
  // A plain repost cannot show a label; a quote must.
  private inspect(deal: DealRecord, spec: Obligation, html: string): { found: Found; result: ReceiptBody['result']; reason?: ReceiptBody['reason'] } {
    const found: Found = {};
    const tracking = deal.tracking.find((t) => t.obligation === spec.id);
    const direct = isPlace(spec) && spec.content?.direct === true;
    if ('target' in spec && typeof spec.target === 'string') found.target = html.includes(spec.target);
    if (direct && isPlace(spec) && spec.destination) {
      const anchor = anchorsIn(html).find((a) => sameTarget(a.href, spec.destination!));
      found.destination = Boolean(anchor);
      if (anchor) found.rel = /\b(sponsored|nofollow)\b/i.test(anchor.rel);
    }
    if (tracking) found.link = html.includes(tracking.link);
    if (tracking?.code) found.code = html.includes(tracking.code);
    if (deal.terms.disclosure) found.disclosure = html.toLowerCase().includes(deal.terms.disclosure.label.toLowerCase());
    const core = spec.action === 'repost' || (spec.action === 'comment' && found.target !== undefined) ? found.target : direct ? found.destination : (found.link ?? found.code);
    if (!core) return { found, result: 'failed', reason: spec.action === 'repost' ? 'not_found' : 'link_missing' };
    if (direct && found.rel === false) return { found, result: 'partial', reason: 'rel_missing' };
    const labelled = spec.action !== 'repost' || (isRepost(spec) && spec.quote === true);
    if (labelled && found.disclosure === false) return { found, result: 'partial', reason: 'disclosure_missing' };
    return { found, result: 'verified' };
  }

  private async verifyProof(deal: DealRecord, obligation: ObligationId, proofUrl: string, re: string): Promise<void> {
    const spec = deal.terms.obligations.find((o) => o.id === obligation)!;
    const page = await this.fetchPublic(proofUrl);
    if (page.status === 0) return;
    if (page.status !== 200) {
      this.receipt(deal, obligation, { result: 'failed', method: 'proof_url', url: proofUrl, reason: page.status === 404 || page.status === 410 ? 'not_found' : 'unreachable' }, re);
      return;
    }
    const { found, result, reason } = this.inspect(deal, spec, page.body);
    deal.obligations.find((o) => o.id === obligation)!.proofUrl = proofUrl;
    const keep = 'keep' in spec && typeof spec.keep === 'string' ? durationMs(spec.keep) : 0;
    const nextCheck = result === 'verified' && keep ? new Date(this.clock().getTime() + Math.min(keep, KEEP_RECHECK_MS)).toISOString().replace(/\.\d{3}Z$/, 'Z') : undefined;
    this.receipt(deal, obligation, { result, method: 'proof_url', url: proofUrl, found, ...(reason ? { reason } : {}), ...(nextCheck ? { next_check: nextCheck } : {}) }, re);
  }

  /** Look again at verified proof URLs whose keep has not run out. Work taken down early fails. */
  async recheck(dealId: string): Promise<void> {
    const deal = this.deals.get(dealId);
    if (!deal) throw new OrpError('not_found', 'no such deal');
    for (const record of deal.obligations) {
      const spec = deal.terms.obligations.find((o) => o.id === record.id);
      if (!spec || record.state !== 'verified' || !record.proofUrl || !record.verifiedAt) continue;
      const keep = 'keep' in spec && typeof spec.keep === 'string' ? durationMs(spec.keep) : 0;
      if (this.clock().getTime() > Date.parse(record.verifiedAt) + keep) continue;
      const page = await this.fetchPublic(record.proofUrl);
      if (page.status === 0) continue;
      const still = page.status === 200 ? this.inspect(deal, spec, page.body) : undefined;
      if (still?.result === 'verified') continue;
      this.receipt(deal, record.id, { result: 'failed', method: 'proof_url', url: record.proofUrl, ...(still ? { found: still.found } : {}), reason: 'removed' }, undefined, true);
    }
  }

  // Obligations past due fail. The host runs this whenever it touches a deal.
  private sweep(deal: DealRecord): void {
    if (deal.state !== 'active') return;
    const now = this.clock().getTime();
    for (const record of deal.obligations) {
      if ((record.state !== 'pending' && record.state !== 'claimed') || !record.due || Date.parse(record.due) > now) continue;
      const spec = deal.terms.obligations.find((o) => o.id === record.id)!;
      const method = ('proof' in spec && Array.isArray(spec.proof) ? (spec.proof[0] as ProofMethod | undefined) : undefined) ?? 'attestation';
      const clicks = isPlace(spec) && spec.clicks ? deal.clicks.get(record.id)?.human : undefined;
      this.receipt(deal, record.id, { result: 'failed', method, reason: 'too_late', ...(clicks !== undefined ? { found: { link: clicks > 0, clicks } } : {}) });
    }
  }

  // ---- inbound

  async receive(token: string | undefined, raw: unknown): Promise<Reply> {
    const sender = token ? this.tokens.get(token) : undefined;
    const candidate = raw && typeof raw === 'object' ? (raw as { id?: unknown }).id : undefined;
    const rawId = typeof candidate === 'string' && /^[A-Za-z0-9_.:-]{8,128}$/.test(candidate) ? candidate : undefined;
    try {
      if (!sender) throw new OrpError('unauthorized', 'a valid bearer token is required');
      const checked = checkMessage(raw);
      if (!checked.ok) throw new OrpError('invalid_message', checked.problems.slice(0, 3).map((p) => `${p.path} ${p.message}`).join('; '), { field: checked.problems[0]?.path });
      const msg = checked.value;
      if (msg.type !== 'hello' && msg.orp !== ORP_VERSION) throw new OrpError('unsupported_version', `this host speaks ${ORP_VERSION}`, { supported: [ORP_VERSION] });
      const self = msg.type === 'hello' && msg.from === `orp:${new URL(this.origin).host}/self`;
      if (msg.from !== sender && !self) throw new OrpError('forbidden', 'from does not match the party of this token', { field: '/from' });

      const key = `${sender}|${msg.id}`;
      const fingerprint = hashJson(msg);
      const previous = this.seen.get(key);
      if (previous) {
        if (previous.hash === fingerprint) return previous.reply;
        throw new OrpError('replay', 'this message id was already used with different content', { field: '/id' });
      }
      const skew = Math.abs(this.clock().getTime() - Date.parse(msg.ts));
      if (skew > MAX_SKEW_MS) throw new OrpError('stale', 'ts is more than 15 minutes from the host clock', { field: '/ts' });
      if (msg.critical?.length) throw new OrpError('unsupported_extension', `unsupported critical extension ${msg.critical[0]}`);

      const party = this.party(sender);
      if (party.jwk || msg.sig) {
        const verified = verifyMessage(msg, (kid) => (party.jwk && (!party.jwk.kid || party.jwk.kid === kid) ? party.jwk : undefined));
        if (!verified.ok) throw new OrpError('bad_signature', `signature ${verified.reason}`, { field: '/sig' });
      }
      if (msg.deal && msg.prev !== undefined) {
        const expected = party.chain.get(msg.deal);
        if (msg.prev !== expected) throw new OrpError('chain_broken', 'prev is not the hash of your previous message in this deal', { field: '/prev' });
      }
      const minute = this.clock().getTime() - 60_000;
      party.sent = party.sent.filter((t) => t > minute);
      if (party.sent.length >= RATE_PER_MINUTE) throw new OrpError('rate_limited', 'too many messages', { retry_after: 60, retryable: true });
      party.sent.push(this.clock().getTime());

      const reply = await this.dispatch(party, msg);
      const dealId = msg.deal ?? (reply.body && 'deal' in reply.body && typeof reply.body.deal === 'string' ? reply.body.deal : undefined);
      if (dealId) party.chain.set(dealId, messageHash(msg));
      this.seen.set(key, { hash: fingerprint, reply });
      return reply;
    } catch (error) {
      const failure = error instanceof OrpError ? error : new OrpError('internal', 'the host failed to process the message');
      if (!(error instanceof OrpError)) console.error(error);
      const body = { code: failure.code, message: failure.message, ...failure.extra } as MessageByType['error']['body'];
      return { status: STATUS[failure.code] ?? 400, body: this.hostMessage('error', sender, body, rawId ? { re: rawId } : {}) };
    }
  }

  private deal(party: PartyRecord, id: string | undefined): DealRecord {
    const deal = id ? this.deals.get(id) : undefined;
    if (!deal || !this.roleOf(deal, party.id)) throw new OrpError('not_found', 'no such deal for this party', { field: '/deal' });
    this.sweep(deal);
    return deal;
  }

  private async dispatch(party: PartyRecord, msg: Message): Promise<Reply> {
    if (HOST_ONLY.has(msg.type)) throw new OrpError('forbidden', `only a host sends ${msg.type}${msg.type === 'consent' ? ': a person approves through the host, never an agent' : ''}`);
    switch (msg.type) {
      case 'hello': {
        if (!msg.body.versions.includes(ORP_VERSION)) throw new OrpError('unsupported_version', `this host speaks ${ORP_VERSION}`, { supported: [ORP_VERSION] });
        return { status: 200, body: this.hostMessage('hello', party.id, { versions: [ORP_VERSION], party: party.id, capabilities: this.capabilities(), limits: { messages_per_minute: RATE_PER_MINUTE, max_body_bytes: 65536 } }, { re: msg.id }) };
      }
      case 'discover':
        return { status: 200, body: this.discover(party, msg) };
      case 'ask': {
        if (!msg.to || !this.parties.has(msg.to) || msg.to === party.id) throw new OrpError('not_found', 'no such party', { field: '/to' });
        this.deliver(msg.to, msg);
        return { status: 202 };
      }
      case 'offer':
        return this.offer(party, msg);
      case 'counter':
        return this.counter(party, msg);
      case 'accept':
        return this.accept(party, msg);
      case 'decline': {
        const deal = this.deal(party, msg.deal);
        if (deal.state !== 'proposed' && deal.state !== 'awaiting') throw new OrpError('state_conflict', `deal is ${deal.state}`);
        deal.state = 'declined';
        deal.waiting = undefined;
        return { status: 200, body: this.broadcast(deal, party.id, msg.id) };
      }
      case 'execute':
        return this.execute(party, msg);
      case 'receipt':
        return this.attest(party, msg);
      case 'revoke':
        return this.revoke(party, msg);
      case 'dispute':
        return this.dispute(party, msg);
      case 'reputation': {
        if ('score' in msg.body) throw new OrpError('forbidden', 'only a host attests reputation');
        const subject = this.party(msg.body.subject);
        const result = score(subject.events, this.clock());
        const valid = new Date(this.clock().getTime() + 86_400_000).toISOString().replace(/\.\d{3}Z$/, 'Z');
        return { status: 200, body: this.hostMessage('reputation', party.id, { subject: subject.id, algorithm: 'orp-rep-1', score: result.score, components: result.components, computed_at: this.ts(), valid_until: valid }, { re: msg.id }) };
      }
      default:
        throw new OrpError('unsupported_type', `unsupported message type ${msg.type}`);
    }
  }

  private discover(party: PartyRecord, msg: MessageByType['discover']): MessageByType['results'] {
    const { want, query, context, filters = {}, limit = 10 } = msg.body;
    const words = wordsOf(`${query ?? ''} ${context?.text ?? ''}`);
    const items: Result[] = [];
    if (want === 'offers') {
      for (const offer of this.offers.values()) {
        if (!this.open(offer) || offer.offerer === party.id) continue;
        if (filters.kinds && (!offer.terms.kind || !filters.kinds.includes(offer.terms.kind))) continue;
        if (filters.categories && !(offer.terms.categories ?? []).some((c) => filters.categories!.some((f) => c === f || c.startsWith(`${f}.`)))) continue;
        const trust = this.trustOf(offer.offerer);
        if (filters.min_reputation !== undefined && (trust.reputation ?? 0) < filters.min_reputation) continue;
        const hits = matches(words, offerText(offer.terms));
        items.push({ ...this.offerResult(offer, hits), trust, pool: 'public' });
      }
    } else {
      for (const other of this.parties.values()) {
        if (other.id === party.id) continue;
        for (const space of other.spaces) {
          if (filters.space_kinds && !filters.space_kinds.includes(space.kind)) continue;
          if (filters.kinds && !(space.accepts ?? []).some((k) => filters.kinds!.includes(k))) continue;
          const { locator: _hidden, ...listed } = space;
          items.push({ space: listed, party: other.id, fit: { score: 50, reasons: [`Accepts ${(space.accepts ?? ['deals']).join(', ')}.`] }, trust: this.trustOf(other.id) });
        }
      }
    }
    items.sort((a, b) => b.fit.score - a.fit.score);
    const page = items.slice(0, limit);
    return this.hostMessage('results', party.id, { items: page, ...(page.length ? {} : { empty_reason: 'no_match' as const }) }, { re: msg.id });
  }

  private open(offer: OfferRecord): boolean {
    if (!offer.listed || offer.revoked) return false;
    if (offer.expires && Date.parse(offer.expires) < this.clock().getTime()) return false;
    return !(offer.limit && offer.opened >= offer.limit);
  }

  private trustOf(id: PartyId): Trust {
    const party = this.party(id);
    return { reputation: this.reputationOf(id), completed_deals: party.completed, ...(party.claimed ? { verified: ['person' as const] } : {}) };
  }

  private offerResult(offer: OfferRecord, hits: string[]): Result {
    const evidence = offer.terms.obligations.flatMap((o) => ('destination' in o && typeof o.destination === 'string' ? [o.destination] : []));
    return {
      offer: { id: offer.id, terms: offer.terms, terms_hash: offer.hash, ...(offer.expires ? { expires: offer.expires } : {}) },
      party: offer.offerer,
      fit: { score: Math.min(100, 40 + 15 * hits.length), reasons: [hits.length ? `Matches ${hits.slice(0, 5).join(', ')}.` : 'Open standing offer.'], ...(evidence.length ? { evidence: evidence.slice(0, 5) } : {}) },
      trust: this.trustOf(offer.offerer),
    };
  }

  private allow(bucket: 'suggest' | 'join', address: string): boolean {
    const perMinute = this.options.limits?.[bucket] ?? (bucket === 'join' ? 10 : 60);
    const key = `${bucket}|${address}`;
    const now = this.clock().getTime();
    const recent = (this.perAddress.get(key) ?? []).filter((t) => t > now - 60_000);
    const ok = recent.length < perMinute;
    if (ok) recent.push(now);
    this.perAddress.set(key, recent);
    return ok;
  }

  private httpError(code: string, text: string, extra: Record<string, unknown> = {}): [number, MessageByType['error']] {
    return [STATUS[code] ?? 400, this.hostMessage('error', undefined, { code, message: text, ...extra } as MessageByType['error']['body'])];
  }

  /**
   * One call for any agent that writes for someone: deals its person could
   * take now, each with a pitch. No credentials. The text is matched here and
   * dropped: it is never stored, logged or shown to anyone else.
   */
  suggest(raw: unknown, address = 'local'): [number, SuggestResponse | MessageByType['error']] {
    if (!this.allow('suggest', address)) return this.httpError('rate_limited', 'too many suggestions from this address', { retry_after: 60, retryable: true });
    const checked = checkSuggestRequest(raw);
    if (!checked.ok) return this.httpError('invalid_message', checked.problems.slice(0, 3).map((p) => `${p.path} ${p.message}`).join('; '), { field: checked.problems[0]?.path });
    const { text, space_kind: kind, want = ['earn', 'grow'], kinds, limit = 3 } = checked.value;
    const words = wordsOf(text);
    const items: Result[] = [];
    for (const offer of this.offers.values()) {
      if (!this.open(offer)) continue;
      if (kinds && (!offer.terms.kind || !kinds.includes(offer.terms.kind))) continue;
      if (![...benefits(offer.terms)].some((b) => want.includes(b))) continue;
      const allowed = offer.terms.obligations.flatMap((o) => (o.by === 'acceptor' && isPlace(o) && o.space?.kinds ? [o.space.kinds] : []));
      if (allowed.some((spaceKinds) => (kind ? !spaceKinds.includes(kind) : !spaceKinds.some((k) => CONTENT_KINDS.has(k))))) continue;
      const hits = matches(words, offerText(offer.terms));
      if (hits.length) items.push({ ...this.offerResult(offer, hits), pitch: pitchOffer(offer.terms) });
    }
    if (want.includes('grow')) {
      for (const other of this.parties.values()) {
        for (const space of other.spaces) {
          if (!(space.accepts ?? []).some((k) => GROW_KINDS.includes(k) && (!kinds || kinds.includes(k)))) continue;
          const hits = matches(words, `${space.title ?? ''} ${(space.topics ?? []).join(' ')}`);
          if (!hits.length) continue;
          const { locator: _hidden, ...listed } = space;
          items.push({ space: listed, party: other.id, fit: { score: Math.min(100, 30 + 15 * hits.length), reasons: [`Trades with posts about ${hits.slice(0, 3).join(', ')}.`] }, trust: this.trustOf(other.id), pitch: pitchSpace(space) });
        }
      }
    }
    items.sort((a, b) => b.fit.score - a.fit.score);
    const page = items.slice(0, limit);
    return [200, { items: page, ...(page.length ? {} : { empty_reason: 'no_match' as const }), join: `${this.endpoint}/join` }];
  }

  /**
   * A pseudonymous party and token, with no account, email, name or website.
   * Until a person claims it, every step that binds waits for their tap.
   */
  join(raw: unknown, address = 'local'): [number, JoinResponse | MessageByType['error']] {
    if (!this.allow('join', address)) return this.httpError('rate_limited', 'too many joins from this address', { retry_after: 60, retryable: true });
    const input = raw ?? {};
    const checked = checkJoinRequest(input);
    if (!checked.ok) {
      const extra = input && typeof input === 'object' && !Array.isArray(input) ? Object.keys(input).filter((key) => key !== 'agent' && key !== 'key') : [];
      if (extra.length) return this.httpError('invalid_message', `join asks for nothing personal: remove ${extra.join(', ')}`, { field: `/${extra[0]}` });
      return this.httpError('invalid_message', checked.problems.slice(0, 3).map((p) => `${p.path} ${p.message}`).join('; '), { field: checked.problems[0]?.path });
    }
    const { agent, key } = checked.value;
    const { party, token } = this.createParty({ claimed: false, ...(key ? { jwk: key as JsonWebKey } : {}), ...(agent ? { agent: agent.name } : {}) });
    const secret = randomBytes(24).toString('base64url');
    this.claims.set(secret, party);
    return [200, { party, token, claim: `${this.origin}/claim/${secret}`, steps: { ...DEFAULT_STEPS }, limits: { open_deals: UNCLAIMED_OPEN_DEALS, messages_per_minute: RATE_PER_MINUTE } }];
  }

  /** A person makes the party theirs (in production: a passkey, no email) and may set limits. */
  claim(secret: string, constraints?: Constraints): PartyId {
    const id = this.claims.get(secret);
    if (!id) throw new OrpError('not_found', 'no such claim');
    const party = this.party(id);
    party.claimed = true;
    if (constraints) party.constraints = constraints;
    this.claims.delete(secret);
    return id;
  }

  /** The page behind waiting.url: what the person is asked, in words. No sign-in to read it. */
  approvalPage(id: string): [number, string] {
    const escalation = this.escalations.get(id);
    if (!escalation) return [404, htmlPage('Nothing to approve', '<h1>Nothing to approve</h1><p>This request was already decided, or it does not exist.</p>')];
    const deal = escalation.deal ? this.deals.get(escalation.deal) : undefined;
    const terms = deal?.terms ?? this.offers.get(escalation.offer ?? '')?.terms;
    if (!terms) return [404, htmlPage('Nothing to approve', '<h1>Nothing to approve</h1>')];
    const role: Role = deal ? this.roleOf(deal, escalation.party)! : 'offerer';
    const { you, get } = sides(terms, role);
    const agent = this.parties.get(escalation.party)?.agent;
    const items = (lines: string[]) => `<ul>${lines.map((line) => `<li>${escapeHtml(line)}</li>`).join('')}</ul>`;
    const body = [
      `<h1>${escapeHtml(terms.title)}</h1>`,
      `<p>Your agent${agent ? ` (${escapeHtml(agent)})` : ''} is waiting for you before it ${STEP_WORDS[escalation.step]}, because ${escalation.reasons.map((r) => REASON_WORDS[r]).join(', and ')}.</p>`,
      you.length ? `<h2>You would</h2>${items(you)}` : '',
      get.length ? `<h2>You get</h2>${items(get)}` : '',
      terms.disclosure ? `<p>Readers see the label &ldquo;${escapeHtml(terms.disclosure.label)}&rdquo;.</p>` : '',
      `<form method="post" action="/approve/${escapeHtml(id)}"><button name="decision" value="approved">Approve with a passkey</button> <button name="decision" value="refused">Refuse</button></form>`,
      `<p>No account, email or password. Approving records your consent to these exact terms (${escapeHtml(escalation.termsHash)}).</p>`,
    ];
    return [200, htmlPage(`Approve: ${terms.title}`, body.join(''))];
  }

  claimPage(secret: string): [number, string] {
    if (!this.claims.has(secret)) return [404, htmlPage('Already claimed', '<h1>Already claimed</h1><p>This link was used, or it does not exist.</p>')];
    return [200, htmlPage('Make these deals yours', [
      '<h1>Make these deals yours</h1>',
      '<p>Create a passkey on this device. You get one place to approve deals, set limits (budget, categories, brand rules) and choose how you get paid.</p>',
      `<form method="post" action="/claim/${escapeHtml(secret)}"><button>Create a passkey</button></form>`,
      '<p>No email, no password.</p>',
    ].join(''))];
  }

  private offer(party: PartyRecord, msg: MessageByType['offer']): Reply {
    const { terms } = msg.body;
    const hash = this.checkTerms(terms, msg.body.terms_hash);
    if (!msg.to) {
      const decision = this.judge(party.id, 'list', 'offerer', terms);
      const offer: OfferRecord = { id: randomId('ofr', 6), offerer: party.id, terms, hash, limit: msg.body.limit, expires: msg.body.expires, opened: 0, listed: false, revoked: false, authority: { type: 'mandate' } };
      if (decision.mode === 'ask') offer.waiting = this.escalate(party.id, 'list', decision.reasons, hash, { offer: offer.id });
      else offer.listed = true;
      this.offers.set(offer.id, offer);
      return { status: 200, body: this.listing(offer, msg.id) };
    }
    if (msg.to === party.id || !this.parties.has(msg.to)) throw new OrpError('not_found', 'no such party', { field: '/to' });
    this.checkOpenDeals(party);
    const decision = this.judge(party.id, 'negotiate', 'offerer', terms, msg.to);
    const deal: DealRecord = {
      id: randomId('deal', 6), state: 'proposed', offerer: party.id, acceptor: msg.to, terms, hash, round: 1, bound: [], tracking: [], obligations: [], spaces: [], clicks: new Map(), conversions: new Map(), updated: this.ts(),
    };
    this.deals.set(deal.id, deal);
    this.proposeOrWait(deal, party.id, decision);
    return { status: 200, body: this.broadcast(deal, party.id, msg.id) };
  }

  private listing(offer: OfferRecord, re?: string): MessageByType['offer'] {
    const body: MessageByType['offer']['body'] = { id: offer.id, terms: offer.terms, terms_hash: offer.hash };
    if (offer.limit) body.limit = offer.limit;
    if (offer.expires) body.expires = offer.expires;
    if (offer.waiting) body.waiting = this.waitingView(offer.waiting, offer.offerer);
    return this.hostMessage('offer', offer.offerer, body, re ? { re } : {});
  }

  private proposeOrWait(deal: DealRecord, sender: PartyId, decision: { mode: 'auto' } | { mode: 'ask'; reasons: EscalationReason[] }): void {
    if (decision.mode === 'ask') {
      deal.state = 'awaiting';
      deal.waiting = this.escalate(sender, 'negotiate', decision.reasons, deal.hash, { deal: deal.id });
    } else {
      this.bind(deal, sender, { type: 'mandate' });
    }
  }

  private counter(party: PartyRecord, msg: MessageByType['counter']): Reply {
    const deal = this.deal(party, msg.deal);
    if (deal.state !== 'proposed') throw new OrpError('state_conflict', `deal is ${deal.state}`);
    if (deal.round >= 20) throw new OrpError('state_conflict', 'too many rounds');
    const hash = this.checkTerms(msg.body.terms, msg.body.terms_hash);
    const role = this.roleOf(deal, party.id)!;
    const other = role === 'offerer' ? deal.acceptor : deal.offerer;
    const decision = this.judge(party.id, 'negotiate', role, msg.body.terms, other);
    deal.terms = msg.body.terms;
    deal.hash = hash;
    deal.round += 1;
    deal.bound = [];
    this.proposeOrWait(deal, party.id, decision);
    return { status: 200, body: this.broadcast(deal, party.id, msg.id) };
  }

  private accept(party: PartyRecord, msg: MessageByType['accept']): Reply {
    const spaces = (msg.body.spaces ?? []).map((s) => s.space);
    for (const space of spaces) if (!party.spaces.some((s) => s.id === space)) throw new OrpError('not_found', `no space ${space} for this party`, { field: '/body/spaces' });
    let deal: DealRecord;
    if (msg.deal) {
      deal = this.deal(party, msg.deal);
      if (deal.state !== 'proposed') throw new OrpError('state_conflict', `deal is ${deal.state}`);
      if (deal.bound.some((b) => b.party === party.id)) throw new OrpError('state_conflict', 'already bound to these terms');
    } else {
      const offer = this.offers.get(msg.body.offer ?? '');
      if (!offer || !offer.listed) throw new OrpError('not_found', 'no such standing offer', { field: '/body/offer' });
      if (offer.revoked || (offer.expires && Date.parse(offer.expires) < this.clock().getTime())) throw new OrpError('state_conflict', 'the offer is closed');
      if (offer.limit && offer.opened >= offer.limit) throw new OrpError('state_conflict', 'the offer is full');
      if (offer.offerer === party.id) throw new OrpError('forbidden', 'you cannot accept your own offer');
      if (msg.body.terms_hash !== offer.hash) throw new OrpError('terms_mismatch', 'terms_hash does not match the offer', { field: '/body/terms_hash' });
      this.checkOpenDeals(party);
      deal = {
        id: randomId('deal', 6), state: 'proposed', offerer: offer.offerer, acceptor: party.id, offer: offer.id, terms: offer.terms, hash: offer.hash, round: 1,
        bound: [{ party: offer.offerer, authority: offer.authority, at: this.ts() }], tracking: [], obligations: [], spaces: [], clicks: new Map(), conversions: new Map(), updated: this.ts(),
      };
    }
    if (msg.body.terms_hash !== deal.hash) throw new OrpError('terms_mismatch', 'terms_hash does not match the current terms', { field: '/body/terms_hash' });
    const role = this.roleOf(deal, party.id)!;
    const other = role === 'offerer' ? deal.acceptor : deal.offerer;
    const decision = this.judge(party.id, 'accept', role, deal.terms, other, spaces);
    if (!this.deals.has(deal.id)) {
      this.deals.set(deal.id, deal);
      this.offers.get(deal.offer!)!.opened += 1;
    }
    deal.spaces = [...new Set([...deal.spaces, ...spaces])];
    if (decision.mode === 'ask') {
      deal.state = 'awaiting';
      deal.waiting = this.escalate(party.id, 'accept', decision.reasons, deal.hash, { deal: deal.id });
    } else {
      this.bind(deal, party.id, { type: 'mandate' });
    }
    return { status: 200, body: this.broadcast(deal, party.id, msg.id) };
  }

  private async execute(party: PartyRecord, msg: MessageByType['execute']): Promise<Reply> {
    const deal = this.deal(party, msg.deal);
    if (deal.state !== 'active') throw new OrpError('state_conflict', `deal is ${deal.state}`);
    if (this.obligor(deal, msg.body.obligation) !== party.id) throw new OrpError('forbidden', 'only the obligor claims an obligation');
    this.requireStep(party.id, 'execute');
    const record = deal.obligations.find((o) => o.id === msg.body.obligation)!;
    if (record.state === 'verified' || record.state === 'revoked' || record.state === 'waived') throw new OrpError('state_conflict', `obligation is ${record.state}`);
    record.state = 'claimed';
    if (msg.body.proof_url) await this.verifyProof(deal, record.id, msg.body.proof_url, msg.id);
    return { status: 200, body: this.broadcast(deal, party.id, msg.id) };
  }

  private attest(party: PartyRecord, msg: MessageByType['receipt']): Reply {
    const deal = this.deal(party, msg.deal);
    if (msg.body.method !== 'attestation') throw new OrpError('forbidden', 'parties attest; other proof methods are the host\u2019s');
    if (this.obligor(deal, msg.body.obligation) === party.id) throw new OrpError('forbidden', 'you cannot attest your own work');
    if (deal.state !== 'active' && deal.state !== 'disputed') throw new OrpError('state_conflict', `deal is ${deal.state}`);
    const { obligation: _o, at: _a, ...rest } = msg.body;
    this.receipt(deal, msg.body.obligation, rest, msg.id);
    return { status: 200, body: this.broadcast(deal, party.id, msg.id) };
  }

  private revoke(party: PartyRecord, msg: MessageByType['revoke']): Reply {
    const { scope } = msg.body;
    this.requireStep(party.id, 'revoke');
    if (scope === 'offer') {
      const offer = this.offers.get(msg.body.offer ?? '');
      if (!offer || offer.offerer !== party.id) throw new OrpError('not_found', 'no such offer', { field: '/body/offer' });
      offer.revoked = true;
      return { status: 202 };
    }
    const deal = this.deal(party, msg.deal);
    if (!['proposed', 'awaiting', 'active', 'disputed'].includes(deal.state)) throw new OrpError('state_conflict', `deal is ${deal.state}`);
    const stop = (id: ObligationId) => {
      const record = deal.obligations.find((o) => o.id === id);
      if (record && (record.state === 'pending' || record.state === 'claimed')) record.state = 'revoked';
      for (const t of deal.tracking) if (t.obligation === id) t.status = 'plain';
    };
    if (scope === 'obligation') {
      if (!deal.terms.obligations.some((o) => o.id === msg.body.obligation)) throw new OrpError('not_found', 'no such obligation', { field: '/body/obligation' });
      stop(msg.body.obligation!);
    } else {
      deal.terms.obligations.forEach((o) => stop(o.id));
      deal.state = 'revoked';
      deal.waiting = undefined;
    }
    return { status: 200, body: this.broadcast(deal, party.id, msg.id) };
  }

  private dispute(party: PartyRecord, msg: MessageByType['dispute']): Reply {
    const deal = this.deal(party, msg.deal);
    const body = msg.body;
    const other = deal.offerer === party.id ? deal.acceptor : deal.offerer;
    if (body.action === 'resolve') throw new OrpError('forbidden', 'the host or its named resolver resolves disputes');
    if (body.action === 'open') {
      if (deal.state !== 'active' && deal.state !== 'completed') throw new OrpError('state_conflict', `deal is ${deal.state}`);
      this.obligor(deal, body.obligation!);
      const record: DisputeRecord = { id: randomId('dsp', 6), deal: deal.id, obligation: body.obligation!, opener: party.id, state: 'open' };
      this.disputes.set(record.id, record);
      deal.before = deal.state;
      deal.state = 'disputed';
      deal.updated = this.ts();
      const relayed = { ...body, dispute: record.id };
      this.deliver(other, this.hostMessage('dispute', other, relayed, { deal: deal.id }));
      const own = this.hostMessage('dispute', party.id, relayed, { deal: deal.id, re: msg.id });
      this.deliver(party.id, own);
      return { status: 200, body: own };
    }
    const record = this.disputes.get(body.dispute ?? '');
    if (!record || record.deal !== deal.id) throw new OrpError('not_found', 'no such dispute', { field: '/body/dispute' });
    if (record.state !== 'open') throw new OrpError('state_conflict', `dispute is ${record.state}`);
    if (body.action === 'withdraw') {
      if (record.opener !== party.id) throw new OrpError('forbidden', 'only the opener withdraws');
      record.state = 'withdrawn';
      deal.state = deal.before ?? 'active';
      return { status: 200, body: this.broadcast(deal, party.id, msg.id) };
    }
    if (record.opener === party.id) throw new OrpError('forbidden', 'the other party responds');
    const relayed = this.hostMessage('dispute', other, body, { deal: deal.id });
    this.deliver(other, relayed);
    return { status: 202 };
  }

  // ---- reads

  inbox(token: string | undefined, after?: string | null): Reply {
    const party = token ? this.tokens.get(token) : undefined;
    if (!party) return this.receiveError('unauthorized', 'a valid bearer token is required');
    const inbox = this.party(party).inbox;
    const start = after && /^c_\d+$/.test(after) ? Number(after.slice(2)) : 0;
    const messages = inbox.slice(start, start + 100);
    return { status: 200, body: { messages, next: `c_${start + messages.length}` } };
  }

  readDeal(token: string | undefined, id: string): Reply {
    const party = token ? this.tokens.get(token) : undefined;
    if (!party) return this.receiveError('unauthorized', 'a valid bearer token is required');
    const deal = this.deals.get(id);
    if (!deal || !this.roleOf(deal, party)) return this.receiveError('not_found', 'no such deal for this party', party);
    this.sweep(deal);
    return { status: 200, body: this.snapshot(deal, party) };
  }

  readOutcome(token: string | undefined, id: string): Reply {
    const party = token ? this.tokens.get(token) : undefined;
    if (!party) return this.receiveError('unauthorized', 'a valid bearer token is required');
    const deal = this.deals.get(id);
    if (!deal || !this.roleOf(deal, party)) return this.receiveError('not_found', 'no such deal for this party', party);
    this.sweep(deal);
    return { status: 200, body: this.outcome(deal, party) };
  }

  private receiveError(code: string, text: string, to?: PartyId): Reply {
    return { status: STATUS[code] ?? 400, body: this.hostMessage('error', to, { code, message: text } as MessageByType['error']['body']) };
  }

  private outcome(deal: DealRecord, to: PartyId, final = false): MessageByType['outcome'] {
    const rows: OutcomeRow[] = [];
    for (const o of deal.terms.obligations) {
      const clicks = deal.clicks.get(o.id);
      if (!clicks) continue;
      const sales = deal.conversions.get(o.id);
      const row: OutcomeRow = { obligation: o.id, clicks: { ...clicks } };
      if (sales) {
        row.conversions = sales.count;
        row.refunds = sales.refunds;
        row.revenue = { amount_minor: sales.revenue, currency: sales.currency };
      }
      const earned = this.earned(deal, o.id, clicks.human, sales);
      if (earned) row.earned = earned;
      rows.push(row);
    }
    return this.hostMessage('outcome', to, { period: { from: deal.activatedAt ?? deal.updated, to: this.ts() }, rows, final }, { deal: deal.id });
  }

  // Only human clicks earn. Caps apply per pay obligation.
  private earned(deal: DealRecord, obligation: ObligationId, humanClicks: number, sales?: { count: number; refunds: number; revenue: number; currency: string }): Money | undefined {
    let total = 0;
    let currency: string | undefined;
    for (const pay of deal.terms.obligations.filter(isPay)) {
      if (pay.after && pay.after !== obligation) continue;
      let amount: number;
      if (pay.per === 'click' && pay.amount) {
        amount = pay.amount.amount_minor * humanClicks;
        currency = pay.amount.currency;
      } else if (pay.per === 'conversion' && sales) {
        amount = pay.rate_bps ? Math.floor((Math.max(0, sales.revenue) * pay.rate_bps) / 10_000) : (pay.amount?.amount_minor ?? 0) * Math.max(0, sales.count - sales.refunds);
        currency = pay.amount?.currency ?? sales.currency;
      } else continue;
      total += pay.cap ? Math.min(amount, pay.cap.amount_minor) : amount;
    }
    return currency ? { amount_minor: total, currency } : undefined;
  }

  // ---- tracking links: the universal adapter

  click(code: string, method: string, headers: Record<string, string | string[] | undefined>, query: URLSearchParams): { status: 302; location: string } {
    const link = this.links.get(code);
    if (!link) return { status: 302, location: `${this.origin}/` };
    const deal = this.deals.get(link.deal);
    if (deal) this.sweep(deal);
    const tracking = deal?.tracking.find((t) => t.obligation === link.obligation);
    if (!deal || !tracking || tracking.status !== 'live' || !['active', 'disputed', 'completed'].includes(deal.state)) {
      return { status: 302, location: link.destination };
    }
    const kind = classifyClick(method, headers, query);
    const counts = deal.clicks.get(link.obligation) ?? emptyClicks();
    counts[kind] += 1;
    deal.clicks.set(link.obligation, counts);
    if (kind === 'human') {
      const record = deal.obligations.find((o) => o.id === link.obligation);
      const spec = deal.terms.obligations.find((o) => o.id === link.obligation);
      const proofs = spec && 'proof' in spec ? (spec.proof as string[] | undefined) : undefined;
      // Without a clicks target the first human click proves the placement;
      // a traffic swap is done when its target is reached.
      const target = spec && isPlace(spec) && spec.clicks ? spec.clicks : 1;
      if (record && (record.state === 'pending' || record.state === 'claimed') && proofs?.includes('link') && counts.human >= target) {
        this.receipt(deal, link.obligation, { result: 'verified', method: 'link', found: { link: true, ...(target > 1 ? { clicks: counts.human } : {}) } });
      }
    }
    const target = new URL(link.destination);
    target.searchParams.set('orp', code);
    return { status: 302, location: target.toString() };
  }

  // ---- test harness (never in production)

  decideEscalation(id: string, decision: 'approved' | 'refused', assurance: 'session' | 'passkey' | 'email_link' | 'client_prompt' = 'session'): Message | undefined {
    const escalation = this.escalations.get(id);
    if (!escalation) throw new OrpError('not_found', 'no such escalation');
    this.escalations.delete(id);
    // The first passkey approval also claims an unclaimed party: one tap.
    if (assurance === 'passkey') this.party(escalation.party).claimed = true;
    const consentId = randomId('cns', 6);
    const body = { consent: consentId, escalation: escalation.id, party: escalation.party, decision, step: escalation.step, terms_hash: escalation.termsHash, reasons: escalation.reasons, assurance, at: this.ts() };
    if (escalation.offer) {
      const offer = this.offers.get(escalation.offer)!;
      offer.waiting = undefined;
      if (decision === 'approved') {
        offer.listed = true;
        offer.authority = { type: 'consent', ref: consentId };
      } else offer.revoked = true;
      const listing = this.listing(offer);
      this.deliver(offer.offerer, listing);
      return listing;
    }
    const deal = this.deals.get(escalation.deal!)!;
    const own = this.hostMessage('consent', escalation.party, body, { deal: deal.id });
    this.deliver(escalation.party, own);
    const other = deal.offerer === escalation.party ? deal.acceptor : deal.offerer;
    const { reasons: _r, escalation: _e, ...shared } = body;
    this.deliver(other, this.hostMessage('consent', other, shared, { deal: deal.id }));
    if (decision === 'approved') this.bind(deal, escalation.party, { type: 'consent', ref: consentId });
    else {
      deal.waiting = undefined;
      deal.state = 'declined';
    }
    deal.updated = this.ts();
    this.deliver(other, this.snapshot(deal, other));
    this.deliver(escalation.party, this.snapshot(deal, escalation.party));
    return own;
  }

  recordConversion(code: string, amount: { amount_minor: number; currency: string }, refund = false): boolean {
    const link = this.links.get(code);
    const deal = link ? this.deals.get(link.deal) : undefined;
    const tracking = deal?.tracking.find((t) => t.obligation === link!.obligation);
    if (!deal || !link || tracking?.status !== 'live') return false;
    const sales = deal.conversions.get(link.obligation) ?? { count: 0, refunds: 0, revenue: 0, currency: amount.currency };
    if (refund) {
      sales.refunds += 1;
      sales.revenue -= amount.amount_minor;
    } else {
      sales.count += 1;
      sales.revenue += amount.amount_minor;
    }
    deal.conversions.set(link.obligation, sales);
    return true;
  }

  resolveDispute(id: string, outcome: 'upheld' | 'rejected' | 'split', remedy: 'redo' | 'pay' | 'refund' | 'void' | 'none', note?: string): void {
    const record = this.disputes.get(id);
    if (!record || record.state !== 'open') throw new OrpError('not_found', 'no open dispute');
    const deal = this.deals.get(record.deal)!;
    const doer = this.obligor(deal, record.obligation);
    const atFault = outcome === 'upheld' ? doer : outcome === 'rejected' ? record.opener : undefined;
    if (atFault) this.recordEvent(atFault, atFault === deal.offerer ? deal.acceptor : deal.offerer, 'dispute_lost');
    record.state = 'resolved';
    deal.state = deal.before ?? 'active';
    if (remedy === 'redo') {
      const o = deal.obligations.find((x) => x.id === record.obligation);
      if (o) o.state = 'pending';
    }
    if (remedy === 'void') {
      const o = deal.obligations.find((x) => x.id === record.obligation);
      if (o) o.state = 'waived';
    }
    const body = { action: 'resolve' as const, dispute: id, obligation: record.obligation, resolution: { outcome, remedy, ...(atFault ? { at_fault: atFault } : {}), ...(note ? { note } : {}) } };
    for (const to of [deal.offerer, deal.acceptor]) this.deliver(to, this.hostMessage('dispute', to, body, { deal: deal.id }));
  }

  emitOutcome(dealId: string, final = false): void {
    const deal = this.deals.get(dealId);
    if (!deal) throw new OrpError('not_found', 'no such deal');
    for (const to of [deal.offerer, deal.acceptor]) this.deliver(to, this.outcome(deal, to, final));
  }

  setPublicPage(url: string, status: number, body: string): void {
    this.fakeWeb.set(url, { status, body });
  }

  // ---- HTTP

  async handle(req: IncomingMessage, res: ServerResponse): Promise<void> {
    const url = new URL(req.url ?? '/', this.origin);
    const token = (req.headers.authorization ?? '').match(/^Bearer\s+(.+)$/i)?.[1];
    const send = (status: number, body?: unknown, headers: Record<string, string> = {}) => {
      res.writeHead(status, { ...(body === undefined ? {} : { 'content-type': 'application/json' }), 'cache-control': 'no-store', ...headers });
      res.end(body === undefined ? undefined : JSON.stringify(body));
    };
    const html = (status: number, page: string) => {
      res.writeHead(status, { 'content-type': 'text/html; charset=utf-8', 'cache-control': 'no-store', 'referrer-policy': 'no-referrer', 'x-robots-tag': 'noindex' });
      res.end(page);
    };
    const address = req.socket.remoteAddress ?? 'unknown';
    try {
      const path = url.pathname;
      const open = path === '/.well-known/orp.json' || path === '/orp/suggest' || path === '/orp/join';
      if (open) res.setHeader('access-control-allow-origin', '*');
      if (req.method === 'OPTIONS' && open) return send(204, undefined, { 'access-control-allow-methods': 'GET, POST', 'access-control-allow-headers': 'content-type', 'access-control-max-age': '86400' });
      if (req.method === 'POST' && path === '/orp/suggest') return send(...this.suggest(await readJson(req), address));
      if (req.method === 'POST' && path === '/orp/join') return send(...this.join(await readJson(req), address));
      if ((req.method === 'GET' || req.method === 'HEAD') && path.startsWith('/go/')) {
        const { location } = this.click(decodeURIComponent(path.slice(4)), req.method, req.headers, url.searchParams);
        res.writeHead(302, { location, 'cache-control': 'no-store', 'referrer-policy': 'no-referrer', 'x-robots-tag': 'noindex' });
        res.end();
        return;
      }
      if (req.method === 'GET' && path === '/.well-known/orp.json') return send(200, this.discovery());
      if (req.method === 'POST' && path === '/orp') {
        const raw = await readJson(req);
        const reply = raw === undefined ? this.receiveError('invalid_message', 'the body is not JSON') : await this.receive(token, raw);
        const retry = reply.body && 'body' in reply.body && reply.body.type === 'error' && typeof reply.body.body.retry_after === 'number' ? { 'retry-after': String(reply.body.body.retry_after) } : undefined;
        return send(reply.status, reply.body, retry);
      }
      if (req.method === 'GET' && path === '/orp/inbox') {
        const reply = this.inbox(token, url.searchParams.get('after'));
        return send(reply.status, reply.body);
      }
      const dealPath = path.match(/^\/orp\/deals\/([A-Za-z0-9_-]{3,64})(\/outcome)?$/);
      if (req.method === 'GET' && dealPath) {
        const reply = dealPath[2] ? this.readOutcome(token, dealPath[1]!) : this.readDeal(token, dealPath[1]!);
        return send(reply.status, reply.body);
      }
      if (req.method === 'POST' && path === '/mcp') {
        const session = typeof req.headers['mcp-session-id'] === 'string' ? req.headers['mcp-session-id'] : undefined;
        return send(...(await this.mcp(token, session, address, await readJson(req))));
      }
      const approve = path.match(/^\/approve\/([\w-]{3,64})$/);
      if (approve && req.method === 'GET') return html(...this.approvalPage(approve[1]!));
      const claim = path.match(/^\/claim\/([\w-]{20,64})$/);
      if (claim && req.method === 'GET') return html(...this.claimPage(claim[1]!));
      if ((approve || claim) && req.method === 'POST') {
        return html(501, htmlPage('Passkeys needed', '<h1>Not in the reference host</h1><p>A real host checks a passkey here (WebAuthn with user verification) before it records consent or a claim. The reference host has no passkeys; its tests use the /orp-test harness.</p>'));
      }
      if (this.options.harness && path.startsWith('/orp-test/') && req.method === 'POST') return send(...(await this.harness(path, ((await readJson(req)) ?? {}) as Record<string, unknown>)));
      send(404, { error: 'not_found' });
    } catch (error) {
      if (error instanceof OrpError) return send(STATUS[error.code] ?? 400, { error: error.code, message: error.message });
      console.error(error);
      send(500, { error: 'internal' });
    }
  }

  private async harness(path: string, input: Record<string, unknown>): Promise<[number, unknown]> {
    if (path === '/orp-test/parties') return [200, this.createParty(input as { constraints?: Constraints; spaces?: Space[]; jwk?: JsonWebKey })];
    const claimMatch = path.match(/^\/orp-test\/claims\/([\w-]+)$/);
    if (claimMatch) return [200, { party: this.claim(claimMatch[1]!, input.constraints as Constraints | undefined), claimed: true }];
    const recheckMatch = path.match(/^\/orp-test\/recheck\/([\w-]+)$/);
    if (recheckMatch) {
      await this.recheck(recheckMatch[1]!);
      return [200, { ok: true }];
    }
    const decideMatch = path.match(/^\/orp-test\/escalations\/([\w-]+)\/decide$/);
    if (decideMatch) return [200, this.decideEscalation(decideMatch[1]!, input.decision as 'approved' | 'refused', input.assurance as 'session' | undefined)];
    if (path === '/orp-test/conversions') return [200, { recorded: this.recordConversion(String(input.code), input.amount as { amount_minor: number; currency: string }, Boolean(input.refund)) }];
    if (path === '/orp-test/web') {
      this.setPublicPage(String(input.url), Number(input.status ?? 200), String(input.body ?? ''));
      return [200, { ok: true }];
    }
    const resolveMatch = path.match(/^\/orp-test\/disputes\/([\w-]+)\/resolve$/);
    if (resolveMatch) {
      this.resolveDispute(resolveMatch[1]!, input.outcome as 'upheld', input.remedy as 'redo', input.note as string | undefined);
      return [200, { ok: true }];
    }
    const outcomeMatch = path.match(/^\/orp-test\/outcomes\/([\w-]+)$/);
    if (outcomeMatch) {
      this.emitOutcome(outcomeMatch[1]!, Boolean(input.final));
      return [200, { ok: true }];
    }
    return [404, { error: 'not_found' }];
  }

  // MCP clients cannot change their own headers mid-conversation, so orp_join
  // binds the new party to the Mcp-Session-Id. A bearer token, when sent, wins.
  private async mcp(bearer: string | undefined, session: string | undefined, address: string, request: unknown): Promise<[number, unknown?, Record<string, string>?]> {
    const rpc = (request ?? {}) as { jsonrpc?: string; id?: string | number; method?: string; params?: Record<string, unknown> };
    const result = (value: unknown, headers?: Record<string, string>): [number, unknown, Record<string, string>?] => [200, { jsonrpc: '2.0', id: rpc.id ?? null, result: value }, headers];
    const failure = (code: number, text: string): [number, unknown] => [200, { jsonrpc: '2.0', id: rpc.id ?? null, error: { code, message: text } }];
    const tool = (body: unknown, isError: boolean) => result({ content: [{ type: 'text', text: JSON.stringify(body) }], structuredContent: body, isError });
    if (rpc.jsonrpc !== '2.0' || typeof rpc.method !== 'string') return failure(-32600, 'invalid request');
    if (session !== undefined && rpc.method !== 'initialize' && !this.sessions.has(session)) return [404, { jsonrpc: '2.0', id: rpc.id ?? null, error: { code: -32001, message: 'unknown session' } }];
    if (rpc.id === undefined) return [202];
    const token = bearer ?? (session ? this.sessions.get(session) : undefined);
    switch (rpc.method) {
      case 'initialize': {
        const id = randomBytes(16).toString('base64url');
        this.sessions.set(id, undefined);
        return result({ protocolVersion: MCP_PROTOCOL_VERSION, capabilities: { tools: {}, experimental: orpCapability(`${this.origin}/.well-known/orp.json`) }, serverInfo: { name: 'orp-reference-host', version: '0.3.0' } }, { 'mcp-session-id': id });
      }
      case 'ping':
        return result({});
      case 'tools/list':
        return result({ tools: ORP_TOOLS });
      case 'tools/call': {
        const name = rpc.params?.name;
        const args = (rpc.params?.arguments ?? {}) as Record<string, unknown>;
        if (name === 'orp_suggest') {
          const [status, body] = this.suggest(args, address);
          return tool(body, status !== 200);
        }
        if (name === 'orp_join') {
          const [status, body] = this.join(args, address);
          if (status === 200 && session && 'token' in body) this.sessions.set(session, body.token);
          return tool(body, status !== 200);
        }
        let reply: Reply;
        if (name === 'orp_send') reply = await this.receive(token, args.message);
        else if (name === 'orp_inbox') reply = this.inbox(token, typeof args.after === 'string' ? args.after : undefined);
        else if (name === 'orp_deal') reply = this.readDeal(token, String(args.deal ?? ''));
        else return failure(-32602, `unknown tool ${String(name)}`);
        const content = reply.body ?? { delivered: true };
        return tool(content, 'type' in content && content.type === 'error');
      }
      default:
        return failure(-32601, `method ${rpc.method} not found`);
    }
  }

  listen(port = 0, hostname = '127.0.0.1'): Promise<Server> {
    const server = createServer((req, res) => void this.handle(req, res));
    return new Promise((resolve) => server.listen(port, hostname, () => resolve(server)));
  }
}

async function readJson(req: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of req) {
    size += (chunk as Buffer).length;
    if (size > 262_144) return undefined;
    chunks.push(chunk as Buffer);
  }
  if (!chunks.length) return undefined;
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'));
  } catch {
    return undefined;
  }
}

/** Start a host on a free local port. */
export async function startHost(options: Omit<HostOptions, 'origin'> & { port?: number } = {}): Promise<{ host: OrpHost; server: Server; origin: string; close: () => Promise<void> }> {
  const placeholder = new OrpHost({ origin: 'http://127.0.0.1', ...options });
  const server = await placeholder.listen(options.port ?? 0);
  const address = server.address();
  const origin = `http://127.0.0.1:${typeof address === 'object' && address ? address.port : 0}`;
  const host = new OrpHost({ ...options, origin });
  server.removeAllListeners('request');
  server.on('request', (req, res) => void host.handle(req, res));
  return { host, server, origin, close: () => new Promise((resolve) => server.close(() => resolve())) };
}
