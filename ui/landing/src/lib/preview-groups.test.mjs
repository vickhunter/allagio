import assert from 'node:assert/strict'
import test from 'node:test'
import { fixtureBody } from './preview-map.mjs'
import { adaptPreview, closestLead, groupsFixtureBody, includeUnverifiedFixture, notifyLabel, notifyLimit, orderedNames, publicLinksNote, readFailureCopy, readNext, readNotifyIntent, storeNotifyIntent, suggestedNote, unverifiedFixture } from './preview-groups.mjs'

function memory() {
  const data = new Map()
  return {
    getItem(key) { return data.has(key) ? data.get(key) : null },
    setItem(key, value) { data.set(key, String(value)) },
  }
}

test('groups a verified offer ahead of topic-matched unverified programs', () => {
  assert.ok(unverifiedFixture.length >= 3 && unverifiedFixture.length <= 5)
  const grouped = adaptPreview(groupsFixtureBody(), { includeFixture: true })
  assert.equal(grouped.kind, 'groups')
  assert.equal(grouped.readFailure, null)
  assert.deepEqual(grouped.verified.map((offer) => offer.name), ['Field Notes'])
  assert.equal(grouped.verified[0].verified, true)
  assert.deepEqual(grouped.suggested.map((offer) => offer.name), ['Baron Fig', 'Leuchtturm'])
  assert.equal(grouped.closest.length, 0)
  assert.deepEqual(orderedNames(grouped), ['Field Notes', 'Baron Fig', 'Leuchtturm'])
  for (const program of grouped.suggested) {
    assert.equal(program.verified, false)
    assert.equal(program.program_url.startsWith('https://programs.example/'), true)
    assert.equal(program.program_url.includes('via='), false)
    assert.equal(program.program_url.includes('/v1/go'), false)
    assert.equal('commission_value' in program, false)
    assert.equal(program.source_url.length > 0, true)
    assert.equal(program.checked_at.length > 0, true)
  }
  assert.equal(suggestedNote, "Not on Allagio yet. Payouts go through the company's own affiliate program.")
})

test('defaults an existing offer to verified and leaves the fixture out', () => {
  const live = adaptPreview(fixtureBody('success'))
  assert.equal(live.verified.length, 1)
  assert.equal(live.verified[0].verified, true)
  assert.equal(live.verified[0].name, 'Field Notes')
  assert.equal(live.suggested.length, 0)
  assert.equal(includeUnverifiedFixture(''), false)
  assert.equal(includeUnverifiedFixture('?previewGroups=1'), false)
  assert.equal(includeUnverifiedFixture('?previewFixture=groups'), true)
  const withFlag = adaptPreview({
    title: 'Notes',
    text: 'A pocket notebook',
    offer: { name: 'Field Notes', summary: 'A pocket notebook for the habit.' },
  }, { includeFixture: includeUnverifiedFixture('?previewGroups=1') })
  assert.equal(withFlag.suggested.length, 0)
  assert.equal(withFlag.closest.length, 0)
  const fromApi = adaptPreview({
    title: 'Notes',
    text: 'A pocket notebook',
    offers: [
      { name: 'Field Notes', summary: 'A pocket notebook for the habit.', verified: true },
      { name: 'Outside Co', summary: 'Their own program.', verified: false, program_url: 'https://programs.example/outside?via=plugin&utm_source=ad', source_url: 'https://sources.example/outside', checked_at: '2026-10-02', topics: ['notebook'] },
      { name: 'Tracked', summary: 'A go link.', verified: false, program_url: 'https://offer-rails.example/v1/go/abc' },
    ],
  })
  assert.deepEqual(orderedNames(fromApi), ['Field Notes', 'Outside Co'])
  assert.equal(fromApi.suggested[0].program_url, 'https://programs.example/outside')
  assert.equal(fromApi.suggested[0].verified, false)
})

test('names unreadable, private, and empty pages and never calls them no match', () => {
  const cases = [
    [{ title: 'Draft', error: 'fetch failed' }, 'unread', "We couldn't read that page."],
    [{ result: 'unreadable_or_private', message: 'only public links work' }, 'unread', "We couldn't read that page."],
    [{ title: 'Draft', error: 'blocked address' }, 'private', 'That page is private.'],
    [{ title: 'Draft', text: '', offer: null }, 'empty', 'That page returned no text.'],
  ]
  for (const [body, reason, message] of cases) {
    const result = adaptPreview(body, { includeFixture: true })
    assert.equal(result.kind, 'unreadable')
    assert.equal(result.readFailure, reason)
    assert.equal(result.verified.length, 0)
    assert.equal(result.suggested.length, 0)
    assert.equal(result.closest.length, 0)
    const copy = readFailureCopy(reason)
    assert.equal(copy.message, message)
    assert.equal(copy.next, readNext)
    assert.doesNotMatch(`${copy.message} ${copy.next}`, /no match/i)
  }
  assert.equal(publicLinksNote, "Public links only. Private posts and accounts can't be read.")
})

test('shows closest programs and stores a local notify intent when nothing matches', () => {
  const result = adaptPreview(fixtureBody('nomatch'), { includeFixture: true })
  assert.equal(result.kind, 'closest')
  assert.equal(result.verified.length, 0)
  assert.equal(result.suggested.length, 0)
  assert.deepEqual(result.closest.map((program) => program.name), ['Baron Fig', 'Leuchtturm', 'Skillshare'])
  assert.equal(closestLead.includes('no match'), false)
  assert.equal(notifyLabel, 'Notify me when a match joins')
  const saved = storeNotifyIntent(memory())
  assert.equal(saved.length, 1)
  assert.equal(typeof saved[0].at, 'number')
  const again = storeNotifyIntent({ getItem: () => JSON.stringify(saved), setItem() {} })
  assert.equal(again.length, 2)
})

test('drops go links and tracking params without caring about case, and strips userinfo', () => {
  const grouped = adaptPreview({
    title: 'Notes',
    offers: [
      { name: 'Outside Co', summary: 'Their own program.', verified: false, program_url: 'https://User:Secret@programs.example/outside?VIA=plugin&UTM_SOURCE=ad&Ref=home' },
      { name: 'Go', summary: 'A go link.', verified: false, program_url: 'https://user:pass@offer-rails.example/V1/GO/abc' },
      { name: 'Short', summary: 'A short go link.', verified: false, program_url: 'https://offer-rails.example/Go' },
      { name: 'Exact', summary: 'The go path itself.', verified: false, program_url: 'https://offer-rails.example/v1/go' },
      { name: 'Nested', summary: 'A go path later in the URL.', verified: false, program_url: 'https://docs.example/guides/v1/go/abc' },
      { name: 'Golden', summary: 'A path that only shares letters with a go link.', verified: false, program_url: 'https://programs.example/v1/golden' },
      { name: 'Goodies', summary: 'Another path that only shares letters.', verified: false, program_url: 'https://archive.example/v1/goodies' },
    ],
  })
  assert.deepEqual(grouped.suggested.map((item) => item.name), ['Outside Co', 'Golden', 'Goodies'])
  assert.equal(grouped.suggested[0].program_url, 'https://programs.example/outside')
  assert.equal(grouped.suggested[1].program_url, 'https://programs.example/v1/golden')
  assert.equal(grouped.suggested[2].program_url, 'https://archive.example/v1/goodies')
})

test('notify intent never persists the pasted url, and legacy stored urls are dropped', () => {
  const store = memory()
  store.setItem('offer-rails.preview.notify', JSON.stringify(['https://journal.example/legacy']))
  const saved = storeNotifyIntent(store)
  assert.equal(saved.length, 1)
  assert.equal(typeof saved[0].at, 'number')
  assert.equal(store.getItem('offer-rails.preview.notify'), JSON.stringify(saved))
  assert.equal(store.getItem('offer-rails.preview.notify').includes('journal.example'), false)
})

test('keeps the newest notify intents and drops the oldest past the cap', () => {
  const store = memory()
  for (let i = 0; i < notifyLimit + 5; i += 1) storeNotifyIntent(store)
  storeNotifyIntent(store)
  const saved = readNotifyIntent(store)
  assert.equal(saved.length, notifyLimit)
  assert.equal(saved.every((item) => typeof item.at === 'number'), true)
  assert.equal(saved.at(-1).at >= saved[0].at, true)
})
