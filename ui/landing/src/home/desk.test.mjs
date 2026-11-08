import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import {
  deskAsk,
  deskComing,
  deskContent,
  deskLabel,
  deskMcp,
  deskOffer,
  deskParagraphs,
  deskPlacement,
  deskWorks,
  howLead,
  makerBenefit,
  publisherBenefit,
} from './desk-copy.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')

test('the illustrative desk uses the preview fixture and keeps Coming beside Works today', () => {
  assert.match(deskOffer, /Field Notes/)
  assert.match(deskOffer, /A pocket notebook for the habit/)
  assert.match(deskOffer, /20% commission/)
  assert.equal(deskOffer.includes('relevance'), false)
  assert.equal(deskContent, 'How to start a journal')
  assert.match(deskPlacement, /After the intro/)
  assert.match(deskPlacement, /rel="sponsored"/)
  assert.match(deskWorks, /Works today/)
  assert.match(deskWorks, /Not verified end to end/)
  assert.match(deskComing, /^Coming\./)
  assert.match(deskAsk, /publish step as ask/)
  assert.match(deskMcp, /Works over MCP/)
  assert.match(deskMcp, /A2A support is not implemented/)
  assert.equal(deskParagraphs.includes(deskLabel), true)
  for (const line of [...deskParagraphs, howLead, makerBenefit, publisherBenefit]) {
    assert.equal(/\bads?\b|fully automatic|open[\s-]?source|best-converting/i.test(line), false, line)
  }
})

test('the homepage keeps the approved H1 and renders one labelled desk', () => {
  const app = readFileSync(resolve(root, 'src/home/App.tsx'), 'utf8')
  const desk = readFileSync(resolve(root, 'src/home/DistributionDesk.tsx'), 'utf8')
  assert.match(app, /inWriteOffer/)
  assert.match(app, /<DistributionDesk \/>/)
  assert.match(app, /id="how"/)
  assert.match(app, /id="publishers"/)
  assert.match(app, /id="join-h"/)
  assert.match(desk, /deskLabel/)
  assert.match(desk, /deskContent/)
  assert.match(desk, /deskPlacement/)
  assert.match(desk, /fixtureName/)
  assert.match(desk, /aria-hidden="true"/)
  assert.equal(desk.includes('<button'), false)
  assert.equal(/<figure[^>]*aria-hidden/.test(desk), false)
  assert.equal(app.includes('Someone has the product'), false)
})

test('the no-JS shell and home.md keep the example, the return target, and the approved phrases', () => {
  const stamp = readFileSync(resolve(root, 'scripts/stamp-hosted.mjs'), 'utf8')
  const md = readFileSync(resolve(root, '../../affilio-wp/admin/build/landing-v9/home.md'), 'utf8')
  assert.match(stamp, /staticDeskParagraphs/)
  assert.match(stamp, /return_to=%2Fowner%3Fsetup%3Dmaker/)
  assert.match(stamp, /data-primary-action/)
  assert.match(stamp, /id="how"/)
  assert.match(stamp, /id="publishers"/)
  for (const line of [...deskParagraphs, howLead, makerBenefit, publisherBenefit]) {
    assert.equal(md.includes(line), true, `home.md dropped ${line}`)
  }
})
