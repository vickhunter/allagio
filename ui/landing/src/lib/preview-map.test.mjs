import assert from 'node:assert/strict'
import test from 'node:test'
import { barCount, codeFromStatus, fixtureBody, parsePreview, previewErrorCopy, previewErrors, readPreviewFixture, samplePreview, slotAfterBar, slotLabel, slots } from './preview-map.mjs'

test('maps every preview slot to a plain label and a bar', () => {
  const want = {
    after_intro: ['After the intro', 1],
    mid_article: ['Mid-article', 2],
    before_conclusion: ['Before the conclusion', 3],
    end_of_article: ['End of article', 4],
  }
  assert.deepEqual(slots, Object.keys(want))
  for (const slot of slots) {
    assert.equal(slotLabel(slot), want[slot][0])
    assert.equal(slotAfterBar(slot), want[slot][1])
    assert.ok(slotAfterBar(slot) <= barCount)
    const body = fixtureBody(slot)
    const parsed = parsePreview(body)
    assert.equal(parsed.slot, slot)
    assert.equal(parsed.offer.name, 'Field Notes')
    assert.equal(parsed.offer.blurb, samplePreview.offer.summary)
  }
})

test('maps every preview error status to its own line and next step', () => {
  const seen = new Set()
  for (const code of [400, 403, 422, 429]) {
    const copy = previewErrorCopy(code)
    assert.equal(copy.message, previewErrors[code].message)
    assert.equal(copy.next, previewErrors[code].next)
    assert.equal(copy.message.includes('{'), false)
    assert.equal(copy.next.includes('{'), false)
    seen.add(copy.message)
  }
  assert.equal(seen.size, 4)
  assert.equal(previewErrorCopy(400).message, "That doesn't look like a post URL.")
  assert.match(previewErrorCopy(400).next, /https:\/\/example\.com\/your-post/)
  assert.equal(previewErrorCopy(403).next, 'Try a public post.')
  assert.match(previewErrorCopy(422).next, /public/)
  assert.match(previewErrorCopy(429).next, /minute/)
  assert.deepEqual(previewErrorCopy(500), previewErrors[422])
  assert.deepEqual(previewErrorCopy(0), previewErrors[422])
  assert.equal(codeFromStatus(400, false), 400)
  assert.equal(codeFromStatus(403, false), 403)
  assert.equal(codeFromStatus(422, false), 422)
  assert.equal(codeFromStatus(429, false), 429)
  assert.equal(codeFromStatus(404, false), 422)
  assert.equal(codeFromStatus(500, false), 422)
  assert.equal(codeFromStatus(400, true), 422)
})

test('reads a no-match body and ignores a card with no summary', () => {
  const miss = parsePreview({ title: 'Tomato notes', offer: null, slot: null })
  assert.equal(miss.offer, null)
  assert.equal(miss.slot, null)
  assert.equal(parsePreview({ title: 'Tomato notes', offer: { name: 'Field Notes' }, slot: 'end_of_article' }), null)
  assert.equal(parsePreview(fixtureBody('nomatch')).offer, null)
  assert.equal(readPreviewFixture('?previewFixture=429'), '429')
  assert.equal(readPreviewFixture('?previewFixture=mid_article'), 'mid_article')
  assert.equal(readPreviewFixture('?previewFixture=after_p2'), '')
})
