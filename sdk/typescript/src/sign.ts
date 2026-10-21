// Detached compact JWS (RFC 7515 Appendix F) over the JCS form of a message
// without its sig field. Only EdDSA (Ed25519) is implemented here.
import { createPublicKey, generateKeyPairSync, sign as edSign, verify as edVerify, type JsonWebKey, type KeyObject } from 'node:crypto';
import { canonicalize } from './canonical.ts';

export interface SigningKey {
  kid: string;
  privateKey: KeyObject;
  publicJwk: JsonWebKey & { kid: string; alg: 'EdDSA'; use: 'sig' };
}

export function generateSigningKey(kid: string): SigningKey {
  const { privateKey, publicKey } = generateKeyPairSync('ed25519');
  return { kid, privateKey, publicJwk: { ...publicKey.export({ format: 'jwk' }), kid, alg: 'EdDSA', use: 'sig' } };
}

const b64 = (data: string | Buffer) => Buffer.from(data).toString('base64url');

function signingInput(protectedHeader: string, message: object): Buffer {
  const { sig: _sig, ...unsigned } = message as { sig?: string };
  return Buffer.from(`${protectedHeader}.${b64(canonicalize(unsigned))}`, 'ascii');
}

export function signMessage<T extends object>(message: T, key: SigningKey): T & { sig: string } {
  const header = b64(JSON.stringify({ alg: 'EdDSA', kid: key.kid }));
  const signature = edSign(null, signingInput(header, message), key.privateKey);
  return { ...message, sig: `${header}..${b64(signature)}` };
}

export type KeyResolver = (kid: string) => JsonWebKey | undefined;

export type Verification = { ok: true; kid: string } | { ok: false; reason: 'missing' | 'malformed' | 'unknown_key' | 'bad_signature' };

export function verifyMessage(message: { sig?: string }, resolve: KeyResolver): Verification {
  if (!message.sig) return { ok: false, reason: 'missing' };
  const [header, payload, signature] = message.sig.split('.');
  if (!header || payload !== '' || !signature) return { ok: false, reason: 'malformed' };
  let parsed: { alg?: string; kid?: string };
  try {
    parsed = JSON.parse(Buffer.from(header, 'base64url').toString('utf8')) as { alg?: string; kid?: string };
  } catch {
    return { ok: false, reason: 'malformed' };
  }
  if (parsed.alg !== 'EdDSA' || !parsed.kid) return { ok: false, reason: 'malformed' };
  const jwk = resolve(parsed.kid);
  if (!jwk) return { ok: false, reason: 'unknown_key' };
  const publicKey = createPublicKey({ key: jwk, format: 'jwk' });
  const ok = edVerify(null, signingInput(header, message), publicKey, Buffer.from(signature, 'base64url'));
  return ok ? { ok: true, kid: parsed.kid } : { ok: false, reason: 'bad_signature' };
}
