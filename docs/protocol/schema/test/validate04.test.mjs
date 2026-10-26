import { createHash } from 'node:crypto';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';

const here = dirname(fileURLToPath(import.meta.url));
const schema03 = join(here, '..', '0.3');
const schema04 = join(here, '..', '0.4');
const examples04 = join(here, '..', 'examples', '0.4');
const specRoot = join(here, '..', '..');
const BASE = 'https://offer-rails.onrender.com/orp/schemas/';

function walk(dir, suffix) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return walk(path, suffix);
    return entry.name.endsWith(suffix) ? [path] : [];
  });
}

const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'));

function canonicalize(value) {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalize).join(',')}]`;
  return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonicalize(value[key])}`).join(',')}}`;
}
const digest = (value) => `sha256-${createHash('sha256').update(canonicalize(value), 'utf8').digest('base64url')}`;

const warnings = [];
const ajv = new Ajv2020({
  allErrors: true,
  strict: true,
  strictRequired: false,
  logger: { log() {}, warn: (...args) => warnings.push(args.join(' ')), error: (...args) => warnings.push(args.join(' ')) },
});
addFormats(ajv);
for (const file of [...walk(schema03, '.schema.json'), ...walk(schema04, '.schema.json')]) ajv.addSchema(readJson(file));

function explain(validate) {
  return JSON.stringify(validate.errors?.slice(0, 6), null, 2);
}

test('0.4 schemas compile in strict mode', () => {
  for (const file of walk(schema04, '.schema.json')) {
    const { $id } = readJson(file);
    assert.ok($id?.startsWith(`${BASE}0.4/`), `${file} $id`);
    assert.equal($id, `${BASE}0.4/${file.slice(schema04.length + 1).split('\\').join('/')}`);
    assert.ok(ajv.getSchema($id), `${file} compiles`);
  }
  assert.deepEqual(warnings, []);
});

test('0.4 examples match their schemas', () => {
  const groups = [
    ['referral-chain.json', 'message'],
    ['link-swap-ring.json', 'message'],
    ['five-party-home-sale.json', 'message'],
    ['bad-split.json', 'message'],
    ['mediator-left.json', 'message'],
  ];
  for (const [name, kind] of groups) {
    const file = join(examples04, name);
    const doc = readJson(file);
    const validate = ajv.getSchema(kind === 'message' ? `${BASE}0.4/message.schema.json` : `${BASE}${kind}`);
    assert.ok(validate(doc), `${name}\n${explain(validate)}`);
    if (doc.type === 'deal') {
      const seats = doc.body.parties.map(({ id, role, required, terms, identity }) => {
        const seat = { id, role, required };
        if (terms) seat.terms = terms;
        if (identity) seat.identity = identity;
        return seat;
      });
      assert.equal(doc.body.terms_hash, digest({ terms: doc.body.terms, parties: seats }), `${name} terms_hash`);
    }
  }
  const bad = readJson(join(examples04, 'bad-split.json'));
  const percents = bad.body.parties.reduce((sum, seat) => sum + (seat.terms?.percent || 0), 0);
  assert.equal(percents, 101);
  const left = readJson(join(examples04, 'mediator-left.json'));
  assert.equal(left.body.state, 'pending');
  assert.ok(left.body.vacancies.includes('facilitator'));
});

test('0.4 invalid examples fail schema', () => {
  const validate = ajv.getSchema(`${BASE}0.4/message.schema.json`);
  for (const file of walk(join(examples04, 'invalid-schema'), '.json')) {
    const { why, path, instance } = readJson(file);
    assert.equal(validate(instance), false, `${file} should fail: ${why}`);
    const paths = validate.errors.map((error) => error.instancePath || '');
    assert.ok(paths.some((item) => item === path || item.startsWith(`${path}/`) || path.startsWith(item)), `${file} failed elsewhere: ${paths.join(', ')}`);
  }
});

test('JSON blocks marked orp:0.4 validate as 0.4', () => {
  const validate = ajv.getSchema(`${BASE}0.4/message.schema.json`);
  let checked = 0;
  for (const name of readdirSync(specRoot).filter((item) => item.endsWith('.md'))) {
    const text = readFileSync(join(specRoot, name), 'utf8');
    for (const match of text.matchAll(/<!-- orp:0\.4 -->\s*```json\n([\s\S]*?)```/g)) {
      const doc = JSON.parse(match[1]);
      assert.ok(validate(doc), `${name}\n${explain(validate)}`);
      checked += 1;
    }
  }
  assert.ok(checked > 0);
});
