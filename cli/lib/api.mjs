import { defaultPublicBaseURL } from './public-base.mjs';

export function apiBase(env = process.env) {
  const raw = String(env.ALLAGIO_API_URL || env.OFFER_RAILS_API_URL || env.PUBLIC_BASE_URL || defaultPublicBaseURL).trim();
  let url;
  try {
    url = new URL(raw);
  } catch {
    throw new Error('ALLAGIO_API_URL is not a URL');
  }
  const host = url.hostname.replace(/^\[|\]$/g, '');
  const local = host === 'localhost' || host === '127.0.0.1' || host === '::1';
  if (url.protocol !== 'https:' && !(url.protocol === 'http:' && local)) {
    throw new Error('ALLAGIO_API_URL must use https unless the host is localhost');
  }
  return raw.replace(/\/+$/, '');
}

export function dashboardURL(env = process.env) {
  return `${apiBase(env)}/owner`;
}

export async function apiRequest(io, path, { method = 'GET', body, headers = {} } = {}) {
  const base = apiBase(io.env);
  const res = await io.fetch(`${base}${path}`, {
    method,
    headers: {
      accept: 'application/json',
      ...(body !== undefined ? { 'content-type': 'application/json' } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let json = null;
  if (text) {
    try {
      json = JSON.parse(text);
    } catch {
      json = { raw: text };
    }
  }
  return { status: res.status, json: json || {}, text };
}
