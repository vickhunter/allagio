// RFC 8785 JSON Canonicalization Scheme (JCS). ORP messages carry only strings,
// integers, booleans, null, arrays and objects, so ES2015 JSON serialization of
// scalars already matches JCS; object keys are sorted by UTF-16 code units,
// which is what Array.prototype.sort does by default.
export function canonicalize(value: unknown): string {
  if (value === null) return 'null';
  switch (typeof value) {
    case 'boolean':
      return value ? 'true' : 'false';
    case 'number':
      if (!Number.isFinite(value)) throw new TypeError('JCS cannot encode a non-finite number');
      return JSON.stringify(value);
    case 'string':
      return JSON.stringify(value);
    case 'object': {
      if (Array.isArray(value)) {
        return '[' + value.map((item) => canonicalize(item === undefined ? null : item)).join(',') + ']';
      }
      const record = value as Record<string, unknown>;
      const keys = Object.keys(record)
        .filter((key) => record[key] !== undefined)
        .sort();
      return '{' + keys.map((key) => JSON.stringify(key) + ':' + canonicalize(record[key])).join(',') + '}';
    }
    default:
      throw new TypeError(`JCS cannot encode a ${typeof value}`);
  }
}
