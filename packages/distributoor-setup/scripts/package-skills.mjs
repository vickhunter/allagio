import { cp, mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const target = new URL('../skills/', import.meta.url);
await mkdir(target, { recursive: true });
for (const name of ['distributoor-onboard', 'distributoor-instrument', 'distributoor-launch']) {
  await cp(fileURLToPath(new URL(`../../../distributoor-plugin/skills/${name}`, import.meta.url)), fileURLToPath(new URL(name, target)), { recursive: true });
}
