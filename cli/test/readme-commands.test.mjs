import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { REQUIRED_COMMANDS, readmeCommands } from '../scripts/readme-commands.mjs';

const script = fileURLToPath(new URL('../scripts/readme-commands.mjs', import.meta.url));
const readme = fs.readFileSync(new URL('../README.md', import.meta.url), 'utf8');

function fenced(commands) {
  return `<!-- paste:begin -->\n\`\`\`bash\n${commands.join('\n')}\n\`\`\`\n<!-- paste:end -->\n`;
}

test('the documented quickstart is the six local commands', () => {
  const result = readmeCommands(readme);
  assert.equal(result.ok, true);
  assert.deepEqual(result.lines, REQUIRED_COMMANDS);
  assert.equal(readme.includes('npx offer-rails'), false);
  const printed = execFileSync(process.execPath, [script, fileURLToPath(new URL('../README.md', import.meta.url))], { encoding: 'utf8' });
  assert.equal(printed.trim().split('\n').length, 6);
});

test('a missing documented command fails', () => {
  const result = readmeCommands(fenced(REQUIRED_COMMANDS.slice(0, 5)));
  assert.equal(result.ok, false);
  assert.equal(result.found, 5);
  assert.match(result.error, /expected 6 README lines, found 5/);
});

test('an npx command fails', () => {
  const wrong = REQUIRED_COMMANDS.map((line, index) => (index === 0 ? 'npx offer-rails login' : line));
  const result = readmeCommands(fenced(wrong));
  assert.equal(result.ok, false);
  assert.match(result.error, /paste block claims npx/);
  assert.match(result.error, /expected 6 README lines, found 5/);
});

test('a different local command fails', () => {
  const wrong = REQUIRED_COMMANDS.map((line, index) => (index === 2 ? 'node cli/bin/offer-rails.mjs status' : line));
  const result = readmeCommands(fenced(wrong));
  assert.equal(result.ok, false);
  assert.match(result.error, /expected 6 README lines, found 5/);
  assert.equal(result.error.includes('npx'), false);
});

test('the valid quickstart completes from a temp copy', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'readme-commands-'));
  const file = path.join(dir, 'README.md');
  fs.writeFileSync(file, readme);
  const printed = execFileSync(process.execPath, [script, file], { encoding: 'utf8' });
  assert.deepEqual(printed.trim().split('\n'), REQUIRED_COMMANDS);
});
