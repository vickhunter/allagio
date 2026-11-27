import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { deals, inWriteOffer, receiptExample, receiptLabel, youPageLead } from './vision.mjs'

const working = ['Suggest an offer while you write', 'Performance-based placements', 'Link swaps', 'Cross-sells', 'Upsells', 'Reposts']

test('deal labels stay honest', () => {
  for (const deal of deals) {
    assert.notEqual(deal.status, 'Verified end to end')
    assert.equal(deal.status === 'Coming' || deal.status === 'Works today', true)
  }
  for (const name of working) {
    assert.equal(deals.find((deal) => deal.name === name).status, 'Works today')
  }
  assert.equal(receiptExample, 'Your agent agreed: link to X in your post Y, 8% per sale, ends 30 Nov.')
  assert.match(receiptLabel, /Not a live agreement/)
  assert.match(youPageLead, /not a signup form/i)
  assert.match(inWriteOffer, /this fits your post, want to monetize it/)
  assert.match(inWriteOffer, /works on the test host/)
  assert.equal(inWriteOffer.includes('—'), false)
  const you = readFileSync(new URL('./you/App.tsx', import.meta.url), 'utf8')
  assert.match(you, /Sign in to set your limits/)
  assert.match(you, /No deals yet\./)
  assert.equal(you.includes('POST /v1/agent-tokens'), false)
  assert.match(you, /publishStepParagraph/)
})
