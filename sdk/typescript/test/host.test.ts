import assert from 'node:assert/strict';
import { test } from 'node:test';
import { message, messageHash, OrpProtocolError, termsHash, verifyMessage, type Message, type MessageByType } from '../src/index.ts';
import { activeAffiliateDeal, affiliateTerms, click, post, publisherSpaces, swapCounterTerms, swapTerms, UA, world } from './fixture.ts';

const isDeal = (m: Message): m is MessageByType['deal'] => m.type === 'deal';

async function rejects(promise: Promise<unknown>, code: string) {
  await assert.rejects(promise, (error: unknown) => error instanceof OrpProtocolError && error.reply.body.code === code);
}

test('swap: ask, firm offer, counter, escalation to a person, consent, receipts without clicks', async (t) => {
  const w = await world();
  t.after(w.close);
  const maker = await w.join({ steps: { negotiate: 'auto', execute: 'auto' } }, [], 'maker-agent');
  const publisher = await w.join({ steps: { negotiate: 'auto', accept: 'ask', execute: 'auto' } }, publisherSpaces, 'publisher-agent');

  const ask = maker.client.build('ask', { space: 'sp_news_1', kinds: ['swap'], actions: ['place'], give: ['repost'], note: 'Room in an issue that week?' }, { to: publisher.party });
  assert.equal(await maker.client.send(ask), undefined, 'ask is relayed: 202, nothing binds');
  const page = await publisher.client.inbox();
  assert.ok(page.messages.some((m) => m.type === 'ask' && m.id === ask.id));

  const terms = swapTerms();
  const offered = (await publisher.client.send(publisher.client.build('offer', { terms, terms_hash: termsHash(terms) }, { to: maker.party, re: ask.id }))) as MessageByType['deal'];
  assert.equal(offered.body.state, 'proposed');
  assert.deepEqual(offered.body.bound.map((b) => [b.party, b.authority.type]), [[publisher.party, 'mandate']], 'an offer binds its sender on delivery');
  const dealId = offered.deal!;

  const counterTerms = swapCounterTerms();
  const countered = (await maker.client.send(maker.client.build('counter', { terms: counterTerms, terms_hash: termsHash(counterTerms) }, { deal: dealId }))) as MessageByType['deal'];
  assert.equal(countered.body.round, 2);
  assert.deepEqual(countered.body.bound.map((b) => b.party), [maker.party], 'a counter clears earlier bindings and binds its sender');

  await rejects(publisher.client.send(publisher.client.build('accept', { terms_hash: termsHash(terms) }, { deal: dealId })), 'terms_mismatch');

  const awaiting = (await publisher.client.send(publisher.client.build('accept', { terms_hash: termsHash(counterTerms), spaces: [{ obligation: 'mention', space: 'sp_news_1' }] }, { deal: dealId }))) as MessageByType['deal'];
  assert.equal(awaiting.body.state, 'awaiting');
  assert.deepEqual(awaiting.body.waiting?.reasons, ['step_ask']);
  assert.ok(awaiting.body.waiting?.url, 'the waiting party gets the url for its person');
  const escalation = awaiting.body.waiting!.escalation!;

  const makerCopy = (await maker.client.deal(dealId)).body;
  assert.equal(makerCopy.state, 'awaiting');
  assert.equal(makerCopy.waiting?.party, publisher.party);
  assert.equal(makerCopy.waiting?.reasons, undefined, "the counterparty never sees the owner's reasons");
  assert.equal(makerCopy.waiting?.escalation, undefined);
  assert.equal(makerCopy.spaces, undefined, 'space details stay hidden until the deal is active');

  const approved = await post(w.origin, `/orp-test/escalations/${escalation}/decide`, { decision: 'approved', assurance: 'passkey' });
  assert.equal(approved.status, 200);
  const consent = approved.body as unknown as MessageByType['consent'];
  assert.equal(consent.body.terms_hash, termsHash(counterTerms), 'consent is bound to the exact terms');
  assert.equal(consent.body.assurance, 'passkey');

  const makerInbox = (await maker.client.inbox()).messages;
  const makerConsent = makerInbox.find((m): m is MessageByType['consent'] => m.type === 'consent');
  assert.ok(makerConsent);
  assert.equal(makerConsent.body.reasons, undefined, 'the counterparty learns that a person approved, not why it asked');

  const active = (await publisher.client.deal(dealId)).body;
  assert.equal(active.state, 'active');
  assert.deepEqual(active.bound.map((b) => b.authority.type).sort(), ['consent', 'mandate']);
  const mention = active.tracking?.find((t) => t.obligation === 'mention');
  assert.ok(mention?.code, 'the counter asked for a code, so the link comes with one');
  assert.equal(active.spaces?.[0]?.id, 'sp_news_1');

  const proofUrl = 'https://x.com/acmenotes/status/1848100000000000000';
  await post(w.origin, '/orp-test/web', { url: proofUrl, status: 200, body: `<p>Reposted https://x.com/indiestackweekly/status/1848000000000000000</p>` });
  const executed = (await maker.client.send(maker.client.build('execute', { obligation: 'repost', proof_url: proofUrl }, { deal: dealId }))) as MessageByType['deal'];
  assert.equal(executed.body.obligations.find((o) => o.id === 'repost')?.state, 'verified', 'a repost has no click: the proof URL is the receipt');
  const receipt = (await maker.client.inbox()).messages.find((m): m is MessageByType['receipt'] => m.type === 'receipt' && m.body.obligation === 'repost');
  assert.equal(receipt?.body.method, 'proof_url');

  await rejects(publisher.client.send(publisher.client.build('receipt', { obligation: 'mention', result: 'verified', method: 'attestation', at: new Date().toISOString().replace(/\.\d{3}Z$/, 'Z') }, { deal: dealId })), 'forbidden');

  await click(mention!.link, { 'user-agent': UA.mailApp });
  const done = (await maker.client.deal(dealId)).body;
  assert.equal(done.obligations.find((o) => o.id === 'mention')?.state, 'verified');
  assert.equal(done.state, 'completed', 'a swap with every obligation verified completes');
});

test('a person can refuse; the deal is declined and nothing is minted', async (t) => {
  const w = await world();
  t.after(w.close);
  const maker = await w.join({ steps: { negotiate: 'auto' } });
  const publisher = await w.join({ steps: { negotiate: 'auto' } }, publisherSpaces);
  const terms = swapTerms();
  const offered = (await publisher.client.send(publisher.client.build('offer', { terms, terms_hash: termsHash(terms) }, { to: maker.party }))) as MessageByType['deal'];
  const awaiting = (await maker.client.send(maker.client.build('accept', { terms_hash: termsHash(terms) }, { deal: offered.deal! }))) as MessageByType['deal'];
  assert.equal(awaiting.body.state, 'awaiting', 'accept asks by default');
  await post(w.origin, `/orp-test/escalations/${awaiting.body.waiting!.escalation}/decide`, { decision: 'refused' });
  const after = (await maker.client.deal(offered.deal!)).body;
  assert.equal(after.state, 'declined');
  assert.equal(after.tracking, undefined);
});

test('a proposal waiting for its own sender is not shown to the counterparty yet', async (t) => {
  const w = await world();
  t.after(w.close);
  const maker = await w.join({ steps: { negotiate: 'auto' } });
  const publisher = await w.join(undefined, publisherSpaces);
  const terms = swapTerms();
  const sent = (await publisher.client.send(publisher.client.build('offer', { terms, terms_hash: termsHash(terms) }, { to: maker.party }))) as MessageByType['deal'];
  assert.equal(sent.body.state, 'awaiting', 'negotiate asks by default, because an offer binds');
  assert.equal((await maker.client.inbox()).messages.filter(isDeal).length, 0);
  await post(w.origin, `/orp-test/escalations/${sent.body.waiting!.escalation}/decide`, { decision: 'approved' });
  const seen = (await maker.client.inbox()).messages.filter(isDeal);
  assert.equal(seen.at(-1)?.body.state, 'proposed', 'after approval the offer is firm and delivered');
});

test('hard rules refuse; allowances escalate', async (t) => {
  const w = await world();
  t.after(w.close);
  const maker = await w.join({ steps: { list: 'auto' }, budget: { max_rate_bps: 3000 } });
  const terms = affiliateTerms();
  const listing = (await maker.client.send(maker.client.build('offer', { terms, terms_hash: termsHash(terms) }))) as MessageByType['offer'];
  const accept = (client: typeof maker.client) => client.send(client.build('accept', { offer: listing.body.id!, terms_hash: listing.body.terms_hash! }));

  const strict = await w.join({ steps: { accept: 'auto' }, categories: { deny: ['software.productivity'] } });
  await rejects(accept(strict.client), 'brand_rule');
  const careful = await w.join({ steps: { accept: 'auto' }, brand: { must_not_include: ['30%'] } });
  await rejects(accept(careful.client), 'brand_rule');
  const off = await w.join({ steps: { accept: 'off' } });
  await rejects(accept(off.client), 'step_off');

  const narrow = await w.join({ steps: { accept: 'auto' }, kinds: ['swap'], categories: { allow: ['business'] } });
  const waiting = (await accept(narrow.client)) as MessageByType['deal'];
  assert.equal(waiting.body.state, 'awaiting');
  assert.deepEqual([...waiting.body.waiting!.reasons!].sort(), ['category_outside', 'kind_outside']);

  const greedy = await w.join({ steps: { list: 'auto' }, budget: { max_rate_bps: 1000 } });
  const listed = (await greedy.client.send(greedy.client.build('offer', { terms, terms_hash: termsHash(terms) }))) as MessageByType['offer'];
  assert.deepEqual(listed.body.waiting?.reasons, ['over_budget'], 'a 30% share above the 10% limit waits for the owner');
  const browsing = await w.join();
  const results = (await browsing.client.send(browsing.client.build('discover', { want: 'offers' }))) as MessageByType['results'];
  assert.equal(results.body.items.length, 1, 'an unapproved listing is not discoverable');
});

test('only the host speaks for people and outcomes', async (t) => {
  const { publisher, deal, close } = await activeAffiliateDeal();
  t.after(close);
  const at = new Date().toISOString().replace(/\.\d{3}Z$/, 'Z');
  const fakeConsent = publisher.client.build(
    'consent',
    { consent: 'cns_fake', escalation: 'esc_fake', party: publisher.party, decision: 'approved', step: 'accept', terms_hash: deal.body.terms_hash, assurance: 'passkey', at },
    { deal: deal.deal! },
  );
  await rejects(publisher.client.send(fakeConsent), 'forbidden');
  await rejects(publisher.client.send(publisher.client.build('outcome', { period: { from: at, to: at }, rows: [] }, { deal: deal.deal! })), 'forbidden');
  await rejects(publisher.client.send(publisher.client.build('reputation', { subject: publisher.party, algorithm: 'orp-rep-1', score: 999, computed_at: at })), 'forbidden');
});

test('wire safety: auth, from, idempotency, replay, staleness, chain', async (t) => {
  const { publisher, maker, deal, origin, token: _unused, close } = { ...(await activeAffiliateDeal()), token: undefined };
  t.after(close);
  const hello = message('hello', publisher.party, { versions: ['0.3'] });
  assert.equal((await post(origin, '/orp', hello)).status, 401, 'no token');
  assert.equal((await post(origin, '/orp', { ...hello, from: maker.party }, publisher.token)).status, 403, 'from must match the token');

  const discover = publisher.client.build('discover', { want: 'offers' });
  const first = await post(origin, '/orp', discover, publisher.token);
  const again = await post(origin, '/orp', discover, publisher.token);
  assert.equal(first.status, 200);
  assert.deepEqual(again.body, first.body, 'a retried id returns the same reply');
  const changed = await post(origin, '/orp', { ...discover, body: { want: 'spaces' } }, publisher.token);
  assert.equal(changed.status, 409);
  assert.equal((changed.body as MessageByType['error']).body.code, 'replay');

  const old = publisher.client.build('discover', { want: 'offers' }, { ts: '2020-01-01T00:00:00Z' });
  assert.equal(((await post(origin, '/orp', old, publisher.token)).body as MessageByType['error']).body.code, 'stale');

  const wrongPrev = publisher.client.build('revoke', { scope: 'deal', reason: 'owner_request' }, { deal: deal.deal!, prev: 'sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA' });
  assert.equal(((await post(origin, '/orp', wrongPrev, publisher.token)).body as MessageByType['error']).body.code, 'chain_broken');

  const invalid = await post(origin, '/orp', { orp: '0.3', type: 'discover', id: 'x', from: publisher.party, body: {} }, publisher.token);
  assert.equal(invalid.status, 400);
  assert.equal((invalid.body as MessageByType['error']).body.code, 'invalid_message');
});

test('the chain: prev is the hash of my previous message in the deal', async (t) => {
  const w = await world();
  t.after(w.close);
  const maker = await w.join({ steps: { negotiate: 'auto' } });
  const publisher = await w.join({ steps: { negotiate: 'auto' } }, publisherSpaces);
  const terms = swapTerms();
  const offer = publisher.client.build('offer', { terms, terms_hash: termsHash(terms) }, { to: maker.party });
  const proposed = (await publisher.client.send(offer)) as MessageByType['deal'];
  const declined = publisher.client.build('revoke', { scope: 'deal', reason: 'owner_request' }, { deal: proposed.deal!, prev: messageHash(offer) });
  const revoked = (await publisher.client.send(declined)) as MessageByType['deal'];
  assert.equal(revoked.body.state, 'revoked');
});

test('a signing host signs every message with a key from its discovery document', async (t) => {
  const w = await world({ sign: true });
  t.after(w.close);
  const discovery = await (await fetch(`${w.origin}/.well-known/orp.json`)).json();
  const keys = discovery.keys as Array<{ kid: string }>;
  const agent = await w.join();
  const results = (await agent.client.send(agent.client.build('discover', { want: 'offers' }))) as MessageByType['results'];
  const verified = verifyMessage(results, (kid) => keys.find((k) => k.kid === kid) as never);
  assert.deepEqual(verified, { ok: true, kid: keys[0]!.kid });
  const tampered = { ...results, ts: '2020-01-01T00:00:00Z' };
  assert.equal(verifyMessage(tampered, (kid) => keys.find((k) => k.kid === kid) as never).ok, false);
  assert.ok(discovery.conformance.levels.includes('L3'));
});

test('disputes and reputation: only verified outcomes count, and a lost dispute costs', async (t) => {
  const w = await world();
  t.after(w.close);
  const maker = await w.join({ steps: { negotiate: 'auto', execute: 'auto' } });
  const publisher = await w.join({ steps: { negotiate: 'auto', accept: 'auto', execute: 'auto' } }, publisherSpaces);
  const terms = swapTerms();
  const proposed = (await maker.client.send(maker.client.build('offer', { terms, terms_hash: termsHash(terms) }, { to: publisher.party }))) as MessageByType['deal'];
  const active = (await publisher.client.send(publisher.client.build('accept', { terms_hash: termsHash(terms) }, { deal: proposed.deal! }))) as MessageByType['deal'];
  assert.equal(active.body.state, 'active');

  const opened = (await maker.client.send(
    maker.client.build('dispute', { action: 'open', obligation: 'repost', claim: 'not_done', statement: 'No repost after the due date.', remedy: 'redo' }, { deal: proposed.deal! }),
  )) as MessageByType['dispute'];
  assert.equal((await publisher.client.deal(proposed.deal!)).body.state, 'disputed');
  await post(w.origin, `/orp-test/disputes/${opened.body.dispute}/resolve`, { outcome: 'upheld', remedy: 'redo', note: 'No repost found.' });
  assert.equal((await publisher.client.deal(proposed.deal!)).body.state, 'active');

  const rep = (await maker.client.send(maker.client.build('reputation', { subject: publisher.party }))) as MessageByType['reputation'];
  assert.equal(rep.body.algorithm, 'orp-rep-1');
  assert.equal(rep.body.score, null, 'too little evidence for a score');
  assert.equal(rep.body.components?.disputes_lost, 1);
});

test('a standing offer can be withdrawn; open deals keep their terms', async (t) => {
  const { maker, publisher, listing, deal, close } = await activeAffiliateDeal();
  t.after(close);
  assert.equal(await maker.client.send(maker.client.build('revoke', { scope: 'offer', offer: listing.body.id!, reason: 'owner_request' })), undefined);
  const late = await world();
  t.after(late.close);
  const results = (await publisher.client.send(publisher.client.build('discover', { want: 'offers' }))) as MessageByType['results'];
  assert.equal(results.body.items.length, 0);
  await rejects(publisher.client.send(publisher.client.build('accept', { offer: listing.body.id!, terms_hash: listing.body.terms_hash! })), 'state_conflict');
  assert.equal((await publisher.client.deal(deal.deal!)).body.state, 'active');
});
