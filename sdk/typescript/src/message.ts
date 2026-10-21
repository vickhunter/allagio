import { randomBytes } from 'node:crypto';
import type { Envelope, Message, PartyId } from './types.ts';

export const ORP_VERSION = '0.3';

export type MessageByType = { [M in Message as M['type']]: M };
export type CoreType = keyof MessageByType;
export type EnvelopeFields = Partial<Pick<Envelope, 'id' | 'ts' | 'to' | 'deal' | 're' | 'agent' | 'prev' | 'ext' | 'critical'>>;

// UUIDv7: time-ordered, so ids sort by creation and double as idempotency keys.
export function newId(): string {
  const bytes = randomBytes(16);
  bytes.writeUIntBE(Date.now(), 0, 6);
  bytes[6] = (bytes[6]! & 0x0f) | 0x70;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  const hex = bytes.toString('hex');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

export function now(): string {
  return new Date().toISOString().replace(/\.\d{3}Z$/, 'Z');
}

export function message<T extends CoreType>(type: T, from: PartyId, body: MessageByType[T]['body'], fields: EnvelopeFields = {}): MessageByType[T] {
  const { id = newId(), ts = now(), ...rest } = fields;
  return { orp: ORP_VERSION, type, id, ts, from, ...rest, body } as unknown as MessageByType[T];
}

export function isType<T extends CoreType>(value: Message, type: T): value is MessageByType[T] {
  return value.type === type;
}
