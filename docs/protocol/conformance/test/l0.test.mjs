import { test } from 'node:test';
import assert from 'node:assert/strict';
import { check, digest, generateEd25519, publicJwk, signEnvelope } from '../../sdk/src/index.mjs';
import {
  AUTO, OrpClient, ReferenceHost, affiliateTerms, codeOf, inbox, party, tempStore, withHost,
} from './helpers.mjs';

test('L0-SCHEMA', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const other = await party(host, AUTO);

    const missing = await maker.send({ orp: '0.3', type: 'hello', id: 'm_missing_ts', from: maker.party, body: { versions: ['0.3'] } });
    assert.equal(missing.status, 400);
    assert.equal(codeOf(missing), 'invalid_message');

    const hello = maker.msg({ type: 'hello', body: { versions: ['0.3'] } });
    hello.orp = '0.4';
    const version = await maker.send(hello);
    assert.equal(version.status, 400);
    assert.equal(codeOf(version), 'unsupported_version');
    assert.deepEqual(version.message.body.supported, ['0.3']);

    const critical = await maker.send(maker.msg({
      type: 'hello',
      critical: ['com.example.extra'],
      body: { versions: ['0.3'] },
    }));
    assert.equal(critical.status, 400);
    assert.equal(codeOf(critical), 'unsupported_extension');

    const foreign = await maker.send(maker.msg({ type: 'hello', from: other.party, body: { versions: ['0.3'] } }));
    assert.equal(foreign.status, 403);
    assert.equal(codeOf(foreign), 'forbidden');

    const hostOnly = await maker.send(maker.msg({
      type: 'results',
      re: 'm_re_0001',
      body: { items: [] },
    }));
    assert.equal(hostOnly.status, 403);
    assert.equal(codeOf(hostOnly), 'forbidden');

    const dirty = await maker.send(maker.msg({
      type: 'offer',
      body: { terms: affiliateTerms({ title: 'Notes\u200Bapp' }) },
    }));
    assert.equal(dirty.status, 400);
    assert.equal(codeOf(dirty), 'invalid_message');
    assert.match(dirty.message.body.field, /title/);

    const markup = await maker.send(maker.msg({
      type: 'offer',
      body: { terms: affiliateTerms({ title: 'Notes <b>app</b>' }) },
    }));
    assert.equal(markup.status, 400);
    assert.equal(codeOf(markup), 'invalid_message');

    const secret = generateEd25519('party-key');
    const keyed = await OrpClient.join(host.origin, { key: publicJwk(secret) });
    const unsigned = await keyed.send(keyed.msg({ type: 'offer', body: { terms: affiliateTerms() } }));
    assert.equal(unsigned.status, 401);
    assert.equal(codeOf(unsigned), 'unauthorized');
    const forged = keyed.msg({ type: 'offer', body: { terms: affiliateTerms() }, sig: 'aaaa..bbbb' });
    const badSig = await keyed.send(forged);
    assert.equal(badSig.status, 401);
    assert.equal(codeOf(badSig), 'bad_signature');
    const signedHello = signEnvelope(keyed.msg({ type: 'hello', body: { versions: ['0.3'] } }), secret);
    const okHello = await keyed.send(signedHello);
    assert.equal(okHello.status, 200);
    assert.equal(okHello.message.type, 'hello');

    assert.ok(host.outbound.length > 0);
    for (const message of host.outbound) {
      const verdict = check('message', message);
      assert.equal(verdict.ok, true, `${message.type} invalid at ${verdict.field}`);
    }
  });
});

test('L0-HASH', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const pub = await party(host, AUTO);
    const terms = affiliateTerms();
    const mismatch = await maker.send(maker.msg({
      type: 'offer',
      body: { terms, terms_hash: `sha256-${'b'.repeat(43)}` },
    }));
    assert.equal(mismatch.status, 409);
    assert.equal(codeOf(mismatch), 'terms_mismatch');
    assert.equal(host.offers.size, 0);

    const listed = await maker.send(maker.msg({
      type: 'offer',
      body: { terms, terms_hash: digest(terms) },
    }));
    assert.equal(listed.status, 200);
    assert.equal(listed.message.body.terms_hash, digest(terms));
    assert.equal(host.offers.size, 1);

    const offered = await maker.send(maker.msg({
      type: 'offer',
      to: pub.party,
      body: { terms },
    }));
    assert.equal(offered.status, 200);
    const dealId = offered.message.deal;
    const broken = await maker.send(maker.msg({
      type: 'counter',
      deal: dealId,
      prev: `sha256-${'a'.repeat(43)}`,
      body: { terms: affiliateTerms({ title: 'Notes counter one' }) },
    }));
    assert.equal(broken.status, 409);
    assert.equal(codeOf(broken), 'chain_broken');
    assert.equal(host.deals.get(dealId).round, 1);

    const first = maker.msg({
      type: 'counter',
      deal: dealId,
      body: { terms: affiliateTerms({ title: 'Notes counter two' }) },
    });
    const opened = await maker.send(first);
    assert.equal(opened.status, 200, opened.text);
    assert.equal(opened.message.body.round, 2);

    const linked = maker.msg({
      type: 'counter',
      deal: dealId,
      prev: digest(first),
      body: { terms: affiliateTerms({ title: 'Notes counter three' }) },
    });
    const good = await maker.send(linked);
    assert.equal(good.status, 200, good.text);
    assert.equal(good.message.body.round, 3);

    const omitted = await maker.send(maker.msg({
      type: 'counter',
      deal: dealId,
      body: { terms: affiliateTerms({ title: 'Notes counter four' }) },
    }));
    assert.equal(omitted.status, 409);
    assert.equal(codeOf(omitted), 'chain_broken');
    assert.equal(host.deals.get(dealId).round, 3);
  });
});

test('L0-IDEMPOTENT', async () => {
  const storePath = tempStore();
  const host = await ReferenceHost.start({ storePath });
  host.registerPage('https://notes.example/pricing?plan=pro');
  try {
    const maker = await party(host, AUTO);
    const message = maker.msg({ type: 'offer', body: { terms: affiliateTerms() } });
    const first = await maker.send(message);
    assert.equal(first.status, 200);
    assert.equal(host.offers.size, 1);
    await host.close();

    const restarted = await ReferenceHost.start({ storePath });
    try {
      maker.origin = restarted.origin;
      const second = await maker.send(message);
      assert.equal(second.status, 200);
      assert.deepEqual(second.message, first.message);
      assert.equal(restarted.offers.size, 1);
    } finally {
      await restarted.close();
    }
  } finally {
    if (host.server) await host.close();
  }
});

test('L0-REPLAY', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const message = maker.msg({ type: 'offer', body: { terms: affiliateTerms() } });
    const first = await maker.send(message);
    assert.equal(first.status, 200);
    const replay = await maker.send({
      ...message,
      body: { terms: affiliateTerms({ title: 'A different notes offer' }) },
    });
    assert.equal(replay.status, 409);
    assert.equal(codeOf(replay), 'replay');
    assert.equal(host.offers.size, 1);
    assert.equal([...host.offers.values()][0].terms.title, affiliateTerms().title);
  });
});

test('L0-STALE', async () => {
  await withHost({}, async (host) => {
    const maker = await party(host, AUTO);
    const stalePast = maker.msg({ type: 'hello', body: { versions: ['0.3'] } });
    stalePast.ts = new Date(Date.now() - 16 * 60 * 1000).toISOString();
    const past = await maker.send(stalePast);
    assert.equal(past.status, 400);
    assert.equal(codeOf(past), 'stale');

    const fresh = await maker.send({ ...stalePast, ts: new Date().toISOString() });
    assert.equal(fresh.status, 200, fresh.text);
    assert.equal(fresh.message.type, 'hello');

    const staleFuture = maker.msg({ type: 'hello', body: { versions: ['0.3'] } });
    staleFuture.ts = new Date(Date.now() + 16 * 60 * 1000).toISOString();
    const future = await maker.send(staleFuture);
    assert.equal(future.status, 400);
    assert.equal(codeOf(future), 'stale');

    const recent = maker.msg({ type: 'hello', body: { versions: ['0.3'] } });
    recent.ts = new Date(Date.now() - 60 * 1000).toISOString();
    const ok = await maker.send(recent);
    assert.equal(ok.status, 200);
    const messages = await inbox(maker);
    assert.ok(messages.some((message) => message.type === 'hello' && message.re === recent.id));
  });
});
