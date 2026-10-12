import fs from 'node:fs';
import path from 'node:path';

export function configPath(io) {
  if (io.env.OFFER_RAILS_CONFIG) return io.env.OFFER_RAILS_CONFIG;
  return path.join(io.cwd, '.offer-rails', 'config.json');
}

export function loadConfig(io) {
  const file = configPath(io);
  if (!fs.existsSync(file)) return {};
  return JSON.parse(fs.readFileSync(file, 'utf8'));
}

export function saveConfig(io, config) {
  const file = configPath(io);
  const dir = path.dirname(file);
  fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  fs.chmodSync(dir, 0o700);
  fs.writeFileSync(file, `${JSON.stringify(config, null, 2)}\n`, { mode: 0o600 });
  fs.chmodSync(file, 0o600);
  return file;
}

export function authHeaders(config) {
  if (!config.access_token || !config.key_header) {
    throw new Error('Run offer-rails login first.');
  }
  return { [config.key_header]: config.access_token };
}

export function offerFromScope(scope) {
  const match = /^offer:([0-9a-f-]{36})$/i.exec(scope || '');
  return match ? match[1] : '';
}

export function siteFromScope(scope) {
  const match = /^site:([0-9a-f-]{36})$/i.exec(scope || '');
  return match ? match[1] : '';
}
