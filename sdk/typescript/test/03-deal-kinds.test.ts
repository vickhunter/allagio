// Deal kinds beyond affiliate. Each is verified either by human clicks on its
// tracking link or, when nothing is clicked, by a receipt from a public page.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { OrpProtocolError, termsHash, type MessageByType, type ReceiptBody, type Terms } from '../src/index.ts';
import { click, crossSellTerms, linkExchangeTerms, paidRepostTerms, post, publisherSpaces, trafficSwapTerms, UA, world } from './fixture.ts';

type World = Awaited<ReturnType<typeof world>>;
type Party = Awaited<ReturnType<World['join']>>;

async function directDeal(w: World, terms: Terms, offerer: Party, acceptor: Party) {
  const proposed = (await offerer.client.send(offerer.client.build('offer', { terms, terms_hash: termsHash(terms) }, { to: acceptor.party }))) as MessageByType['deal'];
  const active = (await acceptor.client.send(acceptor.client.build('accept', { terms_hash: termsHash(terms) }, { deal: proposed.deal! }))) as MessageByType['deal'];
  assert.equal(active.body.state, 'active');
  return active;
}

const stateOf = async (party: Party, deal: string, obligation: string) => (await party.client.deal(deal)).body.obligations?.find((o) => o.id === obligation)?.state;

async function receipts(party: Party, obligation: string): Promise<ReceiptBody[]> {
  return (await party.client.inbox()).messages.flatMap((m) => (m.type === 'receipt' && m.body.obligation === obligation ? [m.body] : []));
}

test('traffic swap: each side is done when its link has sent the agreed number of human readers', async (t) => {
  const w = await world();
  t.after(w.close);
  const maker = await w.join({ steps: { negotiate: 'auto' } }, [], 'maker-agent');
  const publisher = await w.join({ steps: { accept: 'auto' } }, publisherSpaces, 'publisher-agent');
  const terms = trafficSwapTerms();
  for (const o of terms.obligations) if (o.action === 'place') Object.assign(o, { clicks: 3 });
  const deal = await directDeal(w, terms, maker, publisher);
  const linkFor = (id: string) => deal.body.tracking!.find((tracking) => tracking.obligation === id)!.link;

  for (let i = 0; i < 4; i += 1) await click(linkFor('send_readers'), { 'user-agent': UA.xPreview });
  await click(linkFor('send_readers'), { 'user-agent': UA.curl });
  await click(linkFor('send_readers'), { 'user-agent': UA.desktop });
  await click(linkFor('send_readers'), { 'user-agent': UA.mailApp });
  assert.equal(await stateOf(maker, deal.deal!, 'send_readers'), 'pending', 'previews and bots never count toward the target');
  await click(linkFor('send_readers'), { 'user-agent': UA.windows });
  assert.equal(await stateOf(maker, deal.deal!, 'send_readers'), 'verified');
  assert.deepEqual((await receipts(maker, 'send_readers'))[0]?.found, { link: true, clicks: 3 });

  for (let i = 0; i < 3; i += 1) await click(linkFor('send_back'), { 'user-agent': UA.desktop });
  assert.equal((await maker.client.deal(deal.deal!)).body.state, 'completed', 'both sides sent their readers');
});

test('traffic swap: a target still unmet at the due date fails', async (t) => {
  let now = Date.now();
  const w = await world({ clock: () => new Date(now) });
  t.after(w.close);
  const maker = await w.join({ steps: { negotiate: 'auto' } });
  const publisher = await w.join({ steps: { accept: 'auto' } }, publisherSpaces);
  const deal = await directDeal(w, trafficSwapTerms(), maker, publisher);
  await click(deal.body.tracking![1]!.link, { 'user-agent': UA.desktop });
  now += 31 * 86_400_000;
  assert.equal(await stateOf(publisher, deal.deal!, 'send_back'), 'failed');
  const [failed] = await receipts(publisher, 'send_back');
  assert.equal(failed?.reason, 'too_late');
  assert.deepEqual(failed?.found, { link: true, clicks: 1 }, 'the receipt says how far it got');
});

test('link exchange: direct links proven by the page, with rel sponsored or nofollow, and rechecked while kept', async (t) => {
  const w = await world();
  t.after(w.close);
  const dev = await w.join({ steps: { negotiate: 'auto' } }, [], 'dev-agent');
  const publisher = await w.join({ steps: { accept: 'auto' } }, publisherSpaces, 'publisher-agent');
  const deal = await directDeal(w, linkExchangeTerms(), dev, publisher);
  assert.equal(deal.body.tracking, undefined, 'a direct link has no tracking link and counts no clicks');

  const devPage = 'https://devnotes.example/resources';
  const execute = (party: Party, obligation: string, url: string) => party.client.send(party.client.build('execute', { obligation, proof_url: url }, { deal: deal.deal! }));
  await post(w.origin, '/orp-test/web', { url: devPage, body: '<ul><li>Partner link: <a href="https://indiestackweekly.example/tools">Indie Stack tools</a></li></ul>' });
  await execute(dev, 'link_out', devPage);
  assert.equal(await stateOf(dev, deal.deal!, 'link_out'), 'claimed');
  assert.equal((await receipts(dev, 'link_out'))[0]?.reason, 'rel_missing', 'a followed link would trade rankings, not readers');

  await post(w.origin, '/orp-test/web', { url: devPage, body: '<ul><li>Partner link: <a rel="sponsored noopener" href="https://www.indiestackweekly.example/tools/">Indie Stack tools</a></li></ul>' });
  await execute(dev, 'link_out', devPage);
  const verified = (await receipts(dev, 'link_out')).at(-1)!;
  assert.equal(verified.result, 'verified');
  assert.deepEqual(verified.found, { destination: true, rel: true, disclosure: true });
  assert.ok(verified.next_check, 'kept links are checked again');

  const publisherPage = 'https://indiestackweekly.example/tools';
  await post(w.origin, '/orp-test/web', { url: publisherPage, body: '<p>Partner link: <a href="https://devnotes.example/" rel="nofollow">Dev Notes</a></p>' });
  await execute(publisher, 'link_back', publisherPage);
  assert.equal((await dev.client.deal(deal.deal!)).body.state, 'completed');

  await post(w.origin, '/orp-test/web', { url: publisherPage, body: '<p>Tools we like.</p>' });
  await post(w.origin, `/orp-test/recheck/${deal.deal}`, {});
  assert.equal(await stateOf(dev, deal.deal!, 'link_back'), 'failed', 'taking the link down before keep ends breaks the deal');
  assert.equal((await receipts(dev, 'link_back')).at(-1)?.reason, 'removed');
});

test('cross-sell: a checkout slot earns a share of what customers spend', async (t) => {
  const w = await world();
  t.after(w.close);
  const maker = await w.join({ steps: { list: 'auto' }, budget: { max_rate_bps: 2000 } }, [], 'maker-agent');
  const terms = crossSellTerms();
  const listing = (await maker.client.send(maker.client.build('offer', { terms, terms_hash: termsHash(terms) }))) as MessageByType['offer'];
  const shop = await w.join({ steps: { accept: 'auto' } }, publisherSpaces, 'shop-agent');
  const deal = (await shop.client.send(shop.client.build('accept', { offer: listing.body.id!, terms_hash: listing.body.terms_hash!, spaces: [{ obligation: 'slot', space: 'sp_app_1' }] }))) as MessageByType['deal'];
  assert.equal(deal.body.state, 'active');
  assert.equal(deal.body.spaces?.[0]?.kind, 'app.slot');
  const link = deal.body.tracking![0]!.link;
  await click(link, { 'user-agent': UA.mailApp });
  const code = new URL(link).pathname.split('/').pop()!;
  await post(w.origin, '/orp-test/conversions', { code, amount: { amount_minor: 12000, currency: 'EUR' } });
  const row = (await shop.client.outcome(deal.deal!)).body.rows[0]!;
  assert.deepEqual(row.earned, { amount_minor: 2400, currency: 'EUR' }, '20% of the first payment');
});

test('paid repost: it must be a quote, and the quote must carry the label', async (t) => {
  const w = await world();
  t.after(w.close);
  const maker = await w.join({ steps: { list: 'auto' }, budget: { per_deal: { amount_minor: 5000, currency: 'EUR' } } }, [], 'maker-agent');
  const plain = paidRepostTerms();
  for (const o of plain.obligations) if (o.action === 'repost') Object.assign(o, { quote: false });
  await assert.rejects(
    maker.client.send(maker.client.build('offer', { terms: plain, terms_hash: termsHash(plain) })),
    (error: unknown) => error instanceof OrpProtocolError && error.reply.body.code === 'invalid_message' && /quote/.test(error.reply.body.message),
  );

  const terms = paidRepostTerms();
  const listing = (await maker.client.send(maker.client.build('offer', { terms, terms_hash: termsHash(terms) }))) as MessageByType['offer'];
  const poster = await w.join({ steps: { accept: 'auto', execute: 'auto' } }, publisherSpaces, 'poster-agent');
  const deal = (await poster.client.send(poster.client.build('accept', { offer: listing.body.id!, terms_hash: listing.body.terms_hash! }))) as MessageByType['deal'];
  const target = 'https://x.com/acmenotes/status/1848200000000000000';
  const quote = 'https://x.com/indiestackweekly/status/1848300000000000000';
  const execute = () => poster.client.send(poster.client.build('execute', { obligation: 'repost', proof_url: quote }, { deal: deal.deal! }));

  await post(w.origin, '/orp-test/web', { url: quote, body: `<article>Best notes app I have tried. ${target}</article>` });
  await execute();
  assert.equal((await receipts(poster, 'repost')).at(-1)?.reason, 'disclosure_missing');
  await post(w.origin, '/orp-test/web', { url: quote, body: `<article>Paid partnership. Best notes app I have tried. ${target}</article>` });
  await execute();
  assert.equal(await stateOf(maker, deal.deal!, 'repost'), 'verified', 'verified with no click: the public quote is the receipt');
});
