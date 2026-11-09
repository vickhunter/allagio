import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';

function assertSpawnUsesInstall() {
  const installed = process.env.OFFER_RAILS_INSTALLED_ROOT;
  const source = process.env.OFFER_RAILS_SOURCE_ROOT;
  if (!installed || !source) return;
  const root = fs.realpathSync(installed);
  const src = fs.realpathSync(source);
  for (const rel of ['cli/bin/offer-rails.mjs', 'cli/lib/commands.mjs', 'cli/lib/mcp.mjs']) {
    const file = fs.realpathSync(path.resolve(rel));
    assert.ok(file === root || file.startsWith(`${root}${path.sep}`), `${rel} is not the installed package: ${file}`);
    assert.ok(!(file === src || file.startsWith(`${src}${path.sep}`)), `${rel} falls back to source: ${file}`);
  }
}

function readmeText() {
  return fs.readFileSync(new URL('../README.md', import.meta.url), 'utf8');
}

function fenced(section, lang) {
  const start = readmeText().indexOf(section);
  assert.ok(start >= 0, section);
  const fence = readmeText().slice(start).match(new RegExp(`\`\`\`${lang}\\n([\\s\\S]*?)\\n\`\`\``));
  assert.ok(fence, `${section} ${lang} fence`);
  return fence[1];
}

export function configuredServers() {
  const cursor = JSON.parse(fenced('## Cursor', 'json'));
  const claude = JSON.parse(fenced('JSON form', 'json'));
  const claudeLine = fenced('## Claude Code', 'bash').trim();
  assert.equal(claudeLine, 'claude mcp add allagio -- node cli/bin/offer-rails.mjs mcp');
  const fromLine = claudeLine.split(' -- ')[1].trim().split(/\s+/);
  assert.deepEqual(fromLine, [claude.mcpServers.allagio.command, ...claude.mcpServers.allagio.args]);
  return [
    { label: 'cursor .cursor/mcp.json', server: cursor.mcpServers.allagio },
    { label: 'claude mcp add', server: { command: fromLine[0], args: fromLine.slice(1) } },
  ];
}

function speak(child) {
  let buf = '';
  const pending = new Map();
  child.stdout.on('data', (chunk) => {
    buf += chunk.toString('utf8');
    let nl = buf.indexOf('\n');
    while (nl >= 0) {
      const line = buf.slice(0, nl).trim();
      buf = buf.slice(nl + 1);
      nl = buf.indexOf('\n');
      if (!line.startsWith('{')) continue;
      const msg = JSON.parse(line);
      if (msg.id !== undefined && pending.has(msg.id)) {
        pending.get(msg.id)(msg);
        pending.delete(msg.id);
      }
    }
  });
  return (id, method, params) => new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`timed out waiting for ${method}`)), 60000);
    pending.set(id, (msg) => {
      clearTimeout(timer);
      resolve(msg);
    });
    child.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', id, method, params })}\n`);
  });
}

function serverSnapshot() {
  const db = process.env.OFFER_RAILS_PASTE_DB;
  assert.ok(db, 'OFFER_RAILS_PASTE_DB');
  return execFileSync('psql', [
    '-v', 'ON_ERROR_STOP=1',
    '-h', process.env.PGHOST || '127.0.0.1',
    '-p', process.env.PGPORT || '5432',
    '-U', process.env.PGUSER || 'affilio',
    '-d', db,
    '-At',
    '-c', `SELECT COALESCE(json_build_object(
      'offers', (SELECT json_agg(row_to_json(t) ORDER BY t.id) FROM (
        SELECT o.id::text AS id, o.status, o.updated_at, ap.mode, ap.kill_switch, ap.version
        FROM offers o LEFT JOIN approval_policies ap ON ap.offer_id = o.id
      ) t),
      'placements', (SELECT count(*) FROM placement_proposals),
      'makers', (SELECT json_agg(row_to_json(m) ORDER BY m.id) FROM (SELECT id::text AS id, trusted FROM makers) m)
    )::text, '');`,
  ], { encoding: 'utf8' });
}

async function checkServer(label, server) {
  assertSpawnUsesInstall();
  const stderr = [];
  const child = spawn(server.command, server.args, { stdio: ['pipe', 'pipe', 'pipe'] });
  child.stderr.on('data', (chunk) => stderr.push(chunk.toString('utf8')));
  const request = speak(child);
  const exited = new Promise((resolve) => child.on('exit', resolve));
  try {
    const init = await request(1, 'initialize', {
      protocolVersion: '2025-06-18',
      capabilities: {},
      clientInfo: { name: 'offer-rails-paste', version: '0' },
    });
    assert.equal(init.result.serverInfo.name, 'allagio', label);
    child.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' })}\n`);
    const listed = await request(2, 'tools/list', {});
    const names = listed.result.tools.map((tool) => tool.name);
    for (const name of ['status', 'kill', 'restore_placement']) assert.ok(names.includes(name), `${label} missing ${name}`);
    const status = await request(3, 'tools/call', { name: 'status', arguments: {} });
    assert.equal(status.result.isError, false, `${label} status`);
    const statusText = status.result.content[0].text;
    assert.match(statusText, /^scope: offer:/m, statusText);
    assert.match(statusText, /^key_header: X-Maker-Key/m, statusText);
    assert.match(statusText, /^status: /m, statusText);
    const before = serverSnapshot();
    const restored = await request(4, 'tools/call', {
      name: 'restore_placement',
      arguments: { placement_id: '00000000-0000-4000-8000-000000000000' },
    });
    assert.equal(restored.result.isError, false, `${label} restore_placement`);
    const body = JSON.parse(restored.result.content[0].text);
    assert.match(body.message.toLowerCase(), /sign in again/);
    assert.match(body.sign_in_url, /\/owner$/);
    const after = serverSnapshot();
    assert.equal(after, before, `${label} session-only changed the server`);
    const killed = await request(5, 'tools/call', { name: 'kill', arguments: {} });
    assert.equal(killed.result.isError, false, `${label} kill`);
    const killText = killed.result.content[0].text;
    assert.match(killText, /^kill_switch: true$/m, killText);
    process.stdout.write(`-- ${label} --\n`);
    process.stdout.write(`command: ${server.command} ${server.args.join(' ')}\n`);
    process.stdout.write(`server: ${init.result.serverInfo.name}\n`);
    process.stdout.write(`tools: ${names.join(', ')}\n`);
    process.stdout.write(`read tool status:\n${statusText}`);
    if (!statusText.endsWith('\n')) process.stdout.write('\n');
    process.stdout.write(`session-only restore_placement: ${body.message}\n`);
    process.stdout.write(`sign_in_url: ${body.sign_in_url}\n`);
    process.stdout.write('server unchanged after restore_placement\n');
    process.stdout.write(`kill:\n${killText}`);
    if (!killText.endsWith('\n')) process.stdout.write('\n');
  } catch (err) {
    process.stderr.write(stderr.join(''));
    throw err;
  } finally {
    child.stdin.end();
    child.kill();
    await Promise.race([exited, new Promise((resolve) => setTimeout(resolve, 2000))]);
  }
}

test('documented local mcp command speaks stdio', async (t) => {
  if (process.env.OFFER_RAILS_MCP_SPAWN !== '1') {
    t.skip('runs from paste-test.sh after the loopback install');
    return;
  }
  for (const { label, server } of configuredServers()) {
    await checkServer(label, server);
  }
});
