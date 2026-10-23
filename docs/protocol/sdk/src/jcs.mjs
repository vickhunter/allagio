import { createHash } from 'node:crypto';

// RFC 8785 (JCS) for the integers, strings, booleans, nulls, arrays and objects ORP uses.
// Keep this in step with canonicalize() in schema/test/validate.test.mjs.
export function canonicalize(value) {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalize).join(',')}]`;
  const keys = Object.keys(value).filter((key) => value[key] !== undefined).sort();
  return `{${keys.map((key) => `${JSON.stringify(key)}:${canonicalize(value[key])}`).join(',')}}`;
}

export function digest(value) {
  return `sha256-${createHash('sha256').update(canonicalize(value), 'utf8').digest('base64url')}`;
}

export const termsHash = (terms) => digest(terms);
