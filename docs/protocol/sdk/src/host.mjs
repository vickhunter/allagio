import { createServer } from 'node:http';
import { randomBytes } from 'node:crypto';
import { mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';
import { canonicalize, digest } from './jcs.mjs';
import { generateEd25519, publicJwk, signEnvelope, verifyEnvelope } from './sign.mjs';
import { check } from './validate.mjs';

const DAY = 86400000;
const CORE = new Set(['hello', 'discover', 'results', 'space', 'request', 'offer', 'counter', 'accept', 'decline', 'deal', 'execute', 'receipt', 'outcome', 'payout', 'revoke', 'consent', 'dispute', 'reputation', 'error']);
const HOST_ONLY = new Set(['results', 'deal', 'outcome', 'consent']);
const BINDING = new Set(['offer', 'counter', 'accept', 'revoke', 'payout']);
const SIGNED = new Set(['deal', 'consent', 'receipt', 'payout', 'reputation']);
const CREATIVE = new Set(['place', 'comment', 'repost']);
const HTTP_STATUS = {
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
const DEFAULT_STEPS = {
  list: 'ask',
  negotiate: 'ask',
  accept: 'ask',
  execute: 'ask',
  amend: 'ask',
  settle: 'ask',
  share: 'ask',
  revoke: 'auto',
};
const STEP_RANK = { off: 0, ask: 1, auto: 2 };
const METHOD_WEIGHT = { provider: 1, platform_check: 1, link: 0.9, witness: 0.8, proof_url: 0.7, attestation: 0 };
const PREVIEW_AGENTS = ['facebookexternalhit', 'twitterbot', 'slackbot', 'discordbot', 'whatsapp', 'telegrambot', 'linkedinbot', 'embedly', 'iframely'];
const BOT_AGENTS = ['curl/', 'wget', 'python-requests', 'python-urllib', 'go-http-client', 'headlesschrome', 'spider', 'crawler', 'scrapy', 'bot'];

const token = (bytes = 32) => randomBytes(bytes).toString('base64url');
const localId = (prefix) => `${prefix}${token(6)}`;

function durationMs(value) {
  const match = /^P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$/.exec(value || '');
  if (!match) return 0;
  const [, y, mo, w, d, h, mi, s] = match.map((part) => (part ? Number(part) : 0));
  return (((y * 365 + mo * 30 + w * 7 + d) * 24 + h) * 60 + mi) * 60 * 1000 + s * 1000;
}

function dirty(value, path = '') {
  const bad = /[\u202A-\u202E\u2066-\u2069\u200B-\u200D\uFEFF\u0000-\u0008\u000B\u000C\u000E-\u001F\u007F-\u009F]|<[A-Za-z!/]/;
  if (typeof value === 'string') return bad.test(value) ? (path || '/') : '';
  if (Array.isArray(value)) {
    for (let i = 0; i < value.length; i += 1) {
      const found = dirty(value[i], `${path}/${i}`);
      if (found) return found;
    }
    return '';
  }
  if (value && typeof value === 'object') {
    for (const key of Object.keys(value)) {
      const found = dirty(value[key], `${path}/${key}`);
      if (found) return found;
    }
  }
  return '';
}

function textsOf(value) {
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) return value.map(textsOf).join('\n');
  if (value && typeof value === 'object') return Object.values(value).map(textsOf).join('\n');
  return '';
}

function roleParty(deal, role) {
  return role === 'offerer' ? deal.offerer : deal.acceptor;
}

function otherParty(deal, partyId) {
  return deal.offerer === partyId ? deal.acceptor : deal.offerer;
}

function partyRole(deal, partyId) {
  if (deal.offerer === partyId) return 'offerer';
  if (deal.acceptor === partyId) return 'acceptor';
  return '';
}

function wilsonLower(positive, total, z = 1.96) {
  if (total <= 0) return 0;
  const phat = Math.min(1, Math.max(0, positive / total));
  const z2 = z * z;
  const denom = 1 + z2 / total;
  const centre = phat + z2 / (2 * total);
  const margin = z * Math.sqrt((phat * (1 - phat) + z2 / (4 * total)) / total);
  return Math.max(0, (centre - margin) / denom);
}

function hashMatches(terms, hash) {
  if (!hash) return '';
  return digest(terms) === hash ? '' : 'terms_hash does not match the terms.';
}

function sourceOf(referer) {
  if (!referer) return 'direct';
  try {
    const host = new URL(referer).hostname.toLowerCase().replace(/^www\./, '');
    if (!/^[a-z0-9-]+(\.[a-z0-9-]+)+$/.test(host)) return 'other';
    return host;
  } catch {
    return 'other';
  }
}

export class ReferenceHost {
  constructor(options = {}) {
    this.dns = 'ref.example';
    this.hostId = `orp:${this.dns}/host`;
    this.claims = options.claims || 'real';
    this.withPolicies = options.policies !== false;
    this.storePath = options.storePath || '';
    this.businessShift = 0;
    this.inboxSeq = 0;
    this.pages = new Set();
    this.parties = new Map();
    this.byToken = new Map();
    this.byOwner = new Map();
    this.byClaim = new Map();
    this.offers = new Map();
    this.deals = new Map();
    this.links = new Map();
    this.clicks = new Map();
    this.idempotency = new Map();
    this.inbox = new Map();
    this.spaces = new Map();
    this.escalations = new Map();
    this.payouts = new Map();
    this.disputes = new Map();
    this.events = [];
    this.repCache = new Map();
    this.outbound = [];
    this.recheckFail = new Set();
    this.chain = new Map();
    this.providerToken = token(18);
    this.hostKey = generateEd25519('host-ref');
    this.server = null;
    this.origin = '';
    if (this.storePath) this.load();
  }

  static async start(options = {}) {
    const host = new ReferenceHost(options);
    await host.listen();
    return host;
  }

  wallNow() {
    return Date.now();
  }

  businessNow() {
    return this.wallNow() + this.businessShift;
  }

  iso(ms = this.wallNow()) {
    return new Date(ms).toISOString();
  }

  advanceBusiness(ms) {
    this.businessShift += ms;
    this.save();
  }

  levels() {
    const levels = ['L0'];
    if (this.withPolicies) levels.push('L1', 'L2', 'L3');
    return levels;
  }

  discovery() {
    const doc = {
      orp: ['0.3'],
      host: this.hostId,
      name: 'ORP Reference Host',
      description: 'Reference host for the ORP 0.3 conformance suite. Not the hosted affilio-api.',
      endpoint: `${this.origin}/orp`,
      suggest: `${this.origin}/orp/suggest`,
      join: `${this.origin}/orp/join`,
      auth: { bearer: true },
      tracking: { links: true, codes: true, base: `${this.origin}/go/` },
      conformance: { levels: this.levels() },
      capabilities: this.capabilities(),
      keys: [publicJwk(this.hostKey)],
      docs: `${this.origin}/docs/orp`,
    };
    if (this.withPolicies) {
      doc.policies = {
        terms: `${this.origin}/terms`,
        privacy: `${this.origin}/privacy`,
        abuse: `${this.origin}/abuse`,
      };
    }
    return doc;
  }

  capabilities() {
    return {
      messages: [...CORE],
      proofs: ['link', 'proof_url', 'platform_check', 'attestation', 'witness', 'provider'],
      tracking: { links: true, codes: true },
      signatures: { algs: ['EdDSA'], required: false },
      inbox: { poll: true },
      checks: { claims: this.claims },
    };
  }

  registerPage(url) {
    this.pages.add(url);
    this.save();
  }

  markTestPool(dealId) {
    const deal = this.deals.get(dealId);
    if (!deal) throw new Error(`no deal ${dealId}`);
    deal.pool = 'test';
    this.save();
  }

  failNextRecheck(dealId, obligation) {
    this.recheckFail.add(`${dealId}/${obligation}`);
    this.save();
  }

  async listen() {
    this.server = createServer((req, res) => {
      this.onRequest(req, res).catch((error) => {
        if (!res.headersSent) {
          res.writeHead(500, { 'content-type': 'application/json', 'cache-control': 'no-store' });
          res.end(JSON.stringify({ error: 'internal', message: error.message }));
        }
      });
    });
    await new Promise((resolve) => this.server.listen(0, '127.0.0.1', resolve));
    const { port } = this.server.address();
    this.origin = `http://127.0.0.1:${port}`;
    return this.origin;
  }

  async close() {
    if (!this.server) return;
    const server = this.server;
    this.server = null;
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(() => resolve()));
  }

  async onRequest(req, res) {
    const url = new URL(req.url || '/', this.origin || 'http://127.0.0.1');
    const cors = url.pathname === '/.well-known/orp.json' || url.pathname === '/orp/suggest' || url.pathname === '/orp/join';
    if (req.method === 'OPTIONS' && cors) {
      this.write(res, 204, null, { cors });
      return;
    }
    if (req.method === 'GET' && url.pathname === '/.well-known/orp.json') {
      const doc = this.discovery();
      const verdict = check('discovery', doc);
      if (!verdict.ok) throw new Error(`discovery invalid at ${verdict.field}`);
      this.write(res, 200, doc, { cors: true });
      return;
    }
    if (req.method === 'GET' && (url.pathname === '/' || url.pathname === '/terms' || url.pathname === '/privacy' || url.pathname === '/abuse' || url.pathname === '/docs/orp')) {
      this.write(res, 200, { ok: true }, { cors: false });
      return;
    }
    if ((req.method === 'GET' || req.method === 'HEAD') && url.pathname.startsWith('/go/')) {
      const result = this.click(req, url);
      res.writeHead(result.status, result.headers);
      res.end(req.method === 'HEAD' ? undefined : '');
      return;
    }
    const body = await this.readBody(req);
    if (body.error) {
      this.write(res, body.status, body.message, { cors });
      return;
    }
    if (req.method === 'POST' && url.pathname === '/orp/suggest') {
      this.write(res, ...this.suggest(body.json), { cors: true });
      return;
    }
    if (req.method === 'POST' && url.pathname === '/orp/join') {
      const [status, payload] = this.join(body.json);
      this.write(res, status, payload, { cors: true });
      return;
    }
    const claim = url.pathname.match(/^\/claim\/([^/]+)$/);
    if (req.method === 'POST' && claim) {
      const [status, payload] = this.claim(claim[1], body.json);
      this.write(res, status, payload);
      return;
    }
    if (req.method === 'POST' && url.pathname === '/orp/provider') {
      if (req.headers['x-provider-test'] !== this.providerToken) {
        this.write(res, 401, this.plainError('unauthorized', 'Provider token was not accepted.'));
        return;
      }
      const [status, payload] = this.provider(body.json);
      this.write(res, status, payload);
      return;
    }
    const auth = this.authenticate(req.headers.authorization || '');
    if (!auth) {
      this.write(res, 401, this.plainError('unauthorized', 'No credential or a bad credential.'));
      return;
    }
    if (req.method === 'PUT' && url.pathname === '/orp/constraints') {
      const [status, payload] = this.setConstraints(auth, body.json);
      this.write(res, status, payload);
      return;
    }
    if (req.method === 'POST' && url.pathname === '/orp/conversions') {
      const [status, payload] = this.reportConversion(auth, body.json);
      this.write(res, status, payload);
      return;
    }
    const approve = url.pathname.match(/^\/approve\/([^/]+)$/);
    if (req.method === 'POST' && approve) {
      const [status, payload] = this.approve(auth, approve[1], body.json);
      this.write(res, status, payload);
      return;
    }
    if (req.method === 'GET' && url.pathname === '/orp/inbox') {
      this.write(res, 200, this.inboxPage(auth.party.id, url.searchParams.get('after') || ''));
      return;
    }
    const dealPath = url.pathname.match(/^\/orp\/deals\/([^/]+)$/);
    if (req.method === 'GET' && dealPath) {
      const deal = this.deals.get(dealPath[1]);
      if (!deal || (deal.offerer !== auth.party.id && deal.acceptor !== auth.party.id)) {
        this.write(res, 404, this.plainError('not_found', 'No such deal for this sender.'));
        return;
      }
      this.write(res, 200, this.snapshot(deal, auth.party.id, ''));
      return;
    }
    const keys = url.pathname.match(/^\/orp\/keys\/([^/]+)$/);
    if (req.method === 'GET' && keys) {
      const party = [...this.parties.values()].find((item) => item.id.endsWith(`/${keys[1]}`));
      if (!party?.key) {
        this.write(res, 404, { error: 'not_found' });
        return;
      }
      this.write(res, 200, { keys: [publicJwk(party.key)] });
      return;
    }
    if (req.method === 'POST' && url.pathname === '/orp') {
      if (auth.owner) {
        this.write(res, 403, this.plainError('forbidden', 'The owner credential does not send agent messages.'));
        return;
      }
      const result = this.handleMessage(auth.party, body.json);
      this.write(res, result.status, result.message);
      return;
    }
    this.write(res, 404, this.plainError('not_found', 'No such path.'));
  }

  write(res, status, payload, { cors = false } = {}) {
    const headers = { 'cache-control': 'no-store' };
    if (cors) headers['access-control-allow-origin'] = '*';
    if (payload == null) {
      res.writeHead(status, headers);
      res.end();
      return;
    }
    headers['content-type'] = 'application/json; charset=utf-8';
    res.writeHead(status, headers);
    res.end(JSON.stringify(payload));
  }

  async readBody(req) {
    const chunks = [];
    let size = 0;
    for await (const chunk of req) {
      size += chunk.length;
      if (size > 65536) {
        return { error: true, status: 400, message: this.plainError('invalid_message', 'The body is over 64 KiB.', { field: '' }) };
      }
      chunks.push(chunk);
    }
    if (!chunks.length) return { json: {} };
    try {
      return { json: JSON.parse(Buffer.concat(chunks).toString('utf8')) };
    } catch {
      return { error: true, status: 400, message: this.plainError('invalid_message', 'The body is not JSON.', { field: '' }) };
    }
  }

  authenticate(header) {
    const match = /^Bearer\s+(\S+)$/.exec(header);
    if (!match) return null;
    const partyId = this.byToken.get(match[1]);
    if (partyId) return { party: this.parties.get(partyId), owner: false };
    const ownerId = this.byOwner.get(match[1]);
    if (ownerId) return { party: this.parties.get(ownerId), owner: true };
    return null;
  }

  plainError(code, message, extra = {}) {
    return this.emit({
      type: 'error',
      to: this.hostId,
      body: { code, message, ...extra },
    }, { inbox: false });
  }

  handleMessage(party, message) {
    const shape = check('message', message);
    if (!shape.ok) return { status: 400, message: this.plainError('invalid_message', 'The message is not valid ORP 0.3.', { field: shape.field || '' }) };
    const dirtyAt = dirty(message);
    if (dirtyAt) return { status: 400, message: this.plainError('invalid_message', 'The message contains disallowed characters or markup.', { field: dirtyAt }) };
    if (message.orp !== '0.3') {
      return { status: 400, message: this.plainError('unsupported_version', 'This host speaks 0.3.', { supported: ['0.3'] }) };
    }
    if (!CORE.has(message.type)) return { status: 400, message: this.plainError('unsupported_type', 'The type is not supported.') };
    if ((message.critical || []).some((name) => name !== 'com.offerrails.none')) {
      return { status: 400, message: this.plainError('unsupported_extension', 'A critical extension is unknown.') };
    }
    if (message.from !== party.id) return this.remember(party, message, this.failure('forbidden', 'from does not match the credential.', message));
    if (HOST_ONLY.has(message.type) || this.hostForm(message)) {
      return this.remember(party, message, this.failure('forbidden', 'Only the host may send this.', message));
    }
    const ts = Date.parse(message.ts);
    if (!Number.isFinite(ts) || Math.abs(ts - this.wallNow()) > 15 * 60 * 1000) {
      return this.failure('stale', 'ts is more than 15 minutes from the host clock.', message);
    }
    const key = `${message.from}\n${message.id}`;
    const jcs = canonicalize(message);
    const prior = this.idempotency.get(key);
    if (prior) {
      if (prior.jcs === jcs) return structuredClone(prior.response);
      return {
        status: 409,
        message: this.emit({
          type: 'error',
          to: party.id,
          re: message.id,
          body: { code: 'replay', message: 'This id was already used with different content.' },
        }),
      };
    }
    const chainError = this.checkChain(message);
    if (chainError) return this.remember(party, message, chainError);
    const sigError = this.checkSig(party, message);
    if (sigError) return this.remember(party, message, sigError);
    const response = this.apply(party, message);
    this.noteChain(message);
    return this.remember(party, message, response);
  }

  hostForm(message) {
    if (message.type === 'space' && message.body?.action === 'listed') return true;
    if (message.type === 'receipt' && message.body?.method !== 'attestation') return true;
    if (message.type === 'payout' && (message.body?.state || message.body?.amount)) return true;
    if (message.type === 'reputation' && message.body?.algorithm) return true;
    if (message.type === 'dispute' && message.body?.action === 'resolve') return true;
    return false;
  }

  checkChain(message) {
    if (!message.deal) return null;
    const key = `${message.from}\n${message.deal}`;
    const prev = this.chain.get(key);
    if (message.prev) {
      if (!prev || message.prev !== digest(prev.message)) {
        return this.failure('chain_broken', 'prev is not the hash of the previous message in this deal.', message);
      }
    } else if (prev?.required) {
      return this.failure('chain_broken', 'A later message in this deal omitted prev.', message);
    }
    return null;
  }

  noteChain(message) {
    if (!message.deal) return;
    const key = `${message.from}\n${message.deal}`;
    const prev = this.chain.get(key);
    this.chain.set(key, { message, required: Boolean(message.prev) || Boolean(prev?.required) });
  }

  checkSig(party, message) {
    if (message.sig) {
      if (!party.key || !verifyEnvelope(message, party.key)) {
        return this.failure('bad_signature', 'sig does not verify.', message);
      }
    } else if (party.key && BINDING.has(message.type)) {
      return this.failure('unauthorized', 'This party registered a key, so this message must be signed.', message);
    }
    return null;
  }

  remember(party, message, response) {
    this.idempotency.set(`${message.from}\n${message.id}`, {
      jcs: canonicalize(message),
      response: structuredClone(response),
    });
    this.save();
    return response;
  }

  apply(party, message) {
    switch (message.type) {
      case 'hello': return this.onHello(party, message);
      case 'discover': return this.onDiscover(party, message);
      case 'space': return this.onSpace(party, message);
      case 'request': return this.onRequestMessage(party, message);
      case 'offer': return this.onOffer(party, message);
      case 'counter': return this.onCounter(party, message);
      case 'accept': return this.onAccept(party, message);
      case 'decline': return this.onDecline(party, message);
      case 'execute': return this.onExecute(party, message);
      case 'receipt': return this.onReceipt(party, message);
      case 'revoke': return this.onRevoke(party, message);
      case 'dispute': return this.onDispute(party, message);
      case 'reputation': return this.onReputation(party, message);
      case 'payout': return this.onPayout(party, message);
      default: return this.failure('unsupported_type', 'The type is not supported.', message);
    }
  }

  onHello(party, message) {
    const reply = this.emit({
      type: 'hello',
      to: party.id,
      re: message.id,
      body: {
        versions: ['0.3'],
        party: party.id,
        capabilities: this.capabilities(),
        limits: { messages_per_minute: 60, max_body_bytes: 65536 },
      },
    });
    this.deliver(party.id, reply);
    return { status: 200, message: reply };
  }

  onDiscover(party, message) {
    const items = message.body.want === 'offers' ? this.offerItems() : this.spaceItems();
    const reply = this.emit({
      type: 'results',
      to: party.id,
      re: message.id,
      body: items.length ? { items } : { items, empty_reason: 'no_match' },
    });
    this.deliver(party.id, reply);
    return { status: 200, message: reply };
  }

  onSpace(party, message) {
    if (message.body.action === 'withdraw') return this.withdrawSpace(party, message);
    const decision = this.evaluate(party, 'list', {});
    if (decision.error) return this.failure(decision.error.code, decision.error.message, message, decision.error);
    const space = structuredClone(message.body.space);
    const record = { owner: party.id, space, listed: false };
    this.spaces.set(`${party.id}/${space.id}`, record);
    if (decision.escalate) {
      const esc = this.openEscalation(party, 'list', decision.escalate.reasons, { kind: 'space', spaceId: space.id });
      record.waiting = esc.waiting;
      const reply = this.spaceReply(party, message, record, 'waiting');
      return { status: 200, message: reply };
    }
    record.listed = true;
    const reply = this.spaceReply(party, message, record, 'listed');
    return { status: 200, message: reply };
  }

  withdrawSpace(party, message) {
    const record = this.spaces.get(`${party.id}/${message.body.id}`);
    if (!record) return this.failure('not_found', 'No such space for this sender.', message);
    record.listed = false;
    record.withdrawn = true;
    const reply = this.emit({
      type: 'space',
      to: party.id,
      re: message.id,
      body: { action: 'listed', space: this.visibleSpace(record, true), status: 'withdrawn' },
    });
    this.deliver(party.id, reply);
    return { status: 200, message: reply };
  }

  spaceReply(party, message, record, status) {
    const body = { action: 'listed', space: this.visibleSpace(record, true), status };
    if (record.waiting) body.waiting = record.waiting;
    const reply = this.emit({ type: 'space', to: party.id, re: message.id, body });
    this.deliver(party.id, reply);
    return reply;
  }

  visibleSpace(record, owner) {
    const space = structuredClone(record.space);
    if (!owner) delete space.locator;
    return space;
  }

  onRequestMessage(party, message) {
    const recipient = this.parties.get(message.to);
    if (!recipient) return this.failure('not_found', 'No such party for this sender.', message);
    const decision = this.evaluate(party, 'negotiate', { counterparty: recipient, terms: null });
    if (decision.error) return this.failure(decision.error.code, decision.error.message, message, decision.error);
    if (decision.escalate) {
      this.openEscalation(party, 'negotiate', decision.escalate.reasons, { kind: 'request', message });
      return { status: 200, message: null };
    }
    this.deliver(recipient.id, message);
    return { status: 202, message: null };
  }

  onOffer(party, message) {
    const hashError = hashMatches(message.body.terms, message.body.terms_hash);
    if (hashError) return this.failure('terms_mismatch', hashError, message);
    const directed = Boolean(message.to);
    const recipient = directed ? this.parties.get(message.to) : null;
    if (directed && !recipient) return this.failure('not_found', 'No such party for this sender.', message);
    const step = directed ? 'negotiate' : 'list';
    const decision = this.evaluate(party, step, {
      counterparty: recipient,
      terms: message.body.terms,
      role: 'offerer',
    });
    if (decision.error) return this.failure(decision.error.code, decision.error.message, message, decision.error);
    const terms = structuredClone(message.body.terms);
    const termsHash = digest(terms);
    if (directed) {
      const deal = this.newDeal({
        offerer: party.id,
        acceptor: recipient.id,
        terms,
        termsHash,
        re: message.id,
        expires: message.body.expires,
      });
      if (!decision.escalate) this.bind(deal, party.id, 'mandate');
      else this.hold(deal, party, 'negotiate', decision.escalate.reasons, { kind: 'bind-offerer' });
      this.deals.set(deal.id, deal);
      return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
    }
    const offer = {
      id: localId('ofr_'),
      owner: party.id,
      terms,
      termsHash,
      limit: message.body.limit,
      expires: message.body.expires,
      listed: !decision.escalate,
      checked: this.offerChecked(terms),
      bound: null,
      created: this.iso(),
    };
    if (!decision.escalate) {
      offer.bound = { party: party.id, authority: { type: 'mandate' }, at: this.iso() };
    }
    this.offers.set(offer.id, offer);
    if (decision.escalate) {
      const esc = this.openEscalation(party, 'list', decision.escalate.reasons, { kind: 'offer', offerId: offer.id, pays: this.owes(terms, 'offerer') });
      offer.waiting = esc.waiting;
    }
    const reply = this.offerReply(party, message, offer);
    return { status: 200, message: reply };
  }

  offerReply(party, message, offer) {
    const body = { id: offer.id, terms: offer.terms, terms_hash: offer.termsHash };
    if (offer.limit) body.limit = offer.limit;
    if (offer.expires) body.expires = offer.expires;
    if (offer.waiting) body.waiting = offer.waiting;
    const reply = this.emit({ type: 'offer', to: party.id, re: message.id, body });
    this.deliver(party.id, reply);
    return reply;
  }

  onCounter(party, message) {
    const deal = this.deals.get(message.deal);
    if (!deal || !partyRole(deal, party.id)) return this.failure('not_found', 'No such deal for this sender.', message);
    if (!['proposed', 'awaiting', 'active'].includes(deal.state)) return this.failure('state_conflict', 'The deal is in the wrong state.', message);
    if ((deal.round || 1) >= 100) return this.failure('state_conflict', 'The round cap is reached.', message);
    const hashError = hashMatches(message.body.terms, message.body.terms_hash);
    if (hashError) return this.failure('terms_mismatch', hashError, message);
    const step = deal.state === 'active' ? 'amend' : 'negotiate';
    const decision = this.evaluate(party, step, {
      counterparty: this.parties.get(otherParty(deal, party.id)),
      terms: message.body.terms,
      role: partyRole(deal, party.id),
    });
    if (decision.error) return this.failure(decision.error.code, decision.error.message, message, decision.error);
    const previous = { terms: deal.terms, termsHash: deal.termsHash, bound: deal.bound, state: deal.state, tracking: deal.tracking };
    deal.terms = structuredClone(message.body.terms);
    deal.termsHash = digest(deal.terms);
    deal.bound = [];
    deal.waiting = null;
    deal.round = (deal.round || 1) + 1;
    deal.obligations = deal.terms.obligations.map((item) => ({ id: item.id, state: 'pending' }));
    if (deal.tracking.length) this.plainLinks(deal);
    deal.re = message.id;
    if (decision.escalate) {
      this.hold(deal, party, step, decision.escalate.reasons, { kind: 'counter', previous, pays: this.owes(deal.terms, partyRole(deal, party.id)) });
    } else {
      this.bind(deal, party.id, 'mandate');
      deal.state = 'proposed';
    }
    deal.updated = this.iso();
    return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
  }

  onAccept(party, message) {
    if (message.body.offer) return this.acceptOffer(party, message);
    const deal = this.deals.get(message.deal);
    if (!deal || !partyRole(deal, party.id)) return this.failure('not_found', 'No such deal for this sender.', message);
    if (message.body.terms_hash !== deal.termsHash) return this.failure('terms_mismatch', 'terms_hash does not match the terms.', message);
    if (!['proposed', 'awaiting'].includes(deal.state)) return this.failure('state_conflict', 'The deal is in the wrong state.', message);
    const decision = this.evaluate(party, 'accept', {
      counterparty: this.parties.get(otherParty(deal, party.id)),
      terms: deal.terms,
      role: partyRole(deal, party.id),
    });
    if (decision.error) return this.failure(decision.error.code, decision.error.message, message, decision.error);
    deal.re = message.id;
    if (decision.escalate) this.hold(deal, party, 'accept', decision.escalate.reasons, { kind: 'bind', pays: this.owes(deal.terms, partyRole(deal, party.id)) });
    else this.finishBind(deal, party.id, 'mandate');
    return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
  }

  acceptOffer(party, message) {
    const offer = this.offers.get(message.body.offer);
    if (!offer || !offer.listed || offer.owner === party.id) return this.failure('not_found', 'No such offer for this sender.', message);
    if (message.body.terms_hash !== offer.termsHash) return this.failure('terms_mismatch', 'terms_hash does not match the terms.', message);
    const decision = this.evaluate(party, 'accept', {
      counterparty: this.parties.get(offer.owner),
      terms: offer.terms,
      role: 'acceptor',
      claimsUnchecked: !offer.checked,
    });
    if (decision.error) return this.failure(decision.error.code, decision.error.message, message, decision.error);
    const deal = this.newDeal({
      offerer: offer.owner,
      acceptor: party.id,
      terms: offer.terms,
      termsHash: offer.termsHash,
      offerId: offer.id,
      re: message.id,
      expires: offer.expires,
      checked: offer.checked,
    });
    if (offer.bound) deal.bound.push(structuredClone(offer.bound));
    this.deals.set(deal.id, deal);
    if (decision.escalate) this.hold(deal, party, 'accept', decision.escalate.reasons, { kind: 'bind', pays: this.owes(deal.terms, 'acceptor') });
    else this.finishBind(deal, party.id, 'mandate');
    return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
  }

  onDecline(party, message) {
    const deal = this.deals.get(message.deal);
    if (!deal || !partyRole(deal, party.id)) return this.failure('not_found', 'No such deal for this sender.', message);
    if (!['proposed', 'awaiting'].includes(deal.state)) return this.failure('state_conflict', 'The deal is in the wrong state.', message);
    deal.state = 'declined';
    deal.waiting = null;
    deal.updated = this.iso();
    deal.re = message.id;
    return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
  }

  onExecute(party, message) {
    const deal = this.deals.get(message.deal);
    if (!deal || !partyRole(deal, party.id)) return this.failure('not_found', 'No such deal for this sender.', message);
    if (deal.state !== 'active') return this.failure('state_conflict', 'The deal is in the wrong state.', message);
    const obligation = deal.terms.obligations.find((item) => item.id === message.body.obligation);
    if (!obligation) return this.failure('not_found', 'No such obligation for this sender.', message);
    if (roleParty(deal, obligation.by) !== party.id) return this.failure('forbidden', 'Only the obligor may claim this obligation.', message);
    const row = deal.obligations.find((item) => item.id === obligation.id);
    const decision = this.evaluate(party, 'execute', {
      counterparty: this.parties.get(otherParty(deal, party.id)),
      terms: deal.terms,
      role: partyRole(deal, party.id),
      claimsUnchecked: CREATIVE.has(obligation.action) && !deal.checked,
    });
    if (decision.error) return this.failure(decision.error.code, decision.error.message, message, decision.error);
    deal.re = message.id;
    if (decision.escalate) {
      this.hold(deal, party, 'execute', decision.escalate.reasons, { kind: 'execute', obligation: obligation.id });
    } else if (row && row.state === 'pending') row.state = 'claimed';
    deal.updated = this.iso();
    return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
  }

  onReceipt(party, message) {
    const deal = this.deals.get(message.deal);
    if (!deal || !partyRole(deal, party.id)) return this.failure('not_found', 'No such deal for this sender.', message);
    const obligation = deal.terms.obligations.find((item) => item.id === message.body.obligation);
    if (!obligation) return this.failure('not_found', 'No such obligation for this sender.', message);
    const obligor = roleParty(deal, obligation.by);
    const counterparty = otherParty(deal, obligor);
    if (message.body.method !== 'attestation') return this.failure('forbidden', 'Only the host issues this receipt.', message);
    if (party.id === obligor) return this.failure('forbidden', 'An obligor cannot receipt its own obligation.', message);
    if (party.id !== counterparty) return this.failure('forbidden', 'Only the counterparty may attest.', message);
    if (!(obligation.proof || []).includes('attestation') && obligation.action !== 'pay') {
      return this.failure('forbidden', 'This obligation does not accept attestation.', message);
    }
    this.applyReceipt(deal, obligation.id, message.body.result, null);
    const receiptBody = {
      obligation: obligation.id,
      result: message.body.result,
      method: 'attestation',
      at: message.body.at || this.iso(this.businessNow()),
    };
    this.deliver(deal.offerer, this.emit({ type: 'receipt', to: deal.offerer, deal: deal.id, re: message.id, body: receiptBody }));
    this.deliver(deal.acceptor, this.emit({ type: 'receipt', to: deal.acceptor, deal: deal.id, re: message.id, body: structuredClone(receiptBody) }));
    if (message.body.result === 'verified') this.maybeAttestedPayout(deal, obligation, party.id);
    deal.updated = this.iso();
    return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
  }

  onRevoke(party, message) {
    if (message.body.scope === 'offer') {
      const offer = this.offers.get(message.body.offer);
      if (!offer || offer.owner !== party.id) return this.failure('not_found', 'No such offer for this sender.', message);
      const decision = this.evaluate(party, 'revoke', { terms: offer.terms, role: 'offerer' });
      if (decision.error) return this.failure(decision.error.code, decision.error.message, message, decision.error);
      offer.listed = false;
      const reply = this.offerReply(party, message, offer);
      return { status: 200, message: reply };
    }
    const deal = this.deals.get(message.deal);
    if (!deal || !partyRole(deal, party.id)) return this.failure('not_found', 'No such deal for this sender.', message);
    const decision = this.evaluate(party, 'revoke', { terms: deal.terms, role: partyRole(deal, party.id) });
    if (decision.error) return this.failure(decision.error.code, decision.error.message, message, decision.error);
    if (message.body.effective && Date.parse(message.body.effective) > this.businessNow()) {
      deal.revokeAt = message.body.effective;
      deal.updated = this.iso();
      deal.re = message.id;
      return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
    }
    this.revokeDeal(deal);
    deal.re = message.id;
    return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
  }

  onDispute(party, message) {
    const deal = this.deals.get(message.deal);
    if (!deal || !partyRole(deal, party.id)) return this.failure('not_found', 'No such deal for this sender.', message);
    if (message.body.action === 'open') {
      if (!['active', 'completed', 'disputed'].includes(deal.state)) return this.failure('state_conflict', 'The deal is in the wrong state.', message);
      const dispute = {
        id: localId('dsp_'),
        dealId: deal.id,
        opener: party.id,
        obligation: message.body.obligation,
        claim: message.body.claim,
        open: true,
      };
      this.disputes.set(dispute.id, dispute);
      deal.state = 'disputed';
      deal.updated = this.iso();
      deal.re = message.id;
      for (const payout of this.payouts.values()) {
        if (payout.dealId === deal.id && payout.obligation === dispute.obligation && payout.state === 'payable') payout.state = 'held';
      }
      return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
    }
    const dispute = this.disputes.get(message.body.dispute);
    if (!dispute || dispute.dealId !== deal.id) return this.failure('not_found', 'No such dispute for this sender.', message);
    if (message.body.action === 'withdraw') {
      if (dispute.opener !== party.id) return this.failure('forbidden', 'Only the opener may withdraw.', message);
      dispute.open = false;
      if (![...this.disputes.values()].some((item) => item.dealId === deal.id && item.open)) deal.state = 'active';
      deal.updated = this.iso();
      return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
    }
    if (message.body.action === 'respond') {
      dispute.response = message.body.statement || '';
      return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
    }
    return this.failure('forbidden', 'Only the host may resolve a dispute.', message);
  }

  onReputation(party, message) {
    const subject = message.body.subject;
    if (!this.parties.has(subject) && subject !== this.hostId) return this.failure('not_found', 'No such party for this sender.', message);
    const score = this.reputation(subject);
    const reply = this.emit({
      type: 'reputation',
      to: party.id,
      re: message.id,
      body: {
        subject,
        algorithm: 'orp-rep-1',
        score: score.score,
        components: score.components,
        computed_at: this.iso(),
        valid_until: this.iso(this.wallNow() + DAY),
      },
    });
    this.deliver(party.id, reply);
    return { status: 200, message: reply };
  }

  onPayout(party, message) {
    const payout = this.payouts.get(message.body.payout);
    if (!payout || payout.payee !== party.id) return this.failure('not_found', 'No such payout for this sender.', message);
    const deal = this.deals.get(payout.dealId);
    const decision = this.evaluate(party, 'settle', { terms: deal.terms, role: partyRole(deal, party.id) });
    if (decision.error) return this.failure(decision.error.code, decision.error.message, message, decision.error);
    if (decision.escalate) {
      this.hold(deal, party, 'settle', decision.escalate.reasons, { kind: 'payout', payout: payout.id, answer: message.body.answer });
      return { status: 200, message: this.publishDeal(deal, message.id, party.id) };
    }
    this.answerPayout(payout, message.body.answer);
    const reply = this.payoutMessage(payout, party.id, message.id);
    return { status: 200, message: reply };
  }

  answerPayout(payout, answer) {
    payout.answer = answer;
    if (answer === 'confirm' && payout.state === 'sent') payout.state = 'paid';
    payout.at = this.iso(this.businessNow());
    const replyOfferer = this.payoutMessage(payout, payout.payer, '');
    const replyPayee = this.payoutMessage(payout, payout.payee, '');
    this.deliver(payout.payer, replyOfferer);
    this.deliver(payout.payee, replyPayee);
  }

  suggest(body) {
    const shape = check('suggest-request', body);
    if (!shape.ok) return [400, this.plainError('invalid_message', 'The suggest request is not valid.', { field: shape.field || '' })];
    const dirtyAt = dirty(body);
    if (dirtyAt) return [400, this.plainError('invalid_message', 'The suggest request contains disallowed characters or markup.', { field: dirtyAt })];
    const items = this.matchOffers(body).slice(0, body.limit || 3);
    const response = {
      items,
      join: `${this.origin}/orp/join`,
    };
    if (!items.length) response.empty_reason = 'no_match';
    if (body.join) {
      const [status, joined] = this.join(body.join);
      if (status !== 201) return [status, joined];
      response.joined = joined;
    }
    const verdict = check('suggest-response', response);
    if (!verdict.ok) throw new Error(`suggest response invalid at ${verdict.field}`);
    this.save();
    return [200, response];
  }

  matchOffers(body) {
    const hay = `${body.text} ${(body.kinds || []).join(' ')}`.toLowerCase();
    const words = new Set(hay.split(/[^a-z0-9]+/).filter((word) => word.length >= 4));
    return [...this.offers.values()].filter((offer) => offer.listed).map((offer) => {
      const blob = `${offer.terms.title} ${(offer.terms.categories || []).join(' ')} ${offer.terms.kind || ''}`.toLowerCase();
      const hit = [...words].some((word) => blob.includes(word));
      return hit ? this.offerItem(offer, body.text) : null;
    }).filter(Boolean);
  }

  join(body) {
    const shape = check('join-request', body || {});
    if (!shape.ok) return [400, this.plainError('invalid_message', 'The join request is not valid.', { field: shape.field || '' })];
    const personal = Object.keys(body || {}).filter((key) => !['agent', 'key'].includes(key));
    if (personal.length) return [400, this.plainError('invalid_message', 'Join asks for nothing personal.', { field: `/${personal[0]}` })];
    const opaque = `p_${token(12)}`;
    const party = {
      id: `orp:${this.dns}/${opaque}`,
      token: token(24),
      ownerToken: '',
      claimSecret: token(18),
      key: body?.key ? structuredClone(body.key) : null,
      claimed: false,
      constraints: { steps: { ...DEFAULT_STEPS } },
    };
    this.parties.set(party.id, party);
    this.byToken.set(party.token, party.id);
    this.byClaim.set(party.claimSecret, party.id);
    const response = {
      party: party.id,
      token: party.token,
      claim: `${this.origin}/claim/${party.claimSecret}`,
      steps: party.constraints.steps,
      limits: { open_deals: 5, messages_per_minute: 60 },
    };
    const verdict = check('join-response', response);
    if (!verdict.ok) throw new Error(`join response invalid at ${verdict.field}`);
    this.save();
    return [201, response];
  }

  claim(secret, body) {
    if (body && Object.keys(body).length) return [400, this.plainError('invalid_message', 'Claim takes an empty body.', { field: `/${Object.keys(body)[0]}` })];
    const partyId = this.byClaim.get(secret);
    if (!partyId) return [404, this.plainError('not_found', 'No such claim.')];
    const party = this.parties.get(partyId);
    party.claimed = true;
    if (!party.ownerToken) {
      party.ownerToken = token(24);
      this.byOwner.set(party.ownerToken, party.id);
    }
    this.save();
    return [200, { owner_token: party.ownerToken }];
  }

  setConstraints(auth, body) {
    const shape = check('constraints', body);
    if (!shape.ok) return [400, this.plainError('invalid_message', 'Constraints are not valid.', { field: shape.field || '' })];
    if (!auth.owner && this.loosens(auth.party.constraints, body)) {
      return [403, this.plainError('forbidden', 'An agent cannot loosen its constraints.')];
    }
    auth.party.constraints = structuredClone(body);
    this.save();
    return [200, { ok: true }];
  }

  loosens(prev, next) {
    const prevSteps = { ...DEFAULT_STEPS, ...prev.steps };
    const nextSteps = { ...DEFAULT_STEPS, ...next.steps };
    for (const step of Object.keys(DEFAULT_STEPS)) {
      if (STEP_RANK[nextSteps[step]] > STEP_RANK[prevSteps[step]]) return true;
    }
    const prevRate = prev.budget?.max_rate_bps;
    const nextRate = next.budget?.max_rate_bps;
    if (prevRate && (!nextRate || nextRate > prevRate)) return true;
    for (const cap of ['per_deal', 'per_day', 'per_month']) {
      const before = prev.budget?.[cap]?.amount_minor;
      const after = next.budget?.[cap]?.amount_minor;
      if (before != null && (after == null || after > before)) return true;
    }
    if (prev.approve_over && (!next.approve_over || next.approve_over.amount_minor > prev.approve_over.amount_minor)) return true;
    if (prev.kinds && (!next.kinds || next.kinds.some((kind) => !prev.kinds.includes(kind)))) return true;
    const prevNew = prev.counterparties?.new;
    const nextNew = next.counterparties?.new;
    if (prevNew && nextNew && STEP_RANK[nextNew] > STEP_RANK[prevNew]) return true;
    if (prevNew === 'off' && !nextNew) return true;
    return false;
  }

  approve(auth, escalationId, body) {
    if (!auth.owner) return [403, this.plainError('forbidden', 'An agent cannot approve for its owner.')];
    const esc = this.escalations.get(escalationId);
    if (!esc || esc.party !== auth.party.id) return [404, this.plainError('not_found', 'No such escalation.')];
    const assurance = body?.assurance || 'session';
    const decision = body?.decision || 'approved';
    if (!['approved', 'refused'].includes(decision) || !['session', 'passkey', 'email_link', 'client_prompt'].includes(assurance)) {
      return [400, this.plainError('invalid_message', 'The approval is not valid.', { field: '/decision' })];
    }
    if (decision === 'approved' && assurance === 'client_prompt' && esc.pays) {
      return [403, this.plainError('forbidden', 'client_prompt cannot bind a party to pay.')];
    }
    const consentId = localId('cns_');
    let consent = null;
    if (esc.dealId) {
      const body = {
        consent: consentId,
        escalation: esc.id,
        party: esc.party,
        decision,
        step: esc.step,
        terms_hash: esc.termsHash,
        reasons: esc.reasons,
        assurance,
        at: this.iso(),
      };
      consent = this.emit({ type: 'consent', to: esc.party, deal: esc.dealId, body });
      this.deliver(esc.party, consent);
      const other = otherParty(this.deals.get(esc.dealId), esc.party);
      const otherBody = { ...body };
      delete otherBody.reasons;
      this.deliver(other, this.emit({ type: 'consent', to: other, deal: esc.dealId, body: otherBody }));
    }
    esc.resolved = true;
    if (decision === 'refused') this.refuseEscalation(esc);
    else this.applyEscalation(esc, consentId);
    this.save();
    const deal = esc.dealId ? this.deals.get(esc.dealId) : null;
    return [200, deal ? this.snapshot(deal, esc.party, consent?.id || '') : { ok: true }];
  }

  refuseEscalation(esc) {
    if (esc.kind === 'offer') {
      const offer = this.offers.get(esc.offerId);
      if (offer) offer.listed = false;
      return;
    }
    if (esc.kind === 'space') {
      const record = this.spaces.get(`${esc.party}/${esc.spaceId}`);
      if (record) record.listed = false;
      return;
    }
    const deal = this.deals.get(esc.dealId);
    if (!deal) return;
    if (esc.previous) {
      deal.terms = esc.previous.terms;
      deal.termsHash = esc.previous.termsHash;
      deal.bound = esc.previous.bound;
      deal.state = esc.previous.state;
      deal.tracking = esc.previous.tracking;
    } else deal.state = 'declined';
    deal.waiting = null;
    deal.updated = this.iso();
  }

  applyEscalation(esc, consentId) {
    if (esc.kind === 'offer') {
      const offer = this.offers.get(esc.offerId);
      offer.listed = true;
      offer.waiting = null;
      offer.bound = { party: esc.party, authority: { type: 'consent', ref: consentId }, at: this.iso() };
      return;
    }
    if (esc.kind === 'space') {
      const record = this.spaces.get(`${esc.party}/${esc.spaceId}`);
      record.listed = true;
      record.waiting = null;
      return;
    }
    if (esc.kind === 'request') {
      this.deliver(esc.message.to, esc.message);
      return;
    }
    const deal = this.deals.get(esc.dealId);
    if (!deal) return;
    deal.waiting = null;
    if (esc.kind === 'bind' || esc.kind === 'bind-offerer') this.finishBind(deal, esc.party, 'consent', consentId);
    else if (esc.kind === 'execute') {
      const row = deal.obligations.find((item) => item.id === esc.obligation);
      if (row && row.state === 'pending') row.state = 'claimed';
    } else if (esc.kind === 'counter') this.finishBind(deal, esc.party, 'consent', consentId);
    else if (esc.kind === 'payout') this.answerPayout(this.payouts.get(esc.payout), esc.answer);
    deal.updated = this.iso();
    if (esc.kind !== 'payout') this.publishDeal(deal, '');
  }

  evaluate(party, step, ctx) {
    const constraints = party.constraints || { steps: {} };
    const mode = constraints.steps?.[step] || DEFAULT_STEPS[step];
    if (mode === 'off') return { error: { code: 'step_off', message: `The owner has turned off the ${step} step.`, step } };
    const counterparty = ctx.counterparty;
    if (counterparty && constraints.counterparties?.deny?.includes(counterparty.id)) {
      return { error: { code: 'forbidden', message: 'The counterparty is denied.' } };
    }
    if (counterparty && constraints.counterparties?.new === 'off' && !this.together(party.id, counterparty.id)) {
      return { error: { code: 'forbidden', message: 'New counterparties are off.' } };
    }
    const brand = ctx.terms ? this.brand(constraints, ctx.terms) : '';
    if (brand) return { error: { code: 'brand_rule', message: brand } };
    const reasons = [];
    if (mode === 'ask') reasons.push('step_ask');
    if (constraints.expires && Date.parse(constraints.expires) <= this.businessNow()) reasons.push('mandate_expired');
    if (ctx.terms) this.collectReasons(party, constraints, ctx, reasons);
    if (ctx.claimsUnchecked) reasons.push('claims_unchecked');
    if (!reasons.length) return { authority: 'mandate' };
    return { escalate: { reasons: [...new Set(reasons)], step } };
  }

  brand(constraints, terms) {
    const blob = textsOf(terms).toLowerCase();
    if ((constraints.brand?.must_not_include || []).some((phrase) => blob.includes(phrase.toLowerCase()))) return 'The terms break a brand rule.';
    const denied = constraints.categories?.deny || [];
    if ((terms.categories || []).some((category) => denied.some((item) => category === item || category.startsWith(`${item}.`)))) return 'The terms use a denied category.';
    const needsDisclosure = (terms.obligations || []).some((item) => item.action === 'place' || item.action === 'comment' || (item.action === 'repost' && item.quote));
    if (needsDisclosure && !terms.disclosure?.label) return 'The terms omit a required disclosure.';
    if (constraints.brand?.disclosure?.label && needsDisclosure && terms.disclosure?.label !== constraints.brand.disclosure.label) return 'The terms omit the owner disclosure.';
    return '';
  }

  collectReasons(party, constraints, ctx, reasons) {
    const { terms } = ctx;
    if (constraints.kinds && terms.kind && !constraints.kinds.includes(terms.kind)) reasons.push('kind_outside');
    if (constraints.categories?.allow?.length && (terms.categories || []).some((category) => !constraints.categories.allow.some((item) => category === item || category.startsWith(`${item}.`)))) {
      reasons.push('category_outside');
    }
    if (ctx.counterparty && constraints.counterparties?.new === 'ask' && !this.together(party.id, ctx.counterparty.id)) {
      reasons.push('new_counterparty');
    }
    if (ctx.counterparty && constraints.counterparties?.min_reputation != null) {
      const score = this.reputation(ctx.counterparty.id).score;
      if (score == null || score < constraints.counterparties.min_reputation) reasons.push('low_reputation');
    }
    const role = ctx.role;
    for (const obligation of terms.obligations || []) {
      if (obligation.action !== 'pay') continue;
      if (role && obligation.by !== role) continue;
      if (obligation.rate_bps) {
        const cap = constraints.budget?.max_rate_bps;
        if (!cap || obligation.rate_bps > cap) reasons.push('over_budget');
      }
      if (obligation.amount) {
        const cap = constraints.budget?.per_deal;
        if (!cap || obligation.amount.amount_minor > cap.amount_minor || obligation.amount.currency !== cap.currency) reasons.push('over_budget');
        if (constraints.approve_over && obligation.amount.amount_minor > constraints.approve_over.amount_minor) reasons.push('over_threshold');
      }
    }
  }

  together(a, b) {
    return [...this.deals.values()].some((deal) => (deal.offerer === a && deal.acceptor === b) || (deal.offerer === b && deal.acceptor === a));
  }

  owes(terms, role) {
    return (terms?.obligations || []).some((item) => item.action === 'pay' && item.by === role);
  }

  offerChecked(terms) {
    if (this.claims !== 'real') return false;
    const destinations = (terms.obligations || []).map((item) => item.destination).filter(Boolean);
    return destinations.every((url) => this.pages.has(url));
  }

  newDeal({ offerer, acceptor, terms, termsHash, offerId, re, expires, checked }) {
    return {
      id: localId('deal_'),
      state: 'proposed',
      offerer,
      acceptor,
      offerId: offerId || '',
      terms,
      termsHash,
      round: 1,
      bound: [],
      waiting: null,
      tracking: [],
      obligations: terms.obligations.map((item) => ({ id: item.id, state: 'pending' })),
      expires: expires || '',
      updated: this.iso(),
      re: re || '',
      pool: '',
      checked: Boolean(checked),
      counts: {},
      sources: {},
      revokeAt: '',
    };
  }

  bind(deal, partyId, type, ref) {
    deal.bound = deal.bound.filter((item) => item.party !== partyId);
    const authority = ref ? { type, ref } : { type };
    deal.bound.push({ party: partyId, authority, at: this.iso() });
  }

  finishBind(deal, partyId, type, ref) {
    this.bind(deal, partyId, type, ref);
    deal.waiting = null;
    const both = deal.bound.some((item) => item.party === deal.offerer) && deal.bound.some((item) => item.party === deal.acceptor);
    if (both) this.activate(deal);
    else deal.state = 'proposed';
    deal.updated = this.iso();
  }

  hold(deal, party, step, reasons, action) {
    const esc = this.openEscalation(party, step, reasons, { ...action, dealId: deal.id, termsHash: deal.termsHash, pays: action.pays || this.owes(deal.terms, partyRole(deal, party.id)) });
    deal.state = 'awaiting';
    deal.waiting = esc.waiting;
    deal.updated = this.iso();
    return esc;
  }

  openEscalation(party, step, reasons, action) {
    const id = localId('esc_');
    const waiting = {
      party: party.id,
      step,
      since: this.iso(),
      escalation: id,
      reasons,
      url: `${this.origin}/approve/${id}`,
    };
    const esc = {
      id,
      party: party.id,
      step,
      reasons,
      waiting,
      termsHash: action.termsHash || digest({ held: id }),
      pays: Boolean(action.pays),
      ...action,
    };
    this.escalations.set(id, esc);
    return esc;
  }

  activate(deal) {
    if (deal.state === 'active') return;
    deal.state = 'active';
    deal.waiting = null;
    deal.checked = deal.checked || this.offerChecked(deal.terms);
    for (const obligation of deal.terms.obligations) {
      if (obligation.action !== 'place') continue;
      const content = obligation.content || {};
      if (!content.link && !content.code) continue;
      if (content.direct && !content.link && !content.code) continue;
      const entry = {
        obligation: obligation.id,
        token: token(9),
        destination: obligation.destination,
        status: 'live',
        param: 'via',
        alias: 'ref',
      };
      if (content.code) entry.code = token(6).replace(/[^A-Za-z0-9]/g, 'x').slice(0, 8);
      deal.tracking.push(entry);
      this.links.set(entry.token, { dealId: deal.id, obligation: obligation.id });
    }
    deal.updated = this.iso();
  }

  revokeDeal(deal) {
    deal.state = 'revoked';
    deal.waiting = null;
    this.plainLinks(deal);
    for (const row of deal.obligations) {
      if (row.state === 'pending' || row.state === 'claimed') row.state = 'revoked';
    }
    deal.updated = this.iso();
  }

  plainLinks(deal) {
    for (const entry of deal.tracking) entry.status = 'plain';
  }

  publishDeal(deal, re, viewer) {
    deal.updated = this.iso();
    const offerer = this.snapshot(deal, deal.offerer, re);
    const acceptor = this.snapshot(deal, deal.acceptor, re);
    this.deliver(deal.offerer, offerer);
    this.deliver(deal.acceptor, acceptor);
    return viewer === deal.acceptor ? acceptor : offerer;
  }

  resolveDispute(id, resolution) {
    const dispute = this.disputes.get(id);
    if (!dispute) throw new Error(`no dispute ${id}`);
    const deal = this.deals.get(dispute.dealId);
    dispute.open = false;
    const body = { action: 'resolve', dispute: id, resolution };
    this.deliver(deal.offerer, this.emit({ type: 'dispute', to: deal.offerer, deal: deal.id, body }));
    this.deliver(deal.acceptor, this.emit({ type: 'dispute', to: deal.acceptor, deal: deal.id, body: structuredClone(body) }));
    if (resolution.outcome === 'rejected') deal.state = 'active';
    else if (resolution.remedy === 'void' || resolution.remedy === 'refund') this.revokeDeal(deal);
    else deal.state = 'active';
    if (resolution.outcome === 'upheld' && resolution.at_fault) {
      this.recordReputation(resolution.at_fault, {
        method: 'platform_check',
        result: 'dispute_lost',
        counterparty: otherParty(deal, resolution.at_fault),
      });
    }
    deal.updated = this.iso();
    this.publishDeal(deal, '');
    this.save();
  }

  snapshot(deal, viewer, re) {
    const body = {
      state: deal.state,
      offerer: deal.offerer,
      acceptor: deal.acceptor,
      terms: deal.terms,
      terms_hash: deal.termsHash,
      updated: deal.updated || this.iso(),
    };
    if (deal.offerId) body.offer = deal.offerId;
    if (deal.round) body.round = deal.round;
    if (deal.bound.length) body.bound = deal.bound.map((item) => structuredClone(item));
    if (deal.waiting) {
      body.waiting = viewer === deal.waiting.party
        ? structuredClone(deal.waiting)
        : { party: deal.waiting.party, since: deal.waiting.since, step: deal.waiting.step };
    }
    if (deal.tracking.length) {
      body.tracking = deal.tracking.map((entry) => {
        const item = {
          obligation: entry.obligation,
          link: `${this.origin}/go/${entry.token}`,
          destination: entry.destination,
          status: entry.status,
          param: entry.param,
        };
        if (entry.code) item.code = entry.code;
        if (entry.alias) item.alias = entry.alias;
        return item;
      });
    }
    if (deal.obligations.length) {
      body.obligations = deal.obligations.map((item) => {
        const row = { id: item.id, state: item.state };
        if (item.due) row.due = item.due;
        if (item.receipt) row.receipt = item.receipt;
        return row;
      });
    }
    if (deal.expires) body.expires = deal.expires;
    return this.emit({ type: 'deal', to: viewer, deal: deal.id, re, body });
  }

  click(req, url) {
    const headers = {
      'cache-control': 'no-store',
      'referrer-policy': 'no-referrer',
      'x-robots-tag': 'noindex',
    };
    const tokenValue = decodeURIComponent(url.pathname.slice('/go/'.length));
    const link = this.links.get(tokenValue);
    const deal = link ? this.deals.get(link.dealId) : null;
    const entry = deal?.tracking.find((item) => item.token === tokenValue);
    if (!entry) {
      headers.location = `${this.origin}/`;
      return { status: 302, headers };
    }
    const dest = new URL(entry.destination);
    if (entry.status === 'live') {
      const cls = this.classify(req, url, deal);
      const clickId = token(16);
      dest.searchParams.append(entry.param || 'via', clickId);
      if (entry.alias) dest.searchParams.append(entry.alias, clickId);
      this.noteClick(deal, entry, cls, clickId, sourceOf(req.headers.referer || req.headers.referrer));
    }
    headers.location = dest.toString();
    if (entry.status !== 'live') this.emitOutcome(deal);
    this.save();
    return { status: 302, headers };
  }

  classify(req, url, deal) {
    if (url.searchParams.get('orp_test') === '1' || deal.pool === 'test') return 'test';
    if (req.method === 'HEAD') return 'preview';
    const purpose = `${req.headers['sec-purpose'] || ''} ${req.headers.purpose || ''}`.toLowerCase();
    if (purpose.includes('prefetch') || purpose.includes('prerender')) return 'preview';
    const ua = String(req.headers['user-agent'] || '');
    if (!ua.trim()) return 'bot';
    const low = ua.toLowerCase();
    if (PREVIEW_AGENTS.some((name) => low.includes(name))) return 'preview';
    if (BOT_AGENTS.some((name) => low.includes(name))) return 'bot';
    return 'human';
  }

  noteClick(deal, entry, cls, clickId, source) {
    const bucket = deal.counts[entry.obligation] || { human: 0, bot: 0, preview: 0, test: 0 };
    bucket[cls] += 1;
    deal.counts[entry.obligation] = bucket;
    const sources = deal.sources[entry.obligation] || {};
    if (cls === 'human') sources[source] = (sources[source] || 0) + 1;
    deal.sources[entry.obligation] = sources;
    this.clicks.set(clickId, { dealId: deal.id, obligation: entry.obligation, code: entry.code || '' });
    if (entry.code) this.clicks.set(entry.code, { dealId: deal.id, obligation: entry.obligation, code: entry.code });
    const obligation = deal.terms.obligations.find((item) => item.id === entry.obligation);
    const row = deal.obligations.find((item) => item.id === entry.obligation);
    const target = obligation?.clicks || 1;
    if (cls === 'human' && row && row.state !== 'verified' && (obligation.proof || []).includes('link') && bucket.human >= target) {
      this.verifyByLink(deal, obligation, bucket.human);
    }
    this.emitOutcome(deal);
  }

  verifyByLink(deal, obligation, humans) {
    const at = this.iso(this.businessNow());
    const body = {
      obligation: obligation.id,
      result: 'verified',
      method: 'link',
      at,
      found: { link: true, clicks: humans },
    };
    if (obligation.keep) body.next_check = this.iso(this.businessNow() + DAY);
    const receipt = this.emit({ type: 'receipt', to: deal.acceptor, deal: deal.id, body });
    this.deliver(deal.acceptor, receipt);
    this.deliver(deal.offerer, this.emit({ type: 'receipt', to: deal.offerer, deal: deal.id, body: structuredClone(body) }));
    this.applyReceipt(deal, obligation.id, 'verified', receipt.id);
    this.recordReputation(roleParty(deal, obligation.by), {
      method: 'link',
      result: 'verified',
      counterparty: otherParty(deal, roleParty(deal, obligation.by)),
    });
    if (obligation.keep) {
      const row = deal.obligations.find((item) => item.id === obligation.id);
      row.rechecks = this.recheckPlan(obligation.keep);
      row.next = 0;
      for (const payout of this.payouts.values()) {
        if (payout.dealId === deal.id) payout.recheckHold = true;
      }
    }
    this.payoutForVerification(deal, obligation);
    deal.updated = this.iso();
    this.publishDeal(deal, receipt.id);
  }

  recheckPlan(keep) {
    const start = this.businessNow();
    const end = start + durationMs(keep);
    const plan = [];
    let next = start + DAY;
    while (next < end) {
      plan.push(next);
      next += 7 * DAY;
    }
    plan.push(end);
    return plan;
  }

  applyReceipt(deal, obligationId, result, receiptId) {
    const row = deal.obligations.find((item) => item.id === obligationId);
    if (!row) return;
    if (result === 'verified') row.state = 'verified';
    else if (result === 'failed') row.state = 'failed';
    else row.state = 'claimed';
    if (receiptId) row.receipt = receiptId;
  }

  payoutForVerification(deal, obligation) {
    for (const pay of deal.terms.obligations) {
      if (pay.action !== 'pay') continue;
      if (pay.after && pay.after !== obligation.id) continue;
      if (pay.per === 'click' || pay.per === 'conversion') continue;
      if (pay.per !== 'deal' && pay.per !== 'placement') continue;
      this.createPayout(deal, pay, {
        amount: pay.amount,
        basis: 'receipt',
        recheckHold: Boolean(obligation.keep),
      });
    }
  }

  createPayout(deal, pay, { amount, basis, recheckHold }) {
    if (!amount) return null;
    const payer = roleParty(deal, pay.by);
    const payee = otherParty(deal, payer);
    const holdMs = pay.hold ? durationMs(pay.hold) : (pay.per === 'conversion' ? 30 * DAY : 0);
    const payout = {
      id: localId('pay_'),
      dealId: deal.id,
      obligation: pay.id,
      payer,
      payee,
      state: 'held',
      amount,
      rail: pay.rail || 'manual',
      basis,
      holdUntil: holdMs ? this.iso(this.businessNow() + holdMs) : '',
      recheckHold: Boolean(recheckHold),
      at: this.iso(this.businessNow()),
      ref: '',
    };
    this.payouts.set(payout.id, payout);
    this.payoutMessage(payout, payer, '');
    this.payoutMessage(payout, payee, '');
    this.deliver(payer, this.payoutMessage(payout, payer, ''));
    this.deliver(payee, this.payoutMessage(payout, payee, ''));
    return payout;
  }

  payoutMessage(payout, to, re) {
    const body = {
      payout: payout.id,
      obligation: payout.obligation,
      state: payout.state,
      amount: payout.amount,
      rail: payout.rail,
      basis: payout.basis,
      at: payout.at,
    };
    if (payout.ref) body.ref = payout.ref;
    if (payout.holdUntil) body.hold_until = payout.holdUntil;
    return this.emit({ type: 'payout', to, deal: payout.dealId, re, body });
  }

  maybeAttestedPayout(deal, obligation, attester) {
    for (const pay of deal.terms.obligations) {
      if (pay.action !== 'pay' || pay.after !== obligation.id) continue;
      if (!(pay.proof || []).includes('attestation')) continue;
      if (roleParty(deal, pay.by) !== attester) continue;
      this.createPayout(deal, pay, { amount: pay.amount, basis: 'attestation', recheckHold: Boolean(obligation.keep) });
    }
  }

  emitOutcome(deal) {
    const rows = Object.entries(deal.counts).map(([obligation, clicks]) => {
      const sources = deal.sources[obligation] || {};
      const folded = {};
      let other = 0;
      for (const [source, human] of Object.entries(sources)) {
        if (human < 5) other += human;
        else folded[source] = human;
      }
      if (other) folded.other = (folded.other || 0) + other;
      const row = { obligation, clicks: { ...clicks } };
      const sourceList = Object.entries(folded).map(([source, human]) => ({ source, human }));
      if (sourceList.length) row.sources = sourceList;
      return row;
    });
    const body = {
      period: { from: this.iso(this.businessNow() - DAY), to: this.iso(this.businessNow()) },
      rows,
    };
    const offerer = this.emit({ type: 'outcome', to: deal.offerer, deal: deal.id, body });
    const acceptor = this.emit({ type: 'outcome', to: deal.acceptor, deal: deal.id, body: structuredClone(body) });
    this.deliver(deal.offerer, offerer);
    this.deliver(deal.acceptor, acceptor);
  }

  runDueChecks() {
    const now = this.businessNow();
    for (const deal of this.deals.values()) {
      let dirty = false;
      if (deal.revokeAt && Date.parse(deal.revokeAt) <= now && deal.state === 'active') {
        this.revokeDeal(deal);
        dirty = true;
      }
      if (deal.state === 'awaiting' && deal.waiting) {
        const deadline = deal.expires ? Date.parse(deal.expires) : Date.parse(deal.waiting.since) + 7 * DAY;
        if (Number.isFinite(deadline) && deadline <= now) {
          deal.state = 'expired';
          deal.waiting = null;
          deal.updated = this.iso();
          dirty = true;
        }
      }
      for (const row of deal.obligations) {
        if (!row.rechecks || row.state !== 'verified') continue;
        const due = row.rechecks[row.next];
        if (due == null || due > now) continue;
        const fail = this.recheckFail.has(`${deal.id}/${row.id}`);
        if (fail) {
          this.recheckFail.delete(`${deal.id}/${row.id}`);
          row.state = 'failed';
          row.rechecks = null;
          const failedBody = {
            obligation: row.id,
            result: 'failed',
            method: 'proof_url',
            at: this.iso(now),
            reason: 'removed',
          };
          this.deliver(deal.offerer, this.emit({ type: 'receipt', to: deal.offerer, deal: deal.id, body: failedBody }));
          this.deliver(deal.acceptor, this.emit({ type: 'receipt', to: deal.acceptor, deal: deal.id, body: structuredClone(failedBody) }));
          this.recordReputation(roleParty(deal, deal.terms.obligations.find((item) => item.id === row.id)?.by || 'acceptor'), {
            method: 'proof_url',
            result: 'failed',
            counterparty: otherParty(deal, roleParty(deal, 'acceptor')),
          });
          const dispute = {
            id: localId('dsp_'),
            dealId: deal.id,
            opener: this.hostId,
            obligation: row.id,
            claim: 'removed_early',
            open: true,
          };
          this.disputes.set(dispute.id, dispute);
          deal.state = 'disputed';
          const disputeBody = {
            action: 'open',
            dispute: dispute.id,
            obligation: row.id,
            claim: 'removed_early',
            statement: 'A recheck found the placement removed before keep ended.',
          };
          this.deliver(deal.offerer, this.emit({ type: 'dispute', to: deal.offerer, deal: deal.id, body: disputeBody }));
          this.deliver(deal.acceptor, this.emit({ type: 'dispute', to: deal.acceptor, deal: deal.id, body: structuredClone(disputeBody) }));
          for (const payout of this.payouts.values()) {
            if (payout.dealId === deal.id && payout.state !== 'reversed') payout.recheckHold = true;
          }
        } else {
          row.next += 1;
          if (row.next >= row.rechecks.length) {
            row.rechecks = null;
            for (const payout of this.payouts.values()) {
              if (payout.dealId === deal.id) payout.recheckHold = false;
            }
          }
        }
        dirty = true;
      }
      if (this.releaseHolds(deal)) dirty = true;
      if (dirty) this.publishDeal(deal, '');
    }
    this.save();
  }

  releaseHolds(deal) {
    let changed = false;
    for (const payout of this.payouts.values()) {
      if (payout.dealId !== deal.id || payout.state !== 'held' || payout.recheckHold) continue;
      if (payout.holdUntil && Date.parse(payout.holdUntil) > this.businessNow()) continue;
      payout.state = 'payable';
      payout.at = this.iso(this.businessNow());
      this.deliver(payout.payer, this.payoutMessage(payout, payout.payer, ''));
      this.deliver(payout.payee, this.payoutMessage(payout, payout.payee, ''));
      changed = true;
    }
    return changed;
  }

  provider(body) {
    const banned = Object.keys(body || {}).filter((key) => /email|phone|name|address|ip|user.?agent/i.test(key));
    if (banned.length) return [400, this.plainError('invalid_message', 'A provider event carries no personal data.', { field: `/${banned[0]}` })];
    if (body.type === 'sent') {
      const payout = this.payouts.get(body.payout);
      if (!payout) return [404, this.plainError('not_found', 'No such payout.')];
      payout.state = 'sent';
      payout.ref = String(body.ref || '');
      payout.at = this.iso(this.businessNow());
      this.deliver(payout.payer, this.payoutMessage(payout, payout.payer, ''));
      this.deliver(payout.payee, this.payoutMessage(payout, payout.payee, ''));
      this.save();
      return [200, { ok: true, state: payout.state }];
    }
    const found = this.clicks.get(body.click) || this.clicks.get(body.code);
    if (!found) return [404, this.plainError('not_found', 'No such click or code.')];
    const deal = this.deals.get(found.dealId);
    const pay = deal.terms.obligations.find((item) => item.action === 'pay' && (item.per === 'conversion' || item.per === 'click'));
    if (!pay) return [409, this.plainError('state_conflict', 'This deal has no conversion pay.')];
    if (body.type === 'refund') {
      for (const payout of this.payouts.values()) {
        if (payout.dealId === deal.id && payout.basis === 'provider' && payout.state !== 'reversed') {
          payout.state = 'reversed';
          payout.at = this.iso(this.businessNow());
          this.deliver(payout.payer, this.payoutMessage(payout, payout.payer, ''));
          this.deliver(payout.payee, this.payoutMessage(payout, payout.payee, ''));
        }
      }
      this.noteOutcomeMoney(deal, found.obligation, { refunds: 1 });
      this.save();
      return [200, { ok: true, state: 'reversed' }];
    }
    if (body.type !== 'conversion') return [400, this.plainError('invalid_message', 'Unknown provider event.', { field: '/type' })];
    const sale = Number(body.amount_minor || 0);
    const minor = pay.rate_bps ? Math.round(sale * pay.rate_bps / 10000) : (pay.amount?.amount_minor || 0);
    const amount = { amount_minor: minor, currency: body.currency || pay.amount?.currency || 'USD' };
    const payout = this.createPayout(deal, pay, { amount, basis: 'provider', recheckHold: false });
    this.noteOutcomeMoney(deal, found.obligation, { conversions: 1, revenue: { amount_minor: sale, currency: amount.currency }, earned: amount });
    this.save();
    return [200, { ok: true, payout: payout?.id, state: payout?.state }];
  }

  reportConversion(auth, body) {
    const party = auth.party;
    const allowed = ['deal', 'code', 'click'];
    const extra = Object.keys(body || {}).filter((key) => !allowed.includes(key));
    if (extra.length) return [400, this.plainError('invalid_message', 'A conversion report carries no personal data.', { field: `/${extra[0]}` })];
    const deal = this.deals.get(body.deal);
    if (!deal || !partyRole(deal, party.id)) return [404, this.plainError('not_found', 'No such deal for this sender.')];
    const pay = deal.terms.obligations.find((item) => item.action === 'pay');
    if (!pay || roleParty(deal, pay.by) !== party.id) return [403, this.plainError('forbidden', 'Only the payer may report a conversion.')];
    this.noteOutcomeMoney(deal, pay.after || deal.terms.obligations[0].id, { attested_conversions: 1 });
    let payout = null;
    if ((pay.proof || []).includes('attestation') && pay.amount) {
      payout = this.createPayout(deal, pay, { amount: pay.amount, basis: 'attestation', recheckHold: false });
    }
    this.save();
    return [200, { ok: true, payable: Boolean(payout), attested_only: !payout }];
  }

  noteOutcomeMoney(deal, obligation, extra) {
    const rows = [{
      obligation,
      clicks: deal.counts[obligation] || { human: 0 },
      ...extra,
    }];
    const body = {
      period: { from: this.iso(this.businessNow() - DAY), to: this.iso(this.businessNow()) },
      rows,
    };
    this.deliver(deal.offerer, this.emit({ type: 'outcome', to: deal.offerer, deal: deal.id, body }));
    this.deliver(deal.acceptor, this.emit({ type: 'outcome', to: deal.acceptor, deal: deal.id, body: structuredClone(body) }));
  }

  recordReputation(subject, { method, result, counterparty }) {
    if (!subject || method === 'attestation') return;
    const raw = result === 'verified' ? (METHOD_WEIGHT[method] || 0) : result === 'failed' ? 1 : result === 'dispute_lost' ? 2 : 0;
    if (!raw) return;
    const prior = this.events.filter((event) => event.subject === subject && event.counterparty === counterparty).length;
    const standing = this.standing(counterparty);
    const weight = raw * (1 / (prior + 1)) * standing;
    this.events.push({
      subject,
      counterparty,
      positive: result === 'verified' ? weight : 0,
      total: weight,
      result,
    });
    this.repCache.delete(subject);
  }

  standing(partyId) {
    const cached = this.repCache.get(partyId);
    const score = cached == null ? 0 : cached;
    return 0.25 + 0.75 * Math.min(1, score / 1000);
  }

  reputation(subject) {
    const mine = this.events.filter((event) => event.subject === subject);
    const positive = mine.reduce((sum, event) => sum + event.positive, 0);
    const total = mine.reduce((sum, event) => sum + event.total, 0);
    const party = this.parties.get(subject);
    let score = null;
    if (total >= 3 && party?.claimed) score = Math.max(0, Math.min(1000, Math.round(1000 * wilsonLower(positive, total))));
    this.repCache.set(subject, score);
    const counterparties = new Set(mine.map((event) => event.counterparty));
    return {
      score,
      components: {
        verified: Number(positive.toFixed(4)),
        failed: Number(mine.filter((event) => event.result === 'failed').reduce((sum, event) => sum + event.total, 0).toFixed(4)),
        disputes_lost: mine.filter((event) => event.result === 'dispute_lost').length,
        counterparties: counterparties.size,
        evidence_weight: Number(total.toFixed(4)),
      },
    };
  }

  offerItems() {
    return [...this.offers.values()].filter((offer) => offer.listed).map((offer) => this.offerItem(offer, ''));
  }

  offerItem(offer, text) {
    const item = {
      offer: { id: offer.id, terms: offer.terms, terms_hash: offer.termsHash },
      party: offer.owner,
      fit: { score: text ? 72 : 60, reasons: [text ? 'The draft shares words with this offer.' : 'Standing offer.'] },
      trust: { reputation: this.reputation(offer.owner).score, completed_deals: 0 },
      checks: { claims: offer.checked ? 'checked' : 'not_checked', at: offer.created || this.iso() },
      pitch: `You can take “${offer.terms.title}”. The link works with no adapter.`,
    };
    if (offer.expires) item.offer.expires = offer.expires;
    return item;
  }

  spaceItems() {
    return [...this.spaces.values()].filter((record) => record.listed).map((record) => ({
      space: this.visibleSpace(record, false),
      party: record.owner,
      fit: { score: 50, reasons: ['A listed space.'] },
      trust: { reputation: null },
    }));
  }

  inboxPage(partyId, after) {
    const rows = this.inbox.get(partyId) || [];
    const cursor = Number(after || 0);
    const messages = rows.filter((row) => row.seq > cursor).slice(0, 100);
    const next = messages.length ? String(messages[messages.length - 1].seq) : String(cursor || 0);
    const page = { messages: messages.map((row) => row.message), next };
    const verdict = check('inbox', page);
    if (!verdict.ok) throw new Error(`inbox invalid at ${verdict.field}`);
    return page;
  }

  deliver(partyId, message) {
    if (!partyId || partyId === this.hostId) return;
    const verdict = check('message', message);
    if (!verdict.ok) throw new Error(`refusing to deliver invalid ${message?.type} at ${verdict.field}`);
    const rows = this.inbox.get(partyId) || [];
    this.inboxSeq += 1;
    rows.push({ seq: this.inboxSeq, message });
    this.inbox.set(partyId, rows);
  }

  emit(message, { inbox = false } = {}) {
    const full = {
      orp: '0.3',
      id: localId('m_'),
      ts: this.iso(),
      from: this.hostId,
      ...message,
    };
    if (!full.re) delete full.re;
    if (!full.to) delete full.to;
    if (!full.deal) delete full.deal;
    const signed = SIGNED.has(full.type) ? signEnvelope(full, this.hostKey) : full;
    const verdict = check('message', signed);
    if (!verdict.ok) {
      throw new Error(`host emitted invalid ${full.type} at ${verdict.field}: ${JSON.stringify(verdict.errors?.slice(0, 3))}`);
    }
    if (signed.sig && !verifyEnvelope(signed, this.hostKey)) throw new Error(`host signature failed for ${full.type}`);
    this.outbound.push(signed);
    if (inbox && signed.to) this.deliver(signed.to, signed);
    return signed;
  }

  failure(code, message, inbound, extra = {}) {
    const body = { code, message };
    if (extra.step) body.step = extra.step;
    if (extra.field) body.field = extra.field;
    return {
      status: HTTP_STATUS[code] || 400,
      message: this.emit({ type: 'error', to: inbound?.from, re: inbound?.id, body }, { inbox: false }),
    };
  }

  save() {
    if (!this.storePath) return;
    const data = {
      hostKey: this.hostKey,
      providerToken: this.providerToken,
      businessShift: this.businessShift,
      inboxSeq: this.inboxSeq,
      pages: [...this.pages],
      parties: [...this.parties.values()],
      offers: [...this.offers.values()],
      deals: [...this.deals.entries()],
      links: [...this.links.entries()],
      clicks: [...this.clicks.entries()],
      idempotency: [...this.idempotency.entries()],
      inbox: [...this.inbox.entries()],
      spaces: [...this.spaces.entries()],
      escalations: [...this.escalations.entries()],
      payouts: [...this.payouts.entries()],
      disputes: [...this.disputes.entries()],
      events: this.events,
      recheckFail: [...this.recheckFail],
      chain: [...this.chain.entries()],
    };
    mkdirSync(dirname(this.storePath), { recursive: true });
    const tmp = `${this.storePath}.tmp`;
    writeFileSync(tmp, JSON.stringify(data));
    renameSync(tmp, this.storePath);
  }

  load() {
    let raw;
    try {
      raw = readFileSync(this.storePath, 'utf8');
    } catch {
      return;
    }
    const data = JSON.parse(raw);
    this.hostKey = data.hostKey;
    this.providerToken = data.providerToken;
    this.businessShift = data.businessShift || 0;
    this.inboxSeq = data.inboxSeq || 0;
    this.pages = new Set(data.pages || []);
    this.parties = new Map((data.parties || []).map((party) => [party.id, party]));
    this.byToken = new Map([...this.parties.values()].map((party) => [party.token, party.id]));
    this.byOwner = new Map([...this.parties.values()].filter((party) => party.ownerToken).map((party) => [party.ownerToken, party.id]));
    this.byClaim = new Map([...this.parties.values()].map((party) => [party.claimSecret, party.id]));
    this.offers = new Map((data.offers || []).map((offer) => [offer.id, offer]));
    this.deals = new Map(data.deals || []);
    this.links = new Map(data.links || []);
    this.clicks = new Map(data.clicks || []);
    this.idempotency = new Map(data.idempotency || []);
    this.inbox = new Map(data.inbox || []);
    this.spaces = new Map(data.spaces || []);
    this.escalations = new Map(data.escalations || []);
    this.payouts = new Map(data.payouts || []);
    this.disputes = new Map(data.disputes || []);
    this.events = data.events || [];
    this.recheckFail = new Set(data.recheckFail || []);
    this.chain = new Map(data.chain || []);
  }
}
