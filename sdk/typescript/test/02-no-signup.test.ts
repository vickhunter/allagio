// Agents never email people or sign up on websites. A writing agent suggests a
// deal mid-task, joins with nothing personal, and its person approves with one
// tap. Nothing in the flow asks for an email, a name, a phone or a website.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { checkDiscovery, checkJoinResponse, checkSuggestResponse, message, OrpClient, OrpProtocolError, suggest, type JoinResponse, type MessageByType, type SuggestResponse } from '../src/index.ts';
import { click, DRAFT, listedAffiliateOffer, post, UA, world } from './fixture.ts';

const PERSONAL_FIELDS = /type=["']?(email|password|tel)\b|name=["']?(email|password|phone|name)\b/i;

test('discovery says where to suggest and join; both answer with no credentials', async (t) => {
  const w = await world();
  t.after(w.close);
  const discovery = checkDiscovery(await (await fetch(`${w.origin}/.well-known/orp.json`)).json());
  assert.ok(discovery.ok);
  assert.ok(discovery.value.suggest && discovery.value.join);
  assert.ok(discovery.value.mcp?.tools?.includes('orp_suggest') && discovery.value.mcp.tools.includes('orp_join'));
  const preflight = await fetch(new URL(discovery.value.suggest, w.origin), { method: 'OPTIONS' });
  assert.equal(preflight.headers.get('access-control-allow-origin'), '*', 'a browser-based writing tool can call it too');
});

test('a writing agent gets pitched deals for its draft in one call, and the draft goes nowhere', async (t) => {
  const w = await world();
  t.after(w.close);
  const { maker } = await listedAffiliateOffer(w);
  const canary = 'quokkacanary';
  const response = await post(w.origin, '/orp/suggest', { text: `${DRAFT} ${canary}`, platform: 'web', space_kind: 'web.article' });
  assert.equal(response.status, 200);
  const checked = checkSuggestResponse(response.body);
  assert.ok(checked.ok);
  const [item] = checked.value.items;
  assert.ok(item?.offer, 'the listed offer fits a post about note-taking software');
  assert.match(item.pitch!, /^You can monetize this post: .+ Want to test it\?$/);
  assert.match(item.pitch!, /labelled 'Affiliate link'/, 'the pitch says what the person would do, disclosure included');
  assert.ok(!JSON.stringify(response.body).includes(canary), 'the draft is not echoed');
  assert.ok(!JSON.stringify(await maker.client.inbox()).includes(canary), 'the offerer never hears about the draft');

  const unrelated = await suggest(w.origin, { text: 'Our week hiking in the Dolomites and the boots that survived it.' });
  assert.deepEqual(unrelated.items, []);
  assert.equal(unrelated.empty_reason, 'no_match', 'nothing is pushed at a post it does not fit');
  const growOnly = await suggest(w.origin, { text: DRAFT, want: ['grow'] });
  assert.ok(!growOnly.items.some((i) => i.offer?.id === item.offer!.id), 'want: grow leaves out deals that only pay');
});

test('join asks for nothing personal: an empty request works, personal fields are refused', async (t) => {
  const w = await world();
  t.after(w.close);
  const empty = await fetch(`${w.origin}/orp/join`, { method: 'POST' });
  assert.equal(empty.status, 200);
  const joined = checkJoinResponse(await empty.json());
  assert.ok(joined.ok);
  assert.match(joined.value.party, /^orp:[^/]+\/p_[A-Za-z0-9_-]+$/, 'a pseudonymous party id');
  assert.ok(Object.values(joined.value.steps ?? {}).filter((mode) => mode === 'auto').length <= 1, 'until claimed, every binding step asks');

  for (const field of ['email', 'name', 'phone', 'website', 'password']) {
    const refused = await post(w.origin, '/orp/join', { [field]: field === 'email' ? 'someone@example.com' : 'x' });
    assert.equal(refused.status, 400, `${field} is refused`);
    const error = refused.body as unknown as MessageByType['error'];
    assert.equal(error.body.code, 'invalid_message');
    assert.match(error.body.message, /nothing personal/);
  }

  const claim = await fetch(joined.value.claim);
  assert.equal(claim.status, 200);
  assert.doesNotMatch(await claim.text(), PERSONAL_FIELDS, 'claiming takes a passkey, not an email');
});

test('mid-task: suggest, a yes, join, one-tap approval, link in the post, readers counted', async (t) => {
  const w = await world();
  t.after(w.close);
  const { maker } = await listedAffiliateOffer(w);
  const { items } = await suggest(w.origin, { text: DRAFT, want: ['earn'] });
  const offer = items[0]!.offer!;

  // The person said yes in chat. The agent joins with nothing personal.
  const { client, joined } = await OrpClient.join(w.origin, { agent: { name: 'writer-assistant', version: '2.1.0' } });
  assert.equal(client.party, joined.party);
  const waiting = (await client.send(client.build('accept', { offer: offer.id, terms_hash: offer.terms_hash }))) as MessageByType['deal'];
  assert.equal(waiting.body.state, 'awaiting', 'an unclaimed party binds nothing alone');
  assert.deepEqual(waiting.body.waiting?.reasons, ['step_ask']);

  const page = await fetch(waiting.body.waiting!.url!, { redirect: 'manual' });
  assert.equal(page.status, 200, 'the approval page needs no sign-in');
  const html = await page.text();
  assert.match(html, /Acme Notes/);
  assert.match(html, /Affiliate link/, 'the person sees the label readers will see');
  assert.match(html, /writer-assistant/);
  assert.doesNotMatch(html, PERSONAL_FIELDS);

  // One tap with a passkey. The harness stands in for WebAuthn.
  await post(w.origin, `/orp-test/escalations/${waiting.body.waiting!.escalation}/decide`, { decision: 'approved', assurance: 'passkey' });
  const active = (await client.deal(waiting.deal!)).body;
  assert.equal(active.state, 'active');
  const link = active.tracking![0]!.link;
  const published = `${DRAFT}\n\nAffiliate link: ${link}`;
  assert.ok(published.includes('/go/'));

  await click(link, { 'user-agent': UA.windows, referer: 'https://blog.example/notes' });
  await click(link, { 'user-agent': UA.mailApp });
  await click(link, { 'user-agent': UA.xPreview });
  assert.deepEqual((await client.outcome(waiting.deal!)).body.rows[0]?.clicks, { human: 2, bot: 0, preview: 1, test: 0 });

  const consent = (await maker.client.inbox()).messages.find((m): m is MessageByType['consent'] => m.type === 'consent');
  assert.equal(consent?.body.assurance, 'passkey', 'the maker learns a person approved, and how strongly');
  assert.equal(consent?.body.party, joined.party, 'and nothing else about them');
});

test('an unclaimed party can hold a few open deals; claiming it lifts the limit', async (t) => {
  const w = await world();
  t.after(w.close);
  const { listing } = await listedAffiliateOffer(w);
  const { client, joined } = await OrpClient.join(w.origin);
  const accept = () => client.send(client.build('accept', { offer: listing.body.id!, terms_hash: listing.body.terms_hash! }));
  for (let i = 0; i < (joined.limits?.open_deals ?? 3); i += 1) await accept();
  await assert.rejects(accept(), (error: unknown) => error instanceof OrpProtocolError && error.reply.body.code === 'forbidden' && /claim/.test(error.reply.body.message));
  const secret = new URL(joined.claim).pathname.split('/').pop()!;
  assert.equal((await post(w.origin, `/orp-test/claims/${secret}`, { constraints: { steps: { accept: 'auto' }, kinds: ['affiliate'] } })).status, 200);
  const after = (await accept()) as MessageByType['deal'];
  assert.equal(after.body.state, 'active', 'once claimed, the limits the person set apply');
});

test('MCP: orp_suggest and orp_join need no credentials, and the session then acts as the new party', async (t) => {
  const w = await world();
  t.after(w.close);
  await listedAffiliateOffer(w);
  let id = 0;
  const rpc = async (method: string, params: Record<string, unknown>, session?: string) => {
    const response = await fetch(`${w.origin}/mcp`, {
      method: 'POST',
      headers: { 'content-type': 'application/json', accept: 'application/json, text/event-stream', ...(session ? { 'mcp-session-id': session } : {}) },
      body: JSON.stringify({ jsonrpc: '2.0', id: (id += 1), method, params }),
    });
    return { status: response.status, session: response.headers.get('mcp-session-id') ?? undefined, body: (await response.json()) as { result?: { structuredContent?: unknown; isError?: boolean; tools?: Array<{ name: string }> } } };
  };
  const init = await rpc('initialize', { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 'writer', version: '1.0.0' } });
  const session = init.session!;
  assert.ok(session, 'the server assigns a session');
  const names = (await rpc('tools/list', {}, session)).body.result?.tools?.map((tool) => tool.name);
  assert.deepEqual(names, ['orp_suggest', 'orp_join', 'orp_send', 'orp_inbox', 'orp_deal']);

  const suggested = (await rpc('tools/call', { name: 'orp_suggest', arguments: { text: DRAFT } }, session)).body.result!;
  assert.equal(suggested.isError, false);
  const offer = (suggested.structuredContent as SuggestResponse).items[0]!.offer!;
  const joined = (await rpc('tools/call', { name: 'orp_join', arguments: { agent: { name: 'writer' } } }, session)).body.result!.structuredContent as JoinResponse;

  const accept = message('accept', joined.party, { offer: offer.id, terms_hash: offer.terms_hash });
  const sent = (await rpc('tools/call', { name: 'orp_send', arguments: { message: accept } }, session)).body.result!;
  assert.equal(sent.isError, false, 'no Authorization header: the session is the party');
  assert.equal((sent.structuredContent as MessageByType['deal']).body.state, 'awaiting');

  const stranger = (await rpc('initialize', { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 'other', version: '1' } })).session!;
  assert.equal((await rpc('tools/call', { name: 'orp_inbox', arguments: {} }, stranger)).body.result?.isError, true, 'a session that never joined acts as nobody');
  assert.equal((await rpc('tools/list', {}, 'not-a-session')).status, 404);
});
