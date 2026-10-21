import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { OrpClient, startHost, termsHash, type Constraints, type HostOptions, type Message, type MessageByType, type Space, type Terms } from '../src/index.ts';
import { SCHEMA_DIR } from '../src/validate.ts';

export const EXAMPLES = join(SCHEMA_DIR, '..', 'examples');

export function readExample<T = Message>(name: string): T {
  return JSON.parse(readFileSync(join(EXAMPLES, name), 'utf8')) as T;
}

export function exampleFiles(dir: string): string[] {
  return readdirSync(join(EXAMPLES, dir)).filter((name) => name.endsWith('.json')).sort().map((name) => join(dir, name));
}

export const affiliateTerms = (): Terms => structuredClone(readExample<MessageByType['offer']>('messages/03-offer-standing.json').body.terms);
export const swapTerms = (): Terms => structuredClone(readExample<MessageByType['offer']>('messages/11-offer-directed.json').body.terms);
export const swapCounterTerms = (): Terms => structuredClone(readExample<MessageByType['counter']>('messages/12-counter.json').body.terms);

export const UA = {
  mailApp: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148',
  desktop: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15',
  windows: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36',
  xPreview: 'Twitterbot/1.0',
  slackPreview: 'Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)',
  curl: 'curl/8.5.0',
};

export const publisherSpaces: Space[] = [
  { id: 'sp_news_1', kind: 'newsletter.issue', platform: 'newsletter', title: 'Indie Stack Weekly', locator: 'https://indiestackweekly.example/archive', accepts: ['affiliate', 'swap', 'sponsored'] },
  { id: 'sp_x_1', kind: 'social.post', platform: 'x', title: '@indiestackweekly', locator: 'https://x.com/indiestackweekly', accepts: ['swap', 'repost'] },
];

export async function world(options: Omit<HostOptions, 'origin'> = {}) {
  const started = await startHost({ harness: true, ...options });
  const { host, origin } = started;
  const join = async (constraints?: Constraints, spaces?: Space[], name = 'test-agent') => {
    const { party, token } = host.createParty({ constraints, spaces });
    const client = await OrpClient.connect(origin, token, { name });
    return { party, token, client };
  };
  return { ...started, join };
}

/** A maker that lists the example affiliate offer, and a publisher that accepted it. */
export async function activeAffiliateDeal(options: Omit<HostOptions, 'origin'> = {}) {
  const w = await world(options);
  const maker = await w.join({ steps: { list: 'auto', revoke: 'auto' }, budget: { max_rate_bps: 3000 } }, [], 'maker-agent');
  const terms = affiliateTerms();
  const listing = (await maker.client.send(maker.client.build('offer', { terms, terms_hash: termsHash(terms) }))) as MessageByType['offer'];
  const publisher = await w.join({ steps: { accept: 'auto' }, kinds: ['affiliate'], categories: { allow: ['software'] } }, publisherSpaces, 'publisher-agent');
  const deal = (await publisher.client.send(publisher.client.build('accept', { offer: listing.body.id!, terms_hash: listing.body.terms_hash! }))) as MessageByType['deal'];
  return { ...w, maker, publisher, listing, deal };
}

export async function click(link: string, headers: Record<string, string>, method = 'GET') {
  const response = await fetch(link, { method, redirect: 'manual', headers });
  return { status: response.status, location: response.headers.get('location') ?? '' };
}

export async function post(origin: string, path: string, body: unknown, token?: string) {
  const response = await fetch(new URL(path, origin), {
    method: 'POST',
    headers: { 'content-type': 'application/json', ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify(body),
  });
  const text = await response.text();
  return { status: response.status, body: text ? (JSON.parse(text) as Message & Record<string, unknown>) : undefined };
}
