import assert from 'node:assert/strict'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { buildCatalog } from '../../scripts/build-orp-docs.mjs'
import { catalog } from './generated/catalog.mjs'
import { noMoney, notSpoken04, otherRefusal, payRefusal } from './hosted.mjs'
import { routeFor } from './route.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const schemaRoot = join(here, '../../../../docs/protocol/schema')

function schemaFiles(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) schemaFiles(path, out)
    else if (name.endsWith('.schema.json')) out.push(relative(schemaRoot, path))
  }
  return out
}

test('every 0.3 and 0.4 schema file has one generated page', () => {
  const built = buildCatalog()
  const files = schemaFiles(schemaRoot).filter((file) => file.startsWith('0.3/') || file.startsWith('0.4/'))
  const byFile = new Map(built.schemas.map((schema) => [schema.file, schema]))
  const missing = files.filter((file) => !byFile.has(file))
  assert.deepEqual(missing, [])
  assert.equal(built.schemas.length, files.length)
  const keys = built.schemas.map((schema) => `${schema.version}/${schema.slug}`)
  assert.equal(new Set(keys).size, keys.length)
  for (const schema of built.schemas) {
    const route = routeFor(schema.href)
    assert.equal(route.name, 'spec')
    assert.equal(route.slug, schema.slug)
    assert.equal(route.version, schema.version)
    assert.ok(schema.sections.length > 0, schema.file)
  }
  const hello = built.schemas.find((schema) => schema.version === '0.3' && schema.slug === 'hello')
  assert.equal(routeFor('/protocol/spec/hello/').version, '0.3')
  assert.equal(routeFor('/protocol/spec/hello/').slug, 'hello')
  assert.equal(hello.href, '/protocol/spec/0.3/hello/')
  const deal = built.schemas.find((schema) => schema.version === '0.4' && schema.slug === 'deal')
  assert.ok(deal.valid.some((example) => example.name.endsWith('five-party-home-sale.json')))
  assert.ok(deal.valid.some((example) => example.name.endsWith('referral-chain.json')))
  assert.ok(deal.valid.some((example) => example.name.endsWith('mediator-left.json')))
  const offer = built.schemas.find((schema) => schema.version === '0.4' && schema.slug === 'offer')
  assert.ok(offer.valid.some((example) => example.name.endsWith('bad-split.json')))
  assert.ok(offer.invalid.some((example) => example.name.endsWith('identity-email.json')))
})

test('the five-party and leave fixtures match the files', () => {
  const deal = catalog.schemas.find((schema) => schema.version === '0.4' && schema.slug === 'deal')
  const home = JSON.parse(deal.valid.find((example) => example.name.endsWith('five-party-home-sale.json')).json)
  const service = home.body.parties.find((seat) => seat.id.endsWith('p_door01'))
  const signer = home.body.parties.find((seat) => seat.role === 'signer')
  const facilitator = home.body.parties.find((seat) => seat.role === 'facilitator')
  assert.equal(service.role, 'service')
  assert.equal(service.required, false)
  assert.equal(service.signed, false)
  assert.equal(service.terms.amount.amount_minor, 4000)
  assert.equal(service.terms.amount.currency, 'EUR')
  assert.equal(service.terms.per, 'hour')
  assert.equal(service.terms.hours, 2)
  assert.equal(service.terms.deposit.amount.amount_minor, 2000)
  assert.equal(service.terms.deposit.holder, 'viewing-desk')
  assert.equal(service.terms.deposit.purpose, 'viewing')
  assert.deepEqual(Object.keys(facilitator.identity).sort(), ['hash', 'issuer'])
  assert.equal(signer.required, true)
  assert.equal(signer.signed, true)
  assert.equal(signer.terms, undefined)
  assert.equal(signer.identity, undefined)
  assert.equal(JSON.stringify(home).includes('payer'), false)
  assert.equal(JSON.stringify(home).includes('human'), false)
  const left = JSON.parse(deal.valid.find((example) => example.name.endsWith('mediator-left.json')).json)
  assert.equal(left.body.state, 'pending')
  assert.deepEqual(left.body.vacancies, ['facilitator'])
})

test('the committed catalog matches the spec files', () => {
  assert.deepEqual(catalog, buildCatalog())
})

test('quickstart names the hosted thin layer', () => {
  assert.equal(otherRefusal, 'This host is in testing and does not run this message.')
  assert.equal(payRefusal, 'This testing host does not settle money.')
  assert.equal(noMoney, 'Offer Rails does not hold money.')
  assert.equal(notSpoken04, 'This host does not speak ORP 0.4 leave or seat.')
  const page = readFileSync(join(here, 'Docs.tsx'), 'utf8')
  assert.match(page, /otherRefusal/)
  assert.match(page, /FivePartyWalkthrough/)
  assert.match(page, /ThreePartyExample/)
  assert.equal(page.includes('hostedRefusal'), false)
  assert.equal(page.includes('refusedTypes'), false)
  assert.equal(routeFor('/protocol/many/').name, 'many')
})

test('protocol docs do not say open source, affilio, distributoor, or offerrails.com', () => {
  const banned = /open[\s-]?source|offerrails\.com|\baffilio\b|\bdistributoor\b/i
  assert.equal(catalog.license, 'Apache-2.0 planned')
  assert.equal(banned.test(JSON.stringify(catalog)), false)
  for (const name of ['Docs.tsx', 'App.tsx', 'hosted.mjs', 'route.mjs', 'many.tsx']) {
    assert.equal(banned.test(readFileSync(join(here, name), 'utf8')), false, name)
  }
  assert.equal(banned.test(readFileSync(join(here, '../../scripts/build-orp-docs.mjs'), 'utf8')), false)
})

test('conformance names come from the suite', () => {
  for (const name of ['L0-SCHEMA', 'L2-CONSENT-HOST-ONLY', 'L3-PAYOUT', 'INVALID-68', 'ORP04-HOME-SALE', 'ORP04-MEDIATOR-LEAVES', 'ORP04-BAD-SPLIT']) {
    assert.ok(catalog.conformance.includes(name), name)
  }
})
