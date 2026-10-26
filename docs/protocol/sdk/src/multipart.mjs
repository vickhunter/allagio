import { digest } from './jcs.mjs';

export const FRESH_MS = 90 * 24 * 60 * 60 * 1000;
const RESERVED_HOLDERS = new Set(['host', 'offerrails', 'hostedapi']);
const PII_KEYS = new Set(['email', 'name', 'phone', 'address', 'given_name', 'family_name', 'tax_id', 'ssn', 'dob']);

export function holderKey(value) {
  return String(value || '').toLowerCase().replace(/[\s\-_.]/g, '');
}

export function expiresFrom(checkedAtIso) {
  return new Date(Date.parse(checkedAtIso) + FRESH_MS).toISOString();
}

export function canonicalParty(seat) {
  const item = { id: seat.id, role: seat.role, required: Boolean(seat.required) };
  if (seat.terms && Object.keys(seat.terms).length) item.terms = seat.terms;
  if (seat.identity) item.identity = seat.identity;
  return item;
}

export function dealDocument(terms, parties) {
  return { terms, parties: parties.map(canonicalParty) };
}

export function dealHash(terms, parties) {
  return digest(dealDocument(terms, parties));
}

function hasOwn(object, key) {
  return Object.prototype.hasOwnProperty.call(object, key);
}

export function validateParties(parties, { hostId = '' } = {}) {
  if (!Array.isArray(parties) || parties.length < 2 || parties.length > 16) {
    return { ok: false, field: '/body/parties', message: 'A deal has 2 to 16 parties.' };
  }
  const ids = new Set();
  let percentSum = 0;
  let percentCount = 0;
  let fixedCount = 0;
  let offerers = 0;
  let acceptors = 0;
  let required = 0;
  for (let index = 0; index < parties.length; index += 1) {
    const party = parties[index];
    const field = `/body/parties/${index}`;
    if (!party || typeof party !== 'object') return { ok: false, field, message: 'A party is an object.' };
    if (ids.has(party.id)) return { ok: false, field: `${field}/id`, message: 'Party ids are unique.' };
    ids.add(party.id);
    if (party.required) required += 1;
    if (party.role === 'offerer') offerers += 1;
    if (party.role === 'acceptor') acceptors += 1;
    if (offerers > 1 || acceptors > 1) return { ok: false, field, message: 'A deal has at most one offerer and one acceptor.' };
    for (const key of Object.keys(party)) {
      if (PII_KEYS.has(key)) return { ok: false, field: `${field}/${key}`, message: 'A deal stores no personal data.' };
    }
    if (party.identity) {
      const keys = Object.keys(party.identity);
      if (keys.some((key) => key !== 'hash' && key !== 'issuer')) {
        return { ok: false, field: `${field}/identity`, message: 'Identity is a hash and an issuer name only.' };
      }
    }
    const terms = party.terms || {};
    for (const key of Object.keys(terms)) {
      if (PII_KEYS.has(key)) return { ok: false, field: `${field}/terms/${key}`, message: 'A deal stores no personal data.' };
    }
    if (terms.deposit) {
      const holder = holderKey(terms.deposit.holder);
      if (!holder || RESERVED_HOLDERS.has(holder) || (hostId && holder === holderKey(hostId))) {
        return { ok: false, field: `${field}/terms/deposit/holder`, message: 'Offer Rails does not hold money. A deposit term names an outside holder.' };
      }
    }
    const percent = hasOwn(terms, 'percent');
    const fixed = Boolean(terms.amount);
    if (percent) {
      if (!Number.isInteger(terms.percent) || terms.percent < 0 || terms.percent > 100) {
        return { ok: false, field: `${field}/terms/percent`, message: 'A percent split is an integer from 0 to 100.' };
      }
      percentCount += 1;
      percentSum += terms.percent;
    }
    if (fixed) fixedCount += 1;
    if (percent && fixed && !terms.cap) {
      return { ok: false, field: `${field}/terms/cap`, message: 'A mixed percent and fixed split needs a cap on each percent term.' };
    }
  }
  if (!required) return { ok: false, field: '/body/parties', message: 'A deal needs a required party.' };
  if (percentCount && fixedCount) {
    for (let index = 0; index < parties.length; index += 1) {
      const terms = parties[index].terms || {};
      if (hasOwn(terms, 'percent') && !terms.cap) {
        return { ok: false, field: `/body/parties/${index}/terms/cap`, message: 'A mixed percent and fixed split needs a cap on each percent term.' };
      }
    }
    if (percentSum > 100) {
      return { ok: false, field: '/body/parties', message: 'Percent splits over 100 are rejected.' };
    }
    return { ok: true };
  }
  if (percentCount) {
    if (percentSum !== 100) {
      return { ok: false, field: '/body/parties', message: 'Percent splits must sum to 100.' };
    }
    return { ok: true };
  }
  return { ok: true };
}

export function signatureState(parties, signedIds, vacancies = []) {
  const signed = new Set(signedIds);
  const ordered = [...signed].sort();
  if (vacancies.length) return { state: 'pending', signed: ordered };
  const required = parties.filter((party) => party.required);
  const state = required.length > 0 && required.every((party) => signed.has(party.id)) ? 'active' : 'pending';
  return { state, signed: ordered };
}

export function foldSigns(parties, order) {
  const signed = [];
  for (const id of order) {
    if (parties.some((party) => party.id === id) && !signed.includes(id)) signed.push(id);
  }
  return signatureState(parties, signed, []);
}

export function normalizeClaims(terms, nowIso, checkedKeys) {
  const next = structuredClone(terms);
  const expires = expiresFrom(nowIso);
  next.checked_at = nowIso;
  next.expires_at = expires;
  if (!next.claims) return next;
  const claims = {};
  for (const [key, value] of Object.entries(next.claims)) {
    if (checkedKeys.has(key) && value !== 'unknown') {
      const raw = value && typeof value === 'object' && hasOwn(value, 'value') ? value.value : value;
      if (raw === 'unknown' || raw === false || raw === 0 || raw === '') claims[key] = 'unknown';
      else claims[key] = { value: raw, checked_at: nowIso, expires_at: expires };
    } else {
      claims[key] = 'unknown';
    }
  }
  next.claims = claims;
  return next;
}
