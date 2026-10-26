import { createHash } from 'node:crypto';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';

const here = dirname(fileURLToPath(import.meta.url));
const schemaRoot = join(here, '..', '0.3');
const examplesRoot = join(here, '..', 'examples');
const specRoot = join(here, '..', '..');
const BASE = 'https://offer-rails.onrender.com/orp/schemas/';

const CORE_TYPES = ['hello', 'discover', 'results', 'space', 'request', 'offer', 'counter', 'accept', 'decline', 'deal', 'execute', 'receipt', 'outcome', 'payout', 'revoke', 'consent', 'dispute', 'reputation', 'error'];

function walk(dir, suffix) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return walk(path, suffix);
    return entry.name.endsWith(suffix) ? [path] : [];
  });
}

const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'));

const warnings = [];
const ajv = new Ajv2020({
  allErrors: true,
  strict: true,
  strictRequired: false,
  logger: { log() {}, warn: (...args) => warnings.push(args.join(' ')), error: (...args) => warnings.push(args.join(' ')) },
});
addFormats(ajv);

const schemaFiles = walk(schemaRoot, '.schema.json');
for (const file of schemaFiles) ajv.addSchema(readJson(file));

const HTTP = ['inbox', 'suggest-request', 'suggest-response', 'join-request', 'join-response'];
const validators = {
  message: ajv.getSchema(`${BASE}message.schema.json`),
  discovery: ajv.getSchema(`${BASE}discovery.schema.json`),
  constraints: ajv.getSchema(`${BASE}constraints.schema.json`),
  ...Object.fromEntries(HTTP.map((name) => [name, ajv.getSchema(`${BASE}http/${name}.schema.json`)])),
};

function explain(validate) {
  return JSON.stringify(validate.errors?.slice(0, 5), null, 2);
}

test('every schema compiles in strict mode without warnings', () => {
  for (const file of schemaFiles) {
    const { $id } = readJson(file);
    assert.ok($id?.startsWith(BASE), `${file} has $id under ${BASE}`);
    assert.ok(ajv.getSchema($id), `${file} compiles`);
  }
  assert.deepEqual(warnings, []);
});

test('every core message type has its own schema', () => {
  for (const type of CORE_TYPES) {
    const schema = readJson(join(schemaRoot, 'messages', `${type}.schema.json`));
    assert.equal(schema.properties.type.const, type);
  }
  const declared = readJson(join(schemaRoot, 'defs.schema.json')).$defs.MessageType.anyOf[0].enum;
  assert.deepEqual([...declared].sort(), [...CORE_TYPES].sort());
});

test('valid message examples pass, and every core type has one', () => {
  const seen = new Set();
  for (const file of walk(join(examplesRoot, 'messages'), '.json')) {
    const message = readJson(file);
    assert.ok(validators.message(message), `${file}\n${explain(validators.message)}`);
    seen.add(message.type);
  }
  for (const type of CORE_TYPES) assert.ok(seen.has(type), `no valid example for ${type}`);
});

test('discovery, constraints, inbox, suggest and join examples pass', () => {
  for (const [name, kind] of [['discovery.json', 'discovery'], ['constraints.json', 'constraints'], ['inbox.json', 'inbox']]) {
    const validate = validators[kind];
    assert.ok(validate(readJson(join(examplesRoot, name))), `${name}\n${explain(validate)}`);
  }
  for (const name of HTTP.filter((n) => n !== 'inbox')) {
    const validate = validators[name];
    assert.ok(validate(readJson(join(examplesRoot, 'http', `${name}.json`))), `http/${name}.json\n${explain(validate)}`);
  }
});

test('joining asks for nothing personal: an empty request is complete', () => {
  assert.ok(validators['join-request']({}));
  const { properties } = readJson(join(schemaRoot, 'http', 'join-request.schema.json'));
  for (const field of Object.keys(properties)) assert.ok(['agent', 'key'].includes(field), `join-request has no ${field} field`);
});

test('invalid examples fail at the expected place', () => {
  const files = walk(join(examplesRoot, 'invalid'), '.json');
  assert.ok(files.length >= 10);
  for (const file of files) {
    const { why, schema, path, instance } = readJson(file);
    const validate = validators[schema];
    assert.ok(why && validate, `${file} names why and a known schema`);
    assert.equal(validate(instance), false, `${file} should fail: ${why}`);
    const paths = validate.errors.map((error) => error.instancePath);
    assert.ok(paths.some((p) => p === path || p.startsWith(`${path}/`)), `${file} failed elsewhere: ${paths.join(', ')}`);
  }
});

// RFC 8785 (JCS) for the integers, strings, booleans, nulls, arrays and objects ORP uses.
function canonicalize(value) {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalize).join(',')}]`;
  return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonicalize(value[key])}`).join(',')}}`;
}
const termsHash = (terms) => `sha256-${createHash('sha256').update(canonicalize(terms), 'utf8').digest('base64url')}`;

function* termsWithHash(doc) {
  if (doc === null || typeof doc !== 'object') return;
  if (doc.terms && typeof doc.terms_hash === 'string') yield doc;
  for (const value of Object.values(doc)) yield* termsWithHash(value);
}

test('every terms_hash next to its terms is the JCS SHA-256 of those terms', () => {
  let checked = 0;
  const files = [...walk(join(examplesRoot, 'messages'), '.json'), ...walk(join(examplesRoot, 'http'), '.json')];
  for (const file of files) {
    for (const holder of termsWithHash(readJson(file))) {
      assert.equal(holder.terms_hash, termsHash(holder.terms), `${file}: terms_hash does not match its terms`);
      checked += 1;
    }
  }
  assert.ok(checked >= 10);
});

// Every ```json block in docs/protocol/*.md that looks like an ORP document must be valid.
// A block preceded by <!-- orp:invalid --> must be invalid instead, and is checked as a message
// when its shape is not recognised. <!-- orp:skip --> skips a block.
test('JSON blocks in docs/protocol/*.md validate', () => {
  if (!existsSync(specRoot)) return;
  let checked = 0;
  const specFiles = readdirSync(specRoot).filter((name) => name.endsWith('.md')).map((name) => join(specRoot, name));
  for (const file of specFiles) {
    const text = readFileSync(file, 'utf8');
    for (const match of text.matchAll(/(<!-- orp:(invalid|skip|0\.4) -->\s*)?```json\n([\s\S]*?)```/g)) {
      const marker = match[2];
      if (marker === 'skip' || marker === '0.4') continue;
      let doc;
      try {
        doc = JSON.parse(match[3]);
      } catch (error) {
        assert.fail(`${file}: a json block does not parse: ${error.message}\n${match[3].slice(0, 200)}`);
      }
      const kind =
        typeof doc.orp === 'string' && doc.type ? 'message'
        : Array.isArray(doc.orp) ? 'discovery'
        : doc.token && doc.claim ? 'join-response'
        : Array.isArray(doc.items) && doc.join ? 'suggest-response'
        : typeof doc.text === 'string' ? 'suggest-request'
        : doc.steps ? 'constraints'
        : Array.isArray(doc.messages) ? 'inbox'
        : marker === 'invalid' ? 'message'
        : null;
      if (!kind) continue;
      for (const holder of termsWithHash(doc)) assert.equal(holder.terms_hash, termsHash(holder.terms), `${file}: a terms_hash in a json block does not match its terms`);
      const ok = validators[kind](doc);
      if (marker === 'invalid') assert.equal(ok, false, `${file}: block marked invalid passed`);
      else assert.ok(ok, `${file}: ${kind} block fails\n${match[3].slice(0, 300)}\n${explain(validators[kind])}`);
      checked += 1;
    }
  }
  assert.ok(checked > 0, 'the spec has json examples');
});
