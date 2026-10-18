import assert from 'node:assert/strict'
import test from 'node:test'
import { fixtureBody } from './preview-map.mjs'
import { adaptPreview, closestLead, groupsFixtureBody, includeUnverifiedFixture, notifyLabel, orderedNames, publicLinksNote, readFailureCopy, readNext, storeNotifyIntent, suggestedNote, unverifiedFixture } from './preview-groups.mjs'

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
  assert.equal(suggestedNote, "Not on Offer Rails yet. Payouts go through the company's own affiliate program.")
})

test('defaults an existing offer to verified and leaves the fixture out', () => {
  const live = adaptPreview(fixtureBody('success'))
  assert.equal(live.verified.length, 1)
  assert.equal(live.verified[0].verified, true)
  assert.equal(live.verified[0].name, 'Field Notes')
  assert.equal(live.suggested.length, 0)
  assert.equal(includeUnverifiedFixture(''), false)
  assert.equal(includeUnverifiedFixture('?previewGroups=1'), true)
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
  const saved = storeNotifyIntent(memory(), 'https://journal.example/start')
  assert.deepEqual(saved, ['https://journal.example/start'])
  const again = storeNotifyIntent({ getItem: () => JSON.stringify(saved), setItem() {} }, 'https://journal.example/start')
  assert.deepEqual(again, saved)
})
