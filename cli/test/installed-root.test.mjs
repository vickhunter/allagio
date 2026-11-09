import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { assertInstalledCommand } from '../scripts/installed-root.mjs';

const script = fileURLToPath(new URL('../scripts/installed-root.mjs', import.meta.url));

function tree(root) {
  fs.mkdirSync(path.join(root, 'bin'), { recursive: true });
  fs.mkdirSync(path.join(root, 'lib'), { recursive: true });
  fs.writeFileSync(path.join(root, 'bin', 'offer-rails.mjs'), '#!/usr/bin/env node\n');
  fs.writeFileSync(path.join(root, 'lib', 'commands.mjs'), 'export {}\n');
}

test('an installed tree is accepted and a source symlink is rejected', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'installed-root-'));
  const source = path.join(dir, 'source');
  const installed = path.join(dir, 'installed');
  tree(source);
  tree(installed);
  const project = path.join(dir, 'project');
  fs.mkdirSync(project);
  fs.symlinkSync(installed, path.join(project, 'cli'));
  const ok = assertInstalledCommand({
    entry: path.join(project, 'cli', 'bin', 'offer-rails.mjs'),
    lib: path.join(project, 'cli', 'lib', 'commands.mjs'),
    installedRoot: installed,
    sourceRoot: source,
  });
  assert.equal(ok.ok, true);
  assert.equal(ok.entry.startsWith(fs.realpathSync(installed)), true);

  fs.rmSync(path.join(project, 'cli'));
  fs.symlinkSync(source, path.join(project, 'cli'));
  const refused = assertInstalledCommand({
    entry: path.join(project, 'cli', 'bin', 'offer-rails.mjs'),
    lib: path.join(project, 'cli', 'lib', 'commands.mjs'),
    installedRoot: installed,
    sourceRoot: source,
  });
  assert.equal(refused.ok, false);
  assert.match(refused.error, /falls back to source|does not resolve inside the installed package/);

  const printed = execFileSync(process.execPath, [
    script,
    '--entry', path.join(dir, 'installed', 'bin', 'offer-rails.mjs'),
    '--lib', path.join(dir, 'installed', 'lib', 'commands.mjs'),
    '--installed', installed,
    '--source', source,
  ], { encoding: 'utf8' });
  assert.equal(printed, '');
  assert.throws(() => execFileSync(process.execPath, [
    script,
    '--entry', path.join(project, 'cli', 'bin', 'offer-rails.mjs'),
    '--lib', path.join(project, 'cli', 'lib', 'commands.mjs'),
    '--installed', installed,
    '--source', source,
  ], { encoding: 'utf8' }));
});
