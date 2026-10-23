import { randomBytes } from 'node:crypto';

const id = () => `m_${randomBytes(9).toString('base64url')}`;

async function readJson(res) {
  const text = await res.text();
  const message = text ? JSON.parse(text) : null;
  return {
    status: res.status,
    message,
    headers: res.headers,
    text,
  };
}

export class OrpClient {
  constructor({ origin, token, party, claim, ownerToken }) {
    this.origin = origin.replace(/\/$/, '');
    this.token = token || '';
    this.party = party || '';
    this.claimUrl = claim || '';
    this.ownerToken = ownerToken || '';
  }

  endpoint() {
    return `${this.origin}/orp`;
  }

  msg(fields = {}) {
    return {
      orp: '0.3',
      id: id(),
      ts: new Date().toISOString(),
      from: this.party,
      ...fields,
    };
  }

  async send(message) {
    const res = await fetch(this.endpoint(), {
      method: 'POST',
      headers: {
        authorization: `Bearer ${this.token}`,
        'content-type': 'application/json',
      },
      body: JSON.stringify(message),
    });
    return readJson(res);
  }

  async inbox(after = '') {
    const url = new URL(`${this.origin}/orp/inbox`);
    if (after) url.searchParams.set('after', after);
    const res = await fetch(url, { headers: { authorization: `Bearer ${this.token}` } });
    return readJson(res);
  }

  async deal(dealId) {
    const res = await fetch(`${this.origin}/orp/deals/${dealId}`, {
      headers: { authorization: `Bearer ${this.token}` },
    });
    return readJson(res);
  }

  async claim() {
    const res = await fetch(this.claimUrl, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: '{}',
    });
    const body = await readJson(res);
    if (body.status !== 200) throw new Error(`claim failed: ${body.status} ${body.text}`);
    this.ownerToken = body.message.owner_token;
    return body.message;
  }

  async setConstraints(constraints, token = this.ownerToken) {
    const res = await fetch(`${this.origin}/orp/constraints`, {
      method: 'PUT',
      headers: {
        authorization: `Bearer ${token}`,
        'content-type': 'application/json',
      },
      body: JSON.stringify(constraints),
    });
    return readJson(res);
  }

  async approve(escalation, { decision = 'approved', assurance = 'passkey' } = {}, token = this.ownerToken) {
    const res = await fetch(`${this.origin}/approve/${escalation}`, {
      method: 'POST',
      headers: {
        authorization: `Bearer ${token}`,
        'content-type': 'application/json',
      },
      body: JSON.stringify({ decision, assurance }),
    });
    return readJson(res);
  }

  async reportConversion(body) {
    const res = await fetch(`${this.origin}/orp/conversions`, {
      method: 'POST',
      headers: {
        authorization: `Bearer ${this.token}`,
        'content-type': 'application/json',
      },
      body: JSON.stringify(body),
    });
    return readJson(res);
  }

  static async discovery(origin) {
    const res = await fetch(`${origin.replace(/\/$/, '')}/.well-known/orp.json`);
    return readJson(res);
  }

  static async suggest(origin, body) {
    const res = await fetch(`${origin.replace(/\/$/, '')}/orp/suggest`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(body),
    });
    return readJson(res);
  }

  static async join(origin, body = {}) {
    const res = await fetch(`${origin.replace(/\/$/, '')}/orp/join`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(body),
    });
    const parsed = await readJson(res);
    if (parsed.status !== 201) return parsed;
    return new OrpClient({
      origin,
      token: parsed.message.token,
      party: parsed.message.party,
      claim: parsed.message.claim,
    });
  }

  static async post(origin, token, body) {
    const headers = { 'content-type': 'application/json' };
    if (token) headers.authorization = `Bearer ${token}`;
    const res = await fetch(`${origin.replace(/\/$/, '')}/orp`, {
      method: 'POST',
      headers,
      body: typeof body === 'string' ? body : JSON.stringify(body),
    });
    return readJson(res);
  }

  static async click(url, { method = 'GET', headers = {} } = {}) {
    const res = await fetch(url, { method, headers, redirect: 'manual' });
    return {
      status: res.status,
      location: res.headers.get('location'),
      headers: res.headers,
    };
  }
}

export async function inboxAll(client) {
  const messages = [];
  let after = '';
  for (let page = 0; page < 20; page += 1) {
    const res = await client.inbox(after);
    if (res.status !== 200) throw new Error(`inbox ${res.status}`);
    messages.push(...res.message.messages);
    if (!res.message.messages.length || res.message.next === after) break;
    after = res.message.next;
  }
  return messages;
}
