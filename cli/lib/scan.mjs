import fs from 'node:fs';
import path from 'node:path';

const SKIP_DIRS = new Set(['.git', 'node_modules', 'vendor', '.offer-rails']);
const TEXT_EXT = new Set([
  '.md', '.txt', '.json', '.js', '.mjs', '.cjs', '.ts', '.tsx', '.jsx', '.go', '.php', '.py',
  '.rb', '.html', '.css', '.yml', '.yaml', '.toml', '.xml', '.svg', '.vue', '.rs', '.java',
  '.kt', '.swift', '.cs', '.sql',
]);
const SIGNAL_NAMES = ['payments', 'pricing', 'landing', 'features', 'offer_link', 'disclosure'];
const DEFAULT_SUMMARY = 'Local readiness scan. No page text is included.';
const URL_SUMMARY = 'Fetched listing page. No page text is included.';

function clip(value, max) {
  return [...String(value || '')].slice(0, max).join('').trim();
}

function looksSensitive(value) {
  const text = String(value || '');
  if (/sk_live_|sk_test_|rk_live_|pk_live_|whsec_|-----BEGIN |ghp_|github_pat_|xox[baprs]-|sk-ant-|sk-proj-|glpat-|AKIA[0-9A-Z]|<\?php|<\s*script\b|\bfunc\s+\w+\s*\(|\bfunction\s+\w+\s*\(/i.test(text)) {
    return true;
  }
  // Same shapes that skip a file: an .env assignment, or a private-key block.
  if (/(?:^|\n)\s*(?:export\s+)?[A-Z][A-Z0-9_]{2,}\s*=\s*\S/.test(text)) return true;
  if (/-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----/.test(text)) return true;
  return false;
}

function isSensitiveFile(name) {
  const lower = name.toLowerCase();
  if (lower === '.env' || lower.startsWith('.env.')) return true;
  if (/\.(pem|key|p12|pfx|cer|crt)$/i.test(name)) return true;
  if (/^(id_rsa|id_ed25519)/.test(name)) return true;
  return false;
}

function loadIgnore(root) {
  const file = path.join(root, '.gitignore');
  if (!fs.existsSync(file)) return [];
  return fs.readFileSync(file, 'utf8')
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#') && !line.startsWith('!'));
}

function ignored(rel, patterns) {
  const norm = rel.split(path.sep).join('/');
  const base = path.basename(norm);
  for (const raw of patterns) {
    const pat = raw.replace(/^\//, '').replace(/\/$/, '');
    if (!pat) continue;
    if (pat.includes('*')) {
      const re = new RegExp(`^${pat.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*')}$`);
      if (re.test(base) || re.test(norm)) return true;
      continue;
    }
    if (pat.includes('/')) {
      if (norm === pat || norm.startsWith(`${pat}/`)) return true;
      continue;
    }
    if (base === pat || norm.split('/').includes(pat)) return true;
  }
  return false;
}

function walk(dir, root, patterns, files, state) {
  if (state.count >= 400) return;
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return;
  }
  for (const ent of entries) {
    if (state.count >= 400) return;
    const full = path.join(dir, ent.name);
    const rel = path.relative(root, full);
    if (isSensitiveFile(ent.name) || ignored(rel, patterns)) continue;
    if (ent.isDirectory()) {
      if (SKIP_DIRS.has(ent.name) || ent.name.startsWith('.')) continue;
      walk(full, root, patterns, files, state);
      continue;
    }
    if (!ent.isFile() || ent.name.startsWith('.')) continue;
    const ext = path.extname(ent.name).toLowerCase();
    if (ext && !TEXT_EXT.has(ext) && ent.name !== 'README' && ent.name !== 'Makefile') continue;
    let buf;
    try {
      buf = fs.readFileSync(full);
    } catch {
      continue;
    }
    if (buf.includes(0)) continue;
    files.push({ rel: rel.split(path.sep).join('/'), name: ent.name, text: buf.subarray(0, 65536).toString('utf8') });
    state.count += 1;
  }
}

function signalsFrom(files) {
  const text = files.map((file) => file.text).join('\n');
  const names = files.map((file) => file.name.toLowerCase());
  const checks = {
    payments: /stripe|polar|lemonsqueezy|lemon\s*squeezy|paddle|webhook/i.test(text),
    pricing: /\bpric(?:e|ing)\b|\bplans?\b|\$\d/i.test(text),
    landing: names.some((name) => name === 'readme.md' || name === 'readme' || name === 'index.html' || name.includes('landing')),
    features: /\bfeatures?\b/i.test(text),
    offer_link: /https?:\/\//i.test(text),
    disclosure: /disclos|sponsor|affiliate/i.test(text),
  };
  const signals = SIGNAL_NAMES.map((name) => ({ name, pass: Boolean(checks[name]) }));
  const passes = signals.filter((signal) => signal.pass).length;
  return {
    signals,
    score: Math.round((100 * passes) / signals.length),
    payments_check: checks.payments ? 'pass' : 'fail',
  };
}

function listingPayload({ name, summary, signals, score, payments_check, offer_link_health }) {
  const safeName = looksSensitive(name) ? '' : clip(name, 80);
  const safeSummary = looksSensitive(summary) ? '' : clip(summary, 280);
  return {
    card: {
      name: safeName || 'listing',
      summary: safeSummary || DEFAULT_SUMMARY,
      category: 'software',
      features: signals.filter((signal) => signal.pass).map((signal) => signal.name).slice(0, 8),
      pricing_summary: signals.some((signal) => signal.name === 'pricing' && signal.pass) ? 'Pricing is mentioned.' : '',
    },
    signals,
    score,
    payments_check,
    offer_link_health,
  };
}

function cardCopy(root, files) {
  let name = '';
  let summary = '';
  const pkg = files.find((file) => file.rel === 'package.json');
  if (pkg) {
    try {
      const parsed = JSON.parse(pkg.text);
      if (typeof parsed.name === 'string') name = parsed.name;
      if (typeof parsed.description === 'string') summary = parsed.description;
    } catch {
      // A package.json that is not JSON still counts as a local file for signals.
    }
  }
  if (!name) {
    const readme = files.find((file) => /^readme(\.md|\.txt)?$/i.test(file.name));
    const heading = readme && readme.text.match(/^#\s+(\S[^\n]{0,120})/m);
    if (heading) name = heading[1].trim();
  }
  if (!name) name = path.basename(root);
  if (!summary) summary = DEFAULT_SUMMARY;
  return { name, summary };
}

export function scanPath(target, cwd) {
  const root = path.resolve(cwd, target || '.');
  if (!fs.existsSync(root) || !fs.statSync(root).isDirectory()) {
    throw new Error(`scan path is not a directory: ${target || '.'}`);
  }
  const files = [];
  walk(root, root, loadIgnore(root), files, { count: 0 });
  const copy = cardCopy(root, files);
  return listingPayload({ ...copy, ...signalsFrom(files), offer_link_health: 'unknown' });
}

function localURL(target) {
  const url = new URL(target);
  const host = url.hostname.replace(/^\[|\]$/g, '');
  const local = host === 'localhost' || host === '127.0.0.1' || host === '::1';
  if (url.protocol !== 'https:' && !(url.protocol === 'http:' && local)) {
    throw new Error('scan accepts an https URL, or http only on localhost');
  }
  return url;
}

export async function scanURL(target, io) {
  const url = localURL(target);
  const res = await io.fetch(url.href, { redirect: 'manual' });
  const ok = res.status >= 200 && res.status < 300;
  const html = ok ? await res.text() : '';
  const stripped = html
    .replace(/<script[\s\S]*?<\/script>/gi, ' ')
    .replace(/<style[\s\S]*?<\/style>/gi, ' ');
  const title = (stripped.match(/<title>([^<]{1,200})<\/title>/i) || [])[1] || '';
  const text = stripped.replace(/<[^>]+>/g, ' ');
  const built = signalsFrom([{ name: 'page', rel: 'page', text }]);
  return listingPayload({
    name: title.trim() || url.hostname,
    summary: URL_SUMMARY,
    ...built,
    offer_link_health: ok ? 'ok' : 'broken',
  });
}

export async function scanTarget(target, io) {
  if (/^https?:\/\//i.test(target || '')) return scanURL(target, io);
  return scanPath(target, io.cwd);
}

export function formatPayload(payload) {
  return JSON.stringify(payload, null, 2);
}
