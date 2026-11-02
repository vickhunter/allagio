import assert from 'node:assert/strict';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { apiBase } from '../lib/api.mjs';
import { defaultPublicBaseURL } from '../lib/public-base.mjs';
import { configPath, saveConfig } from '../lib/config.mjs';
import { handleMcpMessage, toolDefs } from '../lib/mcp.mjs';
import { run } from '../lib/main.mjs';

function tempIO(extra = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'offer-rails-'));
  const stdout = [];
  const stderr = [];
  const calls = [];
  const io = {
    env: { ...extra.env },
    cwd: extra.cwd || dir,
    stdout(chunk) { stdout.push(chunk); },
    stderr(chunk) { stderr.push(chunk); },
    sleep() { return Promise.resolve(); },
    readLine: extra.readLine || (async () => 'n'),
    fetch: extra.fetch || (async (url, opts = {}) => {
      calls.push({ url, method: opts.method || 'GET', body: opts.body, headers: opts.headers });
      throw new Error(`unexpected fetch ${opts.method || 'GET'} ${url}`);
    }),
    dir,
  };
  io.env.OFFER_RAILS_CONFIG = extra.config || path.join(dir, 'config.json');
  return { io, stdout: () => stdout.join(''), stderr: () => stderr.join(''), calls, dir };
}

function jsonResponse(status, body) {
  return { status, text: async () => JSON.stringify(body) };
}

test('https is required unless the host is localhost', () => {
  assert.equal(apiBase({ ALLAGIO_URL: defaultPublicBaseURL }), defaultPublicBaseURL);
  assert.equal(apiBase({}), defaultPublicBaseURL);
  assert.equal(apiBase({ OFFER_RAILS_URL: 'https://legacy.example' }), 'https://legacy.example');
  assert.equal(apiBase({ ALLAGIO_URL: 'https://allagio.example', OFFER_RAILS_URL: 'https://legacy.example', PUBLIC_BASE_URL: 'https://ignored.example' }), 'https://allagio.example');
  assert.equal(apiBase({ OFFER_RAILS_API_URL: 'http://127.0.0.1:18081' }), 'http://127.0.0.1:18081');
  assert.equal(apiBase({ OFFER_RAILS_API_URL: 'http://localhost:8080/' }), 'http://localhost:8080');
  assert.equal(apiBase({ OFFER_RAILS_API_URL: 'http://[::1]:8080' }), 'http://[::1]:8080');
  assert.throws(() => apiBase({ OFFER_RAILS_API_URL: 'http://example.com' }), /https/);
});

test('OFFER_RAILS_API_URL is used only after ALLAGIO_URL and OFFER_RAILS_URL', () => {
  assert.equal(apiBase({ OFFER_RAILS_API_URL: 'https://api.example' }), 'https://api.example');
  assert.equal(apiBase({ OFFER_RAILS_URL: 'https://legacy.example', OFFER_RAILS_API_URL: 'https://api.example' }), 'https://legacy.example');
  assert.equal(apiBase({ ALLAGIO_URL: 'https://allagio.example', OFFER_RAILS_URL: 'https://legacy.example', OFFER_RAILS_API_URL: 'https://api.example', PUBLIC_BASE_URL: 'https://ignored.example' }), 'https://allagio.example');
});

test('login stores the project key at mode 0600 and logs a person approval', async () => {
  const { io, stdout } = tempIO({
    fetch: async (url, opts) => {
      const pathname = new URL(url).pathname;
      if (pathname === '/v1/device/code') {
        return jsonResponse(200, {
          device_code: 'device-1', user_code: 'ABCD-EFGH', verification_uri: 'http://127.0.0.1:9/device', interval: 0, expires_in: 300,
        });
      }
      if (pathname === '/v1/device/token') {
        const body = JSON.parse(opts.body);
        assert.equal(body.device_code, 'device-1');
        assert.equal(body.grant_type, 'urn:ietf:params:oauth:grant-type:device_code');
        return jsonResponse(200, {
          access_token: 'or_secret_project_key', token_type: 'Bearer', key_header: 'X-Maker-Key', scope: 'offer:11111111-1111-4111-8111-111111111111', expires_in: 28800,
        });
      }
      throw new Error(pathname);
    },
  });
  const result = await run(['login'], io);
  assert.equal(result.exitCode, 0);
  const text = stdout();
  assert.match(text, /user_code: ABCD-EFGH/);
  assert.match(text, /A person approved the device code\./);
  assert.equal(text.includes('or_secret_project_key'), false);
  const saved = JSON.parse(fs.readFileSync(configPath(io), 'utf8'));
  assert.equal(saved.access_token, 'or_secret_project_key');
  assert.equal(saved.key_header, 'X-Maker-Key');
  assert.equal(saved.account_kind, 'person');
  assert.equal(saved.human_login, true);
  assert.equal(fs.statSync(configPath(io)).mode & 0o777, 0o600);
  assert.equal(fs.statSync(path.dirname(configPath(io))).mode & 0o777, 0o700);
});

test('login --agent logs that an agent alone created the account', async () => {
  const { io, stdout } = tempIO({
    fetch: async (url, opts) => {
      assert.equal(new URL(url).pathname, '/v1/agent-accounts');
      const body = JSON.parse(opts.body);
      assert.equal(body.source, 'cli');
      assert.equal(body.role, 'maker');
      return jsonResponse(201, {
        api_key: 'or_agent_secret', key_header: 'X-Maker-Key', maker_id: '22222222-2222-4222-8222-222222222222', agent_only: true, human_login: false,
      });
    },
  });
  const result = await run(['login', '--agent', '--role', 'maker', '--name', 'Agent', '--email', 'agent@example.com'], io);
  assert.equal(result.exitCode, 0);
  assert.match(stdout(), /An agent alone created the account\./);
  assert.equal(stdout().includes('or_agent_secret'), false);
  const saved = JSON.parse(fs.readFileSync(configPath(io), 'utf8'));
  assert.equal(saved.account_kind, 'agent');
  assert.equal(saved.human_login, false);
  assert.equal(fs.statSync(configPath(io)).mode & 0o777, 0o600);
});

test('scan sends only the card, signals, and score', async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'offer-rails-src-'));
  fs.writeFileSync(path.join(root, 'package.json'), JSON.stringify({
    name: 'Paste Widget',
    description: 'A small tool for the fixture.',
    privateNote: 'sk_live_from_package',
  }));
  fs.writeFileSync(path.join(root, 'README.md'), '# Ignored heading\n\nPricing is $12 and this has a feature list.\nhttps://example.com/docs\nAffiliate disclosure.\n');
  fs.mkdirSync(path.join(root, 'src'));
  fs.writeFileSync(path.join(root, 'src', 'pay.js'), 'const stripe = "webhook";\nfunc secretThing() {}\nconst key = "sk_live_abcdefghijklmnopqrst";\n');
  fs.writeFileSync(path.join(root, '.env'), 'SECRET=hunter2\n');
  fs.writeFileSync(path.join(root, '.env.local'), 'ALSO=hidden\n');
  fs.writeFileSync(path.join(root, 'id_rsa'), '-----BEGIN PRIVATE KEY-----\n');
  fs.writeFileSync(path.join(root, 'cert.pem'), 'pem-bytes\n');
  fs.writeFileSync(path.join(root, '.gitignore'), 'ignored.txt\n*.secret\n');
  fs.writeFileSync(path.join(root, 'ignored.txt'), 'UNIQUE_IGNORED_TOKEN\n');
  fs.writeFileSync(path.join(root, 'notes.secret'), 'UNIQUE_SECRET_FILE\n');
  const bodies = [];
  const { io, stdout } = tempIO({
    cwd: root,
    readLine: async () => 'y',
    fetch: async (url, opts) => {
      assert.equal(opts.method, 'POST');
      assert.equal(new URL(url).pathname, '/v1/listing-cards');
      bodies.push(opts.body);
      return jsonResponse(201, { id: '33333333-3333-4333-8333-333333333333', score: 100 });
    },
  });
  fs.writeFileSync(configPath(io), `${JSON.stringify({ access_token: 'or_test', key_header: 'X-Maker-Key', scope: 'offer:11111111-1111-4111-8111-111111111111' })}\n`, { mode: 0o600 });
  const result = await run(['scan', '.'], io);
  assert.equal(result.exitCode, 0);
  const text = stdout();
  assert.match(text, /--- payload ---/);
  assert.match(text, /Send this listing card\? \[y\/N\]/);
  const payload = JSON.parse(text.split('--- payload ---\n')[1].split('\n--- end ---')[0]);
  const allowed = ['card', 'signals', 'score', 'payments_check', 'offer_link_health'];
  assert.deepEqual(Object.keys(payload), allowed);
  assert.deepEqual(Object.keys(payload.card), ['name', 'summary', 'category', 'features', 'pricing_summary']);
  for (const signal of payload.signals) assert.deepEqual(Object.keys(signal).sort(), ['name', 'pass']);
  assert.equal(payload.card.name, 'Paste Widget');
  assert.equal(payload.card.summary, 'A small tool for the fixture.');
  assert.equal(payload.payments_check, 'pass');
  assert.equal(payload.offer_link_health, 'unknown');
  const sent = bodies[0];
  assert.equal(sent, JSON.stringify(payload));
  for (const secret of ['sk_live_', 'hunter2', 'UNIQUE_IGNORED_TOKEN', 'UNIQUE_SECRET_FILE', 'func secretThing', 'privateNote', 'BEGIN PRIVATE']) {
    assert.equal(sent.includes(secret), false, secret);
    assert.equal(text.includes(secret), false, secret);
  }
});

test('scan drops a summary that looks like an env line, a private key, or a token', async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'offer-rails-secret-summary-'));
  fs.writeFileSync(path.join(root, 'package.json'), JSON.stringify({
    name: 'Safe Widget',
    description: 'SECRET=hunter2\n-----BEGIN OPENSSH PRIVATE KEY-----\nxoxb-1234567890',
  }));
  const { io, stdout } = tempIO({
    cwd: root,
    readLine: async () => 'y',
    fetch: async () => jsonResponse(201, { id: '33333333-3333-4333-8333-333333333333', score: 0 }),
  });
  fs.writeFileSync(configPath(io), `${JSON.stringify({ access_token: 'or_test', key_header: 'X-Maker-Key', scope: 'offer:11111111-1111-4111-8111-111111111111' })}\n`, { mode: 0o600 });
  const result = await run(['scan', '.'], io);
  assert.equal(result.exitCode, 0);
  const text = stdout();
  const payload = JSON.parse(text.split('--- payload ---\n')[1].split('\n--- end ---')[0]);
  assert.equal(payload.card.summary, 'Local readiness scan. No page text is included.');
  for (const secret of ['hunter2', 'BEGIN OPENSSH', 'xoxb-']) {
    assert.equal(text.includes(secret), false, secret);
  }
});

test('an empty answer does not upload the scan', async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'offer-rails-empty-'));
  fs.writeFileSync(path.join(root, 'README.md'), '# Empty\n');
  const { io, stdout, calls } = tempIO({ cwd: root, readLine: async () => '' });
  const result = await run(['scan', '.'], io);
  assert.equal(result.exitCode, 0);
  assert.match(stdout(), /Not sent\./);
  assert.equal(calls.length, 0);
});

test('a url scan does not upload the page body', async () => {
  const html = '<html><script>SECRET_IN_SCRIPT sk_live_page</script><style>.x{color:red}</style><title>Safe Title</title><body>stripe webhook pricing feature https://example.com disclosure</body></html>';
  const server = http.createServer((req, res) => {
    assert.equal(req.url, '/only');
    res.writeHead(200, { 'content-type': 'text/html' });
    res.end(html);
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const port = server.address().port;
  const { io, stdout } = tempIO({
    readLine: async () => 'n',
    fetch: globalThis.fetch.bind(globalThis),
  });
  try {
    const result = await run(['scan', `http://127.0.0.1:${port}/only`], io);
    assert.equal(result.exitCode, 0);
    const text = stdout();
    assert.equal(text.includes('SECRET_IN_SCRIPT'), false);
    assert.equal(text.includes('sk_live_page'), false);
    assert.equal(text.includes('<script'), false);
    const payload = JSON.parse(text.split('--- payload ---\n')[1].split('\n--- end ---')[0]);
    assert.equal(payload.card.name, 'Safe Title');
    assert.equal(payload.card.summary, 'Fetched listing page. No page text is included.');
    assert.equal(payload.offer_link_health, 'ok');
    assert.equal(payload.payments_check, 'pass');
  } finally {
    server.close();
  }
});

test('session-only commands do not call the API', async () => {
  for (const command of ['loosen', 'raise-cap', 'resume', 'account-key', 'plugin-key', 'restore']) {
    const { io, stdout, calls } = tempIO({ env: { OFFER_RAILS_API_URL: 'https://offer-rails.example' } });
    const result = await run([command], io);
    assert.equal(result.exitCode, 0, command);
    const text = stdout();
    assert.match(text.toLowerCase(), /sign in again/);
    assert.match(text, /https:\/\/offer-rails\.example\/owner/);
    assert.equal(calls.length, 0, command);
  }
});

test('mcp session-only tools return a sign-in link and scan does not upload without confirm', async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'offer-rails-mcp-'));
  fs.writeFileSync(path.join(root, 'app.js'), 'sk_live_should_not_leave func hidden() {}\n');
  const { io, calls } = tempIO({ cwd: root, env: { OFFER_RAILS_API_URL: 'http://127.0.0.1:9' } });
  const listed = await handleMcpMessage(JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/list' }), io);
  const names = listed.result.tools.map((item) => item.name);
  for (const name of ['login', 'scan', 'list', 'status', 'digest', 'undo', 'kill', 'restore_placement', 'loosen_mode', 'raise_cap', 'resume', 'account_key', 'plugin_key']) {
    assert.ok(names.includes(name), name);
  }
  assert.equal(toolDefs().find((item) => item.name === 'restore_placement').name, 'restore_placement');
  const restored = await handleMcpMessage(JSON.stringify({
    jsonrpc: '2.0', id: 2, method: 'tools/call', params: { name: 'restore_placement', arguments: { placement_id: 'abc' } },
  }), io);
  const body = JSON.parse(restored.result.content[0].text);
  assert.equal(restored.result.isError, false);
  assert.match(body.message.toLowerCase(), /sign in again/);
  assert.equal(body.sign_in_url, 'http://127.0.0.1:9/owner');
  assert.equal(calls.length, 0);
  const scanned = await handleMcpMessage(JSON.stringify({
    jsonrpc: '2.0', id: 3, method: 'tools/call', params: { name: 'scan', arguments: { path: '.' } },
  }), io);
  assert.equal(scanned.result.isError, false);
  assert.match(scanned.result.content[0].text, /Not sent/);
  assert.equal(scanned.result.content[0].text.includes('sk_live_'), false);
  assert.equal(calls.length, 0);
});

function projectIO(offer, routes) {
  const { io, stdout, calls } = tempIO({
    env: { OFFER_RAILS_API_URL: 'http://127.0.0.1:9' },
    fetch: async (url, opts = {}) => {
      const method = opts.method || 'GET';
      const pathname = new URL(url).pathname;
      calls.push({ url, method, body: opts.body });
      const hit = routes.find((route) => route.method === method && route.path === pathname);
      if (!hit) throw new Error(`${method} ${pathname}`);
      return jsonResponse(hit.status || 200, hit.body);
    },
  });
  saveConfig(io, {
    access_token: 'or_test',
    key_header: 'X-Maker-Key',
    scope: `offer:${offer}`,
    listing_card_id: 'card-1',
  });
  return { io, stdout, calls };
}

test('list prints in review when a check fails and does not set a mode', async () => {
  const offer = '11111111-1111-4111-8111-111111111111';
  const { io, stdout, calls } = projectIO(offer, [
    { method: 'GET', path: `/v1/maker/offers/${offer}`, body: { id: offer, status: 'draft' } },
    {
      method: 'POST',
      path: `/v1/maker/offers/${offer}/submit`,
      body: {
        id: offer,
        status: 'submitted',
        review_reason: 'relevance',
        message: 'Relevance is below the minimum.',
      },
    },
    { method: 'GET', path: '/v1/setup/settings', body: { mode: 'manual', mode_preselected: 'yolo' } },
  ]);
  const result = await run(['list'], io);
  assert.equal(result.exitCode, 0);
  const text = stdout();
  assert.match(text, /status: in review/);
  assert.match(text, /^check: relevance$/m);
  assert.match(text, /^message: Relevance is below the minimum\.$/m);
  assert.match(text, /^next: Submit again after this check passes\.$/m);
  assert.match(text, /^mode: manual$/m);
  assert.match(text, /^mode_preselected: yolo$/m);
  assert.equal(text.includes('status: listed'), false);
  assert.equal(calls.some((call) => call.method === 'PUT' || call.url.includes('/mode')), false);
});

test('list prints the claims stub hold from review_reason and message', async () => {
  const offer = '66666666-6666-4666-8666-666666666666';
  const { io, stdout } = projectIO(offer, [
    { method: 'GET', path: `/v1/maker/offers/${offer}`, body: { id: offer, status: 'draft' } },
    {
      method: 'POST',
      path: `/v1/maker/offers/${offer}/submit`,
      body: {
        id: offer,
        status: 'submitted',
        review_reason: 'claims',
        claims_check: 'not_checked',
        message: 'Claims are not checked yet.',
      },
    },
    { method: 'GET', path: '/v1/setup/settings', body: { mode: 'manual', mode_preselected: 'yolo' } },
  ]);
  const result = await run(['list'], io);
  assert.equal(result.exitCode, 0);
  const text = stdout();
  assert.match(text, /status: in review/);
  assert.match(text, /^check: claims$/m);
  assert.match(text, /^message: Claims are not checked yet\.$/m);
  assert.match(text, /^claims: not checked yet$/m);
  assert.equal(/\bchecked\b/.test(text.replace(/not checked yet/g, '')), false);
  assert.match(text, /^next: Submit again after this check passes\.$/m);
  assert.equal(text.includes('status: listed'), false);
});

test('list prints the readiness message next step from reason and message', async () => {
  const offer = '55555555-5555-4555-8555-555555555555';
  const message = 'This product has not passed readiness assessment. Fix the failed checks, then submit again.';
  const { io, stdout } = projectIO(offer, [
    { method: 'GET', path: `/v1/maker/offers/${offer}`, body: { id: offer, status: 'draft' } },
    {
      method: 'POST',
      path: `/v1/maker/offers/${offer}/submit`,
      status: 409,
      body: { error: message, reason: 'head', message, readiness_status: 'failed' },
    },
    { method: 'GET', path: '/v1/setup/settings', body: { mode: 'manual', mode_preselected: 'yolo' } },
  ]);
  const result = await run(['list'], io);
  assert.equal(result.exitCode, 0);
  const text = stdout();
  assert.match(text, /status: in review/);
  assert.match(text, /^check: head$/m);
  assert.match(text, /^message: This product has not passed readiness assessment\. Fix the failed checks, then submit again\.$/m);
  assert.match(text, /^next: Fix the failed checks, then submit again\.$/m);
});

test('list prints listed and the mode the server returns', async () => {
  const offer = '22222222-2222-4222-8222-222222222222';
  const { io, stdout, calls } = projectIO(offer, [
    { method: 'GET', path: `/v1/maker/offers/${offer}`, body: { id: offer, status: 'draft' } },
    {
      method: 'POST',
      path: `/v1/maker/offers/${offer}/submit`,
      body: { id: offer, status: 'listed', review_reason: null, message: null },
    },
    { method: 'GET', path: '/v1/setup/settings', body: { mode: 'manual', mode_preselected: 'yolo' } },
  ]);
  const result = await run(['list'], io);
  assert.equal(result.exitCode, 0);
  const text = stdout();
  assert.match(text, /status: listed/);
  assert.match(text, /^mode: manual$/m);
  assert.match(text, /^mode_preselected: yolo$/m);
  assert.equal(text.includes('in review'), false);
  assert.equal(calls.some((call) => call.method === 'PUT'), false);
});

test('list does not call a bare submitted offer in review', async () => {
  const offer = '44444444-4444-4444-8444-444444444444';
  const { io, stdout } = projectIO(offer, [
    { method: 'GET', path: `/v1/maker/offers/${offer}`, body: { id: offer, status: 'draft' } },
    { method: 'POST', path: `/v1/maker/offers/${offer}/submit`, body: { id: offer, status: 'submitted' } },
    { method: 'GET', path: '/v1/setup/settings', body: { mode: 'manual', mode_preselected: 'yolo' } },
  ]);
  const result = await run(['list'], io);
  assert.equal(result.exitCode, 1);
  const text = stdout();
  assert.match(text, /status: submitted/);
  assert.equal(text.includes('in review'), false);
  assert.match(text, /^mode_preselected: yolo$/m);
});

test('digest prints the mode and mode_preselected the server returns', async () => {
  const offer = '33333333-3333-4333-8333-333333333333';
  const { io, stdout } = projectIO(offer, [
    {
      method: 'GET',
      path: '/v1/maker/digest',
      body: {
        effective_mode: 'manual',
        items: [{ summary: 'Paste Forge', live_now: true, undo_token: 'tok' }],
      },
    },
    { method: 'GET', path: '/v1/setup/settings', body: { mode: 'manual', mode_preselected: 'yolo' } },
  ]);
  const result = await run(['digest'], io);
  assert.equal(result.exitCode, 0);
  const text = stdout();
  assert.match(text, /^mode: manual$/m);
  assert.match(text, /^mode_preselected: yolo$/m);
  assert.match(text, /summary: Paste Forge/);
});

test('digest says when the list was cut and when claims are not checked yet', async () => {
  const offer = '33333333-3333-4333-8333-333333333334';
  const { io, stdout } = projectIO(offer, [
    {
      method: 'GET',
      path: '/v1/maker/digest',
      body: {
        effective_mode: 'manual',
        truncated: true,
        total: 51,
        claims_check: 'not_checked',
        items: [{ summary: 'Paste Forge', live_now: true, undo_token: 'tok', claims_check: 'not_checked' }],
      },
    },
    { method: 'GET', path: '/v1/setup/settings', body: { mode: 'manual', mode_preselected: 'yolo' } },
  ]);
  const result = await run(['digest'], io);
  assert.equal(result.exitCode, 0);
  const text = stdout();
  assert.match(text, /^truncated: true$/m);
  assert.match(text, /^total: 51$/m);
  assert.match(text, /Showing 1 of 51\. The list was cut\./);
  assert.match(text, /^claims: not checked yet$/m);
  assert.equal(/\bchecked\b/.test(text.replace(/not checked yet/g, '')), false);
});

test('the readme paste block is the six CLI lines', () => {
  const readme = fs.readFileSync(new URL('../README.md', import.meta.url), 'utf8');
  const block = readme.split('<!-- paste:begin -->')[1].split('<!-- paste:end -->')[0];
  const lines = block.split('\n').map((line) => line.trim()).filter((line) => line.startsWith('node '));
  assert.deepEqual(lines, [
    'node cli/bin/offer-rails.mjs login',
    'node cli/bin/offer-rails.mjs scan .',
    'node cli/bin/offer-rails.mjs list',
    'node cli/bin/offer-rails.mjs digest',
    'node cli/bin/offer-rails.mjs undo',
    'node cli/bin/offer-rails.mjs kill',
  ]);
  assert.match(readme, /\.cursor\/mcp\.json/);
  assert.match(readme, /node cli\/bin\/offer-rails\.mjs mcp/);
  assert.match(readme, /"command": "node"/);
  assert.match(readme, /"cli\/bin\/offer-rails\.mjs", "mcp"/);
  assert.match(readme, /in review/);
  assert.match(readme, /review_reason/);
  assert.match(readme, /does not set an approval mode/);
  assert.match(readme, /mode_preselected` is yolo/);
  assert.equal(readme.toLowerCase().includes('fully automatic'), false);
});
