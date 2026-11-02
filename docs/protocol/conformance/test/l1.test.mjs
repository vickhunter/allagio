import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { check } from '../../sdk/src/index.mjs';
import {
  AUTO, BROWSER, DESTINATION, OrpClient, SENTINEL, acceptOffer, affiliateTerms, codeOf,
  humanClicks, inbox, latest, listStanding, party, storeText, tempStore, withHost,
} from './helpers.mjs';

const here = dirname(fileURLToPath(import.meta.url));

test('C1-UNIVERSAL-LINK-NO-ADAPTER', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    const listed = await listStanding(maker);
    const accepted = await acceptOffer(pub, listed.body);
    assert.equal(accepted.status, 200, accepted.text);
    assert.equal(accepted.message.body.state, 'active');
    const link = accepted.message.body.tracking[0].link;
    assert.match(link, /\/go\//);

    const contexts = [
      {},
      { referer: 'https://x.com/acme/status/1' },
      { referer: 'https://blog.example/notes' },
    ];
    const vias = [];
    for (const headers of contexts) {
      const click = await OrpClient.click(link, { headers: { 'user-agent': BROWSER, ...headers } });
      assert.equal(click.status, 302);
      assert.equal(click.headers.get('cache-control'), 'no-store');
      assert.equal(click.headers.get('referrer-policy'), 'no-referrer');
      assert.equal(click.headers.get('x-robots-tag'), 'noindex');
      assert.equal(click.headers.get('set-cookie'), null);
      const location = new URL(click.location);
      assert.equal(location.origin + location.pathname, 'https://notes.example/pricing');
      assert.equal(location.searchParams.get('plan'), 'pro');
      const via = location.searchParams.get('via');
      assert.ok(via && via.length >= 8);
      assert.equal(location.searchParams.get('ref'), via);
      vias.push(via);
      if (vias.length === 1) {
        const early = await inbox(pub);
        const receipt = early.find((message) => message.type === 'receipt');
        assert.equal(receipt.body.method, 'link');
        assert.equal(receipt.body.result, 'verified');
        assert.equal(early.some((message) => message.type === 'execute'), false);
      }
    }
    assert.equal(new Set(vias).size, 3);
    const outcome = latest(await inbox(pub), 'outcome');
    assert.equal(humanClicks(outcome), 3);
    assert.equal((await inbox(maker)).some((message) => message.type === 'execute'), false);
    assert.equal(accepted.message.body.tracking[0].destination, DESTINATION);
  });
});

test('C2-NO-EMAIL-NO-SIGNUP', async () => {
  await withHost({ storePath: tempStore() }, async (host) => {
    const maker = await party(host, {
      steps: { list: 'auto', revoke: 'auto' },
      budget: { max_rate_bps: 5000 },
    });
    await listStanding(maker, affiliateTerms({ title: 'Acme Notes for a weekly letter' }));
    const text = `Weekly notes draft ${SENTINEL}`;
    const suggested = await OrpClient.suggest(host.origin, { text, join: {} });
    assert.equal(suggested.status, 200, suggested.text);
    assert.ok(suggested.message.items.length >= 1);
    for (const item of suggested.message.items) {
      assert.ok(item.pitch);
      assert.equal(item.pitch.includes(SENTINEL), false);
      assert.equal(item.pitch.includes(text), false);
    }
    assert.ok(suggested.message.joined.token.length >= 20);
    const taker = new OrpClient({
      origin: host.origin,
      token: suggested.message.joined.token,
      party: suggested.message.joined.party,
      claim: suggested.message.joined.claim,
    });
    const accepted = await acceptOffer(taker, suggested.message.items[0].offer);
    assert.equal(accepted.status, 200, accepted.text);
    assert.equal(accepted.message.body.state, 'awaiting');
    assert.match(accepted.message.body.waiting.url, /\/approve\//);
    const blob = JSON.stringify({ suggested: suggested.message, accepted: accepted.message }).toLowerCase();
    assert.equal(blob.includes('email'), false);
    assert.equal(blob.includes('password'), false);
    assert.equal(blob.includes('signup'), false);
    assert.equal(storeText(host).includes(SENTINEL), false);

    const emailed = await OrpClient.join(host.origin, { email: 'person@example.com' });
    assert.equal(emailed.status, 400);
    assert.equal(codeOf(emailed), 'invalid_message');
    const suggestEmail = await OrpClient.suggest(host.origin, { text: 'notes', email: 'person@example.com' });
    assert.equal(suggestEmail.status, 400);
    assert.equal(codeOf(suggestEmail), 'invalid_message');
  });
});

test('L1-PLAIN-AFTER-REVOKE', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    const accepted = await acceptOffer(pub, (await listStanding(maker)).body);
    const link = accepted.message.body.tracking[0].link;
    const first = await OrpClient.click(link, { headers: { 'user-agent': BROWSER } });
    assert.equal(first.status, 302);
    assert.ok(new URL(first.location).searchParams.get('via'));

    const revoked = await maker.send(maker.msg({
      type: 'revoke',
      deal: accepted.message.deal,
      body: { scope: 'deal', reason: 'owner_request' },
    }));
    assert.equal(revoked.status, 200, revoked.text);
    assert.equal(revoked.message.body.state, 'revoked');
    assert.equal(revoked.message.body.tracking[0].status, 'plain');

    const plain = await OrpClient.click(link, { headers: { 'user-agent': BROWSER, referer: 'https://x.com/post/1' } });
    assert.equal(plain.status, 302);
    const plainUrl = new URL(plain.location);
    assert.equal(plainUrl.searchParams.get('plan'), 'pro');
    assert.equal(plainUrl.searchParams.get('via'), null);
    const outcome = latest(await inbox(pub), 'outcome');
    assert.equal(humanClicks(outcome), 1);

    const unknown = await OrpClient.click(`${host.origin}/go/not-a-real-token`, { headers: { 'user-agent': BROWSER } });
    assert.equal(unknown.status, 302);
    assert.equal(unknown.location, `${host.origin}/`);
  });
});

test('L1-NO-PII', async () => {
  await withHost({ storePath: tempStore() }, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    const accepted = await acceptOffer(pub, (await listStanding(maker)).body);
    const click = await OrpClient.click(accepted.message.body.tracking[0].link, {
      headers: {
        'user-agent': BROWSER,
        referer: 'https://blog.example/hello?utm=secret',
      },
    });
    assert.equal(click.status, 302);
    const outcome = latest(await inbox(pub), 'outcome');
    const sources = outcome.body.rows[0].sources || [];
    assert.equal(sources.some((row) => row.source.includes('/')), false);
    assert.equal(sources.some((row) => row.source.includes('utm')), false);
    const dumped = JSON.stringify(await inbox(pub)) + storeText(host);
    assert.equal(dumped.includes('Chrome/120'), false);
    assert.equal(dumped.includes('utm=secret'), false);
    assert.equal(dumped.includes('@'), false);
    for (const name of ['outcome-with-ip.json', 'outcome-with-emails.json']) {
      const example = JSON.parse(readFileSync(join(here, '..', '..', 'schema', 'examples', 'invalid', name), 'utf8'));
      assert.equal(check('message', example.instance).ok, false);
    }
  });
});

test('L1-TEST-CLICKS', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    const accepted = await acceptOffer(pub, (await listStanding(maker)).body);
    const link = accepted.message.body.tracking[0].link;
    const flagged = await OrpClient.click(`${link}?orp_test=1`, { headers: { 'user-agent': BROWSER } });
    assert.equal(flagged.status, 302);
    host.markTestPool(accepted.message.deal);
    const pooled = await OrpClient.click(link, { headers: { 'user-agent': BROWSER } });
    assert.equal(pooled.status, 302);
    const outcome = latest(await inbox(pub), 'outcome');
    const clicks = outcome.body.rows[0].clicks;
    assert.equal(clicks.human, 0);
    assert.equal(clicks.test, 2);
    const messages = await inbox(pub);
    assert.equal(messages.some((message) => message.type === 'receipt'), false);
    assert.equal(host.deals.get(accepted.message.deal).obligations.find((row) => row.id === 'place').state, 'pending');
  });
});

test('L1-POLICIES', async () => {
  await withHost({ policies: false, registerPage: false }, async (bare) => {
    assert.deepEqual(bare.levels(), ['L0']);
    const doc = await OrpClient.discovery(bare.origin);
    assert.equal(doc.status, 200);
    assert.deepEqual(doc.message.conformance.levels, ['L0']);
    assert.equal(doc.message.policies, undefined);
    assert.match(doc.message.description, /Not the Allagio hosted API/);
  });
  await withHost({}, async (full) => {
    assert.deepEqual(full.levels(), ['L0', 'L1', 'L2', 'L3']);
    const doc = await OrpClient.discovery(full.origin);
    assert.ok(doc.message.policies.terms.endsWith('/terms'));
    assert.ok(doc.message.policies.privacy.endsWith('/privacy'));
    assert.deepEqual(doc.message.conformance.levels, ['L0', 'L1', 'L2', 'L3']);
  });
});
