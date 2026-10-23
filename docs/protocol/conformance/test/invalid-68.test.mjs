import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { check } from '../../sdk/src/index.mjs';
import { OrpClient, codeOf, party, withHost } from './helpers.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const invalidDir = join(here, '..', '..', 'schema', 'examples', 'invalid');
const SHAPES = ['envelope-68-shape.json', 'version-semver.json', 'party-id-role-prefix-68.json'];

test('INVALID-68', async () => {
  await withHost({ registerPage: false }, async (host) => {
    const maker = await party(host);
    for (const name of SHAPES) {
      const example = JSON.parse(readFileSync(join(invalidDir, name), 'utf8'));
      const verdict = check('message', example.instance);
      assert.equal(verdict.ok, false, name);
      const res = await OrpClient.post(host.origin, maker.token, example.instance);
      assert.equal(res.status, 400, `${name} ${res.text}`);
      assert.equal(codeOf(res), 'invalid_message', name);
    }
    assert.equal(host.deals.size, 0);
    assert.equal(host.offers.size, 0);
  });
});
