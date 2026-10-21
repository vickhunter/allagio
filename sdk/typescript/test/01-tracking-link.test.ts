// Principle 1: the tracking link is the universal adapter. A deal's link works
// wherever it is pasted (email, X, a blog) with no platform integration.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { checkMessage, type MessageByType } from '../src/index.ts';
import { activeAffiliateDeal, click, post, UA } from './fixture.ts';

test('one tracking link counts human clicks from email, X and a blog, with no adapter', async (t) => {
  const { deal, publisher, maker, close, origin } = await activeAffiliateDeal();
  t.after(close);

  assert.equal(deal.body.state, 'active');
  const tracking = deal.body.tracking?.[0];
  assert.ok(tracking, 'an active deal with a place obligation carries a tracking link');
  assert.equal(tracking.status, 'live');
  const destination = 'https://acmenotes.example/pricing';

  const surfaces: Array<[string, Record<string, string>, string?]> = [
    ['email: a reader taps the link in a mail app', { 'user-agent': UA.mailApp }],
    ['X: the card preview bot fetches the link', { 'user-agent': UA.xPreview }],
    ['X: a reader clicks through t.co', { 'user-agent': UA.desktop, referer: 'https://t.co/' }],
    ['blog: a reader clicks the link in a post', { 'user-agent': UA.windows, referer: 'https://blog.example/five-tools' }],
    ['Slack unfurl', { 'user-agent': UA.slackPreview }],
    ['link checker using HEAD', { 'user-agent': UA.desktop }, 'HEAD'],
    ['a script', { 'user-agent': UA.curl }],
  ];
  for (const [surface, headers, method] of surfaces) {
    const hop = await click(tracking.link, headers, method);
    assert.equal(hop.status, 302, surface);
    assert.equal(new URL(hop.location).origin + new URL(hop.location).pathname, destination, `${surface} lands on the destination`);
  }

  const outcome = await publisher.client.outcome(deal.deal!);
  assert.deepEqual(outcome.body.rows[0]?.clicks, { human: 3, bot: 1, preview: 3, test: 0 });
  const makerView = await maker.client.outcome(deal.deal!);
  assert.deepEqual(makerView.body.rows[0]?.clicks, outcome.body.rows[0]?.clicks, 'both sides see the same aggregate');

  const snapshot = await publisher.client.deal(deal.deal!);
  assert.equal(snapshot.body.obligations.find((o) => o.id === 'place')?.state, 'verified', 'the first human click proves the placement');

  const text = JSON.stringify([outcome, snapshot]);
  for (const leak of ['Mozilla', 'Twitterbot', 't.co', 'blog.example', 'curl']) assert.ok(!text.includes(leak), `no ${leak} in what agents see`);
  assert.ok(checkMessage(outcome).ok);

  const test = await click(`${tracking.link}?orp_test=1`, { 'user-agent': UA.desktop });
  assert.equal(test.status, 302);
  const after = await publisher.client.outcome(deal.deal!);
  assert.equal(after.body.rows[0]?.clicks.test, 1, 'a marked test click is counted apart and never earns');
  assert.equal(after.body.rows[0]?.clicks.human, 3);

  const unknown = await click(`${origin}/go/doesnotexist`, { 'user-agent': UA.desktop });
  assert.equal(unknown.status, 302, 'an unknown code still redirects, never a dead end');
});

test('a revoked deal leaves a plain link: it still redirects, it no longer counts', async (t) => {
  const { deal, publisher, maker, close } = await activeAffiliateDeal();
  t.after(close);
  const link = deal.body.tracking![0]!.link;
  await click(link, { 'user-agent': UA.desktop });

  const revoked = (await maker.client.send(maker.client.build('revoke', { scope: 'deal', reason: 'owner_request' }, { deal: deal.deal! }))) as MessageByType['deal'];
  assert.equal(revoked.body.state, 'revoked');
  assert.equal(revoked.body.tracking?.[0]?.status, 'plain');

  const hop = await click(link, { 'user-agent': UA.desktop });
  assert.equal(hop.status, 302);
  assert.ok(hop.location.startsWith('https://acmenotes.example/pricing'), 'old emails and posts keep working for readers');
  const outcome = await publisher.client.outcome(deal.deal!);
  assert.equal(outcome.body.rows[0]?.clicks.human, 1, 'clicks after revoke are not counted');
});

test('conversions on the link earn the agreed share; refunds take it back', async (t) => {
  const { deal, publisher, origin, close } = await activeAffiliateDeal();
  t.after(close);
  const code = new URL(deal.body.tracking![0]!.link).pathname.split('/').pop()!;
  await click(deal.body.tracking![0]!.link, { 'user-agent': UA.desktop });
  for (const amount of [9600, 9600, 4800]) {
    const recorded = await post(origin, '/orp-test/conversions', { code, amount: { amount_minor: amount, currency: 'EUR' } });
    assert.equal(recorded.status, 200);
  }
  await post(origin, '/orp-test/conversions', { code, amount: { amount_minor: 4800, currency: 'EUR' }, refund: true });
  const row = (await publisher.client.outcome(deal.deal!)).body.rows[0]!;
  assert.equal(row.conversions, 3);
  assert.equal(row.refunds, 1);
  assert.deepEqual(row.revenue, { amount_minor: 19200, currency: 'EUR' });
  assert.deepEqual(row.earned, { amount_minor: 5760, currency: 'EUR' }, '30% of net revenue');
  assert.equal(deal.body.state, 'active', 'pay per conversion keeps the deal open until it ends or is revoked');
});
