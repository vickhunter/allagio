import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { euro, notebookSplit, splitSale } from './fee.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')

test('the notebook split is 24, publisher 2.40, fee 0.24, maker keeps 21.36', () => {
  assert.deepEqual(notebookSplit, splitSale(2400, 1000, 1000))
  assert.equal(notebookSplit.publisherCents, 240)
  assert.equal(notebookSplit.feeCents, 24)
  assert.equal(notebookSplit.makerCents, 2136)
  assert.equal(notebookSplit.saleCents - notebookSplit.publisherCents - notebookSplit.feeCents, notebookSplit.makerCents)
  assert.equal(euro(notebookSplit.saleCents), '€24.00')
  assert.equal(euro(notebookSplit.publisherCents), '€2.40')
  assert.equal(euro(notebookSplit.feeCents), '€0.24')
  assert.equal(euro(notebookSplit.makerCents), '€21.36')
})

test('home, makers, and the how-it-works figure do not type the maker figure', () => {
  const files = [
    'src/content.ts',
    'src/home/App.tsx',
    'src/home/HowItWorks.tsx',
    'src/makers/App.tsx',
    'public/how-it-works/index.html',
  ]
  const banned = ['21.60', '21.36', '€2.40', '€0.24', '€24.00']
  for (const file of files) {
    const text = readFileSync(resolve(root, file), 'utf8')
    for (const needle of banned) {
      assert.equal(text.includes(needle), false, `${file} types ${needle}`)
    }
  }
  const content = readFileSync(resolve(root, 'src/content.ts'), 'utf8')
  const makers = readFileSync(resolve(root, 'src/makers/App.tsx'), 'utf8')
  const home = readFileSync(resolve(root, 'src/home/App.tsx'), 'utf8')
  const figure = readFileSync(resolve(root, 'public/how-it-works/index.html'), 'utf8')
  assert.match(content, /from '@\/lib\/fee\.mjs'/)
  assert.match(makers, /example\.maker/)
  assert.match(makers, /example\.fee/)
  assert.match(home, /example\.maker/)
  assert.match(figure, /\/landing-v9\/fee\.mjs/)
  assert.match(figure, /notebookSplit/)
  assert.equal(figure.includes('price * (1 - OFFER.rate)'), false)
})
