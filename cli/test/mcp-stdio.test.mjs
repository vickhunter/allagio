import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import test from 'node:test';

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
  assert.equal(claudeLine, 'claude mcp add offer-rails -- npx offer-rails mcp');
  const fromLine = claudeLine.split(' -- ')[1].trim().split(/\s+/);
  assert.deepEqual(fromLine, [claude.mcpServers['offer-rails'].command, ...claude.mcpServers['offer-rails'].args]);
  return [
    { label: 'cursor .cursor/mcp.json', server: cursor.mcpServers['offer-rails'] },
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

async function checkServer(label, server) {
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
    assert.equal(init.result.serverInfo.name, 'offer-rails', label);
    child.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' })}\n`);
    const listed = await request(2, 'tools/list', {});
    const names = listed.result.tools.map((tool) => tool.name);
    for (const name of ['status', 'restore_placement']) assert.ok(names.includes(name), `${label} missing ${name}`);
    const status = await request(3, 'tools/call', { name: 'status', arguments: {} });
    assert.equal(status.result.isError, false, `${label} status`);
    const statusText = status.result.content[0].text;
    assert.match(statusText, /^scope: offer:/m, statusText);
    assert.match(statusText, /^key_header: X-Maker-Key/m, statusText);
    assert.match(statusText, /^status: /m, statusText);
    const restored = await request(4, 'tools/call', {
      name: 'restore_placement',
      arguments: { placement_id: '00000000-0000-4000-8000-000000000000' },
    });
    assert.equal(restored.result.isError, false, `${label} restore_placement`);
    const body = JSON.parse(restored.result.content[0].text);
    assert.match(body.message.toLowerCase(), /sign in again/);
    assert.match(body.sign_in_url, /\/owner$/);
    process.stdout.write(`-- ${label} --\n`);
    process.stdout.write(`command: ${server.command} ${server.args.join(' ')}\n`);
    process.stdout.write(`server: ${init.result.serverInfo.name}\n`);
    process.stdout.write(`tools: ${names.join(', ')}\n`);
    process.stdout.write(`read tool status:\n${statusText}`);
    if (!statusText.endsWith('\n')) process.stdout.write('\n');
    process.stdout.write(`session-only restore_placement: ${body.message}\n`);
    process.stdout.write(`sign_in_url: ${body.sign_in_url}\n`);
  } catch (err) {
    process.stderr.write(stderr.join(''));
    throw err;
  } finally {
    child.stdin.end();
    child.kill();
    await Promise.race([exited, new Promise((resolve) => setTimeout(resolve, 2000))]);
  }
}

test('published npx offer-rails mcp speaks stdio as configured', async (t) => {
  if (process.env.OFFER_RAILS_MCP_SPAWN !== '1') {
    t.skip('runs from paste-test.sh against the Verdaccio install');
    return;
  }
  for (const { label, server } of configuredServers()) {
    await checkServer(label, server);
  }
});
