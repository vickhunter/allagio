import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const landing = resolve(dirname(fileURLToPath(import.meta.url)), '../..')

test('the brand constant is Allagio', () => {
  const source = readFileSync(resolve(landing, 'src/content.ts'), 'utf8')
  assert.match(source, /export const brand = 'Allagio'/)
})

test('the footer sign-off sentence is pinned', () => {
  const source = readFileSync(resolve(landing, 'src/content.ts'), 'utf8')
  const en = readFileSync(resolve(landing, 'src/i18n/en.ts'), 'utf8')
  assert.equal(source.includes('workingName'), false)
  assert.equal(en.includes('workingName'), false)
  assert.match(en, /Your agent finds partners, agrees the terms and places the offer\./)
})

test('the agents page title is Allagio', () => {
  const html = readFileSync(resolve(landing, 'agents/index.html'), 'utf8')
  assert.equal(html.includes('<title>Allagio · MCP server</title>'), true)
  assert.equal(html.includes('Offer Rails'), false)
})
