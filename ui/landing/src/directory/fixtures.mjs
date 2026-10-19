// TEST members for the directory client tests and the proof server.
// The live page does not import this module.

const joined = '2026-10-18T00:00:00Z'

export const testMakers = [
  {
    id: '00000000-0000-4000-8000-000000000001',
    slug: 'test-vibesales',
    kind: 'maker',
    name: 'TEST Vibe Sales',
    url: 'https://vibesales.example',
    logo_url: null,
    summary: 'TEST maker. A short note about a product an agent can offer.',
    languages: ['en', 'it'],
    categories: ['ai-coding'],
    open_offers: 2,
    joined_at: joined,
    verified: true,
  },
  {
    id: '00000000-0000-4000-8000-000000000002',
    slug: 'test-field-kit',
    kind: 'maker',
    name: 'TEST Field Kit',
    url: 'https://fieldkit.example',
    logo_url: '/landing-v9/favicon.svg',
    summary: 'TEST maker with a logo. Notes for people who write by hand.',
    languages: ['en'],
    categories: ['notes'],
    open_offers: 1,
    joined_at: joined,
    verified: false,
  },
]

export const testPublishers = [
  {
    id: '00000000-0000-4000-8000-000000000003',
    slug: 'test-journal',
    kind: 'publisher',
    name: 'TEST Field Journal',
    url: 'https://journal.example',
    logo_url: null,
    summary: 'TEST publisher. A journal with a few public posts.',
    languages: ['en'],
    topics: ['ai', 'dev-tools'],
    monthly_pages: 108,
    joined_at: joined,
    verified: true,
  },
  {
    id: '00000000-0000-4000-8000-000000000004',
    slug: 'test-notes',
    kind: 'publisher',
    name: 'TEST Notes Weekly',
    url: 'https://notes.example',
    logo_url: '/landing-v9/favicon.svg',
    summary: 'TEST publisher with a logo. One post a week.',
    languages: ['it'],
    topics: ['notes'],
    monthly_pages: 12,
    joined_at: joined,
    verified: false,
  },
]

function matches(item, query) {
  const q = String(query.q || '').trim().toLowerCase()
  const lang = String(query.lang || '').trim().toLowerCase()
  const category = String(query.category || '').trim().toLowerCase()
  const topic = String(query.topic || '').trim().toLowerCase()
  if (q && !`${item.name} ${item.summary} ${item.slug}`.toLowerCase().includes(q)) return false
  if (lang && !item.languages.map((value) => value.toLowerCase()).includes(lang)) return false
  if (category && !(item.categories || []).map((value) => value.toLowerCase()).includes(category)) return false
  if (topic && !(item.topics || []).map((value) => value.toLowerCase()).includes(topic)) return false
  return true
}

export function fixtureList(kind, query = {}) {
  const all = (kind === 'maker' ? testMakers : testPublishers).filter((item) => matches(item, query))
  const limit = Number(query.limit || 24)
  const start = query.cursor ? Number(query.cursor) || 0 : 0
  const items = all.slice(start, start + limit)
  const next = start + limit < all.length ? String(start + limit) : null
  return { items, next_cursor: next, total: all.length }
}

export function fixtureMember(kind, slug) {
  return (kind === 'maker' ? testMakers : testPublishers).find((entry) => entry.slug === slug) || null
}
