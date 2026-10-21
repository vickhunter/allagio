import { createHash } from 'node:crypto';
import { canonicalize } from './canonical.ts';
import type { Hash, Terms } from './types.ts';

export function sha256(text: string): Hash {
  return `sha256-${createHash('sha256').update(text, 'utf8').digest('base64url')}`;
}

export function hashJson(value: unknown): Hash {
  return sha256(canonicalize(value));
}

export function termsHash(terms: Terms): Hash {
  return hashJson(terms);
}

// The value a later message puts in prev: the hash of the earlier message
// without its signature, so the chain does not depend on signature encoding.
export function messageHash(message: { sig?: string }): Hash {
  const { sig: _sig, ...unsigned } = message;
  return hashJson(unsigned);
}
