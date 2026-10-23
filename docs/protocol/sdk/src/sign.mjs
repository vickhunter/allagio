import { createPrivateKey, createPublicKey, generateKeyPairSync, sign, verify } from 'node:crypto';
import { canonicalize } from './jcs.mjs';

export function generateEd25519(kid) {
  const { publicKey, privateKey } = generateKeyPairSync('ed25519');
  const pub = publicKey.export({ format: 'jwk' });
  const priv = privateKey.export({ format: 'jwk' });
  const jwk = {
    kty: 'OKP',
    crv: 'Ed25519',
    x: pub.x,
    d: priv.d,
    kid,
    alg: 'EdDSA',
    use: 'sig',
  };
  return jwk;
}

export function publicJwk(jwk) {
  return { kty: 'OKP', crv: 'Ed25519', x: jwk.x, kid: jwk.kid, alg: 'EdDSA', use: 'sig' };
}

function privateKeyFrom(jwk) {
  return createPrivateKey({ key: jwk, format: 'jwk' });
}

function publicKeyFrom(jwk) {
  return createPublicKey({ key: { kty: 'OKP', crv: 'Ed25519', x: jwk.x }, format: 'jwk' });
}

export function signEnvelope(message, jwk) {
  const unsigned = { ...message };
  delete unsigned.sig;
  const header = Buffer.from(JSON.stringify({ alg: 'EdDSA', kid: jwk.kid })).toString('base64url');
  const payload = Buffer.from(canonicalize(unsigned));
  const sig = sign(null, payload, privateKeyFrom(jwk)).toString('base64url');
  return { ...unsigned, sig: `${header}..${sig}` };
}

export function verifyEnvelope(message, jwk) {
  if (!jwk?.x || typeof message?.sig !== 'string') return false;
  const parts = message.sig.split('.');
  if (parts.length !== 3 || parts[1] !== '') return false;
  const unsigned = { ...message };
  delete unsigned.sig;
  try {
    return verify(null, Buffer.from(canonicalize(unsigned)), publicKeyFrom(jwk), Buffer.from(parts[2], 'base64url'));
  } catch {
    return false;
  }
}
