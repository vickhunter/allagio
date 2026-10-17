import { cp, mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { skillNames } from '../lib/setup.mjs';

const target = new URL('../skills/', import.meta.url);
await mkdir(target, { recursive: true });
for (const name of skillNames) {
  await cp(fileURLToPath(new URL(`../../../distributoor-plugin/skills/${name}`, import.meta.url)), fileURLToPath(new URL(name, target)), { recursive: true });
}
