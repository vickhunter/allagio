import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';

const here = dirname(fileURLToPath(import.meta.url));
const schemaRoot = join(here, '..', '..', 'schema', '0.3');
const BASE = 'https://offerrails.com/orp/schema/0.3/';

function walk(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return walk(path);
    return entry.name.endsWith('.schema.json') ? [path] : [];
  });
}

const ajv = new Ajv2020({
  allErrors: true,
  strict: false,
  validateSchema: false,
});
addFormats(ajv);
for (const file of walk(schemaRoot)) ajv.addSchema(JSON.parse(readFileSync(file, 'utf8')));

function validator(name) {
  const validate = ajv.getSchema(`${BASE}${name}`);
  if (!validate) throw new Error(`missing schema ${name}`);
  return validate;
}

export const validators = {
  message: validator('message.schema.json'),
  discovery: validator('discovery.schema.json'),
  constraints: validator('constraints.schema.json'),
  'suggest-request': validator('http/suggest-request.schema.json'),
  'suggest-response': validator('http/suggest-response.schema.json'),
  'join-request': validator('http/join-request.schema.json'),
  'join-response': validator('http/join-response.schema.json'),
  inbox: validator('http/inbox.schema.json'),
};

export function problemPointer(errors) {
  const error = errors?.[0];
  if (!error) return '';
  let pointer = error.instancePath || '';
  const missing = error.params?.missingProperty;
  const extra = error.params?.additionalProperty;
  if (missing) pointer = `${pointer}/${missing}`;
  else if (extra && (error.keyword === 'additionalProperties' || error.keyword === 'propertyNames')) pointer = `${pointer}/${extra}`;
  return pointer;
}

export function check(kind, value) {
  const validate = validators[kind];
  const ok = validate(value);
  return { ok: ok === true, errors: ok ? [] : validate.errors, field: ok ? '' : problemPointer(validate.errors) };
}
