import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { run } from '../lib/main.mjs';

test('integration against this branch API', async (t) => {
  if (process.env.OFFER_RAILS_INTEGRATION !== '1') {
    t.skip('set OFFER_RAILS_INTEGRATION=1 to run against the Go API');
    return;
  }
  const base = process.env.OFFER_RAILS_API_URL;
  assert.ok(base, 'OFFER_RAILS_API_URL is required');
  const health = await fetch(`${base}/v1/health`);
  assert.equal(health.status, 200, 'Go API health');

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'offer-rails-int-'));
  const config = path.join(dir, 'config.json');
  const root = path.join(dir, 'proj');
  fs.mkdirSync(root);
  const secret = 'sk_live_integration_secret_value';
  fs.writeFileSync(path.join(root, 'package.json'), JSON.stringify({ name: 'Integration Widget', description: 'Integration scan summary.' }));
  fs.writeFileSync(path.join(root, 'pay.js'), `stripe webhook\nfunc hidden() {}\n${secret}\n`);
  const calls = [];
  const stdout = [];
  const io = {
    env: { ...process.env, OFFER_RAILS_CONFIG: config, OFFER_RAILS_YES: '1' },
    cwd: root,
    stdout(chunk) { stdout.push(chunk); },
    stderr(chunk) { process.stderr.write(chunk); },
    sleep(ms) { return new Promise((resolve) => setTimeout(resolve, ms)); },
    readLine: async () => 'n',
    fetch: async (url, opts = {}) => {
      calls.push({ url, method: opts.method || 'GET', body: opts.body });
      return fetch(url, opts);
    },
  };
  const email = `agent-${Date.now()}@example.com`;
  const login = await run(['login', '--agent', '--role', 'maker', '--name', 'Integration', '--email', email], io);
  assert.equal(login.exitCode, 0, stdout.join(''));
  assert.match(stdout.join(''), /An agent alone created the account\./);

  stdout.length = 0;
  const scanned = await run(['scan', '.'], io);
  assert.equal(scanned.exitCode, 0, stdout.join(''));
  const upload = calls.find((call) => call.method === 'POST' && call.url.endsWith('/v1/listing-cards'));
  assert.ok(upload, 'scan did not post a listing card');
  assert.equal(upload.body.includes(secret), false);
  assert.equal(upload.body.includes('func hidden'), false);
  const payload = JSON.parse(upload.body);
  assert.deepEqual(Object.keys(payload), ['card', 'signals', 'score', 'payments_check', 'offer_link_health']);
  assert.match(stdout.join(''), /listing_card: /);

  stdout.length = 0;
  const digest = await run(['digest'], io);
  assert.equal(digest.exitCode, 0, stdout.join(''));

  const before = calls.length;
  stdout.length = 0;
  const restored = await run(['restore'], io);
  assert.equal(restored.exitCode, 0);
  assert.match(stdout.join('').toLowerCase(), /sign in again/);
  assert.equal(calls.length, before);
});
