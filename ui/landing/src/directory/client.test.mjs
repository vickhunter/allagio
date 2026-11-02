import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { DirectoryFailure, fixtureHeaders, getMember, httpsUrl, listDirectory, logoSrc, makersUrl, memberRoute, memberUrl, parseItem, parseList, publishersUrl, wantsFixtures } from './client.mjs'
import { fixtureList, fixtureMember, testMakers, testPublishers } from './fixtures.mjs'
import { agentPrompt, installCommand, makerPrompt } from './prompt.mjs'

const maker = {
  id: '11111111-1111-4111-8111-111111111111',
  slug: 'vibesales',
  kind: 'maker',
  name: 'Vibe Sales',
  url: 'https://vibesales.example',
  logo_url: null,
  summary: 'A product an agent can offer.',
  languages: ['en', 'it'],
  categories: ['ai-coding'],
  open_offers: 2,
  joined_at: '2026-10-18T00:00:00Z',
  verified: true,
}

const publisher = {
  id: '22222222-2222-4222-8222-222222222222',
  slug: 'field-journal',
  kind: 'publisher',
  name: 'Field Journal',
  url: 'https://journal.example',
  logo_url: 'https://journal.example/logo.png',
  summary: 'A journal.',
  languages: ['en'],
  topics: ['ai', 'dev-tools'],
  monthly_pages: 108,
  joined_at: '2026-10-18T00:00:00Z',
  verified: true,
}

function jsonResponse(status, body) {
  return { ok: status >= 200 && status < 300, status, json: async () => body }
}

test('list URLs match the directory contract', () => {
  assert.equal(makersUrl({}), '/v1/directory/makers?q=&category=&lang=&cursor=&limit=24')
  assert.equal(makersUrl({ q: 'notes', category: 'ai-coding', lang: 'en', cursor: '20', limit: 24 }), '/v1/directory/makers?q=notes&category=ai-coding&lang=en&cursor=20&limit=24')
  assert.equal(publishersUrl({}), '/v1/directory/publishers?q=&topic=&lang=&cursor=&limit=24')
  assert.equal(publishersUrl({ q: 'journal', topic: 'ai', lang: 'it', cursor: '10' }), '/v1/directory/publishers?q=journal&topic=ai&lang=it&cursor=10&limit=24')
  assert.equal(memberUrl('maker', 'vibesales'), '/v1/directory/maker/vibesales')
  assert.equal(memberUrl('publisher', 'field journal'), '/v1/directory/publisher/field%20journal')
  assert.equal(memberRoute('/makers/vibesales').slug, 'vibesales')
  assert.equal(memberRoute('/publishers/field-journal/').kind, 'publisher')
  assert.equal(memberRoute('/publishers/setup'), null)
  assert.equal(memberRoute('/makers/%'), null)
  assert.equal(memberRoute('/publishers/%E0%A4%A'), null)
  assert.equal(httpsUrl('https://journal.example'), 'https://journal.example/')
  assert.equal(httpsUrl('http://journal.example'), null)
  assert.equal(httpsUrl('javascript:alert(1)'), null)
  assert.equal(httpsUrl('data:text/html,hi'), null)
  assert.equal(httpsUrl('https://good.example\n@evil.example'), null)
  assert.equal(httpsUrl('https://user:pass@journal.example/a'), null)
  assert.equal(logoSrc('https://journal.example/logo.png'), 'https://journal.example/logo.png')
  assert.equal(logoSrc('/landing-v9/favicon.svg'), '/landing-v9/favicon.svg')
  assert.equal(logoSrc('//evil.example/a.png'), null)
  assert.equal(logoSrc('javascript:alert(1)'), null)
  assert.equal(logoSrc('/\t/evil.example/a.png'), null)
  assert.equal(logoSrc('/\n/evil.example/a.png'), null)
  assert.equal(wantsFixtures(), false)
  assert.deepEqual(fixtureHeaders(), {})
})

test('empty, results, and 404 follow the contract', async () => {
  const empty = await listDirectory('maker', {}, async (url) => {
    assert.equal(url, makersUrl({}))
    return jsonResponse(200, { items: [], next_cursor: null, total: 0 })
  })
  assert.deepEqual(empty, { items: [], next_cursor: null, total: 0 })

  const page = await listDirectory('publisher', { q: 'journal', limit: 24 }, async () => jsonResponse(200, { items: [publisher], next_cursor: '24', total: 42 }))
  assert.equal(page.items.length, 1)
  assert.equal(page.items[0].monthly_pages, 108)
  assert.equal(page.next_cursor, '24')
  assert.equal(page.total, 42)

  const detail = await getMember('maker', 'vibesales', async (url) => {
    assert.equal(url, '/v1/directory/maker/vibesales')
    return jsonResponse(200, maker)
  })
  assert.equal(detail.open_offers, 2)

  await assert.rejects(getMember('maker', 'missing', async () => jsonResponse(404, { error: 'not found' })), (error) => error instanceof DirectoryFailure && error.status === 404)
  await assert.rejects(listDirectory('maker', {}, async () => { throw new Error('down') }), (error) => error instanceof DirectoryFailure && error.status === 0)
})

test('the client rejects a body the contract does not allow', () => {
  assert.throws(() => parseItem({ ...maker, summary: 'x'.repeat(201) }, 'maker'), (error) => error.status === 502)
  assert.throws(() => parseItem({ ...maker, open_offers: '2' }, 'maker'), (error) => error.status === 502)
  assert.throws(() => parseItem({ ...publisher, topics: 'ai' }, 'publisher'), (error) => error.status === 502)
  assert.throws(() => parseList({ items: [maker], next_cursor: 4, total: 1 }, 'maker'))
  assert.equal(parseList({ items: [], next_cursor: null, total: 0 }, 'publisher').items.length, 0)
})

test('fixtures are TEST data and are not imported by the page', () => {
  for (const item of [...testMakers, ...testPublishers]) assert.match(item.name, /^TEST /)
  assert.equal(fixtureList('maker', { limit: 1 }).items[0].name, 'TEST Vibe Sales')
  assert.equal(fixtureList('maker', { limit: 1 }).next_cursor, '1')
  assert.equal(fixtureList('maker', { q: 'nope' }).total, 0)
  assert.equal(fixtureMember('publisher', 'missing'), null)
  assert.equal(fixtureMember('maker', 'test-vibesales').name, 'TEST Vibe Sales')
  const page = readFileSync(new URL('./Directory.tsx', import.meta.url), 'utf8')
  const client = readFileSync(new URL('./client.mjs', import.meta.url), 'utf8')
  const makers = readFileSync(new URL('../makers/App.tsx', import.meta.url), 'utf8')
  const publishers = readFileSync(new URL('../publishers/App.tsx', import.meta.url), 'utf8')
  for (const source of [page, client, makers, publishers]) {
    assert.equal(source.includes('fixtures.mjs'), false)
    assert.equal(source.includes('TEST Vibe'), false)
  }
  assert.match(agentPrompt, /Add my site or newsletter to the Allagio directory/)
  assert.match(makerPrompt, /Add my product to the Allagio directory/)
  assert.equal(agentPrompt.includes(installCommand), true)
  assert.equal(makerPrompt.includes(installCommand), true)
  assert.match(installCommand, /claude mcp add/)
  assert.doesNotMatch(agentPrompt, /connector|marketplace|github\.com/i)
})
