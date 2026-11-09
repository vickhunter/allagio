import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

function real(file) {
  return fs.realpathSync(file);
}

export function insideRoot(file, root) {
  const target = real(file);
  const base = real(root);
  return target === base || target.startsWith(`${base}${path.sep}`);
}

export function assertInstalledCommand({ entry, lib, installedRoot, sourceRoot }) {
  if (!insideRoot(entry, installedRoot) || !insideRoot(lib, installedRoot)) {
    return { ok: false, error: 'command does not resolve inside the installed package' };
  }
  if (insideRoot(entry, sourceRoot) || insideRoot(lib, sourceRoot)) {
    return { ok: false, error: 'command falls back to source' };
  }
  return { ok: true, entry: real(entry), lib: real(lib) };
}

function flag(args, name) {
  const index = args.indexOf(name);
  if (index < 0 || !args[index + 1]) {
    process.stderr.write(`missing ${name}\n`);
    process.exit(1);
  }
  return args[index + 1];
}

function main() {
  const args = process.argv.slice(2);
  const result = assertInstalledCommand({
    entry: flag(args, '--entry'),
    lib: flag(args, '--lib'),
    installedRoot: flag(args, '--installed'),
    sourceRoot: flag(args, '--source'),
  });
  if (!result.ok) {
    process.stderr.write(`${result.error}\n`);
    process.exit(1);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main();
}
