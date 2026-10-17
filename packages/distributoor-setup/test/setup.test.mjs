import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, readFile, symlink, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { validateAPI, detectProject, checkServer, mergeMCP, installSkills } from '../lib/setup.mjs';

const execute = promisify(execFile);
async function fixture(t) {
  const root = await mkdtemp(resolve(tmpdir(), 'distributoor-setup-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  return root;
}

test('rejects credential-bearing or non-HTTPS remote API URLs', () => {
  assert.equal(validateAPI('https://pilot.example/v1'), 'https://pilot.example');
  assert.equal(validateAPI('http://127.0.0.1:8086'), 'http://127.0.0.1:8086');
  for (const value of ['http://remote.example', 'https://user:secret@pilot.example', 'https://pilot.example?token=secret', 'https://pilot.example/other']) assert.throws(() => validateAPI(value));
});

test('detects dependencies without reading source files or env', async t => {
  const root = await fixture(t);
  await writeFile(resolve(root, 'package.json'), JSON.stringify({ dependencies: { next: '16', stripe: '18' } }));
  await writeFile(resolve(root, '.env'), 'SECRET=never-read');
  assert.deepEqual(await detectProject(root), { frameworks: ['next'], providers: ['stripe'] });
});

test('MCP configuration preserves other servers and reruns unchanged', async t => {
  const root = await fixture(t);
  await mkdir(resolve(root, '.cursor'));
  const file = resolve(root, '.cursor/mcp.json');
  const other = { command: 'tool', env: { API_KEY: '${EXISTING}' } };
  await writeFile(file, JSON.stringify({ setting: true, mcpServers: { other } }));
  await mergeMCP(root, 'cursor', 'https://pilot.example/mcp');
  const first = await readFile(file, 'utf8');
  await mergeMCP(root, 'cursor', 'https://pilot.example/mcp');
  assert.equal(await readFile(file, 'utf8'), first);
  assert.deepEqual(JSON.parse(first).mcpServers.other, other);
  assert.equal(JSON.parse(first).setting, true);
  await assert.rejects(mergeMCP(root, 'cursor', 'https://different.example/mcp'), /preserved/);
  assert.equal(await readFile(file, 'utf8'), first);
});

test('refuses malformed configs and paths redirected outside the project', async t => {
  const root = await fixture(t);
  await writeFile(resolve(root, '.mcp.json'), 'invalid');
  await assert.rejects(mergeMCP(root, 'claude', 'https://pilot.example/mcp'));
  assert.equal(await readFile(resolve(root, '.mcp.json'), 'utf8'), 'invalid');
  const external = await fixture(t);
  await symlink(external, resolve(root, '.cursor'));
  await assert.rejects(mergeMCP(root, 'cursor', 'https://pilot.example/mcp'), /symbolic link/);
});

test('skills carry setup context and preserve custom changes', async t => {
  const root = await fixture(t);
  const context = { api: 'https://pilot.example', project: { frameworks: ['next'], providers: ['stripe'] } };
  const paths = await installSkills(root, 'skill', context);
  const content = await readFile(paths[0], 'utf8');
  assert.match(content, /Hosted API: https:\/\/pilot.example/);
  await installSkills(root, 'skill', context);
  await writeFile(paths[0], `${content}\nUser edits\n`);
  await assert.rejects(installSkills(root, 'skill', context), /preserved/);
  assert.match(await readFile(paths[0], 'utf8'), /User edits/);
});

test('connection preflight rejects unavailable or mismatched OAuth without redirecting', async () => {
  await assert.rejects(checkServer('https://pilot.example', async () => ({ ok: false })), /not available/);
  await assert.rejects(checkServer('https://pilot.example', async () => ({ ok: true, json: async () => ({}) })), /expected maker/);
  const calls = [];
  const metadata = [{ resource: 'https://pilot.example/mcp', authorization_servers: ['https://pilot.example'] }, { issuer: 'https://pilot.example', scopes_supported: ['maker'], code_challenge_methods_supported: ['S256'] }];
  assert.equal(await checkServer('https://pilot.example', async (url, options) => {
    calls.push({ url, options });
    return { ok: true, json: async () => metadata.shift() };
  }), 'https://pilot.example/mcp');
  assert.ok(calls.every(call => call.options.redirect === 'error'));
});

test('offline CLI prepares a real project without invoking or claiming a connected agent', async t => {
  const root = await fixture(t);
  const cli = resolve(import.meta.dirname, '../bin/setup.mjs');
  const { stdout } = await execute(process.execPath, [cli, '--project', root, '--client', 'cursor', '--api-url', 'http://127.0.0.1:8086', '--offline']);
  assert.match(stdout, /connection has not been verified/);
  assert.match(stdout, /integration and paid conversion verification are still pending/);
  assert.equal(JSON.parse(await readFile(resolve(root, '.cursor/mcp.json'), 'utf8')).mcpServers.distributoor.url, 'http://127.0.0.1:8086/mcp');
});

test('package command is offer-rails and keeps the old names', async () => {
  const pkg = JSON.parse(await readFile(new URL('../package.json', import.meta.url), 'utf8'));
  assert.equal(pkg.name, 'offer-rails');
  assert.equal(pkg.version, '0.1.0');
  for (const name of ['offer-rails', 'affilio', 'distributoor', 'distributoor-setup']) {
    assert.equal(pkg.bin[name], 'bin/setup.mjs');
  }
});
