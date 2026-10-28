import assert from 'node:assert/strict'
import test from 'node:test'
import { deals, receiptExample, receiptLabel, youPageLead } from './vision.mjs'

const coming = ['Suggest an offer while you write', 'Link swaps', 'Cross-sells', 'Upsells', 'Reposts']

test('deal labels stay honest', () => {
  for (const deal of deals) {
    assert.notEqual(deal.status, 'Verified end to end')
    assert.equal(deal.status === 'Coming' || deal.status === 'Owner\'s rule', true)
  }
  assert.equal(deals.find((deal) => deal.name === 'Affiliate links').status, 'Owner\'s rule')
  for (const name of coming) {
    assert.equal(deals.find((deal) => deal.name === name).status, 'Coming')
  }
  assert.equal(receiptExample, 'Your agent agreed: link to X in your post Y, 8% per sale, ends 30 Nov.')
  assert.match(receiptLabel, /Not a live agreement/)
  assert.match(youPageLead, /not a signup form/i)
})
