import assert from 'node:assert/strict'
import test from 'node:test'
import { commissionText, dealsFixtureData, listPublicOffers, listRecentDeals, partyText } from './network-wall.mjs'

const roles = { maker: 'a maker', publisher: 'a publisher', agent: 'an agent' }

test('partyText names a listed party with its url', () => {
  const face = { listed: true, name: 'Vibe Sales', url: 'https://vibesales.example' }
  assert.deepEqual(partyText(face, roles), { text: 'Vibe Sales', url: 'https://vibesales.example', named: true })
})

test('partyText anonymizes to a role word', () => {
  assert.deepEqual(partyText({ listed: false, role: 'publisher' }, roles), { text: 'a publisher', url: '', named: false })
  assert.deepEqual(partyText({ listed: false, role: 'maker' }, roles), { text: 'a maker', url: '', named: false })
  assert.deepEqual(partyText({ listed: false }, roles), { text: 'an agent', url: '', named: false })
})

test('partyText appends the host for foreign parties', () => {
  const face = { listed: false, role: 'agent', host: 'agents.example' }
  assert.equal(partyText(face, roles).text, 'an agent · agents.example')
})

test('commissionText renders percents, recurring, flat amounts, and hides zero', () => {
  assert.equal(commissionText({ commission_type: 'percentage', commission_value: 30 }), '30%')
  assert.equal(commissionText({ commission_type: 'recurring', commission_value: 15 }), '15% recurring')
  // flat commission_value is in major currency units — 500 means €500, not €5
  assert.match(commissionText({ commission_type: 'flat', commission_value: 500, commission_currency: 'EUR' }), /500/)
  assert.equal(commissionText({ commission_value: 0 }), '')
  assert.equal(commissionText({}), '')
})

test('listRecentDeals and listPublicOffers read their endpoints', async () => {
  const seen = []
  const stub = (url) => {
    seen.push(url)
    const body = url.includes('deals') ? { deals: [{ kind: 'swap' }] } : { offers: [{ product_name: 'X' }] }
    return Promise.resolve({ ok: true, json: () => Promise.resolve(body) })
  }
  assert.equal((await listRecentDeals(stub)).length, 1)
  assert.equal((await listPublicOffers(stub)).length, 1)
  assert.deepEqual(seen, ['/v1/deals/recent', '/v1/offers'])
})

test('endpoints reject on http errors', async () => {
  const stub = () => Promise.resolve({ ok: false, status: 500 })
  await assert.rejects(listRecentDeals(stub))
  await assert.rejects(listPublicOffers(stub))
})

test('fixture data shapes stay stable', () => {
  const f = dealsFixtureData()
  assert.ok(f.deals.every((d) => d.kind && d.state && d.offerer && d.acceptor))
  assert.ok(f.offers.every((o) => o.product_name && o.website_url))
})
