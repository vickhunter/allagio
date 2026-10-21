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
  Message,
  Money,
  ObligationId,
  OutcomeRow,
  PartyId,
  ProofMethod,
  ReceiptBody,
  Result,
  Role,
  Space,
  Step,
  Terms,
  Tracking,
  Waiting,
} from './types.ts';
import { decide, DEFAULT_STEPS, durationMs, isPay } from './constraints.ts';
import { hashJson, messageHash, termsHash } from './hash.ts';
import { MCP_PROTOCOL_VERSION, ORP_TOOLS, orpCapability } from './mcp.ts';
import { message, now as nowIso, ORP_VERSION, type CoreType, type MessageByType } from './message.ts';
import { score, type RepEvent } from './reputation.ts';
import { generateSigningKey, signMessage, verifyMessage, type SigningKey } from './sign.ts';
import { checkMessage } from './validate.ts';

export interface HostOptions {
  origin: string;
  name?: string;
  harness?: boolean;
  sign?: boolean;
  web?: (url: string) => Promise<{ status: number; body: string }>;
  clock?: () => Date;
}

export interface Reply {
  status: number;
  body?: Message | InboxPage | Discovery;
}

type ObligationState = 'pending' | 'claimed' | 'verified' | 'failed' | 'revoked' | 'waived';

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
  obligations: Array<{ id: ObligationId; state: ObligationState; due?: string; receipt?: string }>;
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
const PLACE_LIKE = new Set(['place', 'repost', 'comment']);

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

  createParty(input: { constraints?: Constraints; spaces?: Space[]; jwk?: JsonWebKey } = {}): { party: PartyId; token: string } {
    const id: PartyId = `orp:${new URL(this.origin).host}/${randomId('p', 12)}`;
    const token = randomBytes(24).toString('base64url');
    this.parties.set(id, { id, token, constraints: input.constraints, spaces: input.spaces ?? [], jwk: input.jwk, inbox: [], chain: new Map(), sent: [], events: [], completed: 0 });
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
    const escalation: Escalation = { id: randomId('esc', 6), party, step, reasons, termsHash: hash, since: this.ts(), ...link };
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
    }
    return hash;
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
      const content = 'content' in o ? (o.content as { link?: boolean; code?: boolean } | undefined) : undefined;
      const destination = 'destination' in o && typeof o.destination === 'string' ? o.destination : undefined;
      if (!destination || (o.action !== 'place' && !(o.action === 'comment' && content?.link))) continue;
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

  private receipt(deal: DealRecord, obligation: ObligationId, body: Omit<ReceiptBody, 'obligation' | 'at'>, re?: string): void {
    const record = deal.obligations.find((o) => o.id === obligation);
    if (!record || record.state === 'verified' || record.state === 'revoked') return;
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

  private async verifyProof(deal: DealRecord, obligation: ObligationId, proofUrl: string, re: string): Promise<void> {
    const spec = deal.terms.obligations.find((o) => o.id === obligation)!;
    const page = await this.fetchPublic(proofUrl);
    if (page.status === 0) return;
    if (page.status !== 200) {
      this.receipt(deal, obligation, { result: 'failed', method: 'proof_url', url: proofUrl, reason: page.status === 404 || page.status === 410 ? 'not_found' : 'unreachable' }, re);
      return;
    }
    const text = page.body;
    const tracking = deal.tracking.find((t) => t.obligation === obligation);
    const found: NonNullable<ReceiptBody['found']> = {};
    if ('target' in spec && typeof spec.target === 'string') found.target = text.includes(spec.target);
    if (tracking) found.link = text.includes(tracking.link);
    if (tracking?.code) found.code = text.includes(tracking.code);
    if (deal.terms.disclosure) found.disclosure = text.toLowerCase().includes(deal.terms.disclosure.label.toLowerCase());
    const core = spec.action === 'repost' || spec.action === 'comment' && found.target !== undefined ? found.target : found.link ?? found.code;
    if (!core) {
      this.receipt(deal, obligation, { result: 'failed', method: 'proof_url', url: proofUrl, found, reason: spec.action === 'repost' ? 'not_found' : 'link_missing' }, re);
    } else if (found.disclosure === false && spec.action !== 'repost') {
      this.receipt(deal, obligation, { result: 'partial', method: 'proof_url', url: proofUrl, found, reason: 'disclosure_missing' }, re);
    } else {
      this.receipt(deal, obligation, { result: 'verified', method: 'proof_url', url: proofUrl, found }, re);
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
    const words = new Set(`${query ?? ''} ${context?.text ?? ''}`.toLowerCase().match(/[a-z0-9]{3,}/g) ?? []);
    const items: Result[] = [];
    if (want === 'offers') {
      for (const offer of this.offers.values()) {
        if (!offer.listed || offer.revoked || offer.offerer === party.id) continue;
        if (offer.expires && Date.parse(offer.expires) < this.clock().getTime()) continue;
        if (filters.kinds && (!offer.terms.kind || !filters.kinds.includes(offer.terms.kind))) continue;
        if (filters.categories && !(offer.terms.categories ?? []).some((c) => filters.categories!.some((f) => c === f || c.startsWith(`${f}.`)))) continue;
        const reputation = this.reputationOf(offer.offerer);
        if (filters.min_reputation !== undefined && (reputation ?? 0) < filters.min_reputation) continue;
        const text = `${offer.terms.title} ${(offer.terms.categories ?? []).join(' ')}`.toLowerCase();
        const hits = [...words].filter((w) => text.includes(w));
        const fit = Math.min(100, 40 + 15 * hits.length);
        const evidence = offer.terms.obligations.flatMap((o) => ('destination' in o && typeof o.destination === 'string' ? [o.destination] : []));
        items.push({
          offer: { id: offer.id, terms: offer.terms, terms_hash: offer.hash, ...(offer.expires ? { expires: offer.expires } : {}) },
          party: offer.offerer,
          fit: { score: fit, reasons: [hits.length ? `Matches ${hits.slice(0, 5).join(', ')}.` : 'Open standing offer.'], ...(evidence.length ? { evidence: evidence.slice(0, 5) } : {}) },
          trust: { reputation, completed_deals: this.party(offer.offerer).completed },
          pool: 'public',
        });
      }
    } else {
      for (const other of this.parties.values()) {
        if (other.id === party.id) continue;
        for (const space of other.spaces) {
          if (filters.space_kinds && !filters.space_kinds.includes(space.kind)) continue;
          if (filters.kinds && !(space.accepts ?? []).some((k) => filters.kinds!.includes(k))) continue;
          const { locator: _hidden, ...listed } = space;
          items.push({ space: listed, party: other.id, fit: { score: 50, reasons: [`Accepts ${(space.accepts ?? ['deals']).join(', ')}.`] }, trust: { reputation: this.reputationOf(other.id), completed_deals: other.completed } });
        }
      }
    }
    items.sort((a, b) => b.fit.score - a.fit.score);
    const page = items.slice(0, limit);
    return this.hostMessage('results', party.id, { items: page, ...(page.length ? {} : { empty_reason: 'no_match' as const }) }, { re: msg.id });
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
    return { status: 200, body: this.snapshot(deal, party) };
  }

  readOutcome(token: string | undefined, id: string): Reply {
    const party = token ? this.tokens.get(token) : undefined;
    if (!party) return this.receiveError('unauthorized', 'a valid bearer token is required');
    const deal = this.deals.get(id);
    if (!deal || !this.roleOf(deal, party)) return this.receiveError('not_found', 'no such deal for this party', party);
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
      if (record && (record.state === 'pending' || record.state === 'claimed') && proofs?.includes('link')) {
        this.receipt(deal, link.obligation, { result: 'verified', method: 'link', found: { link: true } });
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
    try {
      const path = url.pathname;
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
      if (req.method === 'POST' && path === '/mcp') return send(...(await this.mcp(token, await readJson(req))));
      if (req.method === 'GET' && path.startsWith('/approve/')) {
        res.writeHead(200, { 'content-type': 'text/plain; charset=utf-8' });
        res.end('The owner approves or refuses here, signed in to the host. This reference host has no owner sign-in; tests use the /orp-test harness.\n');
        return;
      }
      if (this.options.harness && path.startsWith('/orp-test/') && req.method === 'POST') return send(...this.harness(path, ((await readJson(req)) ?? {}) as Record<string, unknown>));
      send(404, { error: 'not_found' });
    } catch (error) {
      if (error instanceof OrpError) return send(STATUS[error.code] ?? 400, { error: error.code, message: error.message });
      console.error(error);
      send(500, { error: 'internal' });
    }
  }

  private harness(path: string, input: Record<string, unknown>): [number, unknown] {
    if (path === '/orp-test/parties') return [200, this.createParty(input as { constraints?: Constraints; spaces?: Space[]; jwk?: JsonWebKey })];
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

  private async mcp(token: string | undefined, request: unknown): Promise<[number, unknown?]> {
    const rpc = (request ?? {}) as { jsonrpc?: string; id?: string | number; method?: string; params?: Record<string, unknown> };
    const result = (value: unknown): [number, unknown] => [200, { jsonrpc: '2.0', id: rpc.id ?? null, result: value }];
    const failure = (code: number, text: string): [number, unknown] => [200, { jsonrpc: '2.0', id: rpc.id ?? null, error: { code, message: text } }];
    if (rpc.jsonrpc !== '2.0' || typeof rpc.method !== 'string') return failure(-32600, 'invalid request');
    if (rpc.id === undefined) return [202];
    switch (rpc.method) {
      case 'initialize':
        return result({ protocolVersion: MCP_PROTOCOL_VERSION, capabilities: { tools: {}, experimental: orpCapability(`${this.origin}/.well-known/orp.json`) }, serverInfo: { name: 'orp-reference-host', version: '0.3.0' } });
      case 'ping':
        return result({});
      case 'tools/list':
        return result({ tools: ORP_TOOLS });
      case 'tools/call': {
        const name = rpc.params?.name;
        const args = (rpc.params?.arguments ?? {}) as Record<string, unknown>;
        let reply: Reply;
        if (name === 'orp_send') reply = await this.receive(token, args.message);
        else if (name === 'orp_inbox') reply = this.inbox(token, typeof args.after === 'string' ? args.after : undefined);
        else if (name === 'orp_deal') reply = this.readDeal(token, String(args.deal ?? ''));
        else return failure(-32602, `unknown tool ${String(name)}`);
        const content = reply.body ?? { delivered: true };
        const isError = 'type' in content && content.type === 'error';
        return result({ content: [{ type: 'text', text: JSON.stringify(content) }], structuredContent: content, isError });
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
