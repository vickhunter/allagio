import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  AUTO, BROWSER, DAY, OrpClient, acceptOffer, affiliateTerms, codeOf, directDeal,
  inbox, latest, listStanding, party, place, terms, withHost,
} from './helpers.mjs';

function steps(mode) {
  return { ...AUTO, steps: { ...AUTO.steps, accept: mode } };
}

test('L2-STEP-MODES', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const listed = await listStanding(maker);

    const off = await party(host, steps('off'));
    const before = host.deals.size;
    const refused = await acceptOffer(off, listed.body);
    assert.equal(refused.status, 403);
    assert.equal(codeOf(refused), 'step_off');
    assert.equal(refused.message.body.step, 'accept');
    assert.equal(host.deals.size, before);

    const asking = await party(host);
    const waiting = await acceptOffer(asking, listed.body);
    assert.equal(waiting.status, 200, waiting.text);
    assert.equal(waiting.message.body.state, 'awaiting');
    assert.match(waiting.message.body.waiting.url, /\/approve\//);
    assert.ok(waiting.message.body.waiting.reasons.includes('step_ask'));
    const offererCopy = (await inbox(maker)).find((message) => message.type === 'deal' && message.deal === waiting.message.deal);
    assert.equal(offererCopy.body.waiting.url, undefined);
    assert.equal(offererCopy.body.waiting.reasons, undefined);
    assert.equal(offererCopy.body.waiting.party, asking.party);

    const autoParty = await party(host, AUTO);
    const active = await acceptOffer(autoParty, listed.body);
    assert.equal(active.status, 200, active.text);
    assert.equal(active.message.body.state, 'active');
    assert.equal(active.message.body.tracking[0].status, 'live');
    assert.match(active.message.body.tracking[0].link, /\/go\//);

    const strict = await party(host, {
      ...AUTO,
      brand: { disclosure: { label: 'Partner mention', lang: 'en' } },
    });
    const deals = host.deals.size;
    const branded = await acceptOffer(strict, listed.body);
    assert.equal(branded.status, 403);
    assert.equal(codeOf(branded), 'brand_rule');
    assert.equal(host.deals.size, deals);
  });
});

test('L2-CONSENT-HOST-ONLY', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host);
    const agentWiden = await maker.setConstraints(AUTO, maker.token);
    assert.equal(agentWiden.status, 403);
    assert.equal(codeOf(agentWiden), 'forbidden');
    await maker.claim();
    const owned = await maker.setConstraints(AUTO);
    assert.equal(owned.status, 200, owned.text);
    const tightened = structuredClone(AUTO);
    tightened.steps.accept = 'ask';
    const agentTighten = await maker.setConstraints(tightened, maker.token);
    assert.equal(agentTighten.status, 200, agentTighten.text);
    const agentLoosen = await maker.setConstraints(AUTO, maker.token);
    assert.equal(agentLoosen.status, 403);
    const ownerPost = await OrpClient.post(host.origin, maker.ownerToken, maker.msg({ type: 'hello', body: { versions: ['0.3'] } }));
    assert.equal(ownerPost.status, 403);
    assert.equal(codeOf(ownerPost), 'forbidden');

    const paying = structuredClone(AUTO);
    paying.steps.negotiate = 'ask';
    await maker.setConstraints(paying);
    const pub = await party(host, AUTO);
    const offered = await maker.send(maker.msg({
      type: 'offer',
      to: pub.party,
      body: { terms: affiliateTerms() },
    }));
    assert.equal(offered.status, 200, offered.text);
    assert.equal(offered.message.body.state, 'awaiting');
    const escalation = offered.message.body.waiting.escalation;
    const prompted = await maker.approve(escalation, { assurance: 'client_prompt' });
    assert.equal(prompted.status, 403);
    assert.equal(codeOf(prompted), 'forbidden');
    assert.equal(host.deals.get(offered.message.deal).state, 'awaiting');
    const approved = await maker.approve(escalation, { assurance: 'passkey' });
    assert.equal(approved.status, 200, approved.text);
    assert.equal(approved.message.body.state, 'proposed');
    const makerConsent = (await inbox(maker)).find((message) => message.type === 'consent' && message.deal === offered.message.deal);
    const pubConsent = (await inbox(pub)).find((message) => message.type === 'consent' && message.deal === offered.message.deal);
    assert.equal(makerConsent.body.assurance, 'passkey');
    assert.ok(makerConsent.body.reasons.includes('step_ask'));
    assert.equal(pubConsent.body.reasons, undefined);
    assert.ok(makerConsent.sig);

    const forged = await pub.send(pub.msg({
      type: 'consent',
      deal: offered.message.deal,
      body: {
        consent: 'cns_party01',
        party: pub.party,
        decision: 'approved',
        step: 'accept',
        terms_hash: offered.message.body.terms_hash,
        assurance: 'passkey',
        at: new Date().toISOString(),
      },
    }));
    assert.equal(forged.status, 403);
    assert.equal(codeOf(forged), 'forbidden');

    await maker.setConstraints(AUTO);
    const fresh = await party(host);
    const listed = await listStanding(maker);
    const waiting = await acceptOffer(fresh, listed.body);
    assert.equal(waiting.message.body.state, 'awaiting');
    host.advanceBusiness(8 * DAY);
    host.runDueChecks();
    assert.equal(host.deals.get(waiting.message.deal).state, 'expired');
  });
});

test('L2-NO-SELF-RECEIPT', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    const offerTerms = terms('affiliate', [
      { ...place(), proof: ['link', 'attestation'] },
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
    ]);
    const { accepted } = await directDeal(maker, pub, offerTerms);
    assert.equal(accepted.message.body.state, 'active');
    const claim = await pub.send(pub.msg({
      type: 'execute',
      deal: accepted.message.deal,
      body: { obligation: 'place' },
    }));
    assert.equal(claim.status, 200, claim.text);
    assert.equal(claim.message.body.obligations.find((row) => row.id === 'place').state, 'claimed');
    assert.equal((await inbox(pub)).some((message) => message.type === 'receipt'), false);

    const at = new Date().toISOString();
    const selfLink = await pub.send(pub.msg({
      type: 'receipt',
      deal: accepted.message.deal,
      body: { obligation: 'place', result: 'verified', method: 'link', at },
    }));
    assert.equal(selfLink.status, 403);
    assert.equal(codeOf(selfLink), 'forbidden');
    const selfAttest = await pub.send(pub.msg({
      type: 'receipt',
      deal: accepted.message.deal,
      body: { obligation: 'place', result: 'verified', method: 'attestation', at },
    }));
    assert.equal(selfAttest.status, 403);
    assert.equal(codeOf(selfAttest), 'forbidden');
    const attested = await maker.send(maker.msg({
      type: 'receipt',
      deal: accepted.message.deal,
      body: { obligation: 'place', result: 'verified', method: 'attestation', at },
    }));
    assert.equal(attested.status, 200, attested.text);
    assert.equal(attested.message.body.obligations.find((row) => row.id === 'place').state, 'verified');
  });
});

test('L2-ATTESTATION-NO-REP', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    await pub.claim();
    const offerTerms = terms('sponsored', [
      { ...place(), proof: ['attestation'] },
      {
        id: 'fee',
        by: 'offerer',
        action: 'pay',
        per: 'deal',
        amount: { amount_minor: 2000, currency: 'USD' },
        after: 'place',
        rail: 'manual',
        proof: ['provider'],
      },
    ]);
    const { accepted } = await directDeal(maker, pub, offerTerms);
    const at = new Date().toISOString();
    const attested = await maker.send(maker.msg({
      type: 'receipt',
      deal: accepted.message.deal,
      body: { obligation: 'place', result: 'verified', method: 'attestation', at },
    }));
    assert.equal(attested.status, 200, attested.text);
    const before = await pub.send(pub.msg({ type: 'reputation', body: { subject: pub.party } }));
    assert.equal(before.message.body.score, null);
    assert.equal(before.message.body.components.evidence_weight, 0);
    assert.equal([...host.payouts.values()].some((payout) => payout.state === 'payable'), false);
    assert.equal(host.payouts.size, 0);

    const linked = await directDeal(maker, pub, affiliateTerms());
    assert.equal(linked.accepted.message.body.state, 'active');
    await OrpClient.click(linked.accepted.message.body.tracking[0].link, { headers: { 'user-agent': BROWSER } });
    const after = await pub.send(pub.msg({ type: 'reputation', body: { subject: pub.party } }));
    assert.equal(after.message.body.algorithm, 'orp-rep-1');
    assert.ok(after.message.body.components.evidence_weight > 0);
    assert.equal(JSON.stringify(after.message.body.components).includes(maker.party), false);
  });
});

test('L2-CLAIMS-UNCHECKED', async () => {
  async function unchecked(options) {
    await withHost(options, async (host) => {
      const maker = await party(host, AUTO);
      const pub = await party(host, AUTO);
      const listed = await listStanding(maker);
      const accepted = await acceptOffer(pub, listed.body);
      assert.equal(accepted.status, 200, accepted.text);
      assert.equal(accepted.message.body.state, 'awaiting');
      assert.ok(accepted.message.body.waiting.reasons.includes('claims_unchecked'));
      assert.notEqual(accepted.message.body.state, 'active');
      const suggested = await OrpClient.suggest(host.origin, { text: 'notes for founders' });
      assert.equal(suggested.status, 200, suggested.text);
      assert.equal(suggested.message.items[0].checks.claims, 'not_checked');
    });
  }
  await unchecked({ claims: 'stub', registerPage: false });
  await unchecked({ claims: 'real', registerPage: false });
});

test('L2-RECHECK', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    const offerTerms = terms('sponsored', [
      { ...place({ keep: 'P14D' }), proof: ['link'] },
      {
        id: 'fee',
        by: 'offerer',
        action: 'pay',
        per: 'deal',
        amount: { amount_minor: 2000, currency: 'USD' },
        after: 'place',
        rail: 'manual',
        proof: ['provider'],
      },
    ]);
    const { accepted } = await directDeal(maker, pub, offerTerms);
    assert.equal(accepted.message.body.state, 'active');
    const click = await OrpClient.click(accepted.message.body.tracking[0].link, { headers: { 'user-agent': BROWSER } });
    assert.equal(click.status, 302);
    const receipt = latest(await inbox(pub), 'receipt');
    assert.equal(receipt.body.method, 'link');
    const nextCheck = Date.parse(receipt.body.next_check);
    assert.ok(nextCheck - Date.now() > DAY - 60_000);
    assert.ok(nextCheck - Date.now() < DAY + 60_000);
    const payout = [...host.payouts.values()][0];
    assert.equal(payout.state, 'held');
    assert.equal(payout.recheckHold, true);

    host.advanceBusiness(DAY + 1000);
    host.failNextRecheck(accepted.message.deal, 'place');
    host.runDueChecks();
    const deal = host.deals.get(accepted.message.deal);
    assert.equal(deal.obligations.find((row) => row.id === 'place').state, 'failed');
    assert.equal(deal.state, 'disputed');
    const dispute = [...host.disputes.values()].find((item) => item.dealId === deal.id);
    assert.equal(dispute.claim, 'removed_early');
    assert.notEqual(host.payouts.get(payout.id).state, 'payable');
    assert.equal(host.payouts.get(payout.id).recheckHold, true);
  });
});
