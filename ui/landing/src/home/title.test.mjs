import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { heroSupport } from './hero-copy.mjs'

const landing = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const title = 'Allagio · Agents find and close deals autonomously'

test('home title and description name Allagio the host and ORP the protocol', () => {
  const html = readFileSync(resolve(landing, 'index.html'), 'utf8')
  const md = readFileSync(resolve(landing, '../../affilio-wp/admin/build/landing-v9/home.md'), 'utf8')
  assert.equal(html.includes(`<title>${title}</title>`), true)
  assert.equal(html.includes(`content="${heroSupport}"`), true)
  assert.equal(md.startsWith(`${title}\n`), true)
  assert.equal(md.includes(heroSupport), true)
  assert.equal(html.includes('Offer Rails'), false)
  assert.equal(md.includes('Offer Rails'), false)
})
