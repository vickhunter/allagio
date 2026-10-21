import { message, type CoreType, type EnvelopeFields, type MessageByType } from './message.ts';
import { checkDiscovery, checkInbox, checkJoinResponse, checkMessage, checkSuggestResponse, type Checked } from './validate.ts';
import type { Discovery, InboxPage, JoinRequest, JoinResponse, Message, PartyId, SuggestRequest, SuggestResponse } from './types.ts';

export interface ClientOptions {
  endpoint: string;
  token: string;
  party?: PartyId;
  agent?: { name: string; version?: string };
  fetch?: typeof fetch;
}

export class OrpProtocolError extends Error {
  readonly reply: MessageByType['error'];
  readonly status: number;

  constructor(reply: MessageByType['error'], status: number) {
    super(`${reply.body.code}: ${reply.body.message}`);
    this.reply = reply;
    this.status = status;
  }
}

/** Fetch and check /.well-known/orp.json for an origin. */
export async function fetchDiscovery(origin: string, fetcher: typeof fetch = fetch): Promise<Discovery> {
  const response = await fetcher(new URL('/.well-known/orp.json', origin));
  if (!response.ok) throw new Error(`discovery returned HTTP ${response.status}`);
  const checked = checkDiscovery(await response.json());
  if (!checked.ok) throw new Error(`discovery is invalid: ${checked.problems.map((p) => `${p.path} ${p.message}`).join('; ')}`);
  return checked.value;
}

export function resolveUrl(origin: string, value: string): string {
  return new URL(value, origin).toString();
}

// POST with no credentials to suggest or join. Errors come back as ORP error messages.
async function postOpen<T>(url: string, body: unknown, check: (value: unknown) => Checked<T>, fetcher: typeof fetch): Promise<T> {
  const response = await fetcher(url, { method: 'POST', headers: { 'content-type': 'application/json', accept: 'application/json' }, body: JSON.stringify(body) });
  const reply: unknown = await response.json();
  if (!response.ok) {
    const error = checkMessage(reply);
    if (error.ok && error.value.type === 'error') throw new OrpProtocolError(error.value, response.status);
    throw new Error(`HTTP ${response.status} from ${url}`);
  }
  const checked = check(reply);
  if (!checked.ok) throw new Error(`invalid reply from ${url}: ${checked.problems.map((p) => `${p.path} ${p.message}`).join('; ')}`);
  return checked.value;
}

/** Deals a person could take now for what their agent is writing. No account, no credentials. */
export async function suggest(origin: string, request: SuggestRequest, fetcher: typeof fetch = fetch): Promise<SuggestResponse> {
  const discovery = await fetchDiscovery(origin, fetcher);
  if (!discovery.suggest) throw new Error('this host does not offer suggestions');
  return postOpen(resolveUrl(origin, discovery.suggest), request, checkSuggestResponse, fetcher);
}

export class OrpClient {
  party?: PartyId;
  private readonly options: ClientOptions;

  constructor(options: ClientOptions) {
    this.options = options;
    this.party = options.party;
  }

  static async connect(origin: string, token: string, agent?: ClientOptions['agent']): Promise<OrpClient> {
    const discovery = await fetchDiscovery(origin);
    if (!discovery.endpoint) throw new Error('this host has no HTTP endpoint (Level 0 only)');
    const client = new OrpClient({ endpoint: resolveUrl(origin, discovery.endpoint), token, agent });
    await client.hello();
    return client;
  }

  /** Get a pseudonymous party with no email, name or website, and a client for it. Give `joined.claim` to your person. */
  static async join(origin: string, request: JoinRequest = {}, fetcher: typeof fetch = fetch): Promise<{ client: OrpClient; joined: JoinResponse }> {
    const discovery = await fetchDiscovery(origin, fetcher);
    if (!discovery.join || !discovery.endpoint) throw new Error('this host has no join endpoint');
    const joined = await postOpen(resolveUrl(origin, discovery.join), request, checkJoinResponse, fetcher);
    const client = new OrpClient({ endpoint: resolveUrl(origin, discovery.endpoint), token: joined.token, party: joined.party, fetch: fetcher, ...(request.agent ? { agent: request.agent } : {}) });
    await client.hello();
    return { client, joined };
  }

  private get fetcher(): typeof fetch {
    return this.options.fetch ?? fetch;
  }

  private headers(): Record<string, string> {
    return { authorization: `Bearer ${this.options.token}`, 'content-type': 'application/json', accept: 'application/json' };
  }

  build<T extends CoreType>(type: T, body: MessageByType[T]['body'], fields: EnvelopeFields = {}): MessageByType[T] {
    if (!this.party) throw new Error('call hello() first, or pass party');
    return message(type, this.party, body, { ...(this.options.agent ? { agent: this.options.agent } : {}), ...fields });
  }

  /** POST one message. Resolves to the reply, or undefined for 202 (relayed, nothing to return). */
  async send(outgoing: Message): Promise<Message | undefined> {
    const response = await this.fetcher(this.options.endpoint, { method: 'POST', headers: this.headers(), body: JSON.stringify(outgoing) });
    if (response.status === 202) return undefined;
    const checked = checkMessage(await response.json());
    if (!checked.ok) throw new Error(`host replied with an invalid message: ${checked.problems.map((p) => `${p.path} ${p.message}`).join('; ')}`);
    if (checked.value.type === 'error') throw new OrpProtocolError(checked.value, response.status);
    return checked.value;
  }

  /** Negotiate the version and learn this token's party id (hello may come from orp:<host>/self). */
  async hello(): Promise<MessageByType['hello']> {
    const from = this.party ?? `orp:${new URL(this.options.endpoint).host}/self`;
    const reply = await this.send(message('hello', from, { versions: ['0.3'] }, this.options.agent ? { agent: this.options.agent } : {}));
    if (reply?.type !== 'hello') throw new Error('host did not answer hello');
    this.party = reply.body.party ?? this.party;
    return reply;
  }

  async inbox(after?: string): Promise<InboxPage> {
    const url = new URL(`${this.options.endpoint.replace(/\/$/, '')}/inbox`);
    if (after) url.searchParams.set('after', after);
    const response = await this.fetcher(url, { headers: this.headers() });
    const checked = checkInbox(await response.json());
    if (!checked.ok) throw new Error(`invalid inbox page: ${checked.problems.map((p) => `${p.path} ${p.message}`).join('; ')}`);
    return checked.value;
  }

  async deal(id: string): Promise<MessageByType['deal']> {
    return this.read(`deals/${encodeURIComponent(id)}`, 'deal');
  }

  async outcome(id: string): Promise<MessageByType['outcome']> {
    return this.read(`deals/${encodeURIComponent(id)}/outcome`, 'outcome');
  }

  private async read<T extends 'deal' | 'outcome'>(path: string, type: T): Promise<MessageByType[T]> {
    const response = await this.fetcher(`${this.options.endpoint.replace(/\/$/, '')}/${path}`, { headers: this.headers() });
    const checked = checkMessage(await response.json());
    if (!checked.ok) throw new Error(`invalid ${type}: ${checked.problems.map((p) => `${p.path} ${p.message}`).join('; ')}`);
    if (checked.value.type === 'error') throw new OrpProtocolError(checked.value, response.status);
    if (checked.value.type !== type) throw new Error(`expected ${type}, got ${checked.value.type}`);
    return checked.value as MessageByType[T];
  }
}
