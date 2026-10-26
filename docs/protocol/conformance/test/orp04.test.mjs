import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { check, dealHash, digest } from '../../sdk/src/index.mjs';
import {
  AUTO, DAY, DESTINATION, OrpClient, affiliateTerms, codeOf, directDeal, disclosure, party, withHost,
} from './helpers.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const eur = (amountMinor) => ({ amount_minor: amountMinor, currency: 'EUR' });
const identity = {
  hash: digest({ illustrative: 'facilitator' }),
  issuer: 'example-issuer',
};

function msg(client, fields) {
  return client.msg({ orp: '0.4', ...fields });
}

async function offerMany(maker, terms, parties) {
  return maker.send(msg(maker, { type: 'offer', body: { terms, parties } }));
}

async function acceptDeal(client, snapshot) {
  return client.send(msg(client, {
    type: 'accept',
    deal: snapshot.deal,
    body: { terms_hash: snapshot.body.terms_hash },
  }));
}

function place(id, by) {
  return {
    id,
    by,
    action: 'place',
    content: { link: true, brief: 'Mention the notes app.' },
    destination: DESTINATION,
    proof: ['link'],
  };
}

test('ORP04-REFERRAL-CHAIN', async () => {
  await withHost({}, async (host) => {
    const creator = await party(host, AUTO);
    const curator = await party(host, AUTO);
    const newsletter = await party(host, AUTO);
    const terms = {
      title: 'Referral notes chain',
      kind: 'partnership',
      disclosure,
      obligations: [place('place', newsletter.party)],
    };
    const opened = await offerMany(creator, terms, [
      { id: creator.party, role: 'offerer', required: true, terms: { percent: 50 } },
      { id: curator.party, role: 'curator', required: true, terms: { percent: 30 } },
      { id: newsletter.party, role: 'acceptor', required: true, terms: { percent: 20 } },
    ]);
    assert.equal(opened.status, 200, opened.text);
    assert.equal(opened.message.orp, '0.4');
    assert.equal(opened.message.body.state, 'pending');
    assert.equal(opened.message.body.parties.filter((seat) => seat.signed).length, 1);
    const mid = await acceptDeal(curator, opened.message);
    assert.equal(mid.message.body.state, 'pending');
    const done = await acceptDeal(newsletter, mid.message);
    assert.equal(done.status, 200, done.text);
    assert.equal(done.message.body.state, 'active');
    assert.equal(done.message.body.parties.every((seat) => seat.signed), true);
    assert.equal(done.message.body.terms_hash, dealHash(done.message.body.terms, done.message.body.parties));
    assert.ok(done.message.body.tracking.some((row) => row.obligation === 'place' && row.status === 'live'));
  });
});

test('ORP04-LINK-SWAP-RING', async () => {
  await withHost({}, async (host) => {
    const members = [];
    for (let index = 0; index < 4; index += 1) members.push(await party(host, AUTO));
    const terms = {
      title: 'Link swap ring',
      kind: 'link_exchange',
      disclosure,
      obligations: members.map((client, index) => place(`place${index + 1}`, client.party)),
    };
    const opened = await offerMany(members[0], terms, members.map((client) => ({
      id: client.party,
      role: 'member',
      required: true,
      terms: { percent: 25 },
    })));
    assert.equal(opened.status, 200, opened.text);
    assert.equal(opened.message.body.state, 'pending');
    let snapshot = opened.message;
    for (const client of members.slice(1)) {
      const accepted = await acceptDeal(client, snapshot);
      assert.equal(accepted.status, 200, accepted.text);
      snapshot = accepted.message;
    }
    assert.equal(snapshot.body.state, 'active');
    assert.equal(snapshot.body.parties.length, 4);
    assert.equal(snapshot.body.parties.every((seat) => seat.signed && seat.role === 'member'), true);
    assert.equal(snapshot.body.terms_hash, dealHash(snapshot.body.terms, snapshot.body.parties));
    assert.equal(snapshot.body.tracking.length, 4);
  });
});

test('ORP04-HOME-SALE', async () => {
  await withHost({}, async (host) => {
    const seller = await party(host, AUTO);
    const buyer = await party(host, AUTO);
    const mediator = await party(host, AUTO);
    const door = await party(host, AUTO);
    const notary = await party(host, AUTO);
    const replacement = await party(host, AUTO);
    const fee = eur(100000);
    const terms = {
      title: 'Illustrative five-seat deal, test mode only',
      kind: 'partnership',
      obligations: [{ id: 'share', by: 'signer', action: 'share' }],
    };
    const parties = [
      { id: seller.party, role: 'offerer', required: true, terms: { amount: eur(0) } },
      { id: buyer.party, role: 'acceptor', required: true, terms: { amount: fee } },
      { id: mediator.party, role: 'facilitator', required: true, identity, terms: { amount: fee } },
      {
        id: door.party,
        role: 'service',
        required: false,
        terms: {
          amount: eur(4000),
          per: 'hour',
          hours: 2,
          deposit: { amount: eur(2000), holder: 'viewing-desk', purpose: 'viewing' },
        },
      },
      { id: notary.party, role: 'signer', required: true },
    ];
    const opened = await offerMany(seller, terms, parties);
    assert.equal(opened.status, 200, opened.text);
    assert.equal(opened.message.body.state, 'pending');
    assert.equal(JSON.stringify(opened.message).includes('@'), false);
    const buyerSigned = await acceptDeal(buyer, opened.message);
    const mediatorSigned = await acceptDeal(mediator, buyerSigned.message);
    const notarySigned = await acceptDeal(notary, mediatorSigned.message);
    assert.equal(notarySigned.status, 200, notarySigned.text);
    assert.equal(notarySigned.message.body.state, 'active');
    const doorSeat = notarySigned.message.body.parties.find((seat) => seat.id === door.party);
    assert.equal(doorSeat.signed, false);
    assert.equal(doorSeat.required, false);
    const doorSigned = await acceptDeal(door, notarySigned.message);
    assert.equal(doorSigned.message.body.state, 'active');
    assert.equal(doorSigned.message.body.parties.find((seat) => seat.id === door.party).signed, true);
    const doorLeft = await door.send(msg(door, { type: 'leave', deal: doorSigned.message.deal, body: {} }));
    assert.equal(doorLeft.status, 200, doorLeft.text);
    assert.equal(doorLeft.message.body.state, 'active');
    assert.equal(doorLeft.message.body.parties.length, 4);
    assert.equal(doorLeft.message.body.parties.some((seat) => seat.id === door.party), false);
    const mediatorLeft = await mediator.send(msg(mediator, { type: 'leave', deal: doorLeft.message.deal, body: {} }));
    assert.equal(mediatorLeft.status, 200, mediatorLeft.text);
    assert.equal(mediatorLeft.message.body.state, 'pending');
    assert.ok(mediatorLeft.message.body.vacancies.includes('facilitator'));
    assert.equal(mediatorLeft.message.body.parties.some((seat) => seat.id === mediator.party), false);
    const seated = await seller.send(msg(seller, {
      type: 'seat',
      deal: mediatorLeft.message.deal,
      body: {
        seat: {
          id: replacement.party,
          role: 'facilitator',
          required: true,
          identity,
          terms: { amount: fee },
        },
      },
    }));
    assert.equal(seated.status, 200, seated.text);
    assert.equal(seated.message.body.state, 'pending');
    assert.equal(seated.message.body.parties.find((seat) => seat.id === replacement.party).signed, false);
    assert.ok(seated.message.body.vacancies.includes('facilitator'));
    const replaced = await acceptDeal(replacement, seated.message);
    assert.equal(replaced.status, 200, replaced.text);
    assert.equal(replaced.message.body.state, 'active');
    assert.equal(replaced.message.body.vacancies, undefined);
    assert.equal(replaced.message.body.parties.find((seat) => seat.id === replacement.party).signed, true);
    const held = replaced.message.body.parties.find((seat) => seat.role === 'service');
    assert.equal(held, undefined);

    const mixed = await offerMany(seller, terms, parties.map((seat) => (
      seat.role === 'facilitator' ? { ...seat, terms: { percent: 1, cap: fee } } : seat
    )));
    assert.equal(mixed.status, 200, mixed.text);
    assert.equal(mixed.message.body.state, 'pending');
    assert.equal(mixed.message.body.parties.find((seat) => seat.role === 'facilitator').terms.percent, 1);
  });
});

test('ORP04-BAD-SPLIT', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const other = await party(host, AUTO);
    const terms = {
      title: 'Bad notes split',
      kind: 'partnership',
      obligations: [{ id: 'share', by: 'offerer', action: 'share' }],
    };
    const bad = await offerMany(maker, terms, [
      { id: maker.party, role: 'offerer', required: true, terms: { percent: 60 } },
      { id: other.party, role: 'acceptor', required: true, terms: { percent: 41 } },
    ]);
    assert.equal(bad.status, 400);
    assert.equal(codeOf(bad), 'invalid_message');
    assert.match(bad.message.body.message, /100/);

    const deposit = await offerMany(maker, terms, [
      { id: maker.party, role: 'offerer', required: true, terms: { amount: eur(0) } },
      {
        id: other.party,
        role: 'acceptor',
        required: true,
        terms: { deposit: { amount: eur(2000), holder: 'host', purpose: 'viewing' } },
      },
    ]);
    assert.equal(deposit.status, 400);
    assert.equal(codeOf(deposit), 'invalid_message');
    assert.match(deposit.message.body.message, /does not hold money/);

    const namedHost = await offerMany(maker, terms, [
      { id: maker.party, role: 'offerer', required: true },
      {
        id: other.party,
        role: 'service',
        required: true,
        terms: { deposit: { amount: eur(2000), holder: host.hostId, purpose: 'viewing' } },
      },
    ]);
    assert.equal(namedHost.status, 400);
    assert.equal(codeOf(namedHost), 'invalid_message');

    const pii = await maker.send(msg(maker, {
      type: 'offer',
      body: {
        terms,
        parties: [
          { id: maker.party, role: 'offerer', required: true },
          {
            id: other.party,
            role: 'facilitator',
            required: true,
            identity: { ...identity, email: 'person@example.com' },
          },
        ],
      },
    }));
    assert.equal(pii.status, 400);
    assert.equal(codeOf(pii), 'invalid_message');
  });
});

test('ORP04-MEDIATOR-LEAVES', async () => {
  await withHost({}, async (host) => {
    const seller = await party(host, AUTO);
    const buyer = await party(host, AUTO);
    const mediator = await party(host, AUTO);
    const terms = {
      title: 'Illustrative mediator seat, test mode only',
      kind: 'partnership',
      obligations: [{ id: 'share', by: 'offerer', action: 'share' }],
    };
    const opened = await offerMany(seller, terms, [
      { id: seller.party, role: 'offerer', required: true, terms: { amount: eur(0) } },
      { id: buyer.party, role: 'acceptor', required: true, terms: { amount: eur(100000) } },
      { id: mediator.party, role: 'facilitator', required: true, identity, terms: { amount: eur(100000) } },
    ]);
    assert.equal(opened.status, 200, opened.text);
    const buyerSigned = await acceptDeal(buyer, opened.message);
    const active = await acceptDeal(mediator, buyerSigned.message);
    assert.equal(active.message.body.state, 'active');
    const left = await mediator.send(msg(mediator, { type: 'leave', deal: active.message.deal, body: {} }));
    assert.equal(left.status, 200, left.text);
    assert.equal(left.message.body.state, 'pending');
    assert.deepEqual(left.message.body.vacancies, ['facilitator']);
    assert.equal(left.message.body.parties.every((seat) => seat.id !== mediator.party), true);
    assert.equal(left.message.body.parties.filter((seat) => seat.role !== 'facilitator').every((seat) => seat.signed), true);
  });
});

test('ORP04-EXPIRED-OFFER', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const reader = await party(host, AUTO);
    const terms = {
      title: 'Notes freshness window',
      kind: 'affiliate',
      disclosure,
      claims: { commission: 30, destination: DESTINATION },
      obligations: [place('place', 'acceptor')],
    };
    const listed = await maker.send(msg(maker, { type: 'offer', body: { terms } }));
    assert.equal(listed.status, 200, listed.text);
    const offer = listed.message.body;
    assert.equal(offer.terms.claims.commission, 'unknown');
    assert.equal(offer.terms.claims.destination.value, DESTINATION);
    assert.equal(Date.parse(offer.terms.expires_at) - Date.parse(offer.terms.checked_at), 90 * DAY);
    assert.equal(Object.hasOwn(offer.terms.claims, 'audience'), false);
    for (const value of Object.values(offer.terms.claims)) {
      assert.notEqual(value, 0);
      assert.notEqual(value, false);
      if (value && typeof value === 'object') {
        assert.notEqual(value.value, 0);
        assert.notEqual(value.value, false);
      }
    }
    const visible = async () => {
      const discovered = await reader.send(msg(reader, { type: 'discover', body: { want: 'offers' } }));
      assert.equal(discovered.status, 200, discovered.text);
      const suggested = await OrpClient.suggest(host.origin, { text: 'notes for founders' });
      assert.equal(suggested.status, 200, suggested.text);
      const inDiscover = discovered.message.body.items.some((item) => item.offer?.id === offer.id);
      const inSuggest = (suggested.message.items || []).some((item) => item.offer?.id === offer.id);
      return inDiscover && inSuggest;
    };
    assert.equal(await visible(), true);
    host.advanceBusiness(91 * DAY);
    assert.equal(await visible(), false);
    const confirmed = await maker.send(msg(maker, { type: 'offer', body: { id: offer.id, confirm: true } }));
    assert.equal(confirmed.status, 200, confirmed.text);
    assert.equal(confirmed.message.body.terms.claims.commission, 'unknown');
    assert.equal(Date.parse(confirmed.message.body.terms.expires_at) - Date.parse(confirmed.message.body.terms.checked_at), 90 * DAY);
    assert.equal(await visible(), true);
  });
});

test('ORP04-N2-UNCHANGED', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const acceptor = await party(host, AUTO);
    const { accepted } = await directDeal(maker, acceptor, affiliateTerms());
    assert.equal(accepted.status, 200, accepted.text);
    assert.equal(accepted.message.orp, '0.3');
    assert.equal(accepted.message.body.state, 'active');
    assert.equal(accepted.message.body.parties, undefined);
    assert.equal(accepted.message.body.state === 'pending', false);
    const fixture = JSON.parse(readFileSync(join(here, '../../schema/examples/messages/08-deal-active.json'), 'utf8'));
    const verdict = check('message', fixture);
    assert.equal(verdict.ok, true, verdict.field);
    assert.equal(fixture.orp, '0.3');
    assert.equal(fixture.body.parties, undefined);
  });
});
