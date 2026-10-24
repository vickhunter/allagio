import assert from 'node:assert/strict'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { buildCatalog } from '../../scripts/build-orp-docs.mjs'
import { catalog } from './generated/catalog.mjs'
import { hostedRefusal, refusedTypes } from './hosted.mjs'
import { routeFor } from './route.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const schemaRoot = join(here, '../../../../docs/protocol/schema/0.3')

function schemaFiles(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) schemaFiles(path, out)
    else if (name.endsWith('.schema.json')) out.push(relative(schemaRoot, path))
  }
  return out
}

test('every schema file has one generated page', () => {
  const built = buildCatalog()
  const files = schemaFiles(schemaRoot)
  const byFile = new Map(built.schemas.map((schema) => [schema.file, schema]))
  const missing = files.filter((file) => !byFile.has(file))
  assert.deepEqual(missing, [])
  assert.equal(built.schemas.length, files.length)
  const slugs = built.schemas.map((schema) => schema.slug)
  assert.equal(new Set(slugs).size, slugs.length)
  for (const schema of built.schemas) {
    const route = routeFor(schema.href)
    assert.equal(route.name, 'spec')
    assert.equal(route.slug, schema.slug)
    assert.ok(schema.sections.length > 0, schema.file)
  }
})

test('the committed catalog matches the spec files', () => {
  assert.deepEqual(catalog, buildCatalog())
})

test('quickstart names the hosted 403 for accept and offer', () => {
  assert.equal(hostedRefusal, 'This host is in testing and does not open deals.')
  assert.deepEqual(refusedTypes, ['accept', 'offer'])
  const page = readFileSync(join(here, 'Docs.tsx'), 'utf8')
  assert.match(page, /hostedRefusal/)
  assert.match(page, /refusedTypes/)
})

test('protocol docs do not say open source', () => {
  const banned = /open[\s-]?source/i
  assert.equal(catalog.license, 'Apache-2.0 planned')
  assert.equal(banned.test(JSON.stringify(catalog)), false)
  for (const name of ['Docs.tsx', 'App.tsx', 'hosted.mjs', 'route.mjs']) {
    assert.equal(banned.test(readFileSync(join(here, name), 'utf8')), false, name)
  }
  assert.equal(banned.test(readFileSync(join(here, '../../scripts/build-orp-docs.mjs'), 'utf8')), false)
})

test('conformance names come from the suite', () => {
  for (const name of ['L0-SCHEMA', 'L2-CONSENT-HOST-ONLY', 'L3-PAYOUT', 'INVALID-68']) {
    assert.ok(catalog.conformance.includes(name), name)
  }
})
