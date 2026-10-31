import { readFile, writeFile, mkdir, lstat, realpath } from 'node:fs/promises';
import { resolve, dirname, relative, delimiter } from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { previousSkillBodies } from './previous-skills.mjs';

export function validateAPI(value) {
  const u = new URL(value);
  const local = ['localhost', '127.0.0.1', '[::1]'].includes(u.hostname);
  if ((u.protocol !== 'https:' && !(u.protocol === 'http:' && local)) || u.username || u.password || u.search || u.hash || !['', '/', '/v1', '/v1/'].includes(u.pathname)) {
    throw new Error('Use an HTTPS API origin (or loopback HTTP for local development), without credentials or query parameters.');
  }
  return u.origin;
}

export async function readOptional(path) {
  try { return await readFile(path, 'utf8'); }
  catch (error) { if (error.code === 'ENOENT') return null; throw error; }
}

export async function detectProject(root) {
  const raw = await readOptional(resolve(root, 'package.json'));
  const pkg = raw === null ? {} : JSON.parse(raw);
  const dependencies = { ...pkg.dependencies, ...pkg.devDependencies };
  const frameworks = ['next', 'nuxt', '@sveltejs/kit', 'astro', 'express', 'react'].filter(name => name in dependencies);
  const providers = ['stripe', '@stripe/stripe-js', '@lemonsqueezy/lemonsqueezy.js', '@polar-sh/sdk'].filter(name => name in dependencies);
  if (await readOptional(resolve(root, 'go.mod')) !== null) frameworks.push('go');
  if (await readOptional(resolve(root, 'pyproject.toml')) !== null || await readOptional(resolve(root, 'requirements.txt')) !== null) frameworks.push('python');
  if (await readOptional(resolve(root, 'composer.json')) !== null) frameworks.push('php');
  return { frameworks, providers };
}

export async function checkServer(api, request = fetch) {
  const resource = `${api}/mcp`;
  async function get(path) {
    const response = await request(`${api}${path}`, { redirect: 'error', signal: AbortSignal.timeout(8000), headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error('Hosted sign-in is not available on this API yet. Configure the pilot first, or use --offline to prepare files only.');
    return response.json();
  }
  const metadata = await get('/.well-known/oauth-protected-resource/mcp');
  const authorization = await get('/.well-known/oauth-authorization-server');
  if (metadata.resource !== resource || !metadata.authorization_servers?.includes(api) || authorization.issuer !== api || !authorization.scopes_supported?.includes('maker') || !authorization.code_challenge_methods_supported?.includes('S256')) {
    throw new Error('The API does not advertise the expected maker OAuth connection.');
  }
  return resource;
}

async function writablePath(root, path) {
  const base = await realpath(root);
  let current = base;
  for (const segment of relative(root, path).split('/')) {
    if (segment === '..') throw new Error('Setup path is outside the project.');
    current = resolve(current, segment);
    try {
      const stat = await lstat(current);
      if (stat.isSymbolicLink()) throw new Error(`Setup will not write through a symbolic link: ${current}`);
    } catch (error) { if (error.code !== 'ENOENT') throw error; }
  }
}

export async function mergeMCP(root, client, resource) {
  const path = resolve(root, client === 'cursor' ? '.cursor/mcp.json' : '.mcp.json');
  await writablePath(root, path);
  const raw = await readOptional(path);
  const config = raw === null ? {} : JSON.parse(raw);
  if (!config || Array.isArray(config) || typeof config !== 'object' || config.mcpServers && (typeof config.mcpServers !== 'object' || Array.isArray(config.mcpServers))) throw new Error(`Invalid MCP configuration: ${path}`);
  config.mcpServers ??= {};
  const existing = config.mcpServers.allagio || config.mcpServers.distributoor;
  const expected = client === 'claude' ? { type: 'http', url: resource } : { url: resource };
  if (existing && (existing.url !== resource || existing.command || existing.headers || existing.env || existing.type && existing.type !== 'http')) {
    throw new Error(`An existing Allagio connection differs in ${path}. Resolve it before running setup; it has been preserved.`);
  }
  if (!existing) {
    config.mcpServers.allagio = expected;
    await mkdir(dirname(path), { recursive: true });
    await writeFile(path, `${JSON.stringify(config, null, 2)}\n`, { mode: 0o600 });
  }
  return path;
}

const SKILL_CONTEXT_MARKER = '\n\n<!-- distributoor-setup-context -->\n';
const INSTALLER_CONTEXT = /^## This installation\n\nHosted API: .+\n\nDetected dependencies \(hints to verify\): .+\n\nRequested product URL: .+\n$/;

function acceptsInstalledSkill(existing, source, name) {
  const index = existing.indexOf(SKILL_CONTEXT_MARKER);
  if (index === -1) return false;
  const body = existing.slice(0, index);
  const context = existing.slice(index + SKILL_CONTEXT_MARKER.length);
  const canonical = body === source.trim() || body === previousSkillBodies[name];
  return canonical && INSTALLER_CONTEXT.test(context);
}

export const skillNames = [
  'distributoor-onboard',
  'distributoor-instrument',
  'distributoor-launch',
  'distributoor-revenue',
];

export async function installSkills(root, client, context) {
  const paths = [];
  for (const name of skillNames) {
    const path = resolve(root, client === 'claude' ? '.claude/skills' : '.agents/skills', name, 'SKILL.md');
    await writablePath(root, path);
    let source = await readOptional(fileURLToPath(new URL(`../skills/${name}/SKILL.md`, import.meta.url)));
    source ??= await readOptional(fileURLToPath(new URL(`../../../distributoor-plugin/skills/${name}/SKILL.md`, import.meta.url)));
    if (source === null) throw new Error(`Packaged skill ${name} is missing.`);
    const installed = `${source.trim()}${SKILL_CONTEXT_MARKER}## This installation\n\nHosted API: ${context.api}\n\nDetected dependencies (hints to verify): ${JSON.stringify(context.project)}\n\nRequested product URL: ${JSON.stringify(context.website || 'Infer from project documentation; ask if uncertain.')}\n`;
    const existing = await readOptional(path);
    if (existing !== null && existing !== installed && !acceptsInstalledSkill(existing, source, name)) {
      throw new Error(`A different skill exists at ${path}; it has been preserved. Review it before updating setup.`);
    }
    await mkdir(dirname(path), { recursive: true });
    await writeFile(path, installed, { mode: 0o644 });
    paths.push(path);
  }
  return paths;
}

export async function available(command, env = process.env) {
  for (const directory of (env.PATH || '').split(delimiter)) {
    try {
      const stat = await lstat(resolve(directory, command));
      if (stat.isFile() || stat.isSymbolicLink()) return true;
    } catch (error) { if (error.code !== 'ENOENT') throw error; }
  }
  return false;
}

export async function chooseClient(root, requested) {
  if (requested !== 'auto') return requested;
  for (const [directory, client] of [['.cursor', 'cursor'], ['.claude', 'claude'], ['.codex', 'codex']]) {
    try { if ((await lstat(resolve(root, directory))).isDirectory()) return client; }
    catch (error) { if (error.code !== 'ENOENT') throw error; }
  }
  if (await available('codex')) return 'codex';
  if (await available('claude')) return 'claude';
  return 'skill';
}

export function run(command, args, root, stdio = 'inherit') {
  return new Promise((resolveRun, reject) => {
    const child = spawn(command, args, { cwd: root, stdio, shell: false });
    child.once('error', reject);
    child.once('exit', (code, signal) => code === 0 ? resolveRun() : reject(new Error(`${command} exited ${signal || code}. Setup remains installed; reconnect from your agent to retry.`)));
  });
}

export function codexConfig(resource) {
  return ['-c', `mcp_servers.allagio={url=${JSON.stringify(resource)}}`];
}
