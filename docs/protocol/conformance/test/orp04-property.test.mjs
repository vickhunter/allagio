import { test } from 'node:test';
import assert from 'node:assert/strict';
import { foldSigns, validateParties } from '../../sdk/src/index.mjs';
import { AUTO, codeOf, party, withHost } from './helpers.mjs';

function mulberry32(seed) {
  let state = seed >>> 0;
  return () => {
    state = (state + 0x6D2B79F5) | 0;
    let next = Math.imul(state ^ (state >>> 15), 1 | state);
    next = (next + Math.imul(next ^ (next >>> 7), 61 | next)) ^ next;
    return ((next ^ (next >>> 14)) >>> 0) / 4294967296;
  };
}

function split100(count, random) {
  const cuts = [0, 100];
  for (let index = 0; index < count - 1; index += 1) cuts.push(Math.floor(random() * 101));
  cuts.sort((left, right) => left - right);
  const parts = [];
  for (let index = 1; index < cuts.length; index += 1) parts.push(cuts[index] - cuts[index - 1]);
  return parts;
}

function shuffle(items, random) {
  const copy = [...items];
  for (let index = copy.length - 1; index > 0; index -= 1) {
    const swap = Math.floor(random() * (index + 1));
    [copy[index], copy[swap]] = [copy[swap], copy[index]];
  }
  return copy;
}

function partyId(index) {
  return `orp:ref.example/p_prop${String(index).padStart(2, '0')}`;
}

test('ORP04-PROPERTY', async (t) => {
  const started = performance.now();
  const random = mulberry32(0x04d101);
  const trials = 240;
  for (let trial = 0; trial < trials; trial += 1) {
    const count = 2 + Math.floor(random() * 15);
    const parts = split100(count, random);
    const parties = parts.map((percent, index) => ({
      id: partyId(index),
      role: index === 0 ? 'offerer' : 'member',
      required: true,
      terms: { percent },
    }));
    const valid = validateParties(parties);
    assert.equal(valid.ok, true, `${trial} ${parts.join('+')} ${valid.message || ''}`);
    const bumped = parts.map((percent, index) => (index === parts.findIndex((item) => item < 100) ? percent + 1 : percent));
    const rejected = validateParties(parties.map((seat, index) => ({ ...seat, terms: { percent: bumped[index] } })));
    assert.equal(rejected.ok, false, `${trial} off-by-one ${bumped.join('+')} was accepted`);
    const delta = (Math.floor(random() * 30) + 1) * (random() < 0.5 ? -1 : 1);
    const shifted = parts.map((percent, index) => (index === 0 ? percent + delta : percent));
    const off = validateParties(parties.map((seat, index) => ({ ...seat, terms: { percent: shifted[index] } })));
    assert.equal(off.ok, false, `${trial} delta ${delta} was accepted`);
    const ids = parties.map((seat) => seat.id);
    const left = foldSigns(parties, shuffle(ids, random));
    const right = foldSigns(parties, shuffle(ids, random));
    assert.deepEqual(left, right);
    assert.equal(left.state, 'active');
    assert.deepEqual(left.signed, [...ids].sort());
  }
  const cap = eurParties();
  assert.equal(validateParties(cap).ok, true, 'mixed percent and fixed with a cap');
  cap[0].terms.percent = 101;
  assert.equal(validateParties(cap).ok, false);

  await withHost({}, async (host) => {
    const clients = [];
    for (let index = 0; index < 16; index += 1) clients.push(await party(host, AUTO));
    const hostRandom = mulberry32(0x0416);
    for (let trial = 0; trial < 20; trial += 1) {
      const count = 2 + Math.floor(hostRandom() * 15);
      const seated = clients.slice(0, count);
      const parts = split100(count, hostRandom);
      const terms = {
        title: `Random notes deal ${trial}`,
        kind: 'partnership',
        obligations: [{ id: 'share', by: 'offerer', action: 'share' }],
      };
      const parties = seated.map((client, index) => ({
        id: client.party,
        role: index === 0 ? 'offerer' : 'member',
        required: true,
        terms: { percent: parts[index] },
      }));
      const opened = await seated[0].send(seated[0].msg({
        orp: '0.4',
        type: 'offer',
        body: { terms, parties },
      }));
      assert.equal(opened.status, 200, opened.text);
      let snapshot = opened.message;
      for (const client of shuffle(seated.slice(1), hostRandom)) {
        const accepted = await client.send(client.msg({
          orp: '0.4',
          type: 'accept',
          deal: snapshot.deal,
          body: { terms_hash: snapshot.body.terms_hash },
        }));
        assert.equal(accepted.status, 200, accepted.text);
        snapshot = accepted.message;
      }
      assert.equal(snapshot.body.state, 'active');
      assert.deepEqual(
        snapshot.body.parties.filter((seat) => seat.signed).map((seat) => seat.id).sort(),
        seated.map((client) => client.party).sort(),
      );
    }
    const bad = await clients[0].send(clients[0].msg({
      orp: '0.4',
      type: 'offer',
      body: {
        terms: {
          title: 'Random notes deal bad',
          kind: 'partnership',
          obligations: [{ id: 'share', by: 'offerer', action: 'share' }],
        },
        parties: [
          { id: clients[0].party, role: 'offerer', required: true, terms: { percent: partsPlus(50) } },
          { id: clients[1].party, role: 'member', required: true, terms: { percent: 51 } },
        ],
      },
    }));
    assert.equal(bad.status, 400);
    assert.equal(codeOf(bad), 'invalid_message');
  });

  const duration = performance.now() - started;
  t.diagnostic(`property trials=240 host_deals=20 duration_ms=${duration.toFixed(1)} bug=none`);
  assert.ok(duration < 8000, `property test took ${duration.toFixed(1)}ms`);
});

function eurParties() {
  return [
    { id: partyId(0), role: 'offerer', required: true, terms: { percent: 40, cap: { amount_minor: 1000, currency: 'EUR' } } },
    { id: partyId(1), role: 'member', required: true, terms: { amount: { amount_minor: 500, currency: 'EUR' } } },
  ];
}

function partsPlus(value) {
  return value + 1;
}
