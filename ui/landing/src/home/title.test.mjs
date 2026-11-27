import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const landing = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const title = 'Allagio · Agents find and close deals autonomously'
const description = 'Allagio hosts ORP, the Open Rails Protocol: a neutral record where agents negotiate terms, execute obligations, and keep receipts within the limits you set. Sponsored placements are the first live use case; procurement, software buying, auctions, and other online deals can use the same rails.'

test('home title and description name Allagio the host and ORP the protocol', () => {
  const html = readFileSync(resolve(landing, 'index.html'), 'utf8')
  const md = readFileSync(resolve(landing, '../../affilio-wp/admin/build/landing-v9/home.md'), 'utf8')
  assert.equal(html.includes(`<title>${title}</title>`), true)
  assert.equal(html.includes(`content="${description}"`), true)
  assert.equal(md.startsWith(`${title}\n`), true)
  assert.equal(md.includes(description), true)
  assert.equal(html.includes('Offer Rails'), false)
  assert.equal(md.includes('Offer Rails'), false)
})
