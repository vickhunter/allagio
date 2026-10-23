import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  AUTO, BROWSER, DAY, OrpClient, affiliateTerms, codeOf, digest, directDeal,
  inbox, latest, party, place, terms, withHost,
} from './helpers.mjs';

const KINDS = [
  {
    kind: 'affiliate',
    tracking: true,
    obligations: () => affiliateTerms().obligations,
  },
  {
    kind: 'sponsored',
    tracking: true,
    obligations: () => [
      place(),
      { id: 'fee', by: 'offerer', action: 'pay', per: 'deal', amount: { amount_minor: 2000, currency: 'USD' }, rail: 'manual', proof: ['provider'] },
    ],
  },
  { kind: 'swap', tracking: true, obligations: () => [place()] },
  { kind: 'traffic_swap', tracking: true, obligations: () => [place({ clicks: 5 })] },
  {
    kind: 'link_exchange',
    tracking: false,
    obligations: () => [{
      id: 'link',
      by: 'acceptor',
      action: 'place',
      content: { direct: true, brief: 'Link the notes page.' },
      destination: 'https://notes.example/pricing?plan=pro',
      proof: ['proof_url'],
    }],
  },
  {
    kind: 'repost',
    tracking: false,
    obligations: () => [{ id: 'share', by: 'acceptor', action: 'repost', target: 'https://notes.example/post', proof: ['attestation'] }],
  },
  {
    kind: 'comment',
    tracking: false,
    obligations: () => [{
      id: 'note',
      by: 'acceptor',
      action: 'comment',
      target: 'https://notes.example/post',
      content: { brief: 'A public note.' },
      proof: ['attestation'],
    }],
  },
  { kind: 'cross_sell', tracking: true, obligations: () => [place()] },
  {
    kind: 'data',
    tracking: false,
    obligations: () => [{
      id: 'dataset',
      by: 'acceptor',
      action: 'share',
      data: { class: 'aggregate', fields: ['clicks'], k_min: 50, purpose: 'Compare reach.', retention: 'P30D' },
      proof: ['attestation'],
    }],
  },
  { kind: 'partnership', tracking: true, obligations: () => [place()] },
];

test('L3-KINDS', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    for (const spec of KINDS) {
      const { accepted } = await directDeal(maker, pub, terms(spec.kind, spec.obligations()));
      assert.equal(accepted.status, 200, `${spec.kind} ${accepted.text}`);
      assert.equal(accepted.message.body.state, 'active', spec.kind);
      assert.equal(accepted.message.body.terms.kind, spec.kind);
      const tracking = accepted.message.body.tracking || [];
      assert.equal(tracking.length > 0, spec.tracking, spec.kind);
    }
  });
});

test('L3-NEGOTIATE', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    const request = pub.msg({
      type: 'request',
      to: maker.party,
      body: { kinds: ['affiliate'], actions: ['pay'], note: 'Any notes offer this week?' },
    });
    const sent = await pub.send(request);
    assert.equal(sent.status, 202);
    assert.equal(sent.message, null);
    const received = (await inbox(maker)).find((message) => message.id === request.id);
    assert.equal(received.type, 'request');

    const offerTerms = affiliateTerms();
    const offered = await maker.send(maker.msg({
      type: 'offer',
      to: pub.party,
      body: { terms: offerTerms },
    }));
    assert.equal(offered.status, 200, offered.text);
    assert.equal(offered.message.body.state, 'proposed');
    assert.deepEqual(offered.message.body.bound.map((item) => item.party), [maker.party]);
    const dealId = offered.message.deal;
    const counteredTerms = affiliateTerms({ title: 'Notes offer round two' });
    const countered = await pub.send(pub.msg({
      type: 'counter',
      deal: dealId,
      body: { terms: counteredTerms },
    }));
    assert.equal(countered.status, 200, countered.text);
    assert.equal(countered.message.body.round, 2);
    assert.equal(countered.message.body.terms_hash, digest(counteredTerms));
    assert.notEqual(countered.message.body.terms_hash, offered.message.body.terms_hash);
    assert.deepEqual(countered.message.body.bound.map((item) => item.party), [pub.party]);
    const accepted = await maker.send(maker.msg({
      type: 'accept',
      deal: dealId,
      body: { terms_hash: countered.message.body.terms_hash },
    }));
    assert.equal(accepted.status, 200, accepted.text);
    assert.equal(accepted.message.body.state, 'active');
    assert.equal(accepted.message.body.bound.length, 2);
  });
});

test('L3-SPACE', async () => {
  await withHost({}, async (host) => {
    const owner = await party(host, AUTO);
    const reader = await party(host, AUTO);
    const space = {
      id: 'sp_news_1',
      kind: 'newsletter.issue',
      platform: 'newsletter',
      title: 'Notes weekly',
      locator: 'https://notes.example/news',
    };
    const listed = await owner.send(owner.msg({ type: 'space', body: { action: 'put', space } }));
    assert.equal(listed.status, 200, listed.text);
    assert.equal(listed.message.body.status, 'listed');
    assert.equal(listed.message.body.space.locator, space.locator);
    const found = await reader.send(reader.msg({ type: 'discover', body: { want: 'spaces' } }));
    assert.equal(found.status, 200, found.text);
    assert.equal(found.message.body.items[0].space.locator, undefined);
    assert.equal(found.message.body.items[0].space.id, space.id);
    const withdrawn = await owner.send(owner.msg({ type: 'space', body: { action: 'withdraw', id: space.id } }));
    assert.equal(withdrawn.status, 200, withdrawn.text);
    assert.equal(withdrawn.message.body.status, 'withdrawn');
    const after = await reader.send(reader.msg({ type: 'discover', body: { want: 'spaces' } }));
    assert.equal(after.message.body.items.length, 0);

    const fresh = await party(host);
    const waiting = await fresh.send(fresh.msg({
      type: 'space',
      body: { action: 'put', space: { ...space, id: 'sp_news_2' } },
    }));
    assert.equal(waiting.status, 200, waiting.text);
    assert.equal(waiting.message.body.status, 'waiting');
    assert.match(waiting.message.body.waiting.url, /\/approve\//);
    const hidden = await reader.send(reader.msg({ type: 'discover', body: { want: 'spaces' } }));
    assert.equal(hidden.message.body.items.some((item) => item.space.id === 'sp_news_2'), false);
  });
});

test('L3-DISPUTE', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    const { accepted } = await directDeal(maker, pub, affiliateTerms());
    const opened = await pub.send(pub.msg({
      type: 'dispute',
      deal: accepted.message.deal,
      body: {
        action: 'open',
        obligation: 'place',
        claim: 'not_done',
        statement: 'The placement never went up.',
      },
    }));
    assert.equal(opened.status, 200, opened.text);
    assert.equal(opened.message.body.state, 'disputed');
    const dispute = [...host.disputes.values()].find((item) => item.dealId === accepted.message.deal);
    const resolved = await pub.send(pub.msg({
      type: 'dispute',
      deal: accepted.message.deal,
      body: {
        action: 'resolve',
        dispute: dispute.id,
        resolution: { outcome: 'rejected', remedy: 'none' },
      },
    }));
    assert.equal(resolved.status, 403);
    assert.equal(codeOf(resolved), 'forbidden');
    assert.equal(host.deals.get(accepted.message.deal).state, 'disputed');
    host.resolveDispute(dispute.id, { outcome: 'rejected', remedy: 'none' });
    assert.equal(host.deals.get(accepted.message.deal).state, 'active');
  });
});

test('L3-REPUTATION', async () => {
  await withHost({}, async (host) => {
    const pub = await party(host, AUTO);
    await pub.claim();
    async function oneDeal() {
      const maker = await party(host, AUTO);
      const { accepted } = await directDeal(maker, pub, affiliateTerms({ title: `Notes deal ${maker.party.slice(-6)}` }));
      const click = await OrpClient.click(accepted.message.body.tracking[0].link, { headers: { 'user-agent': BROWSER } });
      assert.equal(click.status, 302);
      return maker;
    }
    await oneDeal();
    const early = await pub.send(pub.msg({ type: 'reputation', body: { subject: pub.party } }));
    assert.equal(early.message.body.score, null);
    assert.equal(early.message.body.algorithm, 'orp-rep-1');
    for (let i = 0; i < 19; i += 1) await oneDeal();
    const scored = await pub.send(pub.msg({ type: 'reputation', body: { subject: pub.party } }));
    assert.equal(typeof scored.message.body.score, 'number');
    assert.equal(scored.message.body.algorithm, 'orp-rep-1');
    assert.equal(typeof scored.message.body.components.counterparties, 'number');
    assert.equal(scored.message.body.components.counterparties, 20);
    const dumped = JSON.stringify(scored.message.body.components);
    assert.equal(dumped.includes('orp:'), false);
  });
});

test('L3-PAYOUT', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    const { accepted } = await directDeal(maker, pub, affiliateTerms());
    const click = await OrpClient.click(accepted.message.body.tracking[0].link, { headers: { 'user-agent': BROWSER } });
    const via = new URL(click.location).searchParams.get('via');
    const provider = (body) => fetch(`${host.origin}/orp/provider`, {
      method: 'POST',
      headers: { 'content-type': 'application/json', 'x-provider-test': host.providerToken },
      body: JSON.stringify(body),
    }).then(async (res) => ({ status: res.status, message: await res.json() }));

    const sale = await provider({ type: 'conversion', click: via, amount_minor: 10000, currency: 'USD' });
    assert.equal(sale.status, 200, JSON.stringify(sale.message));
    assert.equal(sale.message.state, 'held');
    const first = [...host.payouts.values()].find((item) => item.id === sale.message.payout);
    assert.equal(first.amount.amount_minor, 3000);
    assert.equal(first.basis, 'provider');
    const hold = Date.parse(first.holdUntil) - Date.now();
    assert.ok(hold > 29 * DAY && hold < 31 * DAY);

    const refund = await provider({ type: 'refund', click: via });
    assert.equal(refund.status, 200, JSON.stringify(refund.message));
    assert.equal(host.payouts.get(first.id).state, 'reversed');

    const again = await provider({ type: 'conversion', click: via, amount_minor: 10000, currency: 'USD' });
    assert.equal(again.message.state, 'held');
    host.advanceBusiness(31 * DAY);
    host.runDueChecks();
    assert.equal(host.payouts.get(again.message.payout).state, 'payable');
    const sent = await provider({ type: 'sent', payout: again.message.payout, ref: 'tr_test_1' });
    assert.equal(sent.status, 200, JSON.stringify(sent.message));
    assert.equal(host.payouts.get(again.message.payout).state, 'sent');

    const invented = await pub.send(pub.msg({
      type: 'payout',
      deal: accepted.message.deal,
      body: {
        payout: again.message.payout,
        answer: 'confirm',
        amount: { amount_minor: 1, currency: 'USD' },
      },
    }));
    assert.equal(invented.status, 400);
    assert.equal(codeOf(invented), 'invalid_message');
    assert.equal(host.payouts.get(again.message.payout).state, 'sent');

    const confirmed = await pub.send(pub.msg({
      type: 'payout',
      deal: accepted.message.deal,
      body: { payout: again.message.payout, answer: 'confirm' },
    }));
    assert.equal(confirmed.status, 200, confirmed.text);
    assert.equal(confirmed.message.body.state, 'paid');
    const records = (await inbox(pub)).filter((message) => message.type === 'payout');
    const dumped = JSON.stringify(records).toLowerCase();
    assert.equal(dumped.includes('iban'), false);
    assert.equal(dumped.includes('account'), false);
    assert.equal(dumped.includes('card'), false);
    assert.equal(JSON.stringify(await inbox(pub)).includes(via), false);

    const extra = await maker.reportConversion({ deal: accepted.message.deal, click: via, email: 'payer@example.com' });
    assert.equal(extra.status, 400);
    assert.equal(codeOf(extra), 'invalid_message');
    const report = await maker.reportConversion({ deal: accepted.message.deal, click: via });
    assert.equal(report.status, 200, report.text);
    assert.equal(report.message.payable, false);
    assert.equal(report.message.attested_only, true);
    const outcome = latest(await inbox(maker), 'outcome');
    assert.equal(outcome.body.rows[0].attested_conversions, 1);
    assert.equal([...host.payouts.values()].some((item) => item.basis === 'attestation'), false);
  });
});
