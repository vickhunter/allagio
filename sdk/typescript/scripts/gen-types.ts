// Generates src/types.ts from ../../schema/0.3. With --check, fails if the
// committed file differs, so the types cannot drift from the schemas.
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { compile, type JSONSchema } from 'json-schema-to-typescript';

const here = dirname(fileURLToPath(import.meta.url));
const schemaRoot = join(here, '..', '..', '..', 'schema', '0.3');
const target = join(here, '..', 'src', 'types.ts');

export const CORE_TYPES = ['hello', 'discover', 'results', 'ask', 'offer', 'counter', 'accept', 'decline', 'deal', 'execute', 'receipt', 'outcome', 'revoke', 'consent', 'dispute', 'reputation', 'error'] as const;

type Json = { [key: string]: unknown };
const read = (path: string): Json => JSON.parse(readFileSync(path, 'utf8')) as Json;
const titleOf = (type: string) => `${type[0]!.toUpperCase()}${type.slice(1)}Message`;

// A branch that only says which fields are required changes validation, not
// shape. Such oneOf/anyOf/if rules stay in the schemas and Ajv enforces them;
// the types leave them out rather than widen to index signatures.
function requiredOnly(node: unknown): boolean {
  if (!node || typeof node !== 'object' || Array.isArray(node)) return false;
  return Object.entries(node as Json).every(([key, value]) => {
    if (key === 'required' || key === 'type') return true;
    if (key === 'properties') return Object.values(value as Json).every(requiredOnly);
    return false;
  });
}

function localRef(value: string): Json {
  const local = value.match(/^(?:\.\.\/|)defs\.schema\.json#\/\$defs\/(\w+)$/) ?? value.match(/^#\/\$defs\/(\w+)$/);
  if (local) return { $ref: `#/$defs/${local[1]}` };
  if (value === '../envelope.schema.json' || value === 'envelope.schema.json') return { $ref: '#/$defs/Envelope' };
  if (value === '../message.schema.json') return { tsType: 'Message' };
  throw new Error(`unhandled $ref ${value}`);
}

// Every cross-file $ref becomes a local #/$defs ref in one combined schema. A
// $ref's sibling keywords are dropped so the generator reuses the named type.
function rewrite(node: unknown): unknown {
  if (Array.isArray(node)) return node.map(rewrite);
  if (!node || typeof node !== 'object') return node;
  const source = node as Json;
  if (typeof source.$ref === 'string') return localRef(source.$ref);
  const out: Json = {};
  for (const [key, value] of Object.entries(source)) {
    if (key === '$id' || key === '$schema' || key === 'if' || key === 'then' || key === 'else') continue;
    if ((key === 'oneOf' || key === 'anyOf') && (value as unknown[]).every(requiredOnly)) continue;
    if (key === 'allOf') {
      const kept = (value as unknown[]).filter((branch) => !(branch && typeof branch === 'object' && 'if' in branch));
      if (kept.length) out.allOf = rewrite(kept);
      continue;
    }
    out[key] = rewrite(value);
  }
  return out;
}

function addDefs(into: Json, defs: Json | undefined) {
  for (const [name, schema] of Object.entries(defs ?? {})) {
    if (name in into) throw new Error(`duplicate definition ${name}`);
    into[name] = rewrite(schema);
  }
}

export async function generate(): Promise<string> {
  const defs: Json = {};
  addDefs(defs, read(join(schemaRoot, 'defs.schema.json')).$defs as Json);
  const { $defs: _none, ...envelope } = read(join(schemaRoot, 'envelope.schema.json'));
  const envelopeDef = rewrite(envelope) as { properties: Json };
  envelopeDef.properties.body = { tsType: 'object' };
  defs.Envelope = envelopeDef;
  for (const type of CORE_TYPES) {
    const { $defs: bodyDefs, ...message } = read(join(schemaRoot, 'messages', `${type}.schema.json`));
    addDefs(defs, bodyDefs as Json);
    defs[titleOf(type)] = rewrite(message);
  }
  for (const file of ['discovery.schema.json', 'constraints.schema.json', 'http/inbox.schema.json']) {
    const { $defs: localDefs, ...schema } = read(join(schemaRoot, file));
    addDefs(defs, localDefs as Json);
    defs[schema.title as string] = rewrite(schema);
  }
  const root = { title: 'OrpSchemas', type: 'object', additionalProperties: false, properties: {}, $defs: defs } as unknown as JSONSchema;
  const body = await compile(root, 'OrpSchemas', {
    bannerComment: '/* Generated from schema/0.3 by scripts/gen-types.ts. Do not edit; run npm run gen. */',
    additionalProperties: false,
    unreachableDefinitions: true,
    strictIndexSignatures: true,
    ignoreMinAndMaxItems: true,
    format: true,
    style: { singleQuote: true, printWidth: 120 },
  });
  const union = `\nexport type Message =\n${CORE_TYPES.map((type) => `  | ${titleOf(type)}`).join('\n')};\n`;
  return body.replace(/export interface OrpSchemas \{\}\n?/, '') + union;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const next = await generate();
  if (process.argv.includes('--check')) {
    const current = readFileSync(target, 'utf8');
    if (current !== next) {
      console.error('src/types.ts is out of date with schema/0.3. Run: npm run gen');
      process.exit(1);
    }
    console.log('src/types.ts matches schema/0.3');
  } else {
    writeFileSync(target, next);
    console.log(`wrote ${target}`);
  }
}
