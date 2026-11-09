import fs from 'node:fs';
import { pathToFileURL } from 'node:url';

export const REQUIRED_COMMANDS = [
  'node cli/bin/offer-rails.mjs login',
  'node cli/bin/offer-rails.mjs scan .',
  'node cli/bin/offer-rails.mjs list',
  'node cli/bin/offer-rails.mjs digest',
  'node cli/bin/offer-rails.mjs undo',
  'node cli/bin/offer-rails.mjs kill',
];

export function readmeCommands(text) {
  const parts = String(text).split('<!-- paste:begin -->');
  if (parts.length < 2 || !parts[1].includes('<!-- paste:end -->')) {
    return { ok: false, found: 0, lines: [], error: 'paste block missing' };
  }
  const block = parts[1].split('<!-- paste:end -->')[0];
  const lines = block.split('\n').map((line) => line.trim()).filter((line) => line && !line.startsWith('```') && !line.startsWith('<!--'));
  const found = lines.filter((line) => REQUIRED_COMMANDS.includes(line)).length;
  const claimsNpx = lines.some((line) => line.startsWith('npx ') || line.startsWith('npx\t'));
  const exact = lines.length === REQUIRED_COMMANDS.length && lines.every((line, index) => line === REQUIRED_COMMANDS[index]);
  if (!exact || claimsNpx) {
    const error = claimsNpx
      ? `expected 6 README lines, found ${found}; paste block claims npx`
      : `expected 6 README lines, found ${found}`;
    return { ok: false, found, lines, error };
  }
  return { ok: true, found, lines, error: '' };
}

function main() {
  const file = process.argv[2];
  if (!file) {
    process.stderr.write('readme file is required\n');
    process.exit(1);
  }
  const result = readmeCommands(fs.readFileSync(file, 'utf8'));
  if (!result.ok) {
    process.stderr.write(`${result.error}\n`);
    process.exit(1);
  }
  process.stdout.write(`${result.lines.join('\n')}\n`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main();
}
