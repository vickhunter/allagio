import { mkdtempSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { ReferenceHost, OrpClient, inboxAll, digest } from '../../sdk/src/index.mjs';

export const BROWSER = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36';
export const DESTINATION = 'https://notes.example/pricing?plan=pro';
export const DAY = 86400000;
export const SENTINEL = 'SENTINEL-draft-9f3a';

export const AUTO = {
  steps: {
    list: 'auto',
    negotiate: 'auto',
    accept: 'auto',
    execute: 'auto',
    amend: 'auto',
    settle: 'auto',
    share: 'auto',
    revoke: 'auto',
  },
  budget: {
    per_deal: { amount_minor: 100000, currency: 'USD' },
    per_month: { amount_minor: 500000, currency: 'USD' },
    max_rate_bps: 5000,
  },
  approve_over: { amount_minor: 1000000, currency: 'USD' },
};

export const disclosure = { label: 'Affiliate link', lang: 'en' };

export function affiliateTerms(patch = {}) {
  return {
    title: 'Acme Notes: 30 percent of first-year revenue',
    kind: 'affiliate',
    categories: ['software.productivity'],
    disclosure,
    obligations: [
      {
        id: 'place',
        by: 'acceptor',
        action: 'place',
        content: { link: true, brief: 'Mention the notes app.' },
        destination: DESTINATION,
        proof: ['link'],
      },
      {
        id: 'commission',
        by: 'offerer',
        action: 'pay',
        per: 'conversion',
        rate_bps: 3000,
        window: 'P30D',
        rail: 'stripe',
        proof: ['provider'],
      },
    ],
    ...patch,
  };
}

export function terms(kind, obligations, title = `${kind} notes deal`) {
  return {
    title,
    kind,
    categories: ['software.productivity'],
    disclosure,
    obligations,
  };
}

export function place(extra = {}) {
  return {
    id: 'place',
    by: 'acceptor',
    action: 'place',
    content: { link: true, brief: 'A short mention of notes.' },
    destination: DESTINATION,
    proof: ['link'],
    ...extra,
  };
}

export async function boot(options = {}) {
  const { registerPage = true, ...hostOptions } = options;
  const host = await ReferenceHost.start(hostOptions);
  if (registerPage) host.registerPage(DESTINATION);
  return host;
}

export async function withHost(options, fn) {
  const host = await boot(options);
  try {
    return await fn(host);
  } finally {
    await host.close();
  }
}

export function tempStore() {
  return join(mkdtempSync(join(tmpdir(), 'orp-host-')), 'host.json');
}

export async function party(host, constraints) {
  const client = await OrpClient.join(host.origin, {});
  if (!client?.token) throw new Error(`join failed: ${client?.status} ${client?.text}`);
  if (constraints) {
    await client.claim();
    const set = await client.setConstraints(constraints);
    if (set.status !== 200) throw new Error(`constraints: ${set.status} ${set.text}`);
  }
  return client;
}

export async function listStanding(maker, offerTerms = affiliateTerms()) {
  const res = await maker.send(maker.msg({ type: 'offer', body: { terms: offerTerms } }));
  if (res.status !== 200 || res.message?.type !== 'offer') {
    throw new Error(`list failed: ${res.status} ${res.text}`);
  }
  return res.message;
}

export async function acceptOffer(client, summary) {
  return client.send(client.msg({
    type: 'accept',
    body: { offer: summary.id, terms_hash: summary.terms_hash },
  }));
}

export async function directDeal(maker, acceptor, offerTerms) {
  const offered = await maker.send(maker.msg({
    type: 'offer',
    to: acceptor.party,
    body: { terms: offerTerms },
  }));
  if (offered.status !== 200 || offered.message?.type !== 'deal') {
    throw new Error(`offer failed: ${offered.status} ${offered.text}`);
  }
  const accepted = await acceptor.send(acceptor.msg({
    type: 'accept',
    deal: offered.message.deal,
    body: { terms_hash: offered.message.body.terms_hash },
  }));
  return { offered, accepted };
}

export async function inbox(client) {
  return inboxAll(client);
}

export function latest(messages, type) {
  const found = messages.filter((message) => message.type === type);
  return found[found.length - 1];
}

export function codeOf(res) {
  return res.message?.body?.code;
}

export function storeText(host) {
  return readFileSync(host.storePath, 'utf8');
}

export function humanClicks(outcome) {
  const row = outcome?.body?.rows?.find((item) => item.obligation === 'place') || outcome?.body?.rows?.[0];
  return row?.clicks?.human;
}

export { digest, OrpClient, inboxAll, ReferenceHost };
