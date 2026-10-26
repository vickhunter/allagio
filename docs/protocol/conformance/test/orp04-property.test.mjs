import { test } from 'node:test';
import assert from 'node:assert/strict';
import { foldSigns, signatureState, validateParties } from '../../sdk/src/index.mjs';
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
      assert.equal(opened.message.body.state, 'pending');
      let snapshot = opened.message;
      const order = shuffle(seated.slice(1), hostRandom);
      for (let index = 0; index < order.length; index += 1) {
        const client = order[index];
        const accepted = await client.send(client.msg({
          orp: '0.4',
          type: 'accept',
          deal: snapshot.deal,
          body: { terms_hash: snapshot.body.terms_hash },
        }));
        assert.equal(accepted.status, 200, accepted.text);
        snapshot = accepted.message;
        const lastRequired = index === order.length - 1;
        assert.equal(snapshot.body.state, lastRequired ? 'active' : 'pending', `${trial} signed ${index + 1} of ${order.length}`);
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

    const fee = { amount_minor: 1000, currency: 'EUR' };
    for (let trial = 0; trial < 6; trial += 1) {
      const offerer = clients[0];
      const acceptor = clients[1];
      const facilitator = clients[2];
      const service = clients[3];
      const opened = await offerer.send(offerer.msg({
        orp: '0.4',
        type: 'offer',
        body: {
          terms: {
            title: `Random seat leave ${trial}`,
            kind: 'partnership',
            obligations: [{ id: 'share', by: 'offerer', action: 'share' }],
          },
          parties: [
            { id: offerer.party, role: 'offerer', required: true, terms: { amount: { amount_minor: 0, currency: 'EUR' } } },
            { id: acceptor.party, role: 'acceptor', required: true, terms: { amount: fee } },
            { id: facilitator.party, role: 'facilitator', required: true, terms: { amount: fee } },
            { id: service.party, role: 'service', required: false, terms: { amount: { amount_minor: 20, currency: 'EUR' } } },
          ],
        },
      }));
      assert.equal(opened.status, 200, opened.text);
      const seats = opened.message.body.parties.map((seat) => ({ ...seat, signed: seat.signed }));
      let vacancies = (opened.message.body.vacancies || []).map((role, index) => ({ id: `vac_open_${trial}_${index}`, role }));
      let snapshot = opened.message;
      const byId = new Map(clients.map((client) => [client.party, client]));
      let spare = 4;
      const signers = shuffle([acceptor, facilitator], hostRandom);
      for (let index = 0; index < signers.length; index += 1) {
        const client = signers[index];
        const accepted = await client.send(client.msg({
          orp: '0.4',
          type: 'accept',
          deal: snapshot.deal,
          body: { terms_hash: snapshot.body.terms_hash },
        }));
        assert.equal(accepted.status, 200, accepted.text);
        const seat = seats.find((item) => item.id === client.party);
        seat.signed = true;
        vacancies = clearVacancy(seat, vacancies);
        snapshot = accepted.message;
        assert.equal(snapshot.body.state, index === signers.length - 1 ? 'active' : 'pending');
        assert.deepEqual(snapshot.body.vacancies || [], vacancyRoles(vacancies));
      }
      for (let step = 0; step < 4; step += 1) {
        const roll = hostRandom();
        if (roll < 0.34) {
          const seat = seats[Math.floor(hostRandom() * seats.length)];
          const remaining = seats.filter((item) => item.id !== seat.id);
          const allowed = seats.length > 2 && validateParties(remaining).ok;
          const sent = await byId.get(seat.id).send(byId.get(seat.id).msg({
            orp: '0.4',
            type: 'leave',
            deal: snapshot.deal,
            body: {},
          }));
          if (!allowed) {
            assert.equal(sent.status, 400, sent.text);
            assert.equal(codeOf(sent), 'invalid_message');
            assert.equal(host.deals.get(snapshot.deal).parties.length, seats.length);
            continue;
          }
          assert.equal(sent.status, 200, sent.text);
          const index = seats.findIndex((item) => item.id === seat.id);
          seats.splice(index, 1);
          if (seat.required) vacancies.push({ id: `vac_${trial}_${seat.id}`, role: seat.role });
          snapshot = sent.message;
        } else if (roll < 0.67 && spare < clients.length) {
          const client = clients[spare];
          spare += 1;
          const needsFacilitator = vacancies.some((item) => item.role === 'facilitator') && !seats.some((seat) => seat.role === 'facilitator' && seat.required);
          const seat = needsFacilitator
            ? { id: client.party, role: 'facilitator', required: true, terms: { amount: fee } }
            : { id: client.party, role: 'member', required: false, terms: { amount: { amount_minor: 1, currency: 'EUR' } } };
          const sender = byId.get(seats[0].id);
          const sent = await sender.send(sender.msg({
            orp: '0.4',
            type: 'seat',
            deal: snapshot.deal,
            body: { seat },
          }));
          const next = [...seats, { ...seat, signed: false }];
          if (!validateParties(next).ok) {
            assert.equal(sent.status, 400, sent.text);
            continue;
          }
          assert.equal(sent.status, 200, sent.text);
          const record = { ...seat, signed: false };
          if (record.required) {
            const taken = new Set(seats.map((item) => item.fills).filter(Boolean));
            const open = vacancies.find((item) => item.role === record.role && !taken.has(item.id));
            if (open) record.fills = open.id;
          }
          seats.push(record);
          byId.set(client.party, client);
          snapshot = sent.message;
        } else {
          const unsigned = seats.filter((seat) => !seat.signed);
          if (!unsigned.length) continue;
          const seat = unsigned[Math.floor(hostRandom() * unsigned.length)];
          const sent = await byId.get(seat.id).send(byId.get(seat.id).msg({
            orp: '0.4',
            type: 'accept',
            deal: snapshot.deal,
            body: { terms_hash: snapshot.body.terms_hash },
          }));
          assert.equal(sent.status, 200, sent.text);
          seat.signed = true;
          vacancies = clearVacancy(seat, vacancies);
          snapshot = sent.message;
        }
        const expected = signatureState(seats, seats.filter((seat) => seat.signed).map((seat) => seat.id), vacancies);
        assert.equal(snapshot.body.state, expected.state, `${trial}.${step}`);
        assert.deepEqual([...(snapshot.body.vacancies || [])].sort(), vacancyRoles(vacancies).sort());
        assert.deepEqual(
          snapshot.body.parties.filter((seat) => seat.signed).map((seat) => seat.id).sort(),
          seats.filter((seat) => seat.signed).map((seat) => seat.id).sort(),
        );
        const openIds = new Set(vacancies.map((item) => item.id));
        for (const id of seats.filter((item) => item.signed && item.fills).map((item) => item.fills)) {
          assert.equal(openIds.has(id), false);
        }
      }
    }

    const percentOpened = await clients[0].send(clients[0].msg({
      orp: '0.4',
      type: 'offer',
      body: {
        terms: {
          title: 'Random percent leave',
          kind: 'partnership',
          obligations: [{ id: 'share', by: 'offerer', action: 'share' }],
        },
        parties: [
          { id: clients[0].party, role: 'offerer', required: true, terms: { percent: 50 } },
          { id: clients[1].party, role: 'curator', required: true, terms: { percent: 30 } },
          { id: clients[2].party, role: 'service', required: false, terms: { percent: 20 } },
        ],
      },
    }));
    assert.equal(percentOpened.status, 200, percentOpened.text);
    const percentActive = await clients[1].send(clients[1].msg({
      orp: '0.4',
      type: 'accept',
      deal: percentOpened.message.deal,
      body: { terms_hash: percentOpened.message.body.terms_hash },
    }));
    assert.equal(percentActive.message.body.state, 'active');
    const percentLeft = await clients[2].send(clients[2].msg({
      orp: '0.4',
      type: 'leave',
      deal: percentActive.message.deal,
      body: {},
    }));
    assert.equal(percentLeft.status, 400);
    assert.equal(codeOf(percentLeft), 'invalid_message');
    const percentRead = await clients[0].deal(percentActive.message.deal);
    assert.equal(percentRead.message.body.state, 'active');
    assert.equal(percentRead.message.body.parties.length, 3);
  });

  const duration = performance.now() - started;
  t.diagnostic(`property trials=240 host_deals=20 vacancy_sequences=6 duration_ms=${duration.toFixed(1)} bug=none`);
  assert.ok(duration < 8000, `property test took ${duration.toFixed(1)}ms`);
});

function clearVacancy(seat, vacancies) {
  if (!seat.fills) return vacancies;
  return vacancies.filter((item) => item.id !== seat.fills);
}

function vacancyRoles(vacancies) {
  return vacancies.map((item) => item.role);
}

function eurParties() {
  return [
    { id: partyId(0), role: 'offerer', required: true, terms: { percent: 40, cap: { amount_minor: 1000, currency: 'EUR' } } },
    { id: partyId(1), role: 'member', required: true, terms: { amount: { amount_minor: 500, currency: 'EUR' } } },
  ];
}

function partsPlus(value) {
  return value + 1;
}
