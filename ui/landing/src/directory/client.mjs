// Public directory client. Query strings and fields match D-059 draft v0.
// Fixture members are not in this file. A preview page may send X-Directory-Fixtures;
// the live API ignores that header, and this file has no sample members.

export class DirectoryFailure extends Error {
  constructor(status, message) {
    super(message)
    this.name = 'DirectoryFailure'
    this.status = status
  }
}

const DEFAULT_LIMIT = 24

export function wantsFixtures() {
  if (typeof window === 'undefined') return false
  return new URLSearchParams(window.location.search).get('directoryFixtures') === '1'
}

export function fixtureHeaders() {
  return wantsFixtures() ? { 'X-Directory-Fixtures': '1' } : {}
}

function limitOf(query) {
  const limit = query && query.limit
  return typeof limit === 'number' && limit > 0 ? limit : DEFAULT_LIMIT
}

export function makersUrl(query = {}) {
  const params = new URLSearchParams()
  params.set('q', query.q || '')
  params.set('category', query.category || '')
  params.set('lang', query.lang || '')
  params.set('cursor', query.cursor || '')
  params.set('limit', String(limitOf(query)))
  return '/v1/directory/makers?' + params.toString()
}

export function publishersUrl(query = {}) {
  const params = new URLSearchParams()
  params.set('q', query.q || '')
  params.set('topic', query.topic || '')
  params.set('lang', query.lang || '')
  params.set('cursor', query.cursor || '')
  params.set('limit', String(limitOf(query)))
  return '/v1/directory/publishers?' + params.toString()
}

export function memberUrl(kind, slug) {
  if (kind !== 'maker' && kind !== 'publisher') throw new DirectoryFailure(400, 'kind')
  if (!slug) throw new DirectoryFailure(400, 'slug')
  return '/v1/directory/' + kind + '/' + encodeURIComponent(slug)
}

export function memberRoute(pathname) {
  const match = String(pathname || '').match(/^\/(makers|publishers)\/([^/]+)\/?$/)
  if (!match) return null
  if (match[1] === 'publishers' && match[2] === 'setup') return null
  return { kind: match[1] === 'makers' ? 'maker' : 'publisher', slug: decodeURIComponent(match[2]) }
}

function strings(value) {
  return Array.isArray(value) && value.every((item) => typeof item === 'string')
}

function requireString(item, key) {
  if (typeof item[key] !== 'string' || item[key].length === 0) throw new DirectoryFailure(502, key)
}

export function parseItem(value, kind) {
  if (!value || typeof value !== 'object') throw new DirectoryFailure(502, 'item')
  const item = value
  for (const key of ['id', 'slug', 'name', 'url', 'summary', 'joined_at']) requireString(item, key)
  if (item.kind !== kind) throw new DirectoryFailure(502, 'kind')
  if (item.summary.length > 200) throw new DirectoryFailure(502, 'summary')
  if (item.logo_url !== null && typeof item.logo_url !== 'string') throw new DirectoryFailure(502, 'logo_url')
  if (!strings(item.languages)) throw new DirectoryFailure(502, 'languages')
  if (typeof item.verified !== 'boolean') throw new DirectoryFailure(502, 'verified')
  if (Number.isNaN(Date.parse(item.joined_at))) throw new DirectoryFailure(502, 'joined_at')
  if (kind === 'maker') {
    if (!strings(item.categories)) throw new DirectoryFailure(502, 'categories')
    if (typeof item.open_offers !== 'number') throw new DirectoryFailure(502, 'open_offers')
  } else {
    if (!strings(item.topics)) throw new DirectoryFailure(502, 'topics')
    if (typeof item.monthly_pages !== 'number') throw new DirectoryFailure(502, 'monthly_pages')
  }
  return item
}

export function parseList(value, kind) {
  if (!value || typeof value !== 'object' || !Array.isArray(value.items)) throw new DirectoryFailure(502, 'items')
  if (value.next_cursor !== null && typeof value.next_cursor !== 'string') throw new DirectoryFailure(502, 'next_cursor')
  if (typeof value.total !== 'number') throw new DirectoryFailure(502, 'total')
  return { items: value.items.map((item) => parseItem(item, kind)), next_cursor: value.next_cursor, total: value.total }
}

async function read(res, kind, parse) {
  if (res.status === 404) throw new DirectoryFailure(404, 'not found')
  if (!res.ok) throw new DirectoryFailure(res.status, 'directory')
  let body
  try {
    body = await res.json()
  } catch {
    throw new DirectoryFailure(502, 'json')
  }
  return parse(body, kind)
}

export async function listDirectory(kind, query = {}, fetchImpl = fetch) {
  const url = kind === 'maker' ? makersUrl(query) : publishersUrl(query)
  let res
  try {
    res = await fetchImpl(url, { headers: fixtureHeaders() })
  } catch {
    throw new DirectoryFailure(0, 'network')
  }
  return read(res, kind, parseList)
}

export async function getMember(kind, slug, fetchImpl = fetch) {
  let res
  try {
    res = await fetchImpl(memberUrl(kind, slug), { headers: fixtureHeaders() })
  } catch {
    throw new DirectoryFailure(0, 'network')
  }
  return read(res, kind, parseItem)
}
