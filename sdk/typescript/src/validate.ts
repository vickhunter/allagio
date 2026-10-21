import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Ajv2020, type ErrorObject, type ValidateFunction } from 'ajv/dist/2020.js';
import formats from 'ajv-formats';
import type { Constraints, Discovery, InboxPage, Message } from './types.ts';

export const SCHEMA_BASE = 'https://offerrails.com/orp/schema/0.3/';
export const SCHEMA_DIR = join(dirname(fileURLToPath(import.meta.url)), '..', '..', '..', 'schema', '0.3');

function schemaFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return schemaFiles(path);
    return entry.name.endsWith('.schema.json') ? [path] : [];
  });
}

const ajv = new Ajv2020({ allErrors: true, strict: true, strictRequired: false });
formats.default(ajv);
for (const file of schemaFiles(SCHEMA_DIR)) ajv.addSchema(JSON.parse(readFileSync(file, 'utf8')));

function compiled(path: string): ValidateFunction {
  const validate = ajv.getSchema(SCHEMA_BASE + path);
  if (!validate) throw new Error(`schema ${path} not found in ${SCHEMA_DIR}`);
  return validate;
}

const validators = {
  message: compiled('message.schema.json'),
  discovery: compiled('discovery.schema.json'),
  constraints: compiled('constraints.schema.json'),
  inbox: compiled('http/inbox.schema.json'),
};

export type SchemaName = keyof typeof validators;

export interface Problem {
  path: string;
  message: string;
}

export type Checked<T> = { ok: true; value: T } | { ok: false; problems: Problem[] };

function toProblems(errors: ErrorObject[] | null | undefined): Problem[] {
  return (errors ?? []).map((error) => ({ path: error.instancePath || '/', message: error.message ?? error.keyword }));
}

function check<T>(name: SchemaName, value: unknown): Checked<T> {
  const validate = validators[name];
  return validate(value) ? { ok: true, value: value as T } : { ok: false, problems: toProblems(validate.errors) };
}

export const checkMessage = (value: unknown) => check<Message>('message', value);
export const checkDiscovery = (value: unknown) => check<Discovery>('discovery', value);
export const checkConstraints = (value: unknown) => check<Constraints>('constraints', value);
export const checkInbox = (value: unknown) => check<InboxPage>('inbox', value);

export function assertMessage(value: unknown): Message {
  const result = checkMessage(value);
  if (!result.ok) throw new TypeError(`invalid ORP message: ${result.problems.map((p) => `${p.path} ${p.message}`).join('; ')}`);
  return result.value;
}
